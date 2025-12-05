package astro

import (
	"math"
	"testing"
	"time"
)

func TestJulianDate(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected float64
		tol      float64
	}{
		{
			name:     "J2000 epoch",
			time:     time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: 2451545.0,
			tol:      0.0001,
		},
		{
			name:     "Unix epoch",
			time:     time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: 2440587.5,
			tol:      0.0001,
		},
		{
			name:     "Known date 2024-01-01 00:00 UTC",
			time:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: 2460310.5,
			tol:      0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := julianDate(tt.time)
			if math.Abs(got-tt.expected) > tt.tol {
				t.Errorf("julianDate() = %v, want %v (±%v)", got, tt.expected, tt.tol)
			}
		})
	}
}

func TestGreenwichMeanSiderealTime(t *testing.T) {
	// At J2000 epoch (2000-01-01 12:00 UTC), GMST should be approximately 280.46°
	t2000 := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	gmst := greenwichMeanSiderealTime(t2000)

	// GMST at J2000 should be very close to 280.46°
	if math.Abs(gmst-280.46) > 0.1 {
		t.Errorf("GMST at J2000 = %v, want ~280.46", gmst)
	}

	// GMST should be in range 0-360
	if gmst < 0 || gmst >= 360 {
		t.Errorf("GMST out of range: %v", gmst)
	}
}

func TestLocalSiderealTime(t *testing.T) {
	// LST = GMST + longitude
	testTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// At longitude 0 (Greenwich), LST should equal GMST
	gmst := greenwichMeanSiderealTime(testTime)
	lst0 := localSiderealTime(testTime, 0)
	if math.Abs(lst0-gmst) > 0.001 {
		t.Errorf("LST at lon=0 should equal GMST: got %v, want %v", lst0, gmst)
	}

	// At longitude +90° (east), LST should be GMST + 90°
	lst90 := localSiderealTime(testTime, 90)
	expected90 := math.Mod(gmst+90, 360)
	if math.Abs(lst90-expected90) > 0.001 {
		t.Errorf("LST at lon=90 = %v, want %v", lst90, expected90)
	}

	// LST should always be in 0-360 range
	for lon := -180.0; lon <= 180; lon += 30 {
		lst := localSiderealTime(testTime, lon)
		if lst < 0 || lst >= 360 {
			t.Errorf("LST at lon=%v out of range: %v", lon, lst)
		}
	}
}

func TestEquatorialToHorizontal_Polaris(t *testing.T) {
	// Polaris is approximately at RA=37.95°, Dec=89.26° (very close to NCP)
	// From northern latitudes, it should always be visible (El > 0)
	// and approximately at Az=0° (due north) with El ≈ latitude

	polaris := SkyCoord{
		RAdeg:  37.95,
		DecDeg: 89.26,
	}

	// Observer at 35°N (roughly Goldstone latitude)
	observer := Observer{
		LatDeg: 35.0,
		LonDeg: -117.0, // west longitude
	}

	testTime := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	result := EquatorialToHorizontal(polaris, observer, testTime)

	// Polaris elevation should be approximately equal to observer latitude (±5°)
	expectedEl := observer.LatDeg
	if math.Abs(result.ElDeg-expectedEl) > 5 {
		t.Errorf("Polaris elevation = %v°, expected ~%v° (latitude)", result.ElDeg, expectedEl)
	}

	// Polaris should always be visible from northern hemisphere
	if result.ElDeg < 0 {
		t.Errorf("Polaris should be visible from 35°N, got El=%v°", result.ElDeg)
	}

	// Original RA/Dec should be preserved
	if result.RAdeg != polaris.RAdeg || result.DecDeg != polaris.DecDeg {
		t.Error("RA/Dec should be preserved after transformation")
	}
}

func TestEquatorialToHorizontal_ZenithStar(t *testing.T) {
	// A star at the zenith has Dec = observer latitude and HA = 0
	// This means RA = LST at that moment

	observer := Observer{
		LatDeg: 35.0,
		LonDeg: -117.0,
	}

	testTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	lst := localSiderealTime(testTime, observer.LonDeg)

	// Star at zenith: Dec = lat, RA = LST
	zenithStar := SkyCoord{
		RAdeg:  lst,
		DecDeg: observer.LatDeg,
	}

	result := EquatorialToHorizontal(zenithStar, observer, testTime)

	// Elevation should be ~90° (zenith)
	if math.Abs(result.ElDeg-90) > 1 {
		t.Errorf("Zenith star elevation = %v°, expected ~90°", result.ElDeg)
	}
}

func TestEquatorialToHorizontal_SouthernStar(t *testing.T) {
	// A star at Dec = -60° should not be visible from 35°N
	// (it's always below the horizon)

	southernStar := SkyCoord{
		RAdeg:  0,
		DecDeg: -60,
	}

	observer := Observer{
		LatDeg: 35.0,
		LonDeg: -117.0,
	}

	// Test at multiple times
	for hour := 0; hour < 24; hour += 6 {
		testTime := time.Date(2024, 6, 15, hour, 0, 0, 0, time.UTC)
		result := EquatorialToHorizontal(southernStar, observer, testTime)

		// Star at -60° should never rise above horizon from 35°N
		// Max elevation = 90 - lat + dec = 90 - 35 + (-60) = -5°
		if result.ElDeg > 0 {
			t.Errorf("Star at Dec=-60° visible from 35°N at hour %d: El=%v°", hour, result.ElDeg)
		}
	}
}

func TestEquatorialToHorizontal_PreservesRange(t *testing.T) {
	star := SkyCoord{
		RAdeg:   100,
		DecDeg:  20,
		RangeKm: 1.5e8, // ~1 AU
	}

	observer := Observer{LatDeg: 35, LonDeg: -117}
	testTime := time.Now()

	result := EquatorialToHorizontal(star, observer, testTime)

	if result.RangeKm != star.RangeKm {
		t.Errorf("RangeKm not preserved: got %v, want %v", result.RangeKm, star.RangeKm)
	}
}

func TestDegToRad(t *testing.T) {
	tests := []struct {
		deg float64
		rad float64
	}{
		{0, 0},
		{90, math.Pi / 2},
		{180, math.Pi},
		{360, 2 * math.Pi},
		{-90, -math.Pi / 2},
	}

	for _, tt := range tests {
		got := degToRad(tt.deg)
		if math.Abs(got-tt.rad) > 1e-10 {
			t.Errorf("degToRad(%v) = %v, want %v", tt.deg, got, tt.rad)
		}
	}
}

func TestRadToDeg(t *testing.T) {
	tests := []struct {
		rad float64
		deg float64
	}{
		{0, 0},
		{math.Pi / 2, 90},
		{math.Pi, 180},
		{2 * math.Pi, 360},
	}

	for _, tt := range tests {
		got := radToDeg(tt.rad)
		if math.Abs(got-tt.deg) > 1e-10 {
			t.Errorf("radToDeg(%v) = %v, want %v", tt.rad, got, tt.deg)
		}
	}
}

func TestEquatorialToHorizontal_AzimuthRange(t *testing.T) {
	// Test that azimuth is always in 0-360 range
	observer := Observer{LatDeg: 35, LonDeg: -117}

	for ra := 0.0; ra < 360; ra += 30 {
		for dec := -80.0; dec <= 80; dec += 20 {
			star := SkyCoord{RAdeg: ra, DecDeg: dec}
			testTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
			result := EquatorialToHorizontal(star, observer, testTime)

			if result.AzDeg < 0 || result.AzDeg >= 360 {
				t.Errorf("Azimuth out of range for RA=%v, Dec=%v: Az=%v",
					ra, dec, result.AzDeg)
			}
		}
	}
}
