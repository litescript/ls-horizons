// Package dsn spacecraft catalog with full names and metadata.
package dsn

// SpacecraftInfo contains metadata about a spacecraft.
type SpacecraftInfo struct {
	Name   string // Full mission name
	Agency string // Operating agency (NASA, ESA, JAXA, etc.)
	Target string // Mission target (Mars, Jupiter, Deep Space, etc.)
	Launch string // Launch year
}

// SpacecraftCatalog maps short codes to spacecraft info.
// Data sourced from NASA DSN Now and mission pages.
var SpacecraftCatalog = map[string]SpacecraftInfo{
	// Active Deep Space Missions
	"JWST":  {Name: "James Webb Space Telescope", Agency: "NASA/ESA/CSA", Target: "L2 Orbit", Launch: "2021"},
	"VGR1":  {Name: "Voyager 1", Agency: "NASA", Target: "Interstellar", Launch: "1977"},
	"VGR2":  {Name: "Voyager 2", Agency: "NASA", Target: "Interstellar", Launch: "1977"},
	"MRO":   {Name: "Mars Reconnaissance Orbiter", Agency: "NASA", Target: "Mars", Launch: "2005"},
	"MAVEN": {Name: "Mars Atmosphere & Volatile Evolution", Agency: "NASA", Target: "Mars", Launch: "2013"},
	"MSL":   {Name: "Curiosity Rover", Agency: "NASA", Target: "Mars", Launch: "2011"},
	"M20":   {Name: "Perseverance Rover", Agency: "NASA", Target: "Mars", Launch: "2020"},
	"MVN":   {Name: "Mars Atmosphere & Volatile Evolution", Agency: "NASA", Target: "Mars", Launch: "2013"},
	"ODY":   {Name: "Mars Odyssey", Agency: "NASA", Target: "Mars", Launch: "2001"},
	"JUNO":  {Name: "Juno", Agency: "NASA", Target: "Jupiter", Launch: "2011"},
	"JNO":   {Name: "Juno", Agency: "NASA", Target: "Jupiter", Launch: "2011"},
	"NHPC":  {Name: "New Horizons", Agency: "NASA", Target: "Kuiper Belt", Launch: "2006"},
	"NH":    {Name: "New Horizons", Agency: "NASA", Target: "Kuiper Belt", Launch: "2006"},
	"LUCY":  {Name: "Lucy", Agency: "NASA", Target: "Trojan Asteroids", Launch: "2021"},
	"PSYC":  {Name: "Psyche", Agency: "NASA", Target: "16 Psyche Asteroid", Launch: "2023"},
	"EURC":  {Name: "Europa Clipper", Agency: "NASA", Target: "Jupiter/Europa", Launch: "2024"},
	"EMM":   {Name: "Hope Mars Mission", Agency: "UAE", Target: "Mars", Launch: "2020"},
	"TGO":   {Name: "ExoMars Trace Gas Orbiter", Agency: "ESA/Roscosmos", Target: "Mars", Launch: "2016"},
	"MEX":   {Name: "Mars Express", Agency: "ESA", Target: "Mars", Launch: "2003"},
	"ROSE":  {Name: "Rosetta", Agency: "ESA", Target: "Comet", Launch: "2004"},
	"GAIA":  {Name: "Gaia", Agency: "ESA", Target: "L2 Orbit", Launch: "2013"},
	"BEPI":  {Name: "BepiColombo", Agency: "ESA/JAXA", Target: "Mercury", Launch: "2018"},
	"SOLO":  {Name: "Solar Orbiter", Agency: "ESA/NASA", Target: "Sun", Launch: "2020"},
	"JUICE": {Name: "Jupiter Icy Moons Explorer", Agency: "ESA", Target: "Jupiter", Launch: "2023"},

	// Solar & Heliophysics
	"SOHO": {Name: "Solar & Heliospheric Observatory", Agency: "ESA/NASA", Target: "Sun/L1", Launch: "1995"},
	"ACE":  {Name: "Advanced Composition Explorer", Agency: "NASA", Target: "L1 Orbit", Launch: "1997"},
	"WIND": {Name: "WIND", Agency: "NASA", Target: "L1 Orbit", Launch: "1994"},
	"DSCO": {Name: "Deep Space Climate Observatory", Agency: "NASA/NOAA", Target: "L1 Orbit", Launch: "2015"},
	"STA":  {Name: "STEREO-A", Agency: "NASA", Target: "Solar Orbit", Launch: "2006"},
	"STB":  {Name: "STEREO-B", Agency: "NASA", Target: "Solar Orbit", Launch: "2006"},
	"SPP":  {Name: "Parker Solar Probe", Agency: "NASA", Target: "Sun", Launch: "2018"},
	"PSP":  {Name: "Parker Solar Probe", Agency: "NASA", Target: "Sun", Launch: "2018"},

	// Lunar Missions
	"LRO":   {Name: "Lunar Reconnaissance Orbiter", Agency: "NASA", Target: "Moon", Launch: "2009"},
	"LCROS": {Name: "Lunar Crater Observation", Agency: "NASA", Target: "Moon", Launch: "2009"},
	"KPLO":  {Name: "Korea Pathfinder Lunar Orbiter", Agency: "KARI", Target: "Moon", Launch: "2022"},
	"SLIM":  {Name: "Smart Lander for Investigating Moon", Agency: "JAXA", Target: "Moon", Launch: "2023"},
	"CH2":   {Name: "Chandrayaan-2 Orbiter", Agency: "ISRO", Target: "Moon", Launch: "2019"},
	"CH3":   {Name: "Chandrayaan-3", Agency: "ISRO", Target: "Moon", Launch: "2023"},
	"CAPS":  {Name: "Capstone", Agency: "NASA", Target: "Moon", Launch: "2022"},

	// Earth Orbiters tracked by DSN
	"TESS": {Name: "Transiting Exoplanet Survey Satellite", Agency: "NASA", Target: "Earth Orbit", Launch: "2018"},
	"IXPE": {Name: "Imaging X-ray Polarimetry Explorer", Agency: "NASA", Target: "Earth Orbit", Launch: "2021"},
	"SWOT": {Name: "Surface Water & Ocean Topography", Agency: "NASA/CNES", Target: "Earth Orbit", Launch: "2022"},

	// Historic/Inactive (may still appear in DSN)
	"CAS":    {Name: "Cassini", Agency: "NASA/ESA", Target: "Saturn", Launch: "1997"},
	"DAWN":   {Name: "Dawn", Agency: "NASA", Target: "Vesta/Ceres", Launch: "2007"},
	"KEPLER": {Name: "Kepler", Agency: "NASA", Target: "Earth Orbit", Launch: "2009"},
	"SPITZ":  {Name: "Spitzer Space Telescope", Agency: "NASA", Target: "Earth Orbit", Launch: "2003"},
}

// GetSpacecraftInfo returns info for a spacecraft code, or nil if unknown.
func GetSpacecraftInfo(code string) *SpacecraftInfo {
	if info, ok := SpacecraftCatalog[code]; ok {
		return &info
	}
	return nil
}

// GetSpacecraftName returns the full name for a spacecraft code, or the code itself if unknown.
func GetSpacecraftName(code string) string {
	if info, ok := SpacecraftCatalog[code]; ok {
		return info.Name
	}
	return code
}
