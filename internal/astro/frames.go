// Package astro provides astronomical coordinate transformations and sky math.
package astro

import (
	"math"
)

// AU is the Astronomical Unit in kilometers.
const AU = 149597870.7

// Vec3 represents a 3D vector in any reference frame.
type Vec3 struct {
	X, Y, Z float64
}

// Norm returns the magnitude of the vector.
func (v Vec3) Norm() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

// Normalized returns a unit vector in the same direction.
func (v Vec3) Normalized() Vec3 {
	n := v.Norm()
	if n == 0 {
		return Vec3{}
	}
	return Vec3{X: v.X / n, Y: v.Y / n, Z: v.Z / n}
}

// Scale returns the vector scaled by a factor.
func (v Vec3) Scale(s float64) Vec3 {
	return Vec3{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

// Add returns the sum of two vectors.
func (v Vec3) Add(u Vec3) Vec3 {
	return Vec3{X: v.X + u.X, Y: v.Y + u.Y, Z: v.Z + u.Z}
}

// Sub returns the difference of two vectors.
func (v Vec3) Sub(u Vec3) Vec3 {
	return Vec3{X: v.X - u.X, Y: v.Y - u.Y, Z: v.Z - u.Z}
}

// ProjectedPoint represents a 2D projected position with metadata.
type ProjectedPoint struct {
	X float64 // Screen X coordinate (normalized, -1 to 1)
	Y float64 // Screen Y coordinate (normalized, -1 to 1)
	R float64 // Original radial distance in AU
	Z float64 // Original Z offset (for ecliptic latitude display)
}

// ScaleMode defines how radial distances are mapped to screen space.
type ScaleMode int

const (
	// ScaleLogR uses logarithmic scaling: r_display = log10(r_AU + 1) * scale
	ScaleLogR ScaleMode = iota

	// ScaleInner uses linear scaling optimized for 0-5 AU
	ScaleInner

	// ScaleOuter uses compressed scaling for outer solar system (>5 AU)
	ScaleOuter
)

// ProjectionConfig configures the top-down ecliptic projection.
type ProjectionConfig struct {
	Scale float64   // Base scale factor
	Mode  ScaleMode // Scaling mode
}

// DefaultProjectionConfig returns a reasonable default configuration.
func DefaultProjectionConfig() ProjectionConfig {
	return ProjectionConfig{
		Scale: 1.0,
		Mode:  ScaleLogR,
	}
}

// ProjectEclipticTopDown projects a 3D ecliptic vector to 2D screen coordinates.
// The projection is a top-down view with X pointing right (toward vernal equinox)
// and Y pointing up (toward summer solstice direction).
// Z is perpendicular to the ecliptic plane.
func ProjectEclipticTopDown(v Vec3, cfg ProjectionConfig) ProjectedPoint {
	// Compute radial distance in ecliptic plane
	rAU := math.Sqrt(v.X*v.X + v.Y*v.Y)

	// Apply scaling based on mode
	rDisplay := scaleRadius(rAU, cfg)

	// Compute angle in ecliptic plane
	angle := math.Atan2(v.Y, v.X)

	// Project to screen coordinates
	// X points right, Y points up
	return ProjectedPoint{
		X: rDisplay * math.Cos(angle) * cfg.Scale,
		Y: rDisplay * math.Sin(angle) * cfg.Scale,
		R: math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z), // True 3D distance
		Z: v.Z,
	}
}

// scaleRadius applies the configured scaling mode to a radial distance.
func scaleRadius(rAU float64, cfg ProjectionConfig) float64 {
	switch cfg.Mode {
	case ScaleLogR:
		// Logarithmic scaling: good for showing both inner and outer system
		// log10(r + 1) gives 0 at origin, ~0.78 at 5 AU, ~1.04 at 10 AU, ~1.32 at 20 AU
		return math.Log10(rAU + 1)

	case ScaleInner:
		// Linear scaling for inner solar system (0-5 AU)
		// Clamp outer planets to edge
		if rAU > 5 {
			return 5
		}
		return rAU

	case ScaleOuter:
		// Piece-wise scaling: linear to 5 AU, then logarithmic beyond
		if rAU <= 5 {
			return rAU / 5 * 0.5 // Inner planets get half the space
		}
		// Outer: log scale from 5 AU onward
		return 0.5 + math.Log10(rAU/5+1)*0.5

	default:
		return math.Log10(rAU + 1)
	}
}

// KmToAU converts kilometers to Astronomical Units.
func KmToAU(km float64) float64 {
	return km / AU
}

// AUToKm converts Astronomical Units to kilometers.
func AUToKm(au float64) float64 {
	return au * AU
}

// EclipticLatitude returns the ecliptic latitude in degrees for a vector.
func EclipticLatitude(v Vec3) float64 {
	r := v.Norm()
	if r == 0 {
		return 0
	}
	return radToDeg(math.Asin(v.Z / r))
}

// EclipticLongitude returns the ecliptic longitude in degrees for a vector.
func EclipticLongitude(v Vec3) float64 {
	lon := radToDeg(math.Atan2(v.Y, v.X))
	if lon < 0 {
		lon += 360
	}
	return lon
}

// Obliquity is the Earth's axial tilt (J2000 epoch) in radians.
const obliquityRad = 23.439291 * math.Pi / 180

// EquatorialToEcliptic converts equatorial XYZ to ecliptic XYZ.
// Input is in any units (km, AU, etc); output is in the same units.
func EquatorialToEcliptic(eq Vec3) Vec3 {
	// Rotation matrix around X-axis by obliquity
	cosE := math.Cos(obliquityRad)
	sinE := math.Sin(obliquityRad)

	return Vec3{
		X: eq.X,
		Y: eq.Y*cosE + eq.Z*sinE,
		Z: -eq.Y*sinE + eq.Z*cosE,
	}
}

// EclipticToEquatorial converts ecliptic XYZ to equatorial XYZ.
func EclipticToEquatorial(ecl Vec3) Vec3 {
	// Rotation matrix around X-axis by -obliquity
	cosE := math.Cos(obliquityRad)
	sinE := math.Sin(obliquityRad)

	return Vec3{
		X: ecl.X,
		Y: ecl.Y*cosE - ecl.Z*sinE,
		Z: ecl.Y*sinE + ecl.Z*cosE,
	}
}

// LightTimeFromAU returns the one-way light time for a distance in AU.
func LightTimeFromAU(au float64) float64 {
	// Light travels 1 AU in ~499.005 seconds
	return au * 499.005
}

// FormatLightTime formats light time in seconds to a human-readable string.
func FormatLightTime(seconds float64) string {
	if seconds < 60 {
		return formatSeconds(seconds)
	}
	if seconds < 3600 {
		mins := int(seconds / 60)
		secs := int(seconds) % 60
		return formatMinSec(mins, secs)
	}
	hours := int(seconds / 3600)
	mins := (int(seconds) % 3600) / 60
	return formatHrMin(hours, mins)
}

func formatSeconds(s float64) string {
	return fmtFloat(s, 1) + "s"
}

func formatMinSec(m, s int) string {
	return intToString(m) + "m" + intToString(s) + "s"
}

func formatHrMin(h, m int) string {
	return intToString(h) + "h" + intToString(m) + "m"
}

func intToString(n int) string {
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

func fmtFloat(f float64, decimals int) string {
	scale := math.Pow10(decimals)
	rounded := math.Round(f * scale)
	intPart := int(rounded / scale)
	fracPart := int(rounded) % int(scale)
	if fracPart < 0 {
		fracPart = -fracPart
	}

	result := intToString(intPart) + "."
	fracStr := intToString(fracPart)
	for len(fracStr) < decimals {
		fracStr = "0" + fracStr
	}
	return result + fracStr
}
