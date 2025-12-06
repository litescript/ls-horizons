// Package astro provides astronomical coordinate transformations and sky math.
package astro

import (
	"math"
	"time"
)

// SunPosition calculates the apparent equatorial coordinates of the Sun.
// Uses a simplified solar ephemeris based on the Astronomical Almanac.
// Accuracy: ~0.01 degrees for RA, ~0.001 degrees for Dec (sufficient for separation angle).
func SunPosition(t time.Time) (raDeg, decDeg float64) {
	// Julian Date
	jd := julianDate(t)

	// Julian centuries from J2000.0
	T := (jd - 2451545.0) / 36525.0

	// Mean longitude of the Sun (degrees)
	L0 := 280.46646 + 36000.76983*T + 0.0003032*T*T
	L0 = normalizeAngle360(L0)

	// Mean anomaly of the Sun (degrees)
	M := 357.52911 + 35999.05029*T - 0.0001537*T*T
	M = normalizeAngle360(M)
	Mrad := degToRad(M)

	// Sun's equation of center (degrees)
	C := (1.914602 - 0.004817*T - 0.000014*T*T) * math.Sin(Mrad)
	C += (0.019993 - 0.000101*T) * math.Sin(2*Mrad)
	C += 0.000289 * math.Sin(3*Mrad)

	// Sun's true longitude (degrees)
	sunLon := L0 + C

	// Sun's true anomaly (degrees)
	// v := M + C

	// Sun's radius vector (AU) - not used but included for completeness
	// R := (1.000001018 * (1 - e*e)) / (1 + e*math.Cos(degToRad(v)))

	// Apparent longitude (correcting for aberration and nutation)
	omega := 125.04 - 1934.136*T
	sunLonApp := sunLon - 0.00569 - 0.00478*math.Sin(degToRad(omega))

	// Mean obliquity of the ecliptic (degrees)
	eps0 := 23.439291 - 0.0130042*T - 0.00000016*T*T + 0.000000504*T*T*T

	// Corrected obliquity
	eps := eps0 + 0.00256*math.Cos(degToRad(omega))

	// Convert to equatorial coordinates
	sunLonRad := degToRad(sunLonApp)
	epsRad := degToRad(eps)

	// Right Ascension
	ra := math.Atan2(math.Cos(epsRad)*math.Sin(sunLonRad), math.Cos(sunLonRad))
	raDeg = radToDeg(ra)
	if raDeg < 0 {
		raDeg += 360
	}

	// Declination
	dec := math.Asin(math.Sin(epsRad) * math.Sin(sunLonRad))
	decDeg = radToDeg(dec)

	return raDeg, decDeg
}

// SunSeparation calculates the angular separation between the Sun and a target.
// Returns the separation angle in degrees.
func SunSeparation(targetRA, targetDec float64, t time.Time) float64 {
	sunRA, sunDec := SunPosition(t)
	return AngularSeparation(sunRA, sunDec, targetRA, targetDec)
}

// AngularSeparation calculates the angular separation between two points on the celestial sphere.
// All coordinates in degrees. Returns separation in degrees.
func AngularSeparation(ra1, dec1, ra2, dec2 float64) float64 {
	// Convert to radians
	ra1Rad := degToRad(ra1)
	dec1Rad := degToRad(dec1)
	ra2Rad := degToRad(ra2)
	dec2Rad := degToRad(dec2)

	// Haversine formula for angular separation
	dRA := ra2Rad - ra1Rad
	dDec := dec2Rad - dec1Rad

	a := math.Sin(dDec/2)*math.Sin(dDec/2) +
		math.Cos(dec1Rad)*math.Cos(dec2Rad)*math.Sin(dRA/2)*math.Sin(dRA/2)

	// Clamp to avoid numerical errors with asin
	if a > 1 {
		a = 1
	}

	c := 2 * math.Asin(math.Sqrt(a))

	return radToDeg(c)
}

// SunSeparationTier categorizes sun separation for display.
type SunSeparationTier int

const (
	SunSepSafe    SunSeparationTier = iota // >= 20 degrees
	SunSepCaution                          // 10-20 degrees
	SunSepWarning                          // < 10 degrees
)

// GetSunSeparationTier returns the tier for a given separation angle.
func GetSunSeparationTier(sepDeg float64) SunSeparationTier {
	switch {
	case sepDeg < 10:
		return SunSepWarning
	case sepDeg < 20:
		return SunSepCaution
	default:
		return SunSepSafe
	}
}

// normalizeAngle360 normalizes an angle to 0-360 degrees.
func normalizeAngle360(a float64) float64 {
	a = math.Mod(a, 360)
	if a < 0 {
		a += 360
	}
	return a
}
