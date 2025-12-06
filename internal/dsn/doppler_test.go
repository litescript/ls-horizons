package dsn

import (
	"math"
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

func TestComputeDoppler(t *testing.T) {
	// Test observer at Goldstone
	obs := astro.Observer{
		LatDeg: 35.4267,
		LonDeg: -116.8900,
		Name:   "Goldstone",
	}

	now := time.Now()

	tests := []struct {
		name           string
		stateVector    StateVector
		carrierFreqMHz float64
		wantValid      bool
		wantLOSMin     float64 // km/s
		wantLOSMax     float64
	}{
		{
			name: "Stationary spacecraft at geostationary distance - approaching",
			stateVector: StateVector{
				// Spacecraft directly above (simplified - in reality would be at ~36000 km)
				X: 0, Y: 0, Z: 42164, // Geostationary altitude above north pole
				VX: 0, VY: 0, VZ: -1.0, // Moving toward Earth at 1 km/s
				Time: now,
			},
			carrierFreqMHz: FreqXBand,
			wantValid:      true,
			wantLOSMin:     -2, // Approaching (negative)
			wantLOSMax:     0,
		},
		{
			name: "Spacecraft receding",
			stateVector: StateVector{
				X: 0, Y: 0, Z: 42164,
				VX: 0, VY: 0, VZ: 1.0, // Moving away from Earth at 1 km/s
				Time: now,
			},
			carrierFreqMHz: FreqXBand,
			wantValid:      true,
			wantLOSMin:     0, // Receding (positive)
			wantLOSMax:     2,
		},
		{
			name: "Spacecraft at Earth center - still valid (far from observer)",
			stateVector: StateVector{
				X: 0, Y: 0, Z: 0, // At Earth center - ~6378 km from observer
				VX: 0, VY: 0, VZ: 0,
				Time: now,
			},
			carrierFreqMHz: FreqXBand,
			wantValid:      true, // Distance is Earth radius, which is > 1 km
			wantLOSMin:     -1,
			wantLOSMax:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeDoppler(obs, tt.stateVector, tt.carrierFreqMHz)

			if result.Valid != tt.wantValid {
				t.Errorf("ComputeDoppler().Valid = %v, want %v", result.Valid, tt.wantValid)
				return
			}

			if !tt.wantValid {
				return
			}

			if result.LOSVelocity < tt.wantLOSMin || result.LOSVelocity > tt.wantLOSMax {
				t.Errorf("ComputeDoppler().LOSVelocity = %.4f km/s, want between %.4f and %.4f",
					result.LOSVelocity, tt.wantLOSMin, tt.wantLOSMax)
			}

			// Verify carrier frequency is set
			if result.CarrierFreqMHz != tt.carrierFreqMHz {
				t.Errorf("ComputeDoppler().CarrierFreqMHz = %.1f, want %.1f",
					result.CarrierFreqMHz, tt.carrierFreqMHz)
			}
		})
	}
}

func TestComputeDopplerFromRaDec(t *testing.T) {
	obs := astro.Observer{
		LatDeg: 35.4267,
		LonDeg: -116.8900,
		Name:   "Goldstone",
	}

	tests := []struct {
		name           string
		raDeg          float64
		decDeg         float64
		rangeKm        float64
		rangeRateKmS   float64
		carrierFreqMHz float64
		wantDopplerMin float64 // Hz
		wantDopplerMax float64
	}{
		{
			name:           "Voyager 1 approaching at typical rate",
			raDeg:          260.0, // Approximate
			decDeg:         12.0,  // Approximate
			rangeKm:        24e9,  // ~24 billion km
			rangeRateKmS:   -17.0, // Approaching at ~17 km/s (typical for Voyager)
			carrierFreqMHz: FreqXBand,
			// Expected Doppler: 8420e6 * (-17) / 299792 ≈ -477 kHz
			wantDopplerMin: -500000, // -500 kHz
			wantDopplerMax: -450000, // -450 kHz
		},
		{
			name:           "Near-Earth spacecraft receding slowly",
			raDeg:          45.0,
			decDeg:         0.0,
			rangeKm:        400000, // Moon distance
			rangeRateKmS:   0.5,    // Receding at 0.5 km/s
			carrierFreqMHz: FreqXBand,
			// Expected Doppler: 8420e6 * 0.5 / 299792 ≈ 14 kHz
			wantDopplerMin: 10000, // 10 kHz
			wantDopplerMax: 20000, // 20 kHz
		},
		{
			name:           "Zero range rate - no Doppler",
			raDeg:          0.0,
			decDeg:         0.0,
			rangeKm:        100000,
			rangeRateKmS:   0.0,
			carrierFreqMHz: FreqSBand,
			wantDopplerMin: -1,
			wantDopplerMax: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeDopplerFromRaDec(obs, tt.raDeg, tt.decDeg, tt.rangeKm, tt.rangeRateKmS, tt.carrierFreqMHz)

			if !result.Valid {
				t.Error("ComputeDopplerFromRaDec() returned invalid result")
				return
			}

			if result.DopplerShift < tt.wantDopplerMin || result.DopplerShift > tt.wantDopplerMax {
				t.Errorf("ComputeDopplerFromRaDec().DopplerShift = %.1f Hz, want between %.1f and %.1f",
					result.DopplerShift, tt.wantDopplerMin, tt.wantDopplerMax)
			}
		})
	}
}

func TestObserverToECEF(t *testing.T) {
	// Test that observer position is on Earth's surface
	obs := astro.Observer{
		LatDeg: 35.4267,
		LonDeg: -116.8900,
		Name:   "Goldstone",
	}

	now := time.Now()
	pos := observerToECEF(obs, now)

	// Distance from center should be approximately Earth radius
	dist := math.Sqrt(pos[0]*pos[0] + pos[1]*pos[1] + pos[2]*pos[2])

	// Allow for Earth's oblateness (radius varies from ~6357 to ~6378 km)
	if dist < 6350 || dist > 6400 {
		t.Errorf("Observer distance from Earth center = %.1f km, expected ~6378 km", dist)
	}
}

func TestObserverVelocityECEF(t *testing.T) {
	// Test that observer velocity magnitude is correct for Earth rotation
	obs := astro.Observer{
		LatDeg: 0, // Equator - maximum rotational velocity
		LonDeg: 0,
		Name:   "Equator",
	}

	vel := observerVelocityECEF(obs)

	// Magnitude of velocity at equator should be ~0.465 km/s
	speed := math.Sqrt(vel[0]*vel[0] + vel[1]*vel[1] + vel[2]*vel[2])

	// Allow 5% tolerance
	expected := EarthRadius * EarthAngularVelocity // ~0.465 km/s
	if math.Abs(speed-expected)/expected > 0.05 {
		t.Errorf("Equatorial velocity = %.4f km/s, expected %.4f km/s", speed, expected)
	}

	// At poles, velocity should be near zero
	poleObs := astro.Observer{
		LatDeg: 90,
		LonDeg: 0,
		Name:   "North Pole",
	}
	poleVel := observerVelocityECEF(poleObs)
	poleSpeed := math.Sqrt(poleVel[0]*poleVel[0] + poleVel[1]*poleVel[1] + poleVel[2]*poleVel[2])

	if poleSpeed > 0.01 { // Should be essentially zero
		t.Errorf("Polar velocity = %.4f km/s, expected ~0", poleSpeed)
	}
}

func TestFormatDopplerShift(t *testing.T) {
	tests := []struct {
		hz   float64
		want string
	}{
		{0, "0.0 Hz"},
		{100, "100.0 Hz"},
		{-500, "-500.0 Hz"},
		{1000, "1.00 kHz"},
		{-12500, "-12.50 kHz"},
		{100000, "100.00 kHz"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatDopplerShift(tt.hz)
			if got != tt.want {
				t.Errorf("FormatDopplerShift(%.1f) = %q, want %q", tt.hz, got, tt.want)
			}
		})
	}
}

func TestGetBandFrequency(t *testing.T) {
	tests := []struct {
		band string
		want float64
	}{
		{"S", FreqSBand},
		{"X", FreqXBand},
		{"Ka", FreqKaBand},
		{"unknown", FreqXBand}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.band, func(t *testing.T) {
			got := GetBandFrequency(tt.band)
			if got != tt.want {
				t.Errorf("GetBandFrequency(%q) = %.1f, want %.1f", tt.band, got, tt.want)
			}
		})
	}
}
