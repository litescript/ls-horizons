// Package astro provides astronomical coordinate transformations and sky math.
package astro

import (
	"math"
	"time"
)

// SkyCoord represents celestial coordinates with both equatorial (RA/Dec)
// and horizontal (Az/El) components.
type SkyCoord struct {
	// Equatorial coordinates (J2000)
	RAdeg  float64 // Right Ascension in degrees (0-360)
	DecDeg float64 // Declination in degrees (-90 to +90)

	// Horizontal coordinates (observer-relative)
	AzDeg float64 // Azimuth in degrees (0=N, 90=E, 180=S, 270=W)
	ElDeg float64 // Elevation/Altitude in degrees (0=horizon, 90=zenith)

	// Distance (optional, for spacecraft)
	RangeKm float64
}

// Observer represents a ground-based observer location.
type Observer struct {
	LatDeg float64 // Latitude in degrees (north positive)
	LonDeg float64 // Longitude in degrees (east positive)
	Name   string  // Optional name for the site
}

// EquatorialToHorizontal converts equatorial coordinates (RA/Dec) to horizontal
// coordinates (Az/El) for a given observer and time.
//
// The function preserves the input RA/Dec values and populates Az/El.
// Uses standard astronomical conventions:
//   - Azimuth: 0° = North, 90° = East, 180° = South, 270° = West
//   - Elevation: 0° = horizon, 90° = zenith
func EquatorialToHorizontal(eq SkyCoord, obs Observer, t time.Time) SkyCoord {
	// Convert to radians
	lat := degToRad(obs.LatDeg)
	ra := degToRad(eq.RAdeg)
	dec := degToRad(eq.DecDeg)

	// Calculate Local Sidereal Time
	lst := localSiderealTime(t, obs.LonDeg)
	lstRad := degToRad(lst)

	// Hour Angle = LST - RA
	ha := lstRad - ra

	// Calculate altitude (elevation)
	sinAlt := math.Sin(dec)*math.Sin(lat) + math.Cos(dec)*math.Cos(lat)*math.Cos(ha)
	alt := math.Asin(sinAlt)

	// Calculate azimuth
	cosAz := (math.Sin(dec) - math.Sin(alt)*math.Sin(lat)) / (math.Cos(alt) * math.Cos(lat))
	// Clamp cosAz to [-1, 1] to handle floating point errors
	if cosAz > 1 {
		cosAz = 1
	} else if cosAz < -1 {
		cosAz = -1
	}

	az := math.Acos(cosAz)

	// Adjust azimuth quadrant: if hour angle is positive, azimuth is west of south
	if math.Sin(ha) > 0 {
		az = 2*math.Pi - az
	}

	// Return new SkyCoord with all fields populated
	return SkyCoord{
		RAdeg:   eq.RAdeg,
		DecDeg:  eq.DecDeg,
		AzDeg:   radToDeg(az),
		ElDeg:   radToDeg(alt),
		RangeKm: eq.RangeKm,
	}
}

// localSiderealTime calculates the Local Sidereal Time in degrees
// for a given UTC time and observer longitude.
func localSiderealTime(t time.Time, lonDeg float64) float64 {
	gmst := greenwichMeanSiderealTime(t)
	lst := gmst + lonDeg

	// Normalize to 0-360
	for lst < 0 {
		lst += 360
	}
	for lst >= 360 {
		lst -= 360
	}

	return lst
}

// greenwichMeanSiderealTime calculates GMST in degrees for a given UTC time.
// Uses the IAU formula based on Julian Date.
func greenwichMeanSiderealTime(t time.Time) float64 {
	jd := julianDate(t)

	// Julian centuries since J2000.0
	T := (jd - 2451545.0) / 36525.0

	// GMST in degrees (IAU 1982 formula)
	// GMST = 280.46061837 + 360.98564736629*(JD-2451545) + 0.000387933*T^2 - T^3/38710000
	gmst := 280.46061837 +
		360.98564736629*(jd-2451545.0) +
		0.000387933*T*T -
		T*T*T/38710000.0

	// Normalize to 0-360
	gmst = math.Mod(gmst, 360)
	if gmst < 0 {
		gmst += 360
	}

	return gmst
}

// julianDate calculates the Julian Date for a given time.
func julianDate(t time.Time) float64 {
	// Convert to UTC
	t = t.UTC()

	y := float64(t.Year())
	m := float64(t.Month())
	d := float64(t.Day())

	// Time of day as fraction
	h := float64(t.Hour())
	min := float64(t.Minute())
	sec := float64(t.Second())
	ns := float64(t.Nanosecond())

	dayFrac := (h + min/60 + sec/3600 + ns/3600e9) / 24.0

	// Adjust for January/February (treat as months 13/14 of previous year)
	if m <= 2 {
		y--
		m += 12
	}

	// Gregorian calendar correction
	A := math.Floor(y / 100)
	B := 2 - A + math.Floor(A/4)

	// Julian Date formula
	jd := math.Floor(365.25*(y+4716)) +
		math.Floor(30.6001*(m+1)) +
		d + dayFrac + B - 1524.5

	return jd
}

// degToRad converts degrees to radians.
func degToRad(deg float64) float64 {
	return deg * math.Pi / 180
}

// radToDeg converts radians to degrees.
func radToDeg(rad float64) float64 {
	return rad * 180 / math.Pi
}
