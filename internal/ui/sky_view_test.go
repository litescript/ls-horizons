package ui

import (
	"math"
	"testing"
)

func TestNormalizeAngle(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0, 0},
		{180, 180},
		{-180, -180},
		{360, 0},
		{-360, 0},
		{350, -10},   // wraps to -10
		{370, 10},    // wraps to 10
		{-190, 170},  // wraps to 170
		{540, 180},   // multiple wraps
		{-540, -180}, // multiple wraps
	}

	for _, tt := range tests {
		got := normalizeAngle(tt.input)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("normalizeAngle(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestLerpAngle_ShortestPath(t *testing.T) {
	tests := []struct {
		from     float64
		to       float64
		t        float64
		expected float64
	}{
		// Simple cases
		{0, 90, 0.5, 45},
		{0, 180, 0.5, 90},

		// Wrap-around: 350 to 10 should go +20, not -340
		{350, 10, 0.5, 360}, // halfway is 360 (or 0)
		{350, 10, 0.0, 350},
		{350, 10, 1.0, 370}, // ends at 370, normalizes to 10

		// Other direction: 10 to 350 should go -20
		{10, 350, 0.5, 0},
		{10, 350, 1.0, -10}, // ends at -10, normalizes to 350
	}

	for _, tt := range tests {
		got := lerpAngle(tt.from, tt.to, tt.t)
		// Normalize both for comparison
		gotNorm := normalizeAngle(got)
		expNorm := normalizeAngle(tt.expected)

		// Handle the -180/180 edge case
		diff := math.Abs(gotNorm - expNorm)
		if diff > 180 {
			diff = 360 - diff
		}

		if diff > 0.001 {
			t.Errorf("lerpAngle(%v, %v, %v) = %v (norm: %v), want %v (norm: %v)",
				tt.from, tt.to, tt.t, got, gotNorm, tt.expected, expNorm)
		}
	}
}

func TestProjectToScreen(t *testing.T) {
	m := SkyViewModel{
		camAz: 180,
		camEl: 45,
	}

	width := 100
	height := 50

	tests := []struct {
		az, el  float64
		visible bool
		desc    string
	}{
		{180, 45, true, "center of view"},
		{180, 70, true, "high elevation within FOV"},
		{180, 20, true, "low elevation within FOV"},
		{180, 90, false, "above FOV (camEl=45, fov=60)"},
		{180, 0, false, "below FOV"},
		{0, 45, false, "opposite side (180 away)"},
		{240, 45, true, "within FOV right"},
		{120, 45, true, "within FOV left"},
		{300, 45, false, "outside FOV"},
	}

	for _, tt := range tests {
		_, _, visible := m.projectToScreen(tt.az, tt.el, width, height)
		if visible != tt.visible {
			t.Errorf("projectToScreen(%v, %v) visible = %v, want %v (%s)",
				tt.az, tt.el, visible, tt.visible, tt.desc)
		}
	}
}

func TestProjectToScreen_CenterIsCenter(t *testing.T) {
	m := SkyViewModel{
		camAz: 180,
		camEl: 30,
	}

	width := 100
	height := 50

	// Object at camera center should be near screen center
	x, y, visible := m.projectToScreen(180, 30, width, height)

	if !visible {
		t.Fatal("center object should be visible")
	}

	// Should be roughly center horizontally
	if x < 40 || x > 60 {
		t.Errorf("center x = %d, expected near 50", x)
	}

	// Y depends on FOV mapping, but should be somewhere in middle region
	if y < 10 || y > 40 {
		t.Errorf("center y = %d, expected in middle region", y)
	}
}
