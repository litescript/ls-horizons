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
	NAIFChandra          TargetID = -151
	NAIFHubble           TargetID = -48
	NAIFACE              TargetID = -92
	NAIFMMS              TargetID = -135
	NAIFGeotail          TargetID = -148
	NAIFIBEX             TargetID = -169
	NAIFSpitzer          TargetID = -79
	NAIFNUSTAR           TargetID = -166
	NAIFSuzaku           TargetID = -150
	NAIFXMM              TargetID = -125
	NAIFINTEGRAL         TargetID = -130
	NAIFFermi            TargetID = -160
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

	// Earth Orbit / X-ray / Gamma-ray Observatories
	{Code: "TESS", Name: "TESS", NAIFID: NAIFTESS},
	{Code: "SWOT", Name: "SWOT", NAIFID: NAIFSWOT},
	{Code: "IXPE", Name: "IXPE", NAIFID: NAIFIXPE},
	{Code: "CHDR", Name: "Chandra", NAIFID: NAIFChandra, Aliases: []string{"CXO", "CHANDRA"}},
	{Code: "HST", Name: "Hubble", NAIFID: NAIFHubble, Aliases: []string{"HUBBLE"}},
	{Code: "ACE", Name: "ACE", NAIFID: NAIFACE},
	{Code: "MMS", Name: "MMS", NAIFID: NAIFMMS},
	{Code: "GTAIL", Name: "Geotail", NAIFID: NAIFGeotail, Aliases: []string{"GEOTAIL"}},
	{Code: "IBEX", Name: "IBEX", NAIFID: NAIFIBEX},
	{Code: "SPTZ", Name: "Spitzer", NAIFID: NAIFSpitzer, Aliases: []string{"SPITZER"}},
	{Code: "NUSTAR", Name: "NuSTAR", NAIFID: NAIFNUSTAR},
	{Code: "SUZAKU", Name: "Suzaku", NAIFID: NAIFSuzaku},
	{Code: "XMM", Name: "XMM-Newton", NAIFID: NAIFXMM, Aliases: []string{"XMM-NEWTON"}},
	{Code: "INTEG", Name: "INTEGRAL", NAIFID: NAIFINTEGRAL, Aliases: []string{"INTEGRAL"}},
	{Code: "FERMI", Name: "Fermi", NAIFID: NAIFFermi, Aliases: []string{"GLAST"}},
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

// TargetsByName maps spacecraft names (lowercase) to target info.
// Includes full names, codes, and common DSN variations.
var TargetsByName = func() map[string]TargetInfo {
	m := make(map[string]TargetInfo, len(Targets)*3)
	for _, t := range Targets {
		// Add by full name
		m[normalizeName(t.Name)] = t
		// Add by code (DSN often uses code as name)
		m[normalizeName(t.Code)] = t
		// Add aliases
		for _, alias := range t.Aliases {
			m[normalizeName(alias)] = t
		}
	}
	// Add common DSN name variations that don't match our canonical names
	addVariation := func(variation string, code string) {
		if t, ok := TargetsByCode[code]; ok {
			m[normalizeName(variation)] = t
		}
	}
	// DSN often uses these variations
	addVariation("MSL", "MSL")           // Curiosity
	addVariation("M2020", "M20")         // Perseverance
	addVariation("MARS 2020", "M20")     // Perseverance
	addVariation("PSP", "SPP")           // Parker Solar Probe
	addVariation("PARKER", "SPP")        // Parker Solar Probe
	addVariation("NH", "NHPC")           // New Horizons
	addVariation("EUROPA", "EURC")       // Europa Clipper
	addVariation("BEPICOLOMBO", "BEPI")  // BepiColombo (one word)
	addVariation("BEPI COLOMBO", "BEPI") // BepiColombo (two words)
	addVariation("STEREO AHEAD", "STA")  // STEREO-A
	addVariation("STEREO BEHIND", "STB") // STEREO-B
	addVariation("STEREO-AHEAD", "STA")  // STEREO-A
	addVariation("STEREO-BEHIND", "STB") // STEREO-B
	addVariation("JWST", "JWST")         // James Webb
	addVariation("WEBB", "JWST")         // James Webb
	addVariation("JAMES WEBB", "JWST")   // James Webb
	addVariation("CURIOSITY", "MSL")     // Curiosity Rover
	addVariation("PERSEVERANCE", "M20")  // Perseverance Rover
	addVariation("KPLO", "KPLO")         // Korea Lunar
	addVariation("DANURI", "KPLO")       // Korea Lunar (Korean name)
	addVariation("CH-3", "CH3")          // Chandrayaan-3
	addVariation("CHANDRAYAAN 3", "CH3") // Chandrayaan-3
	addVariation("EXOMARS", "TGO")       // ExoMars TGO
	addVariation("TRACE GAS ORBITER", "TGO")
	addVariation("HOPE", "EMM")      // Hope/Emirates Mars Mission
	addVariation("AL-AMAL", "EMM")   // Hope (Arabic name)
	addVariation("CAPSTONE", "CAPS") // Capstone
	return m
}()

// normalizeName converts a spacecraft name to lowercase for matching.
func normalizeName(name string) string {
	// Simple lowercase, handles most cases
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result = append(result, c)
	}
	return string(result)
}

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

// GetTargetByName returns target info for a spacecraft name (case-insensitive).
func GetTargetByName(name string) (TargetInfo, bool) {
	t, ok := TargetsByName[normalizeName(name)]
	return t, ok
}

// GetNAIFIDByName returns the NAIF ID for a spacecraft name, or 0 if unknown.
func GetNAIFIDByName(name string) TargetID {
	if t, ok := GetTargetByName(name); ok {
		return t.NAIFID
	}
	return 0
}
