package astro

import (
	"math"
	"time"
)

// Planet positions are computed locally from Keplerian orbital elements rather
// than queried from JPL Horizons. Horizons is a live computation service under a
// fair-use policy; planets move imperceptibly on the timescales this app renders,
// so querying them repeatedly is load without benefit. The element set below is
// JPL's own "Approximate Positions of the Major Planets" table (Standish), which
// is accurate to arcminutes over 1800-2050 -- far below the resolution of any
// view in this app.

// PlanetElements holds J2000 Keplerian elements and their per-Julian-century rates.
// Angles are degrees, distances are AU.
type PlanetElements struct {
	Name string

	A, ADot       float64 // semi-major axis
	E, EDot       float64 // eccentricity
	Inc, IncDot   float64 // inclination to the ecliptic
	L, LDot       float64 // mean longitude
	Peri, PeriDot float64 // longitude of perihelion
	Node, NodeDot float64 // longitude of ascending node
}

// PlanetElementTable is the JPL approximate-elements table valid for 1800-2050.
// The Earth entry is the Earth-Moon barycenter; the offset from Earth's center is
// ~4700 km, which is negligible at AU scale.
var PlanetElementTable = []PlanetElements{
	{
		Name: "Mercury",
		A:    0.38709927, ADot: 0.00000037,
		E: 0.20563593, EDot: 0.00001906,
		Inc: 7.00497902, IncDot: -0.00594749,
		L: 252.25032350, LDot: 149472.67411175,
		Peri: 77.45779628, PeriDot: 0.16047689,
		Node: 48.33076593, NodeDot: -0.12534081,
	},
	{
		Name: "Venus",
		A:    0.72333566, ADot: 0.00000390,
		E: 0.00677672, EDot: -0.00004107,
		Inc: 3.39467605, IncDot: -0.00078890,
		L: 181.97909950, LDot: 58517.81538729,
		Peri: 131.60246718, PeriDot: 0.00268329,
		Node: 76.67984255, NodeDot: -0.27769418,
	},
	{
		Name: "Earth",
		A:    1.00000261, ADot: 0.00000562,
		E: 0.01671123, EDot: -0.00004392,
		Inc: -0.00001531, IncDot: -0.01294668,
		L: 100.46457166, LDot: 35999.37244981,
		Peri: 102.93768193, PeriDot: 0.32327364,
		Node: 0.0, NodeDot: 0.0,
	},
	{
		Name: "Mars",
		A:    1.52371034, ADot: 0.00001847,
		E: 0.09339410, EDot: 0.00007882,
		Inc: 1.84969142, IncDot: -0.00813131,
		L: -4.55343205, LDot: 19140.30268499,
		Peri: -23.94362959, PeriDot: 0.44441088,
		Node: 49.55953891, NodeDot: -0.29257343,
	},
	{
		Name: "Jupiter",
		A:    5.20288700, ADot: -0.00011607,
		E: 0.04838624, EDot: -0.00013253,
		Inc: 1.30439695, IncDot: -0.00183714,
		L: 34.39644051, LDot: 3034.74612775,
		Peri: 14.72847983, PeriDot: 0.21252668,
		Node: 100.47390909, NodeDot: 0.20469106,
	},
	{
		Name: "Saturn",
		A:    9.53667594, ADot: -0.00125060,
		E: 0.05386179, EDot: -0.00050991,
		Inc: 2.48599187, IncDot: 0.00193609,
		L: 49.95424423, LDot: 1222.49362201,
		Peri: 92.59887831, PeriDot: -0.41897216,
		Node: 113.66242448, NodeDot: -0.28867794,
	},
	{
		Name: "Uranus",
		A:    19.18916464, ADot: -0.00196176,
		E: 0.04725744, EDot: -0.00004397,
		Inc: 0.77263783, IncDot: -0.00242939,
		L: 313.23810451, LDot: 428.48202785,
		Peri: 170.95427630, PeriDot: 0.40805281,
		Node: 74.01692503, NodeDot: 0.04240589,
	},
	{
		Name: "Neptune",
		A:    30.06992276, ADot: 0.00026291,
		E: 0.00859048, EDot: 0.00005105,
		Inc: 1.77004347, IncDot: 0.00035372,
		L: -55.12002969, LDot: 218.45945325,
		Peri: 44.96476227, PeriDot: -0.32241464,
		Node: 131.78422574, NodeDot: -0.00508664,
	},
}

// PlanetElementsByName returns the element set for a planet, matched case-sensitively.
func PlanetElementsByName(name string) (PlanetElements, bool) {
	for _, el := range PlanetElementTable {
		if el.Name == name {
			return el, true
		}
	}
	return PlanetElements{}, false
}

// keplerMaxIter bounds the Newton-Raphson refinement of the eccentric anomaly.
// Planetary eccentricities are all below 0.21, where convergence is reached in
// three or four passes; the cap only guards against a pathological input.
const keplerMaxIter = 30

// keplerTolerance is the convergence threshold on the eccentric anomaly, in radians.
// 1e-10 rad is ~2e-5 arcsec, far finer than the element table's own accuracy.
const keplerTolerance = 1e-10

// HeliocentricPosition returns the planet's J2000 heliocentric ecliptic position
// in AU at time t.
func (el PlanetElements) HeliocentricPosition(t time.Time) Vec3 {
	// Julian centuries past the J2000.0 epoch.
	centuries := (julianDate(t) - 2451545.0) / 36525.0

	a := el.A + el.ADot*centuries
	e := el.E + el.EDot*centuries
	inc := degToRad(el.Inc + el.IncDot*centuries)
	meanLon := el.L + el.LDot*centuries
	periLon := el.Peri + el.PeriDot*centuries
	nodeLon := degToRad(el.Node + el.NodeDot*centuries)

	// Argument of perihelion, measured from the ascending node.
	argPeri := degToRad(periLon) - nodeLon

	// Mean anomaly, wrapped to [-180, 180) so the solver starts near the root.
	meanAnom := degToRad(wrapDegrees(meanLon - periLon))

	eccAnom := solveKepler(meanAnom, e)

	// Position in the orbital plane, with the x-axis toward perihelion.
	xOrb := a * (math.Cos(eccAnom) - e)
	yOrb := a * math.Sqrt(1-e*e) * math.Sin(eccAnom)

	// Rotate orbital plane into the J2000 ecliptic frame:
	// R_z(-node) * R_x(-inc) * R_z(-argPeri).
	cosArg, sinArg := math.Cos(argPeri), math.Sin(argPeri)
	cosNode, sinNode := math.Cos(nodeLon), math.Sin(nodeLon)
	cosInc, sinInc := math.Cos(inc), math.Sin(inc)

	return Vec3{
		X: (cosArg*cosNode-sinArg*sinNode*cosInc)*xOrb + (-sinArg*cosNode-cosArg*sinNode*cosInc)*yOrb,
		Y: (cosArg*sinNode+sinArg*cosNode*cosInc)*xOrb + (-sinArg*sinNode+cosArg*cosNode*cosInc)*yOrb,
		Z: (sinArg*sinInc)*xOrb + (cosArg*sinInc)*yOrb,
	}
}

// solveKepler solves M = E - e*sin(E) for the eccentric anomaly E, by Newton-Raphson.
func solveKepler(meanAnom, e float64) float64 {
	eccAnom := meanAnom + e*math.Sin(meanAnom)
	for i := 0; i < keplerMaxIter; i++ {
		residual := eccAnom - e*math.Sin(eccAnom) - meanAnom
		denom := 1 - e*math.Cos(eccAnom)
		if denom == 0 {
			break
		}
		delta := residual / denom
		eccAnom -= delta
		if math.Abs(delta) < keplerTolerance {
			break
		}
	}
	return eccAnom
}

// wrapDegrees folds an angle into [-180, 180).
func wrapDegrees(deg float64) float64 {
	wrapped := math.Mod(deg+180, 360)
	if wrapped < 0 {
		wrapped += 360
	}
	return wrapped - 180
}
