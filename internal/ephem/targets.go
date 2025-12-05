package ephem

// TargetInfo contains mapping information for a spacecraft.
type TargetInfo struct {
	Code     string   // DSN short code (e.g., "VGR1")
	Name     string   // Full mission name
	NAIFID   TargetID // NAIF SPICE ID
	DSNID    int      // DSN spacecraft ID (if known)
	Aliases  []string // Alternative DSN codes
	HorizCmd string   // Horizons command string (if different from NAIF ID)
}

// Common NAIF SPICE IDs for DSN-tracked spacecraft.
// Sourced from https://naif.jpl.nasa.gov/pub/naif/toolkit_docs/C/req/naif_ids.html
const (
	NAIFVoyager1         TargetID = -31
	NAIFVoyager2         TargetID = -32
	NAIFMarsOdyssey      TargetID = -53
	NAIFJuno             TargetID = -61
	NAIFMRO              TargetID = -74
	NAIFCuriosity        TargetID = -76
	NAIFLRO              TargetID = -85
	NAIFParkerSolarProbe TargetID = -96
	NAIFNewHorizons      TargetID = -98
	NAIFLucy             TargetID = -49
	NAIFEuropaClipper    TargetID = -159
	NAIFPerseverance     TargetID = -168
	NAIFJWST             TargetID = -170
	NAIFMAVEN            TargetID = -202
	NAIFPsyche           TargetID = -255
	NAIFSolarOrbiter     TargetID = -144
	NAIFBepiColombo      TargetID = -121
	NAIFJUICE            TargetID = -28
	NAIFSTEREO_A         TargetID = -234
	NAIFSTEREO_B         TargetID = -235
	NAIFSOHO             TargetID = -21
	NAIFWIND             TargetID = -8
	NAIFDSCOVER          TargetID = -146
	NAIFGAIA             TargetID = -123
	NAIFMarsExpress      TargetID = -41
	NAIFExoMarsTGO       TargetID = -143
	NAIFTESS             TargetID = -95
	NAIFSWOT             TargetID = -152
	NAIFChandrayaan3     TargetID = -158
	NAIFKoreaLunar       TargetID = -155
	NAIFSLIM             TargetID = -157
	NAIFCapstone         TargetID = -186
	NAIFHopeMars         TargetID = -211
	NAIFIXPE             TargetID = -196
)

// Targets is the canonical list of tracked spacecraft with their NAIF mappings.
var Targets = []TargetInfo{
	// Interstellar
	{Code: "VGR1", Name: "Voyager 1", NAIFID: NAIFVoyager1},
	{Code: "VGR2", Name: "Voyager 2", NAIFID: NAIFVoyager2},

	// Mars
	{Code: "ODY", Name: "Mars Odyssey", NAIFID: NAIFMarsOdyssey},
	{Code: "MRO", Name: "Mars Reconnaissance Orbiter", NAIFID: NAIFMRO},
	{Code: "MSL", Name: "Curiosity Rover", NAIFID: NAIFCuriosity},
	{Code: "M20", Name: "Perseverance Rover", NAIFID: NAIFPerseverance},
	{Code: "MAVEN", Name: "MAVEN", NAIFID: NAIFMAVEN, Aliases: []string{"MVN"}},
	{Code: "MEX", Name: "Mars Express", NAIFID: NAIFMarsExpress},
	{Code: "TGO", Name: "ExoMars Trace Gas Orbiter", NAIFID: NAIFExoMarsTGO},
	{Code: "EMM", Name: "Hope Mars Mission", NAIFID: NAIFHopeMars},

	// Jupiter
	{Code: "JUNO", Name: "Juno", NAIFID: NAIFJuno, Aliases: []string{"JNO"}},
	{Code: "EURC", Name: "Europa Clipper", NAIFID: NAIFEuropaClipper},
	{Code: "JUICE", Name: "JUICE", NAIFID: NAIFJUICE},

	// Outer Solar System
	{Code: "NHPC", Name: "New Horizons", NAIFID: NAIFNewHorizons, Aliases: []string{"NH"}},

	// Asteroids
	{Code: "LUCY", Name: "Lucy", NAIFID: NAIFLucy},
	{Code: "PSYC", Name: "Psyche", NAIFID: NAIFPsyche},

	// Mercury
	{Code: "BEPI", Name: "BepiColombo", NAIFID: NAIFBepiColombo},

	// Solar
	{Code: "SPP", Name: "Parker Solar Probe", NAIFID: NAIFParkerSolarProbe, Aliases: []string{"PSP"}},
	{Code: "SOLO", Name: "Solar Orbiter", NAIFID: NAIFSolarOrbiter},
	{Code: "SOHO", Name: "SOHO", NAIFID: NAIFSOHO},
	{Code: "STA", Name: "STEREO-A", NAIFID: NAIFSTEREO_A},
	{Code: "STB", Name: "STEREO-B", NAIFID: NAIFSTEREO_B},
	{Code: "WIND", Name: "WIND", NAIFID: NAIFWIND},
	{Code: "DSCO", Name: "DSCOVR", NAIFID: NAIFDSCOVER},

	// Lunar
	{Code: "LRO", Name: "Lunar Reconnaissance Orbiter", NAIFID: NAIFLRO},
	{Code: "CAPS", Name: "Capstone", NAIFID: NAIFCapstone},
	{Code: "KPLO", Name: "Korea Pathfinder Lunar Orbiter", NAIFID: NAIFKoreaLunar},
	{Code: "SLIM", Name: "SLIM", NAIFID: NAIFSLIM},
	{Code: "CH3", Name: "Chandrayaan-3", NAIFID: NAIFChandrayaan3},

	// L2/Deep Space
	{Code: "JWST", Name: "James Webb Space Telescope", NAIFID: NAIFJWST},
	{Code: "GAIA", Name: "Gaia", NAIFID: NAIFGAIA},

	// Earth Orbit
	{Code: "TESS", Name: "TESS", NAIFID: NAIFTESS},
	{Code: "SWOT", Name: "SWOT", NAIFID: NAIFSWOT},
	{Code: "IXPE", Name: "IXPE", NAIFID: NAIFIXPE},
}

// TargetsByNAIF maps NAIF IDs to target info for quick lookup.
var TargetsByNAIF = func() map[TargetID]TargetInfo {
	m := make(map[TargetID]TargetInfo, len(Targets))
	for _, t := range Targets {
		m[t.NAIFID] = t
	}
	return m
}()

// TargetsByCode maps DSN codes to target info for quick lookup.
var TargetsByCode = func() map[string]TargetInfo {
	m := make(map[string]TargetInfo, len(Targets)*2)
	for _, t := range Targets {
		m[t.Code] = t
		for _, alias := range t.Aliases {
			m[alias] = t
		}
	}
	return m
}()

// GetNAIFID returns the NAIF ID for a DSN spacecraft code, or 0 if unknown.
func GetNAIFID(code string) TargetID {
	if t, ok := TargetsByCode[code]; ok {
		return t.NAIFID
	}
	return 0
}

// GetTargetByCode returns target info for a DSN code.
func GetTargetByCode(code string) (TargetInfo, bool) {
	t, ok := TargetsByCode[code]
	return t, ok
}

// GetTargetByNAIF returns target info for a NAIF ID.
func GetTargetByNAIF(id TargetID) (TargetInfo, bool) {
	t, ok := TargetsByNAIF[id]
	return t, ok
}
