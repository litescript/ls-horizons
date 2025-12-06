package astro

import (
	"math"
	"testing"
	"time"
)

func TestSunPosition(t *testing.T) {
	tests := []struct {
		name      string
		time      time.Time
		wantRAMin float64 // RA in degrees
		wantRAMax float64
		wantDecMin float64 // Dec in degrees
		wantDecMax float64
	}{
		{
			name:      "Spring Equinox 2024 - Sun near 0h RA, 0° Dec",
			time:      time.Date(2024, 3, 20, 12, 0, 0, 0, time.UTC),
			wantRAMin: 359, // Near 0h (can be 359-1)
			wantRAMax: 2,
			wantDecMin: -1,
			wantDecMax: 1,
		},
		{
			name:      "Summer Solstice 2024 - Sun near 6h RA, +23.5° Dec",
			time:      time.Date(2024, 6, 21, 12, 0, 0, 0, time.UTC),
			wantRAMin: 88, // 6h = 90°
			wantRAMax: 92,
			wantDecMin: 23,
			wantDecMax: 24,
		},
		{
			name:      "Autumn Equinox 2024 - Sun near 12h RA, 0° Dec",
			time:      time.Date(2024, 9, 22, 12, 0, 0, 0, time.UTC),
			wantRAMin: 178, // 12h = 180°
			wantRAMax: 182,
			wantDecMin: -1,
			wantDecMax: 1,
		},
		{
			name:      "Winter Solstice 2024 - Sun near 18h RA, -23.5° Dec",
			time:      time.Date(2024, 12, 21, 12, 0, 0, 0, time.UTC),
			wantRAMin: 268, // 18h = 270°
			wantRAMax: 272,
			wantDecMin: -24,
			wantDecMax: -23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRA, gotDec := SunPosition(tt.time)

			// Handle RA wrap-around for spring equinox
			raOK := false
			if tt.wantRAMin > tt.wantRAMax {
				// Wrap-around case (e.g., 359-2)
				raOK = gotRA >= tt.wantRAMin || gotRA <= tt.wantRAMax
			} else {
				raOK = gotRA >= tt.wantRAMin && gotRA <= tt.wantRAMax
			}

			if !raOK {
				t.Errorf("SunPosition() RA = %.2f°, want between %.2f° and %.2f°",
					gotRA, tt.wantRAMin, tt.wantRAMax)
			}

			if gotDec < tt.wantDecMin || gotDec > tt.wantDecMax {
				t.Errorf("SunPosition() Dec = %.2f°, want between %.2f° and %.2f°",
					gotDec, tt.wantDecMin, tt.wantDecMax)
			}
		})
	}
}

func TestAngularSeparation(t *testing.T) {
	tests := []struct {
		name      string
		ra1, dec1 float64
		ra2, dec2 float64
		wantSep   float64
		tol       float64
	}{
		{
			name:    "Same point",
			ra1:     100, dec1: 30,
			ra2:     100, dec2: 30,
			wantSep: 0,
			tol:     0.001,
		},
		{
			name:    "90 degrees apart on equator",
			ra1:     0, dec1: 0,
			ra2:     90, dec2: 0,
			wantSep: 90,
			tol:     0.001,
		},
		{
			name:    "180 degrees apart on equator",
			ra1:     0, dec1: 0,
			ra2:     180, dec2: 0,
			wantSep: 180,
			tol:     0.001,
		},
		{
			name:    "Pole to equator",
			ra1:     0, dec1: 90,   // North pole
			ra2:     0, dec2: 0,    // On equator
			wantSep: 90,
			tol:     0.001,
		},
		{
			name:    "Pole to pole",
			ra1:     0, dec1: 90,   // North pole
			ra2:     0, dec2: -90,  // South pole
			wantSep: 180,
			tol:     0.001,
		},
		{
			name:    "Small separation",
			ra1:     100, dec1: 30,
			ra2:     101, dec2: 30,
			wantSep: 0.866, // cos(30°) ≈ 0.866
			tol:     0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AngularSeparation(tt.ra1, tt.dec1, tt.ra2, tt.dec2)
			if math.Abs(got-tt.wantSep) > tt.tol {
				t.Errorf("AngularSeparation() = %.4f°, want %.4f° (±%.4f)",
					got, tt.wantSep, tt.tol)
			}
		})
	}
}

func TestSunSeparation(t *testing.T) {
	// Test sun separation for a target near the sun during summer solstice
	// Sun is at ~90° RA, ~+23.5° Dec during summer solstice
	summerSolstice := time.Date(2024, 6, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		targetRA  float64
		targetDec float64
		wantMin   float64
		wantMax   float64
	}{
		{
			name:      "Target at sun position",
			targetRA:  90, // Near summer solstice sun position
			targetDec: 23.5,
			wantMin:   0,
			wantMax:   3, // Allow small tolerance for solar motion
		},
		{
			name:      "Target opposite sun",
			targetRA:  270, // 180° from sun
			targetDec: -23.5,
			wantMin:   175,
			wantMax:   180,
		},
		{
			name:      "Target 90° RA from sun",
			targetRA:  180,
			targetDec: 23.5,
			wantMin:   75, // RA difference is ~90°, but great circle distance depends on declination
			wantMax:   95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SunSeparation(tt.targetRA, tt.targetDec, summerSolstice)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SunSeparation() = %.2f°, want between %.2f° and %.2f°",
					got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestGetSunSeparationTier(t *testing.T) {
	tests := []struct {
		sepDeg float64
		want   SunSeparationTier
	}{
		{5, SunSepWarning},
		{9.9, SunSepWarning},
		{10, SunSepCaution},
		{15, SunSepCaution},
		{19.9, SunSepCaution},
		{20, SunSepSafe},
		{45, SunSepSafe},
		{90, SunSepSafe},
		{180, SunSepSafe},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GetSunSeparationTier(tt.sepDeg)
			if got != tt.want {
				t.Errorf("GetSunSeparationTier(%.1f) = %v, want %v", tt.sepDeg, got, tt.want)
			}
		})
	}
}
