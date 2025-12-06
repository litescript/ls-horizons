package dsn

import (
	"sort"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

// PassStatus classifies a pass relative to current time.
type PassStatus int

const (
	PassPast   PassStatus = iota // Pass has ended
	PassNow                      // Currently in progress
	PassNext                     // Next upcoming pass
	PassFuture                   // Future pass (not next)
)

// String returns the status name.
func (s PassStatus) String() string {
	switch s {
	case PassPast:
		return "PAST"
	case PassNow:
		return "NOW"
	case PassNext:
		return "NEXT"
	case PassFuture:
		return "FUTURE"
	default:
		return "?"
	}
}

// Pass represents a visibility pass over a DSN complex.
type Pass struct {
	Complex   Complex
	Start     time.Time
	Peak      time.Time
	End       time.Time
	MaxElDeg  float64
	SunMinSep float64
	Status    PassStatus
}

// PassPlan contains all passes for a spacecraft across all complexes.
type PassPlan struct {
	SpacecraftCode string
	GeneratedAt    time.Time
	WindowStart    time.Time
	WindowEnd      time.Time
	Passes         []Pass
}

// MinPassElevation is the threshold for pass start/end (degrees).
const MinPassElevation = 5.0

// PassSampleInterval is the time between elevation samples.
const PassSampleInterval = 5 * time.Minute

// PassWindowDuration is the default forecast window.
const PassWindowDuration = 24 * time.Hour

// ComputePassPlan computes passes for a spacecraft over the given time window.
// Takes pre-computed RA/Dec samples (from ephem.Provider.GetPath or similar).
func ComputePassPlan(
	scCode string,
	samples []astro.RADecAtTime,
	now time.Time,
) *PassPlan {
	if len(samples) < 3 {
		// Not enough data - return empty plan
		return &PassPlan{
			SpacecraftCode: scCode,
			GeneratedAt:    now,
			Passes:         nil,
		}
	}

	windowStart := samples[0].Time
	windowEnd := samples[len(samples)-1].Time

	// Compute passes for each complex
	var allPasses []Pass
	complexes := []Complex{ComplexGoldstone, ComplexCanberra, ComplexMadrid}

	for _, c := range complexes {
		passes := computePassesForComplex(c, samples, now)
		allPasses = append(allPasses, passes...)
	}

	// Sort passes by start time
	sort.Slice(allPasses, func(i, j int) bool {
		return allPasses[i].Start.Before(allPasses[j].Start)
	})

	// Classify passes: Past, Now, Next, Future
	classifyPasses(allPasses, now)

	return &PassPlan{
		SpacecraftCode: scCode,
		GeneratedAt:    now,
		WindowStart:    windowStart,
		WindowEnd:      windowEnd,
		Passes:         allPasses,
	}
}

// computePassesForComplex finds all passes for a single complex.
func computePassesForComplex(complex Complex, samples []astro.RADecAtTime, now time.Time) []Pass {
	obs := ObserverForComplex(complex)

	// Convert samples to elevation series
	type elSample struct {
		t      time.Time
		elDeg  float64
		raDeg  float64
		decDeg float64
	}

	elSamples := make([]elSample, len(samples))
	for i, s := range samples {
		coord := astro.SkyCoord{RAdeg: s.RAdeg, DecDeg: s.DecDeg}
		horiz := astro.EquatorialToHorizontal(coord, obs, s.Time)
		elSamples[i] = elSample{
			t:      s.Time,
			elDeg:  horiz.ElDeg,
			raDeg:  s.RAdeg,
			decDeg: s.DecDeg,
		}
	}

	// Find passes: contiguous intervals where elevation >= MinPassElevation
	var passes []Pass
	inPass := false
	var passStart time.Time
	var maxEl float64
	var maxElTime time.Time
	var minSunSep float64 = 360.0

	for i := 0; i < len(elSamples); i++ {
		curr := elSamples[i]
		aboveThreshold := curr.elDeg >= MinPassElevation

		if !inPass && aboveThreshold {
			// Pass starts
			inPass = true
			passStart = curr.t
			maxEl = curr.elDeg
			maxElTime = curr.t
			minSunSep = 360.0

			// Interpolate actual crossing if we have a previous sample
			if i > 0 {
				prev := elSamples[i-1]
				if prev.elDeg < MinPassElevation {
					passStart = interpolateCrossing(prev.t, curr.t, prev.elDeg, curr.elDeg, MinPassElevation)
				}
			}
		}

		if inPass {
			// Track maximum elevation
			if curr.elDeg > maxEl {
				maxEl = curr.elDeg
				maxElTime = curr.t
			}

			// Track minimum sun separation
			sunSep := astro.SunSeparation(curr.raDeg, curr.decDeg, curr.t)
			if sunSep < minSunSep {
				minSunSep = sunSep
			}

			// Check if pass ends
			if !aboveThreshold {
				// Pass ends - interpolate crossing
				passEnd := curr.t
				if i > 0 {
					prev := elSamples[i-1]
					passEnd = interpolateCrossing(prev.t, curr.t, prev.elDeg, curr.elDeg, MinPassElevation)
				}

				passes = append(passes, Pass{
					Complex:   complex,
					Start:     passStart,
					Peak:      maxElTime,
					End:       passEnd,
					MaxElDeg:  maxEl,
					SunMinSep: minSunSep,
				})
				inPass = false
			}
		}
	}

	// Handle pass that extends to end of window
	if inPass {
		lastSample := elSamples[len(elSamples)-1]
		passes = append(passes, Pass{
			Complex:   complex,
			Start:     passStart,
			Peak:      maxElTime,
			End:       lastSample.t,
			MaxElDeg:  maxEl,
			SunMinSep: minSunSep,
		})
	}

	return passes
}

// interpolateCrossing finds the time when elevation crosses a threshold.
func interpolateCrossing(t1, t2 time.Time, el1, el2, threshold float64) time.Time {
	if el2 == el1 {
		return t1
	}
	fraction := (threshold - el1) / (el2 - el1)
	if fraction < 0 {
		fraction = 0
	} else if fraction > 1 {
		fraction = 1
	}
	dt := t2.Sub(t1)
	return t1.Add(time.Duration(float64(dt) * fraction))
}

// classifyPasses assigns status to each pass based on current time.
func classifyPasses(passes []Pass, now time.Time) {
	foundNext := false

	for i := range passes {
		p := &passes[i]

		if now.After(p.End) {
			p.Status = PassPast
		} else if now.After(p.Start) && now.Before(p.End) {
			p.Status = PassNow
		} else if !foundNext && now.Before(p.Start) {
			p.Status = PassNext
			foundNext = true
		} else {
			p.Status = PassFuture
		}
	}
}

// GetPassesForComplex returns passes filtered by complex.
func (p *PassPlan) GetPassesForComplex(c Complex) []Pass {
	var result []Pass
	for _, pass := range p.Passes {
		if pass.Complex == c {
			result = append(result, pass)
		}
	}
	return result
}

// GetCurrentPass returns the pass currently in progress, or nil.
func (p *PassPlan) GetCurrentPass() *Pass {
	for i := range p.Passes {
		if p.Passes[i].Status == PassNow {
			return &p.Passes[i]
		}
	}
	return nil
}

// GetNextPass returns the next upcoming pass, or nil.
func (p *PassPlan) GetNextPass() *Pass {
	for i := range p.Passes {
		if p.Passes[i].Status == PassNext {
			return &p.Passes[i]
		}
	}
	return nil
}

// ComplexShortName returns the short display name for a complex.
func ComplexShortName(c Complex) string {
	switch c {
	case ComplexGoldstone:
		return "GDS"
	case ComplexCanberra:
		return "CDS"
	case ComplexMadrid:
		return "MDS"
	default:
		return "???"
	}
}
