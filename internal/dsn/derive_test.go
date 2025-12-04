package dsn

import (
	"math"
	"testing"
)

func TestDistanceFromRTLT(t *testing.T) {
	tests := []struct {
		name     string
		rtlt     float64
		expected float64
		tolerance float64
	}{
		{
			name:      "Moon (approx 2.5s RTLT)",
			rtlt:      2.5,
			expected:  374688.57, // ~375,000 km
			tolerance: 1000,
		},
		{
			name:      "Mars close approach (approx 3 min RTLT)",
			rtlt:      180,
			expected:  26981321.22, // ~27 million km
			tolerance: 100000,
		},
		{
			name:      "Voyager 1 (approx 44 hours RTLT)",
			rtlt:      160200, // ~44.5 hours in seconds
			expected:  24016831044.8, // ~24 billion km
			tolerance: 1e9,
		},
		{
			name:      "Zero RTLT",
			rtlt:      0,
			expected:  0,
			tolerance: 0,
		},
		{
			name:      "Negative RTLT",
			rtlt:      -5,
			expected:  0,
			tolerance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DistanceFromRTLT(tt.rtlt)
			if math.Abs(got-tt.expected) > tt.tolerance {
				t.Errorf("DistanceFromRTLT(%v) = %v, want %v (±%v)", tt.rtlt, got, tt.expected, tt.tolerance)
			}
		})
	}
}

func TestVelocityFromRTLTDelta(t *testing.T) {
	tests := []struct {
		name      string
		rtlt1     float64
		rtlt2     float64
		deltaTime float64
		wantSign  int // 1 = positive (receding), -1 = negative (approaching), 0 = zero
	}{
		{
			name:      "Spacecraft receding",
			rtlt1:     1200,
			rtlt2:     1205, // RTLT increased = moving away
			deltaTime: 60,
			wantSign:  1,
		},
		{
			name:      "Spacecraft approaching",
			rtlt1:     1200,
			rtlt2:     1195, // RTLT decreased = moving closer
			deltaTime: 60,
			wantSign:  -1,
		},
		{
			name:      "Stationary",
			rtlt1:     1200,
			rtlt2:     1200,
			deltaTime: 60,
			wantSign:  0,
		},
		{
			name:      "Zero delta time",
			rtlt1:     1200,
			rtlt2:     1205,
			deltaTime: 0,
			wantSign:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VelocityFromRTLTDelta(tt.rtlt1, tt.rtlt2, tt.deltaTime)

			switch tt.wantSign {
			case 1:
				if got <= 0 {
					t.Errorf("expected positive velocity, got %v", got)
				}
			case -1:
				if got >= 0 {
					t.Errorf("expected negative velocity, got %v", got)
				}
			case 0:
				if got != 0 {
					t.Errorf("expected zero velocity, got %v", got)
				}
			}
		})
	}
}

func TestStruggleIndex(t *testing.T) {
	tests := []struct {
		name      string
		link      Link
		elevation float64
		wantMin   float64
		wantMax   float64
	}{
		{
			name: "Easy link - close, high rate, good elevation",
			link: Link{
				Distance: 400000, // Moon distance
				DataRate: 1e6,    // 1 Mbps
			},
			elevation: 60,
			wantMin:   0,
			wantMax:   0.3,
		},
		{
			name: "Hard link - far, low rate, low elevation",
			link: Link{
				Distance: 24e9, // Voyager distance
				DataRate: 160,  // 160 bps
			},
			elevation: 10,
			wantMin:   0.7,
			wantMax:   1.0,
		},
		{
			name: "Medium link",
			link: Link{
				Distance: 200e6, // Mars
				DataRate: 100000,
			},
			elevation: 30,
			wantMin:   0.3,
			wantMax:   0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StruggleIndex(tt.link, tt.elevation)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("StruggleIndex() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestStruggleIndex_Bounds(t *testing.T) {
	// Test that struggle index is always between 0 and 1
	testCases := []struct {
		link      Link
		elevation float64
	}{
		{Link{Distance: 0, DataRate: 0}, 0},
		{Link{Distance: 1e20, DataRate: 0.001}, 0},
		{Link{Distance: 100, DataRate: 1e12}, 90},
	}

	for _, tc := range testCases {
		got := StruggleIndex(tc.link, tc.elevation)
		if got < 0 || got > 1 {
			t.Errorf("StruggleIndex() = %v, want between 0 and 1", got)
		}
	}
}

func TestComplexUtilization(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{
				Complex: ComplexGoldstone,
				Antennas: []Antenna{
					{ID: "DSS-14", Targets: []Target{{Name: "Test"}}},
					{ID: "DSS-24", Targets: []Target{}}, // Idle
				},
			},
			{
				Complex: ComplexCanberra,
				Antennas: []Antenna{
					{ID: "DSS-34", Targets: []Target{{Name: "Test1"}, {Name: "Test2"}}}, // MSPA
				},
			},
		},
	}

	loads := ComplexUtilization(data)

	// Goldstone: 1 active out of 2 = 50%
	if loads[ComplexGoldstone].TotalAntennas != 2 {
		t.Errorf("Goldstone antennas = %d, want 2", loads[ComplexGoldstone].TotalAntennas)
	}
	if loads[ComplexGoldstone].ActiveLinks != 1 {
		t.Errorf("Goldstone active links = %d, want 1", loads[ComplexGoldstone].ActiveLinks)
	}
	if loads[ComplexGoldstone].Utilization != 0.5 {
		t.Errorf("Goldstone utilization = %v, want 0.5", loads[ComplexGoldstone].Utilization)
	}

	// Canberra: 2 links on 1 antenna = 100% (capped)
	if loads[ComplexCanberra].ActiveLinks != 2 {
		t.Errorf("Canberra active links = %d, want 2", loads[ComplexCanberra].ActiveLinks)
	}
	if loads[ComplexCanberra].Utilization != 1.0 {
		t.Errorf("Canberra utilization = %v, want 1.0", loads[ComplexCanberra].Utilization)
	}
}

func TestAggregateSpacecraft(t *testing.T) {
	data := &DSNData{
		Links: []Link{
			{SpacecraftID: 1, Spacecraft: "Alpha", Distance: 1000},
			{SpacecraftID: 1, Spacecraft: "Alpha", Distance: 1100}, // Same spacecraft, different link
			{SpacecraftID: 2, Spacecraft: "Beta", Distance: 5000},
		},
	}

	spacecraft := AggregateSpacecraft(data)

	if len(spacecraft) != 2 {
		t.Fatalf("expected 2 spacecraft, got %d", len(spacecraft))
	}

	// Should be sorted by name
	if spacecraft[0].Name != "Alpha" {
		t.Errorf("first spacecraft = %q, want Alpha", spacecraft[0].Name)
	}
	if spacecraft[1].Name != "Beta" {
		t.Errorf("second spacecraft = %q, want Beta", spacecraft[1].Name)
	}

	// Alpha should have 2 links
	if len(spacecraft[0].Links) != 2 {
		t.Errorf("Alpha links = %d, want 2", len(spacecraft[0].Links))
	}

	// Alpha should use the smaller (more conservative) distance
	if spacecraft[0].Distance != 1000 {
		t.Errorf("Alpha distance = %v, want 1000", spacecraft[0].Distance)
	}
}

func TestNextHandoffPrediction(t *testing.T) {
	tests := []struct {
		current   Complex
		elevation float64
		expected  Complex
	}{
		{ComplexGoldstone, 10, ComplexCanberra},  // Low elevation = handoff imminent
		{ComplexCanberra, 10, ComplexMadrid},
		{ComplexMadrid, 10, ComplexGoldstone},
		{ComplexGoldstone, 45, ""},               // High elevation = no handoff
		{ComplexGoldstone, 15, ""},               // At threshold
	}

	for _, tt := range tests {
		t.Run(string(tt.current), func(t *testing.T) {
			got := NextHandoffPrediction(tt.current, tt.elevation)
			if got != tt.expected {
				t.Errorf("NextHandoffPrediction(%q, %v) = %q, want %q",
					tt.current, tt.elevation, got, tt.expected)
			}
		})
	}
}

func TestFormatDistance(t *testing.T) {
	tests := []struct {
		km       float64
		contains string
	}{
		{0, "N/A"},
		{-1, "N/A"},
		{1000, "km"},
		{1e6, "M km"},
		{1e9, "B km"},
		{1e12, "AU"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			got := FormatDistance(tt.km)
			if !containsString(got, tt.contains) {
				t.Errorf("FormatDistance(%v) = %q, want to contain %q", tt.km, got, tt.contains)
			}
		})
	}
}

func TestFormatDataRate(t *testing.T) {
	tests := []struct {
		bps      float64
		contains string
	}{
		{0, "N/A"},
		{500, "bps"},
		{5000, "kbps"},
		{5e6, "Mbps"},
		{5e9, "Gbps"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			got := FormatDataRate(tt.bps)
			if !containsString(got, tt.contains) {
				t.Errorf("FormatDataRate(%v) = %q, want to contain %q", tt.bps, got, tt.contains)
			}
		})
	}
}

func TestFormatRTLT(t *testing.T) {
	tests := []struct {
		seconds  float64
		contains string
	}{
		{0, "N/A"},
		{30, "s"},
		{120, "min"},
		{7200, "hr"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			got := FormatRTLT(tt.seconds)
			if !containsString(got, tt.contains) {
				t.Errorf("FormatRTLT(%v) = %q, want to contain %q", tt.seconds, got, tt.contains)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSkyObjects(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{
				Name:    "mdscc",
				Complex: ComplexMadrid,
				Antennas: []Antenna{
					{
						ID:        "DSS55",
						Azimuth:   213.5,
						Elevation: 18.2,
						Targets: []Target{
							{ID: 62, Name: "EMM", RTLT: 2420},
						},
					},
				},
			},
		},
		Links: []Link{
			{AntennaID: "DSS55", Spacecraft: "EMM", Band: "X", DataRate: 241900, Distance: 1000},
		},
	}

	objs := data.SkyObjects()
	if len(objs) != 1 {
		t.Fatalf("expected 1 sky object, got %d", len(objs))
	}

	obj := objs[0]
	if obj.Spacecraft != "EMM" {
		t.Errorf("spacecraft = %q, want EMM", obj.Spacecraft)
	}
	if obj.Azimuth != 213.5 {
		t.Errorf("azimuth = %v, want 213.5", obj.Azimuth)
	}
	if obj.Elevation != 18.2 {
		t.Errorf("elevation = %v, want 18.2", obj.Elevation)
	}
	if obj.Band != "X" {
		t.Errorf("band = %q, want X", obj.Band)
	}
}

func TestSkyObjects_Empty(t *testing.T) {
	var data *DSNData
	objs := data.SkyObjects()
	if objs != nil {
		t.Errorf("expected nil for nil data, got %v", objs)
	}
}
