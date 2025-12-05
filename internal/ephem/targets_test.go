package ephem

import "testing"

func TestGetNAIFID_KnownSpacecraft(t *testing.T) {
	tests := []struct {
		code     string
		expected TargetID
	}{
		{"VGR1", NAIFVoyager1},
		{"VGR2", NAIFVoyager2},
		{"JUNO", NAIFJuno},
		{"JNO", NAIFJuno}, // alias
		{"MAVEN", NAIFMAVEN},
		{"MVN", NAIFMAVEN}, // alias
		{"JWST", NAIFJWST},
		{"MRO", NAIFMRO},
		{"MSL", NAIFCuriosity},
		{"M20", NAIFPerseverance},
		{"NHPC", NAIFNewHorizons},
		{"NH", NAIFNewHorizons}, // alias
		{"SPP", NAIFParkerSolarProbe},
		{"PSP", NAIFParkerSolarProbe}, // alias
		{"LRO", NAIFLRO},
		{"EURC", NAIFEuropaClipper},
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			got := GetNAIFID(tc.code)
			if got != tc.expected {
				t.Errorf("GetNAIFID(%q) = %d, want %d", tc.code, got, tc.expected)
			}
		})
	}
}

func TestGetNAIFID_Unknown(t *testing.T) {
	got := GetNAIFID("UNKNOWN123")
	if got != 0 {
		t.Errorf("GetNAIFID(UNKNOWN123) = %d, want 0", got)
	}
}

func TestTargetsByNAIF_Coverage(t *testing.T) {
	// Ensure all targets in the list are in the NAIF lookup
	for _, target := range Targets {
		if _, ok := TargetsByNAIF[target.NAIFID]; !ok {
			t.Errorf("Target %s (NAIF %d) missing from TargetsByNAIF", target.Code, target.NAIFID)
		}
	}
}

func TestTargetsByCode_Coverage(t *testing.T) {
	// Ensure all targets in the list are in the code lookup
	for _, target := range Targets {
		if _, ok := TargetsByCode[target.Code]; !ok {
			t.Errorf("Target %s missing from TargetsByCode", target.Code)
		}
		// Check aliases too
		for _, alias := range target.Aliases {
			if _, ok := TargetsByCode[alias]; !ok {
				t.Errorf("Alias %s for %s missing from TargetsByCode", alias, target.Code)
			}
		}
	}
}

func TestGetTargetByCode(t *testing.T) {
	info, ok := GetTargetByCode("VGR1")
	if !ok {
		t.Fatal("GetTargetByCode(VGR1) returned ok=false")
	}
	if info.Name != "Voyager 1" {
		t.Errorf("Name = %q, want %q", info.Name, "Voyager 1")
	}
	if info.NAIFID != NAIFVoyager1 {
		t.Errorf("NAIFID = %d, want %d", info.NAIFID, NAIFVoyager1)
	}
}

func TestGetTargetByNAIF(t *testing.T) {
	info, ok := GetTargetByNAIF(NAIFVoyager2)
	if !ok {
		t.Fatal("GetTargetByNAIF(NAIFVoyager2) returned ok=false")
	}
	if info.Code != "VGR2" {
		t.Errorf("Code = %q, want %q", info.Code, "VGR2")
	}
	if info.Name != "Voyager 2" {
		t.Errorf("Name = %q, want %q", info.Name, "Voyager 2")
	}
}
