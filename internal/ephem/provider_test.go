package ephem

import "testing"

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected Mode
	}{
		{"horizons", ModeHorizons},
		{"dsn", ModeDSN},
		{"auto", ModeAuto},
		{"", ModeAuto},        // default
		{"invalid", ModeAuto}, // default for unknown
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseMode(tc.input)
			if got != tc.expected {
				t.Errorf("ParseMode(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeHorizons, "horizons"},
		{ModeDSN, "dsn"},
		{ModeAuto, "auto"},
		{Mode(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := tc.mode.String()
			if got != tc.expected {
				t.Errorf("Mode(%d).String() = %q, want %q", tc.mode, got, tc.expected)
			}
		})
	}
}

func TestEphemerisPoint_Valid(t *testing.T) {
	point := EphemerisPoint{Valid: true}
	if !point.Valid {
		t.Error("Expected point to be valid")
	}

	point2 := EphemerisPoint{Valid: false}
	if point2.Valid {
		t.Error("Expected point to be invalid")
	}
}
