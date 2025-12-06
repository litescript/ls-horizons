package dsn

import (
	"math"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

// Physical constants for Doppler calculations
const (
	// EarthRadius in km (mean equatorial radius)
	EarthRadius = 6378.137

	// EarthAngularVelocity in rad/s
	EarthAngularVelocity = 7.2921159e-5
)

// Note: SpeedOfLight is defined in derive.go

// Common DSN carrier frequencies in MHz
const (
	FreqSBand  = 2295.0  // S-band downlink (~2.3 GHz)
	FreqXBand  = 8420.0  // X-band downlink (~8.4 GHz)
	FreqKaBand = 32000.0 // Ka-band downlink (~32 GHz)
)

// StateVector represents spacecraft position and velocity in ECEF coordinates.
type StateVector struct {
	// Position in km (Earth-Centered Earth-Fixed)
	X, Y, Z float64

	// Velocity in km/s
	VX, VY, VZ float64

	// Time of state vector
	Time time.Time
}

// DopplerResult contains the computed Doppler shift information.
type DopplerResult struct {
	// LOSVelocity is the line-of-sight velocity in km/s
	// Positive = receding, Negative = approaching
	LOSVelocity float64

	// DopplerShift in Hz (for the given carrier frequency)
	DopplerShift float64

	// CarrierFreqMHz is the carrier frequency used
	CarrierFreqMHz float64

	// Range is the distance to spacecraft in km
	Range float64

	// Valid indicates if computation succeeded
	Valid bool
}

// ComputeDoppler calculates the expected Doppler shift for a spacecraft.
// Uses non-relativistic approximation: Δf = f₀ * v_los / c
func ComputeDoppler(obs astro.Observer, sv StateVector, carrierFreqMHz float64) DopplerResult {
	// Convert observer to ECEF position
	obsPos := observerToECEF(obs, sv.Time)

	// Compute observer velocity due to Earth rotation
	obsVel := observerVelocityECEF(obs)

	// Vector from observer to spacecraft
	dx := sv.X - obsPos[0]
	dy := sv.Y - obsPos[1]
	dz := sv.Z - obsPos[2]

	// Range (distance)
	r := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if r < 1 { // Too close or invalid
		return DopplerResult{Valid: false}
	}

	// Unit vector toward spacecraft
	ux := dx / r
	uy := dy / r
	uz := dz / r

	// Relative velocity (spacecraft velocity minus observer velocity)
	relVX := sv.VX - obsVel[0]
	relVY := sv.VY - obsVel[1]
	relVZ := sv.VZ - obsVel[2]

	// Line-of-sight velocity: projection of relative velocity onto unit vector
	losVel := relVX*ux + relVY*uy + relVZ*uz

	// Doppler shift (non-relativistic): Δf = f₀ * v_los / c
	// Convert carrier frequency to Hz
	carrierHz := carrierFreqMHz * 1e6
	dopplerShift := carrierHz * losVel / SpeedOfLight

	return DopplerResult{
		LOSVelocity:    losVel,
		DopplerShift:   dopplerShift,
		CarrierFreqMHz: carrierFreqMHz,
		Range:          r,
		Valid:          true,
	}
}

// ComputeDopplerFromRaDec computes Doppler shift using RA/Dec and range/velocity.
// This is useful when we have DSN-provided RA/Dec and derived velocity.
func ComputeDopplerFromRaDec(obs astro.Observer, raDeg, decDeg, rangeKm, rangeRateKmS float64, carrierFreqMHz float64) DopplerResult {
	// The range rate IS the line-of-sight velocity when RA/Dec is the pointing direction
	losVel := rangeRateKmS

	// Doppler shift (non-relativistic)
	carrierHz := carrierFreqMHz * 1e6
	dopplerShift := carrierHz * losVel / SpeedOfLight

	return DopplerResult{
		LOSVelocity:    losVel,
		DopplerShift:   dopplerShift,
		CarrierFreqMHz: carrierFreqMHz,
		Range:          rangeKm,
		Valid:          true,
	}
}

// observerToECEF converts observer geodetic coordinates to ECEF position.
func observerToECEF(obs astro.Observer, t time.Time) [3]float64 {
	// Convert to radians
	lat := obs.LatDeg * math.Pi / 180
	lon := obs.LonDeg * math.Pi / 180

	// Assume observer is at sea level (height = 0)
	h := 0.0

	// WGS84 ellipsoid parameters
	a := EarthRadius // equatorial radius
	f := 1 / 298.257223563
	e2 := 2*f - f*f

	// Prime vertical radius of curvature
	sinLat := math.Sin(lat)
	N := a / math.Sqrt(1-e2*sinLat*sinLat)

	// ECEF coordinates
	cosLat := math.Cos(lat)
	cosLon := math.Cos(lon)
	sinLon := math.Sin(lon)

	x := (N + h) * cosLat * cosLon
	y := (N + h) * cosLat * sinLon
	z := (N*(1-e2) + h) * sinLat

	return [3]float64{x, y, z}
}

// observerVelocityECEF computes observer velocity due to Earth rotation.
func observerVelocityECEF(obs astro.Observer) [3]float64 {
	// Convert to radians
	lat := obs.LatDeg * math.Pi / 180
	lon := obs.LonDeg * math.Pi / 180

	// Distance from Earth's rotation axis
	r := EarthRadius * math.Cos(lat)

	// Velocity magnitude at observer's latitude
	v := r * EarthAngularVelocity

	// Velocity direction is perpendicular to the radius vector in the equatorial plane
	// (tangent to the circle of latitude, toward east)
	// In ECEF, this is (-sin(lon), cos(lon), 0) scaled by v
	sinLon := math.Sin(lon)
	cosLon := math.Cos(lon)

	return [3]float64{
		-v * sinLon,
		v * cosLon,
		0,
	}
}

// FormatDopplerShift formats a Doppler shift for display.
func FormatDopplerShift(hz float64) string {
	absHz := math.Abs(hz)
	if absHz >= 1000 {
		return formatFloat(hz/1000, 2) + " kHz"
	}
	return formatFloat(hz, 1) + " Hz"
}

// formatFloat formats a float with specified decimal places.
func formatFloat(f float64, decimals int) string {
	format := "%." + string(rune('0'+decimals)) + "f"
	return formatF(format, f)
}

// formatF is a simple sprintf for floats.
func formatF(format string, f float64) string {
	switch format {
	case "%.1f":
		return fmtFloat(f, 1)
	case "%.2f":
		return fmtFloat(f, 2)
	default:
		return fmtFloat(f, 1)
	}
}

// fmtFloat formats a float with N decimal places without using fmt package.
func fmtFloat(f float64, decimals int) string {
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}

	// Scale and round
	scale := math.Pow10(decimals)
	rounded := math.Round(f * scale)

	intPart := int(rounded / scale)
	fracPart := int(rounded) - intPart*int(scale)

	// Build string
	result := sign + intToStr(intPart) + "."
	fracStr := intToStr(fracPart)

	// Pad with leading zeros
	for len(fracStr) < decimals {
		fracStr = "0" + fracStr
	}

	return result + fracStr
}

// intToStr converts an int to string without fmt.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}

	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// GetBandFrequency returns the typical downlink frequency for a band.
func GetBandFrequency(band string) float64 {
	switch band {
	case "S":
		return FreqSBand
	case "X":
		return FreqXBand
	case "Ka":
		return FreqKaBand
	default:
		return FreqXBand // Default to X-band
	}
}
