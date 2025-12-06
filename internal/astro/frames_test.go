package astro

import (
	"math"
	"testing"
)

func TestVec3Norm(t *testing.T) {
	tests := []struct {
		name string
		v    Vec3
		want float64
	}{
		{"zero", Vec3{0, 0, 0}, 0},
		{"unit x", Vec3{1, 0, 0}, 1},
		{"unit y", Vec3{0, 1, 0}, 1},
		{"unit z", Vec3{0, 0, 1}, 1},
		{"3-4-5", Vec3{3, 4, 0}, 5},
		{"negative", Vec3{-3, -4, 0}, 5},
		{"3D", Vec3{1, 2, 2}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Norm()
			if math.Abs(got-tt.want) > 1e-10 {
				t.Errorf("Norm() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVec3Normalized(t *testing.T) {
	tests := []struct {
		name string
		v    Vec3
		want Vec3
	}{
		{"unit x", Vec3{5, 0, 0}, Vec3{1, 0, 0}},
		{"unit y", Vec3{0, 3, 0}, Vec3{0, 1, 0}},
		{"diagonal", Vec3{1, 1, 0}, Vec3{1 / math.Sqrt(2), 1 / math.Sqrt(2), 0}},
		{"zero", Vec3{0, 0, 0}, Vec3{0, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Normalized()
			if math.Abs(got.X-tt.want.X) > 1e-10 ||
				math.Abs(got.Y-tt.want.Y) > 1e-10 ||
				math.Abs(got.Z-tt.want.Z) > 1e-10 {
				t.Errorf("Normalized() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectEclipticTopDown(t *testing.T) {
	cfg := DefaultProjectionConfig()

	tests := []struct {
		name      string
		v         Vec3
		wantAngle float64 // expected angle in degrees
		wantR     float64 // expected true distance
	}{
		{
			name:      "1 AU along +X",
			v:         Vec3{1, 0, 0},
			wantAngle: 0,
			wantR:     1,
		},
		{
			name:      "1 AU along +Y",
			v:         Vec3{0, 1, 0},
			wantAngle: 90,
			wantR:     1,
		},
		{
			name:      "1 AU along -X",
			v:         Vec3{-1, 0, 0},
			wantAngle: 180,
			wantR:     1,
		},
		{
			name:      "1 AU along -Y",
			v:         Vec3{0, -1, 0},
			wantAngle: -90, // or 270
			wantR:     1,
		},
		{
			name:      "5 AU at 45 degrees",
			v:         Vec3{5 / math.Sqrt(2), 5 / math.Sqrt(2), 0},
			wantAngle: 45,
			wantR:     5,
		},
		{
			name:      "10 AU with Z offset",
			v:         Vec3{10, 0, 2},
			wantAngle: 0,
			wantR:     math.Sqrt(104), // sqrt(100 + 4)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectEclipticTopDown(tt.v, cfg)

			// Check angle
			gotAngle := math.Atan2(got.Y, got.X) * 180 / math.Pi
			angleDiff := math.Abs(gotAngle - tt.wantAngle)
			// Handle wrap-around at ±180
			if angleDiff > 180 {
				angleDiff = 360 - angleDiff
			}
			if angleDiff > 0.1 {
				t.Errorf("angle = %.2f°, want %.2f°", gotAngle, tt.wantAngle)
			}

			// Check distance
			if math.Abs(got.R-tt.wantR) > 0.01 {
				t.Errorf("R = %.4f, want %.4f", got.R, tt.wantR)
			}
		})
	}
}

func TestScaleModes(t *testing.T) {
	tests := []struct {
		name string
		mode ScaleMode
		rAU  float64
	}{
		{"log 1AU", ScaleLogR, 1},
		{"log 5AU", ScaleLogR, 5},
		{"log 10AU", ScaleLogR, 10},
		{"log 20AU", ScaleLogR, 20},
		{"inner 1AU", ScaleInner, 1},
		{"inner 5AU", ScaleInner, 5},
		{"inner 10AU", ScaleInner, 10}, // should clamp
		{"outer 1AU", ScaleOuter, 1},
		{"outer 5AU", ScaleOuter, 5},
		{"outer 20AU", ScaleOuter, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ProjectionConfig{Scale: 1.0, Mode: tt.mode}
			v := Vec3{tt.rAU, 0, 0}
			got := ProjectEclipticTopDown(v, cfg)

			// Verify projection is in correct direction
			if got.X < 0 {
				t.Errorf("X should be positive for +X input, got %v", got.X)
			}
			if math.Abs(got.Y) > 1e-10 {
				t.Errorf("Y should be ~0 for X-axis input, got %v", got.Y)
			}

			// Verify scaled radius is reasonable (positive, bounded)
			rDisplay := math.Sqrt(got.X*got.X + got.Y*got.Y)
			if rDisplay < 0 {
				t.Errorf("scaled radius should be non-negative, got %v", rDisplay)
			}

			// Inner mode should clamp at 5 AU
			if tt.mode == ScaleInner && tt.rAU > 5 && rDisplay > 5.01 {
				t.Errorf("ScaleInner should clamp at 5, got %v for r=%v AU", rDisplay, tt.rAU)
			}
		})
	}
}

func TestKmToAU(t *testing.T) {
	tests := []struct {
		km     float64
		wantAU float64
		tolPct float64 // tolerance as percentage
	}{
		{AU, 1.0, 0.001},           // 1 AU in km = 1 AU
		{AU * 5.2, 5.2, 0.001},     // Jupiter distance
		{AU * 30.07, 30.07, 0.001}, // Neptune distance
		{24e9, 24e9 / AU, 0.001},   // ~160 AU (Voyager range)
	}

	for _, tt := range tests {
		got := KmToAU(tt.km)
		diff := math.Abs(got-tt.wantAU) / tt.wantAU
		if diff > tt.tolPct/100 {
			t.Errorf("KmToAU(%.0f) = %.4f, want %.4f", tt.km, got, tt.wantAU)
		}
	}
}

func TestEquatorialToEcliptic(t *testing.T) {
	// A vector along the equatorial Z-axis (north celestial pole)
	// should tilt toward positive ecliptic Y and negative ecliptic Z
	// by the obliquity angle (~23.4°)
	northPole := Vec3{0, 0, 1}
	ecl := EquatorialToEcliptic(northPole)

	// Expected: X unchanged, Y = sin(23.4°), Z = cos(23.4°)
	expectedY := math.Sin(obliquityRad)
	expectedZ := math.Cos(obliquityRad)

	if math.Abs(ecl.X) > 1e-10 {
		t.Errorf("X should be 0, got %v", ecl.X)
	}
	if math.Abs(ecl.Y-expectedY) > 1e-6 {
		t.Errorf("Y = %v, want %v", ecl.Y, expectedY)
	}
	if math.Abs(ecl.Z-expectedZ) > 1e-6 {
		t.Errorf("Z = %v, want %v", ecl.Z, expectedZ)
	}
}

func TestEclipticToEquatorial(t *testing.T) {
	// Roundtrip test
	original := Vec3{1, 2, 3}
	ecl := EquatorialToEcliptic(original)
	back := EclipticToEquatorial(ecl)

	if math.Abs(back.X-original.X) > 1e-10 ||
		math.Abs(back.Y-original.Y) > 1e-10 ||
		math.Abs(back.Z-original.Z) > 1e-10 {
		t.Errorf("Roundtrip failed: %v -> %v -> %v", original, ecl, back)
	}
}

func TestLightTimeFromAU(t *testing.T) {
	tests := []struct {
		au       float64
		wantSecs float64
		tolSecs  float64
	}{
		{1, 499.005, 0.1},        // 1 AU = ~8.3 minutes
		{0, 0, 0.1},              // 0 AU
		{5.2, 5.2 * 499.005, 1},  // Jupiter
		{160, 160 * 499.005, 10}, // Voyager
	}

	for _, tt := range tests {
		got := LightTimeFromAU(tt.au)
		if math.Abs(got-tt.wantSecs) > tt.tolSecs {
			t.Errorf("LightTimeFromAU(%.1f) = %.1f, want %.1f", tt.au, got, tt.wantSecs)
		}
	}
}

func TestFormatLightTime(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{30, "30.0s"},
		{60, "1m0s"},
		{90, "1m30s"},
		{3600, "1h0m"},
		{3660, "1h1m"},
		{7200, "2h0m"},
		{86400, "24h0m"}, // 1 day
	}

	for _, tt := range tests {
		got := FormatLightTime(tt.seconds)
		if got != tt.want {
			t.Errorf("FormatLightTime(%.0f) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestEclipticLatitude(t *testing.T) {
	tests := []struct {
		v       Vec3
		wantDeg float64
		tol     float64
	}{
		{Vec3{1, 0, 0}, 0, 0.01},
		{Vec3{0, 1, 0}, 0, 0.01},
		{Vec3{0, 0, 1}, 90, 0.01},
		{Vec3{0, 0, -1}, -90, 0.01},
		{Vec3{1, 0, 1}, 45, 0.01},
		{Vec3{1, 1, 0}, 0, 0.01},
	}

	for _, tt := range tests {
		got := EclipticLatitude(tt.v)
		if math.Abs(got-tt.wantDeg) > tt.tol {
			t.Errorf("EclipticLatitude(%v) = %.2f°, want %.2f°", tt.v, got, tt.wantDeg)
		}
	}
}

func TestEclipticLongitude(t *testing.T) {
	tests := []struct {
		v       Vec3
		wantDeg float64
		tol     float64
	}{
		{Vec3{1, 0, 0}, 0, 0.01},
		{Vec3{0, 1, 0}, 90, 0.01},
		{Vec3{-1, 0, 0}, 180, 0.01},
		{Vec3{0, -1, 0}, 270, 0.01},
		{Vec3{1, 1, 0}, 45, 0.01},
	}

	for _, tt := range tests {
		got := EclipticLongitude(tt.v)
		if math.Abs(got-tt.wantDeg) > tt.tol {
			t.Errorf("EclipticLongitude(%v) = %.2f°, want %.2f°", tt.v, got, tt.wantDeg)
		}
	}
}

func TestProjectStarEclipticTopDown(t *testing.T) {
	cfg := ProjectionConfig{
		Scale:             1.0,
		Mode:              ScaleLogR,
		StarShellRadiusAU: 100.0,
	}

	tests := []struct {
		name       string
		raDeg      float64
		decDeg     float64
		wantXSign  int // -1, 0, +1 for expected sign
		wantYSign  int
		wantRShell bool // expect R close to shell radius
	}{
		{
			name:       "RA=0, Dec=0 (vernal equinox)",
			raDeg:      0,
			decDeg:     0,
			wantXSign:  +1, // Should project to positive X
			wantYSign:  0,  // Near zero (equator on ecliptic plane)
			wantRShell: true,
		},
		{
			name:       "RA=90°, Dec=0 (summer solstice direction)",
			raDeg:      90,
			decDeg:     0,
			wantXSign:  0,  // Near zero
			wantYSign:  +1, // Should project to positive Y
			wantRShell: true,
		},
		{
			name:       "RA=180°, Dec=0 (autumnal equinox)",
			raDeg:      180,
			decDeg:     0,
			wantXSign:  -1, // Should project to negative X
			wantYSign:  0,
			wantRShell: true,
		},
		{
			name:       "RA=270°, Dec=0 (winter solstice direction)",
			raDeg:      270,
			decDeg:     0,
			wantXSign:  0,
			wantYSign:  -1, // Should project to negative Y
			wantRShell: true,
		},
		{
			name:       "RA=0, Dec=+45 (northern)",
			raDeg:      0,
			decDeg:     45,
			wantXSign:  +1, // Still mostly positive X
			wantYSign:  0,  // Some Y due to obliquity tilt
			wantRShell: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectStarEclipticTopDown(tt.raDeg, tt.decDeg, cfg)

			// Check X sign
			if tt.wantXSign != 0 {
				if (tt.wantXSign > 0 && got.X <= 0) || (tt.wantXSign < 0 && got.X >= 0) {
					t.Errorf("X = %.4f, expected sign %+d", got.X, tt.wantXSign)
				}
			}

			// Check Y sign
			if tt.wantYSign != 0 {
				if (tt.wantYSign > 0 && got.Y <= 0) || (tt.wantYSign < 0 && got.Y >= 0) {
					t.Errorf("Y = %.4f, expected sign %+d", got.Y, tt.wantYSign)
				}
			}

			// Check R is consistent with shell radius
			if tt.wantRShell {
				// R should be shell radius (100 AU)
				if math.Abs(got.R-100.0) > 1.0 {
					t.Errorf("R = %.4f, want ~100.0 (shell radius)", got.R)
				}
			}
		})
	}
}

func TestProjectStarEclipticTopDown_OppositeStars(t *testing.T) {
	cfg := ProjectionConfig{
		Scale:             1.0,
		Mode:              ScaleLogR,
		StarShellRadiusAU: 100.0,
	}

	// Two stars at opposite RA positions should be on opposite sides
	star1 := ProjectStarEclipticTopDown(0, 0, cfg)   // RA=0
	star2 := ProjectStarEclipticTopDown(180, 0, cfg) // RA=180

	// X should have opposite signs
	if star1.X*star2.X >= 0 {
		t.Errorf("stars at RA=0 and RA=180 should have opposite X: got %.4f and %.4f", star1.X, star2.X)
	}

	// Both should have similar R (same shell)
	if math.Abs(star1.R-star2.R) > 0.1 {
		t.Errorf("stars should have same R: got %.4f and %.4f", star1.R, star2.R)
	}
}

func TestProjectStarEclipticTopDown_DefaultShellRadius(t *testing.T) {
	// With StarShellRadiusAU = 0, should default to 100 AU
	cfg := ProjectionConfig{
		Scale:             1.0,
		Mode:              ScaleLogR,
		StarShellRadiusAU: 0, // Should default to 100
	}

	got := ProjectStarEclipticTopDown(0, 0, cfg)

	// R should be close to default (100 AU)
	if math.Abs(got.R-DefaultStarShellRadiusAU) > 1.0 {
		t.Errorf("R = %.4f with zero StarShellRadiusAU, expected ~%.1f", got.R, DefaultStarShellRadiusAU)
	}
}

func TestProjectStarEclipticTopDown_CustomShellRadius(t *testing.T) {
	// Test with custom shell radius
	cfg := ProjectionConfig{
		Scale:             1.0,
		Mode:              ScaleLogR,
		StarShellRadiusAU: 50.0, // Custom radius
	}

	got := ProjectStarEclipticTopDown(0, 0, cfg)

	// R should be close to custom radius
	if math.Abs(got.R-50.0) > 0.5 {
		t.Errorf("R = %.4f with StarShellRadiusAU=50, expected ~50.0", got.R)
	}
}
