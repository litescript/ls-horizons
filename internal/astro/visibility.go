// Package astro provides astronomical coordinate transformations and sky math.
package astro

import (
	"errors"
	"math"
	"time"
)

// RADecAtTime represents an RA/Dec position at a specific time.
// Used for visibility calculations from Horizons ephemeris data.
type RADecAtTime struct {
	Time   time.Time
	RAdeg  float64
	DecDeg float64
}

// VisibilityWindow represents a rise-transit-set cycle for an object.
type VisibilityWindow struct {
	Rise          time.Time // Time object rises above horizon
	Transit       time.Time // Time object crosses meridian (highest point)
	Set           time.Time // Time object sets below horizon
	MaxElevation  float64   // Peak elevation in degrees
	Valid         bool      // Whether a valid window was found
	AlwaysVisible bool      // Object never sets (circumpolar)
	NeverVisible  bool      // Object never rises
}

// MinElevation is the threshold for considering an object "visible".
// We use a small positive value to account for atmospheric refraction.
const MinElevation = 0.0

// Errors for visibility calculations.
var (
	ErrInsufficientSamples = errors.New("insufficient samples for visibility calculation")
	ErrNoValidWindow       = errors.New("no valid visibility window found in time range")
)

// RiseSet computes rise, transit, and set times for an object given RA/Dec samples.
// The samples must be in chronological order and span sufficient time to capture
// a complete visibility cycle (typically 12-24 hours for most objects).
//
// The function uses linear interpolation between samples to find horizon crossings.
// For deep space objects with slowly-changing positions, this provides good accuracy.
func RiseSet(obs Observer, samples []RADecAtTime) (VisibilityWindow, error) {
	if len(samples) < 3 {
		return VisibilityWindow{}, ErrInsufficientSamples
	}

	// Convert all samples to Az/El
	type elSample struct {
		t     time.Time
		elDeg float64
	}
	elSamples := make([]elSample, len(samples))

	minEl := 90.0
	maxEl := -90.0
	maxElIdx := 0

	for i, s := range samples {
		coord := SkyCoord{RAdeg: s.RAdeg, DecDeg: s.DecDeg}
		horiz := EquatorialToHorizontal(coord, obs, s.Time)
		elSamples[i] = elSample{t: s.Time, elDeg: horiz.ElDeg}

		if horiz.ElDeg < minEl {
			minEl = horiz.ElDeg
		}
		if horiz.ElDeg > maxEl {
			maxEl = horiz.ElDeg
			maxElIdx = i
		}
	}

	// Check for circumpolar or never-visible objects
	if minEl > MinElevation {
		// Always visible - never sets
		return VisibilityWindow{
			Transit:       elSamples[maxElIdx].t,
			MaxElevation:  maxEl,
			Valid:         true,
			AlwaysVisible: true,
		}, nil
	}
	if maxEl < MinElevation {
		// Never visible - never rises
		return VisibilityWindow{
			Valid:        true,
			NeverVisible: true,
		}, nil
	}

	// Find rise time (first crossing from below to above horizon)
	var riseTime time.Time
	riseFound := false
	for i := 1; i < len(elSamples); i++ {
		prev := elSamples[i-1]
		curr := elSamples[i]

		if prev.elDeg <= MinElevation && curr.elDeg > MinElevation {
			// Interpolate to find crossing time
			riseTime = interpolateCrossing(prev.t, curr.t, prev.elDeg, curr.elDeg, MinElevation)
			riseFound = true
			break
		}
	}

	// Find set time (first crossing from above to below horizon after rise)
	var setTime time.Time
	setFound := false
	startIdx := 0
	if riseFound {
		// Start looking after rise
		for i, s := range elSamples {
			if !s.t.Before(riseTime) {
				startIdx = i
				break
			}
		}
	}

	for i := startIdx + 1; i < len(elSamples); i++ {
		prev := elSamples[i-1]
		curr := elSamples[i]

		if prev.elDeg > MinElevation && curr.elDeg <= MinElevation {
			setTime = interpolateCrossing(prev.t, curr.t, prev.elDeg, curr.elDeg, MinElevation)
			setFound = true
			break
		}
	}

	// If no rise found, try to find it after the start of samples
	// (object may already be up)
	if !riseFound && elSamples[0].elDeg > MinElevation {
		// Object is already visible - look for previous rise before sample window
		// In this case, we can still report transit and set
		riseTime = time.Time{} // Unknown rise time
	}

	// Find transit (maximum elevation) between rise and set
	transitTime := elSamples[maxElIdx].t
	transitEl := maxEl

	// Refine transit time if we have a window
	if riseFound || elSamples[0].elDeg > MinElevation {
		transitTime, transitEl = MaxElevation(obs, samples)
	}

	return VisibilityWindow{
		Rise:         riseTime,
		Transit:      transitTime,
		Set:          setTime,
		MaxElevation: transitEl,
		Valid:        riseFound || setFound || elSamples[0].elDeg > MinElevation,
	}, nil
}

// MaxElevation finds the time of maximum elevation for an object.
// Returns the transit time and elevation in degrees.
func MaxElevation(obs Observer, samples []RADecAtTime) (time.Time, float64) {
	if len(samples) == 0 {
		return time.Time{}, 0
	}

	maxEl := -90.0
	maxTime := samples[0].Time

	for _, s := range samples {
		coord := SkyCoord{RAdeg: s.RAdeg, DecDeg: s.DecDeg}
		horiz := EquatorialToHorizontal(coord, obs, s.Time)

		if horiz.ElDeg > maxEl {
			maxEl = horiz.ElDeg
			maxTime = s.Time
		}
	}

	// Refine using quadratic interpolation if we have enough samples
	if len(samples) >= 3 {
		maxTime, maxEl = refineMaxElevation(obs, samples, maxTime)
	}

	return maxTime, maxEl
}

// refineMaxElevation uses quadratic interpolation to refine the maximum elevation time.
func refineMaxElevation(obs Observer, samples []RADecAtTime, approxMax time.Time) (time.Time, float64) {
	// Find samples around the approximate maximum
	var prevSample, maxSample, nextSample *RADecAtTime
	maxIdx := -1

	for i, s := range samples {
		if !s.Time.Before(approxMax) {
			maxIdx = i
			break
		}
	}

	if maxIdx < 0 {
		maxIdx = len(samples) - 1
	}

	maxSample = &samples[maxIdx]
	if maxIdx > 0 {
		prevSample = &samples[maxIdx-1]
	}
	if maxIdx < len(samples)-1 {
		nextSample = &samples[maxIdx+1]
	}

	// If we don't have three samples, return the discrete maximum
	if prevSample == nil || nextSample == nil {
		coord := SkyCoord{RAdeg: maxSample.RAdeg, DecDeg: maxSample.DecDeg}
		horiz := EquatorialToHorizontal(coord, obs, maxSample.Time)
		return maxSample.Time, horiz.ElDeg
	}

	// Calculate elevations
	prevCoord := SkyCoord{RAdeg: prevSample.RAdeg, DecDeg: prevSample.DecDeg}
	maxCoord := SkyCoord{RAdeg: maxSample.RAdeg, DecDeg: maxSample.DecDeg}
	nextCoord := SkyCoord{RAdeg: nextSample.RAdeg, DecDeg: nextSample.DecDeg}

	prevHoriz := EquatorialToHorizontal(prevCoord, obs, prevSample.Time)
	maxHoriz := EquatorialToHorizontal(maxCoord, obs, maxSample.Time)
	nextHoriz := EquatorialToHorizontal(nextCoord, obs, nextSample.Time)

	// Use parabolic interpolation to find true maximum
	// Normalized time: t = -1 (prev), t = 0 (max), t = +1 (next)
	y0 := prevHoriz.ElDeg
	y1 := maxHoriz.ElDeg
	y2 := nextHoriz.ElDeg

	// Parabola: y = at^2 + bt + c
	// At t=-1: y0 = a - b + c
	// At t=0:  y1 = c
	// At t=1:  y2 = a + b + c
	c := y1
	a := (y0+y2)/2 - c
	b := (y2 - y0) / 2

	// Maximum at t = -b/(2a), but only if parabola opens downward (a < 0)
	if a >= 0 {
		return maxSample.Time, maxHoriz.ElDeg
	}

	tMax := -b / (2 * a)

	// Clamp to [-1, 1]
	if tMax < -1 {
		tMax = -1
	} else if tMax > 1 {
		tMax = 1
	}

	// Convert back to actual time
	dt := maxSample.Time.Sub(prevSample.Time)
	refinedTime := maxSample.Time.Add(time.Duration(float64(dt) * tMax))

	// Calculate elevation at refined time
	refinedEl := a*tMax*tMax + b*tMax + c

	return refinedTime, refinedEl
}

// interpolateCrossing finds the time when elevation crosses a threshold.
func interpolateCrossing(t1, t2 time.Time, el1, el2, threshold float64) time.Time {
	if math.Abs(el2-el1) < 0.0001 {
		return t1
	}

	// Linear interpolation: find t where el = threshold
	fraction := (threshold - el1) / (el2 - el1)

	// Clamp to valid range
	if fraction < 0 {
		fraction = 0
	} else if fraction > 1 {
		fraction = 1
	}

	dt := t2.Sub(t1)
	return t1.Add(time.Duration(float64(dt) * fraction))
}

// CurrentElevation computes the current elevation of an object at a given time.
func CurrentElevation(obs Observer, raDeg, decDeg float64, t time.Time) float64 {
	coord := SkyCoord{RAdeg: raDeg, DecDeg: decDeg}
	horiz := EquatorialToHorizontal(coord, obs, t)
	return horiz.ElDeg
}

// ElevationTier categorizes elevation for UI display.
type ElevationTier int

const (
	ElevationNone   ElevationTier = iota // Below horizon
	ElevationLow                         // 0-15 degrees
	ElevationMedium                      // 15-45 degrees
	ElevationHigh                        // 45+ degrees
)

// GetElevationTier returns the tier for a given elevation.
func GetElevationTier(elDeg float64) ElevationTier {
	switch {
	case elDeg <= 0:
		return ElevationNone
	case elDeg < 15:
		return ElevationLow
	case elDeg < 45:
		return ElevationMedium
	default:
		return ElevationHigh
	}
}
