package ephem

import (
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

func TestHorizonsProvider_GetPath_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := NewHorizonsProvider()

	// Goldstone observer
	obs := astro.Observer{
		LatDeg: 35.4267,
		LonDeg: -116.8900,
		Name:   "Goldstone",
	}

	// Test Voyager 1
	target := NAIFVoyager1
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	path, err := provider.GetPath(target, start, end, 10*time.Minute, obs)
	if err != nil {
		t.Fatalf("GetPath failed: %v", err)
	}

	if len(path.Points) == 0 {
		t.Error("Expected non-empty path")
	}

	t.Logf("Got %d points for Voyager 1", len(path.Points))
	for i, pt := range path.Points {
		if i > 3 {
			break
		}
		t.Logf("  %s: Az=%.2f El=%.2f Valid=%v",
			pt.Time.Format("15:04"), pt.Coord.AzDeg, pt.Coord.ElDeg, pt.Valid)
	}

	// Verify points have valid coordinates
	for _, pt := range path.Points {
		if !pt.Valid {
			continue
		}
		// Az should be 0-360
		if pt.Coord.AzDeg < 0 || pt.Coord.AzDeg >= 360 {
			t.Errorf("Invalid azimuth: %v", pt.Coord.AzDeg)
		}
		// El should be -90 to 90
		if pt.Coord.ElDeg < -90 || pt.Coord.ElDeg > 90 {
			t.Errorf("Invalid elevation: %v", pt.Coord.ElDeg)
		}
	}
}

func TestParseEphemerisLine(t *testing.T) {
	obs := astro.Observer{LatDeg: 35.0, LonDeg: -117.0}

	tests := []struct {
		line    string
		wantAz  float64
		wantEl  float64
		wantErr bool
	}{
		{
			line:   "2025-Dec-05 00:00 *   261.032124  32.878027",
			wantAz: 261.032124,
			wantEl: 32.878027,
		},
		{
			line:   "2025-Dec-05 01:00 Cm  270.255103  20.668754",
			wantAz: 270.255103,
			wantEl: 20.668754,
		},
		{
			line:   "2025-Dec-05 02:50  m  285.908122  -1.510301",
			wantAz: 285.908122,
			wantEl: -1.510301,
		},
		{
			line:    "invalid",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		name := tc.line
		if len(name) > 20 {
			name = name[:20]
		}
		t.Run(name, func(t *testing.T) {
			pt, err := parseEphemerisLine(tc.line, obs)
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if pt.Coord.AzDeg != tc.wantAz {
				t.Errorf("Az = %v, want %v", pt.Coord.AzDeg, tc.wantAz)
			}
			if pt.Coord.ElDeg != tc.wantEl {
				t.Errorf("El = %v, want %v", pt.Coord.ElDeg, tc.wantEl)
			}
		})
	}
}
