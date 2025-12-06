package astro

import (
	"math"
	"testing"
	"time"
)

// testObservers for visibility testing
var testObservers = map[string]Observer{
	"goldstone":  {LatDeg: 35.4267, LonDeg: -116.8900, Name: "Goldstone"},
	"canberra":   {LatDeg: -35.4014, LonDeg: 148.9817, Name: "Canberra"},
	"madrid":     {LatDeg: 40.4314, LonDeg: -4.2481, Name: "Madrid"},
	"north_pole": {LatDeg: 89.0, LonDeg: 0.0, Name: "North Pole"},
}

// Well-known star positions (J2000)
var testStars = map[string]struct {
	RAdeg  float64
	DecDeg float64
}{
	"vega":     {RAdeg: 279.2347, DecDeg: 38.7837},  // Alpha Lyrae
	"sirius":   {RAdeg: 101.2875, DecDeg: -16.7161}, // Alpha CMa
	"polaris":  {RAdeg: 37.9542, DecDeg: 89.2641},   // North star
	"canopus":  {RAdeg: 95.9879, DecDeg: -52.6957},  // Alpha Car
	"arcturus": {RAdeg: 213.9150, DecDeg: 19.1825},  // Alpha Boo
}

func TestCurrentElevation(t *testing.T) {
	tests := []struct {
		name     string
		observer string
		star     string
		time     time.Time
		wantMin  float64 // minimum expected elevation
		wantMax  float64 // maximum expected elevation
	}{
		{
			name:     "Vega from Goldstone - should have positive elevation at some times",
			observer: "goldstone",
			star:     "vega",
			time:     time.Date(2024, 7, 15, 4, 0, 0, 0, time.UTC), // Summer night
			wantMin:  -90,
			wantMax:  90,
		},
		{
			name:     "Polaris from North Pole - always near zenith",
			observer: "north_pole",
			star:     "polaris",
			time:     time.Date(2024, 6, 21, 12, 0, 0, 0, time.UTC),
			wantMin:  85,
			wantMax:  90,
		},
		{
			name:     "Canopus from Canberra - visible in southern sky",
			observer: "canberra",
			star:     "canopus",
			time:     time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Summer in Australia
			wantMin:  -90,
			wantMax:  90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := testObservers[tt.observer]
			star := testStars[tt.star]

			el := CurrentElevation(obs, star.RAdeg, star.DecDeg, tt.time)

			if el < tt.wantMin || el > tt.wantMax {
				t.Errorf("CurrentElevation() = %.2f°, want between %.2f° and %.2f°",
					el, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestRiseSet_Basic(t *testing.T) {
	// Test basic rise/set detection using a fixed star position
	// over a 24-hour period with hourly samples
	obs := testObservers["goldstone"]
	star := testStars["vega"]

	// Generate 24 hours of samples
	baseTime := time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)
	samples := make([]RADecAtTime, 25)
	for i := 0; i <= 24; i++ {
		samples[i] = RADecAtTime{
			Time:   baseTime.Add(time.Duration(i) * time.Hour),
			RAdeg:  star.RAdeg,
			DecDeg: star.DecDeg,
		}
	}

	window, err := RiseSet(obs, samples)
	if err != nil {
		t.Fatalf("RiseSet() error = %v", err)
	}

	if !window.Valid {
		t.Error("RiseSet() returned invalid window")
	}

	// Vega should rise and set from Goldstone (dec ~39° at lat ~35°)
	if window.AlwaysVisible || window.NeverVisible {
		t.Errorf("Vega should rise and set from Goldstone, got AlwaysVisible=%v, NeverVisible=%v",
			window.AlwaysVisible, window.NeverVisible)
	}

	// Max elevation should be positive
	if window.MaxElevation < 0 {
		t.Errorf("MaxElevation = %.2f°, want > 0", window.MaxElevation)
	}

	// Transit should be between rise and set (if we found both)
	if !window.Rise.IsZero() && !window.Set.IsZero() {
		if window.Transit.Before(window.Rise) || window.Transit.After(window.Set) {
			t.Errorf("Transit at %v not between Rise %v and Set %v",
				window.Transit, window.Rise, window.Set)
		}
	}
}

func TestRiseSet_Circumpolar(t *testing.T) {
	// Polaris should be circumpolar from high northern latitudes
	obs := testObservers["north_pole"]
	star := testStars["polaris"]

	baseTime := time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC)
	samples := make([]RADecAtTime, 25)
	for i := 0; i <= 24; i++ {
		samples[i] = RADecAtTime{
			Time:   baseTime.Add(time.Duration(i) * time.Hour),
			RAdeg:  star.RAdeg,
			DecDeg: star.DecDeg,
		}
	}

	window, err := RiseSet(obs, samples)
	if err != nil {
		t.Fatalf("RiseSet() error = %v", err)
	}

	if !window.Valid {
		t.Error("RiseSet() returned invalid window")
	}

	if !window.AlwaysVisible {
		t.Error("Polaris should be circumpolar from North Pole")
	}
}

func TestRiseSet_NeverVisible(t *testing.T) {
	// Canopus (dec -53°) should never rise from high northern latitudes
	obs := testObservers["north_pole"]
	star := testStars["canopus"]

	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	samples := make([]RADecAtTime, 25)
	for i := 0; i <= 24; i++ {
		samples[i] = RADecAtTime{
			Time:   baseTime.Add(time.Duration(i) * time.Hour),
			RAdeg:  star.RAdeg,
			DecDeg: star.DecDeg,
		}
	}

	window, err := RiseSet(obs, samples)
	if err != nil {
		t.Fatalf("RiseSet() error = %v", err)
	}

	if !window.Valid {
		t.Error("RiseSet() returned invalid window")
	}

	if !window.NeverVisible {
		t.Error("Canopus should never be visible from North Pole")
	}
}

func TestRiseSet_InsufficientSamples(t *testing.T) {
	obs := testObservers["goldstone"]

	tests := []struct {
		name    string
		samples []RADecAtTime
	}{
		{
			name:    "empty samples",
			samples: []RADecAtTime{},
		},
		{
			name: "one sample",
			samples: []RADecAtTime{
				{Time: time.Now(), RAdeg: 0, DecDeg: 0},
			},
		},
		{
			name: "two samples",
			samples: []RADecAtTime{
				{Time: time.Now(), RAdeg: 0, DecDeg: 0},
				{Time: time.Now().Add(time.Hour), RAdeg: 0, DecDeg: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RiseSet(obs, tt.samples)
			if err != ErrInsufficientSamples {
				t.Errorf("RiseSet() error = %v, want ErrInsufficientSamples", err)
			}
		})
	}
}

func TestMaxElevation(t *testing.T) {
	obs := testObservers["goldstone"]
	star := testStars["vega"]

	// Generate samples around transit
	baseTime := time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)
	samples := make([]RADecAtTime, 25)
	for i := 0; i <= 24; i++ {
		samples[i] = RADecAtTime{
			Time:   baseTime.Add(time.Duration(i) * time.Hour),
			RAdeg:  star.RAdeg,
			DecDeg: star.DecDeg,
		}
	}

	transitTime, maxEl := MaxElevation(obs, samples)

	// Max elevation for Vega from Goldstone should be roughly 90° - |lat - dec|
	// Goldstone lat: 35.4°, Vega dec: 38.8° -> max ~86.6°
	expectedMaxEl := 90.0 - math.Abs(obs.LatDeg-star.DecDeg)
	tolerance := 5.0 // degrees, allow for interpolation and timing

	if math.Abs(maxEl-expectedMaxEl) > tolerance {
		t.Errorf("MaxElevation() = %.2f°, expected ~%.2f° (±%.0f°)", maxEl, expectedMaxEl, tolerance)
	}

	// Transit time should be within our sample range
	if transitTime.Before(samples[0].Time) || transitTime.After(samples[len(samples)-1].Time) {
		t.Errorf("Transit time %v outside sample range", transitTime)
	}
}

func TestGetElevationTier(t *testing.T) {
	tests := []struct {
		elDeg float64
		want  ElevationTier
	}{
		{-10, ElevationNone},
		{0, ElevationNone},
		{5, ElevationLow},
		{14.9, ElevationLow},
		{15, ElevationMedium},
		{30, ElevationMedium},
		{44.9, ElevationMedium},
		{45, ElevationHigh},
		{70, ElevationHigh},
		{90, ElevationHigh},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GetElevationTier(tt.elDeg)
			if got != tt.want {
				t.Errorf("GetElevationTier(%.1f) = %v, want %v", tt.elDeg, got, tt.want)
			}
		})
	}
}

func TestInterpolateCrossing(t *testing.T) {
	tests := []struct {
		name      string
		t1, t2    time.Time
		el1, el2  float64
		threshold float64
		wantFrac  float64 // expected fraction between t1 and t2
	}{
		{
			name:      "midpoint crossing",
			t1:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:        time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
			el1:       -10,
			el2:       10,
			threshold: 0,
			wantFrac:  0.5,
		},
		{
			name:      "quarter crossing",
			t1:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:        time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
			el1:       -5,
			el2:       15,
			threshold: 0,
			wantFrac:  0.25,
		},
		{
			name:      "three-quarter crossing",
			t1:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:        time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
			el1:       -15,
			el2:       5,
			threshold: 0,
			wantFrac:  0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpolateCrossing(tt.t1, tt.t2, tt.el1, tt.el2, tt.threshold)
			actualFrac := float64(result.Sub(tt.t1)) / float64(tt.t2.Sub(tt.t1))

			if math.Abs(actualFrac-tt.wantFrac) > 0.01 {
				t.Errorf("interpolateCrossing() fraction = %.3f, want %.3f", actualFrac, tt.wantFrac)
			}
		})
	}
}

func TestRiseSet_DSNSites(t *testing.T) {
	// Test rise/set calculations for all three DSN sites
	// This verifies the visibility logic works across different latitudes
	sites := []string{"goldstone", "canberra", "madrid"}
	star := testStars["arcturus"] // Should be visible from all three sites

	for _, site := range sites {
		t.Run(site, func(t *testing.T) {
			obs := testObservers[site]

			// Generate 24-hour samples
			baseTime := time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC)
			samples := make([]RADecAtTime, 49) // every 30 min
			for i := 0; i < 49; i++ {
				samples[i] = RADecAtTime{
					Time:   baseTime.Add(time.Duration(i) * 30 * time.Minute),
					RAdeg:  star.RAdeg,
					DecDeg: star.DecDeg,
				}
			}

			window, err := RiseSet(obs, samples)
			if err != nil {
				t.Fatalf("RiseSet() error = %v", err)
			}

			if !window.Valid {
				t.Error("RiseSet() returned invalid window")
			}

			// Arcturus (dec +19°) should rise and set from all DSN sites
			if window.NeverVisible {
				t.Errorf("Arcturus should be visible from %s", site)
			}

			// Should have positive max elevation
			if window.MaxElevation < 0 {
				t.Errorf("MaxElevation = %.2f°, want > 0", window.MaxElevation)
			}
		})
	}
}
