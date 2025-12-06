package dsn

import (
	"math"
	"sync"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

// BodyKind categorizes celestial bodies for rendering.
type BodyKind int

const (
	BodySun BodyKind = iota
	BodyPlanet
	BodySpacecraft
)

// String returns the body kind name.
func (k BodyKind) String() string {
	switch k {
	case BodySun:
		return "sun"
	case BodyPlanet:
		return "planet"
	case BodySpacecraft:
		return "spacecraft"
	default:
		return "unknown"
	}
}

// PlanetClass categorizes planets for rendering glyphs.
type PlanetClass int

const (
	ClassInner PlanetClass = iota // Mercury, Venus, Earth, Mars
	ClassGiant                    // Jupiter, Saturn, Uranus, Neptune
)

// EclipticBody represents a body in heliocentric ecliptic coordinates.
type EclipticBody struct {
	Name  string            // Display name (e.g., "Earth", "VGR1")
	Code  string            // Short code (e.g., "EARTH", "VGR1")
	Kind  BodyKind          // Sun, Planet, or Spacecraft
	Class PlanetClass       // For planets: inner or giant
	Pos   astro.Vec3        // Position in AU (heliocentric ecliptic)
	Meta  map[string]string // Additional metadata
}

// DistanceAU returns the heliocentric distance in AU.
func (b EclipticBody) DistanceAU() float64 {
	return b.Pos.Norm()
}

// EclipticLatDeg returns the ecliptic latitude in degrees.
func (b EclipticBody) EclipticLatDeg() float64 {
	return astro.EclipticLatitude(b.Pos)
}

// EclipticLonDeg returns the ecliptic longitude in degrees.
func (b EclipticBody) EclipticLonDeg() float64 {
	return astro.EclipticLongitude(b.Pos)
}

// LightTimeSec returns the one-way light time in seconds.
func (b EclipticBody) LightTimeSec() float64 {
	return astro.LightTimeFromAU(b.DistanceAU())
}

// SolarSystemSnapshot represents the current state of the solar system.
type SolarSystemSnapshot struct {
	GeneratedAt time.Time
	Bodies      []EclipticBody
}

// GetBody returns a body by code, or nil if not found.
func (s SolarSystemSnapshot) GetBody(code string) *EclipticBody {
	for i := range s.Bodies {
		if s.Bodies[i].Code == code {
			return &s.Bodies[i]
		}
	}
	return nil
}

// GetPlanets returns all planet bodies.
func (s SolarSystemSnapshot) GetPlanets() []EclipticBody {
	var planets []EclipticBody
	for _, b := range s.Bodies {
		if b.Kind == BodyPlanet {
			planets = append(planets, b)
		}
	}
	return planets
}

// GetSpacecraft returns all spacecraft bodies.
func (s SolarSystemSnapshot) GetSpacecraft() []EclipticBody {
	var sc []EclipticBody
	for _, b := range s.Bodies {
		if b.Kind == BodySpacecraft {
			sc = append(sc, b)
		}
	}
	return sc
}

// Planet definitions with NAIF IDs for Horizons queries.
type PlanetDef struct {
	Name        string
	Code        string
	NAIFID      int
	Class       PlanetClass
	SemiMajorAU float64 // Approximate for static fallback
}

// Planets is the list of major planets.
var Planets = []PlanetDef{
	{Name: "Mercury", Code: "MERC", NAIFID: 199, Class: ClassInner, SemiMajorAU: 0.387},
	{Name: "Venus", Code: "VEN", NAIFID: 299, Class: ClassInner, SemiMajorAU: 0.723},
	{Name: "Earth", Code: "EARTH", NAIFID: 399, Class: ClassInner, SemiMajorAU: 1.000},
	{Name: "Mars", Code: "MARS", NAIFID: 499, Class: ClassInner, SemiMajorAU: 1.524},
	{Name: "Jupiter", Code: "JUP", NAIFID: 599, Class: ClassGiant, SemiMajorAU: 5.203},
	{Name: "Saturn", Code: "SAT", NAIFID: 699, Class: ClassGiant, SemiMajorAU: 9.537},
	{Name: "Uranus", Code: "URA", NAIFID: 799, Class: ClassGiant, SemiMajorAU: 19.19},
	{Name: "Neptune", Code: "NEP", NAIFID: 899, Class: ClassGiant, SemiMajorAU: 30.07},
}

// SolarSystemCache caches solar system body positions.
type SolarSystemCache struct {
	mu sync.RWMutex

	// Cached snapshot
	snapshot         SolarSystemSnapshot
	lastPlanetUpdate time.Time
	lastSCUpdate     time.Time

	// Provider interface for Horizons queries
	provider SolarSystemProvider
}

// SolarSystemProvider defines the interface for fetching heliocentric positions.
type SolarSystemProvider interface {
	// GetHeliocentricPosition returns heliocentric ecliptic position in AU.
	GetHeliocentricPosition(naifID int, t time.Time) (astro.Vec3, error)
}

// Planet cache TTL (planets move slowly)
const PlanetCacheTTL = 10 * time.Minute

// Spacecraft cache TTL
const SpacecraftCacheTTL = 5 * time.Minute

// NewSolarSystemCache creates a new cache.
func NewSolarSystemCache(provider SolarSystemProvider) *SolarSystemCache {
	return &SolarSystemCache{
		provider: provider,
		snapshot: SolarSystemSnapshot{
			Bodies: []EclipticBody{
				// Sun at origin (always present)
				{Name: "Sun", Code: "SUN", Kind: BodySun, Pos: astro.Vec3{}},
			},
		},
	}
}

// GetSnapshot returns the current cached snapshot.
func (c *SolarSystemCache) GetSnapshot() SolarSystemSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

// NeedsPlanetRefresh returns true if planet data needs refreshing.
func (c *SolarSystemCache) NeedsPlanetRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastPlanetUpdate) > PlanetCacheTTL
}

// NeedsSpacecraftRefresh returns true if spacecraft data needs refreshing.
func (c *SolarSystemCache) NeedsSpacecraftRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastSCUpdate) > SpacecraftCacheTTL
}

// UpdatePlanets fetches fresh planet positions from the provider.
func (c *SolarSystemCache) UpdatePlanets() error {
	if c.provider == nil {
		// Use static fallback positions
		return c.updatePlanetsStatic()
	}

	now := time.Now()
	var planets []EclipticBody

	for _, p := range Planets {
		pos, err := c.provider.GetHeliocentricPosition(p.NAIFID, now)
		if err != nil {
			// Use approximate position based on semi-major axis
			pos = approximatePlanetPosition(p, now)
		}

		planets = append(planets, EclipticBody{
			Name:  p.Name,
			Code:  p.Code,
			Kind:  BodyPlanet,
			Class: p.Class,
			Pos:   pos,
		})
	}

	c.mu.Lock()
	// Rebuild snapshot with new planets
	newBodies := []EclipticBody{{Name: "Sun", Code: "SUN", Kind: BodySun, Pos: astro.Vec3{}}}
	newBodies = append(newBodies, planets...)

	// Preserve existing spacecraft
	for _, b := range c.snapshot.Bodies {
		if b.Kind == BodySpacecraft {
			newBodies = append(newBodies, b)
		}
	}

	c.snapshot = SolarSystemSnapshot{
		GeneratedAt: now,
		Bodies:      newBodies,
	}
	c.lastPlanetUpdate = now
	c.mu.Unlock()

	return nil
}

// updatePlanetsStatic uses approximate positions without Horizons.
func (c *SolarSystemCache) updatePlanetsStatic() error {
	now := time.Now()
	var planets []EclipticBody

	for _, p := range Planets {
		pos := approximatePlanetPosition(p, now)
		planets = append(planets, EclipticBody{
			Name:  p.Name,
			Code:  p.Code,
			Kind:  BodyPlanet,
			Class: p.Class,
			Pos:   pos,
		})
	}

	c.mu.Lock()
	newBodies := []EclipticBody{{Name: "Sun", Code: "SUN", Kind: BodySun, Pos: astro.Vec3{}}}
	newBodies = append(newBodies, planets...)

	// Preserve existing spacecraft
	for _, b := range c.snapshot.Bodies {
		if b.Kind == BodySpacecraft {
			newBodies = append(newBodies, b)
		}
	}

	c.snapshot = SolarSystemSnapshot{
		GeneratedAt: now,
		Bodies:      newBodies,
	}
	c.lastPlanetUpdate = now
	c.mu.Unlock()

	return nil
}

// approximatePlanetPosition generates a rough position based on orbital period.
// This is a placeholder for when Horizons is unavailable.
func approximatePlanetPosition(p PlanetDef, t time.Time) astro.Vec3 {
	// Orbital period in years (Kepler's 3rd law approximation)
	periodYears := p.SemiMajorAU * p.SemiMajorAU * p.SemiMajorAU
	periodYears = math.Sqrt(periodYears)

	// Days since J2000 epoch
	j2000 := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	daysSinceJ2000 := t.Sub(j2000).Hours() / 24

	// Mean anomaly (very rough, ignores eccentricity)
	meanAnomaly := 2 * math.Pi * (daysSinceJ2000 / (periodYears * 365.25))

	// Simple circular approximation
	x := p.SemiMajorAU * math.Cos(meanAnomaly)
	y := p.SemiMajorAU * math.Sin(meanAnomaly)

	return astro.Vec3{X: x, Y: y, Z: 0}
}

// UpdateSpacecraft updates spacecraft positions from DSN data.
func (c *SolarSystemCache) UpdateSpacecraft(dsnData *DSNData) error {
	if dsnData == nil {
		return nil
	}

	now := time.Now()

	// Build elevation map for position data
	elevMap := BuildElevationMap(dsnData)
	scViews := BuildSpacecraftViews(dsnData, elevMap)

	var spacecraft []EclipticBody
	for _, sv := range scViews {
		// Convert DSN distance to AU
		distanceAU := astro.KmToAU(sv.PrimaryLink.DistanceKm)
		if distanceAU < 0.001 {
			continue // Skip invalid distances
		}

		// Use RA/Dec to compute approximate heliocentric position
		// This is approximate since DSN gives geocentric RA/Dec
		// For deep space missions, heliocentric is close to geocentric direction
		pos := raDecToEclipticVec(sv.Coord().RAdeg, sv.Coord().DecDeg, distanceAU)

		spacecraft = append(spacecraft, EclipticBody{
			Name: sv.Name,
			Code: sv.Code,
			Kind: BodySpacecraft,
			Pos:  pos,
			Meta: map[string]string{
				"antenna": sv.AntennaList(),
				"band":    sv.PrimaryLink.Band,
			},
		})
	}

	c.mu.Lock()
	// Rebuild snapshot preserving planets
	newBodies := []EclipticBody{}
	for _, b := range c.snapshot.Bodies {
		if b.Kind != BodySpacecraft {
			newBodies = append(newBodies, b)
		}
	}
	newBodies = append(newBodies, spacecraft...)

	c.snapshot = SolarSystemSnapshot{
		GeneratedAt: now,
		Bodies:      newBodies,
	}
	c.lastSCUpdate = now
	c.mu.Unlock()

	return nil
}

// raDecToEclipticVec converts RA/Dec + distance to heliocentric ecliptic position.
// Note: This assumes the object is far enough that geocentric ≈ heliocentric direction.
func raDecToEclipticVec(raDeg, decDeg, distAU float64) astro.Vec3 {
	// Convert RA/Dec to unit vector in equatorial frame
	raRad := raDeg * math.Pi / 180
	decRad := decDeg * math.Pi / 180

	cosD := math.Cos(decRad)
	x := cosD * math.Cos(raRad)
	y := cosD * math.Sin(raRad)
	z := math.Sin(decRad)

	// Scale by distance
	eq := astro.Vec3{X: x * distAU, Y: y * distAU, Z: z * distAU}

	// Convert to ecliptic
	return astro.EquatorialToEcliptic(eq)
}
