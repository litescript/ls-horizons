package dsn

import (
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

// ElevationSample represents a single elevation measurement at a point in time.
type ElevationSample struct {
	Time      time.Time
	Elevation float64 // degrees above horizon
}

// ElevationTrace contains elevation samples over a time window.
type ElevationTrace struct {
	SpacecraftCode string
	Complex        Complex
	Samples        []ElevationSample
	GeneratedAt    time.Time
	WindowStart    time.Time
	WindowEnd      time.Time
}

// ElevationTraceWindow is the time span for elevation traces (±2 hours from now).
const ElevationTraceWindow = 2 * time.Hour

// ElevationTraceSampleInterval is the time between samples.
const ElevationTraceSampleInterval = 5 * time.Minute

// ComputeElevationTrace computes elevation samples for a spacecraft as seen from
// a DSN complex over a ±2 hour window centered on 'now'.
// The samples slice contains RA/Dec positions from the ephemeris provider.
func ComputeElevationTrace(
	scCode string,
	complex Complex,
	samples []astro.RADecAtTime,
	now time.Time,
) *ElevationTrace {
	if len(samples) == 0 {
		return &ElevationTrace{
			SpacecraftCode: scCode,
			Complex:        complex,
			GeneratedAt:    now,
			Samples:        nil,
		}
	}

	obs := ObserverForComplex(complex)
	windowStart := now.Add(-ElevationTraceWindow)
	windowEnd := now.Add(ElevationTraceWindow)

	var elevSamples []ElevationSample

	for _, s := range samples {
		// Only include samples within our window
		if s.Time.Before(windowStart) || s.Time.After(windowEnd) {
			continue
		}

		// Convert RA/Dec to horizontal coordinates for this observer
		coord := astro.SkyCoord{RAdeg: s.RAdeg, DecDeg: s.DecDeg}
		horiz := astro.EquatorialToHorizontal(coord, obs, s.Time)

		elevSamples = append(elevSamples, ElevationSample{
			Time:      s.Time,
			Elevation: horiz.ElDeg,
		})
	}

	return &ElevationTrace{
		SpacecraftCode: scCode,
		Complex:        complex,
		Samples:        elevSamples,
		GeneratedAt:    now,
		WindowStart:    windowStart,
		WindowEnd:      windowEnd,
	}
}

// CurrentElevation returns the elevation sample closest to the given time,
// or nil if no samples exist.
func (t *ElevationTrace) CurrentElevation(now time.Time) *ElevationSample {
	if len(t.Samples) == 0 {
		return nil
	}

	// Find the sample closest to 'now'
	var closest *ElevationSample
	var minDelta time.Duration = 1<<63 - 1 // max duration

	for i := range t.Samples {
		delta := t.Samples[i].Time.Sub(now)
		if delta < 0 {
			delta = -delta
		}
		if delta < minDelta {
			minDelta = delta
			closest = &t.Samples[i]
		}
	}

	return closest
}
