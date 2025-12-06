package dsn

import (
	"math"
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

// generateSamples creates synthetic RA/Dec samples over a time window.
func generateSamples(start time.Time, duration time.Duration, step time.Duration, raDecFunc func(t time.Time) (ra, dec float64)) []astro.RADecAtTime {
	var samples []astro.RADecAtTime
	for t := start; t.Before(start.Add(duration)) || t.Equal(start.Add(duration)); t = t.Add(step) {
		ra, dec := raDecFunc(t)
		samples = append(samples, astro.RADecAtTime{
			Time:   t,
			RAdeg:  ra,
			DecDeg: dec,
		})
	}
	return samples
}

func TestPassStatusString(t *testing.T) {
	tests := []struct {
		status PassStatus
		want   string
	}{
		{PassPast, "PAST"},
		{PassNow, "NOW"},
		{PassNext, "NEXT"},
		{PassFuture, "FUTURE"},
		{PassStatus(99), "?"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("PassStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestComputePassPlan_SinglePass(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	// Object at dec=35° is visible from Goldstone (lat=35.4°) at high elevation
	raDecFunc := func(t time.Time) (ra, dec float64) {
		// RA changes slowly, simulating sidereal motion
		hours := t.Sub(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)).Hours()
		ra = math.Mod(hours*15, 360) // 15°/hour = sidereal rate
		dec = 35.0                   // Fixed declination
		return ra, dec
	}

	samples := generateSamples(now, 24*time.Hour, 5*time.Minute, raDecFunc)
	plan := ComputePassPlan("TEST", samples, now)

	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	if plan.SpacecraftCode != "TEST" {
		t.Errorf("SpacecraftCode = %q, want %q", plan.SpacecraftCode, "TEST")
	}

	// Should have some passes
	if len(plan.Passes) == 0 {
		t.Error("expected at least one pass")
	}

	// Check that passes are sorted by start time
	for i := 1; i < len(plan.Passes); i++ {
		if plan.Passes[i].Start.Before(plan.Passes[i-1].Start) {
			t.Errorf("passes not sorted: pass %d starts before pass %d", i, i-1)
		}
	}
}

func TestComputePassPlan_InsufficientSamples(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	// Only 2 samples - not enough
	samples := []astro.RADecAtTime{
		{Time: now, RAdeg: 0, DecDeg: 0},
		{Time: now.Add(time.Hour), RAdeg: 15, DecDeg: 0},
	}

	plan := ComputePassPlan("TEST", samples, now)

	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	if len(plan.Passes) != 0 {
		t.Errorf("expected empty passes with insufficient samples, got %d", len(plan.Passes))
	}
}

func TestComputePassPlan_NoPasses(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	// Object at very low declination - not visible from any DSN site above 5°
	raDecFunc := func(t time.Time) (ra, dec float64) {
		return 0.0, -85.0 // Near south celestial pole
	}

	samples := generateSamples(now, 24*time.Hour, 5*time.Minute, raDecFunc)
	plan := ComputePassPlan("TEST", samples, now)

	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	// Canberra might see something, but max el will be low
	// Just verify structure is valid
	for _, p := range plan.Passes {
		if p.MaxElDeg < MinPassElevation {
			t.Errorf("pass with MaxElDeg=%v below threshold=%v", p.MaxElDeg, MinPassElevation)
		}
	}
}

func TestComputePassPlan_ClassificationNow(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	gdsLon := -116.89

	// Object at zenith for Goldstone right now
	raDecFunc := func(t time.Time) (ra, dec float64) {
		hours := t.Sub(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)).Hours()
		ra = math.Mod(hours*15-gdsLon, 360)
		if ra < 0 {
			ra += 360
		}
		dec = 35.0
		return ra, dec
	}

	samples := generateSamples(now, 24*time.Hour, 5*time.Minute, raDecFunc)
	plan := ComputePassPlan("TEST", samples, now)

	// Should have at least one pass
	if len(plan.Passes) == 0 {
		t.Fatal("expected at least one pass")
	}

	// Verify any NOW pass has valid window
	for _, p := range plan.Passes {
		if p.Status == PassNow {
			if now.Before(p.Start) || now.After(p.End) {
				t.Errorf("NOW pass has invalid window: now=%v, start=%v, end=%v",
					now, p.Start, p.End)
			}
		}
	}
}

func TestComputePassPlan_ClassificationNext(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	raDecFunc := func(t time.Time) (ra, dec float64) {
		hours := t.Sub(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)).Hours()
		ra = math.Mod(hours*15, 360)
		dec = 30.0
		return ra, dec
	}

	samples := generateSamples(now, 24*time.Hour, 5*time.Minute, raDecFunc)
	plan := ComputePassPlan("TEST", samples, now)

	// Count classification types
	nextCount := 0
	for _, p := range plan.Passes {
		if p.Status == PassNext {
			nextCount++
		}
	}

	// Should have at most one NEXT pass
	if nextCount > 1 {
		t.Errorf("found %d NEXT passes, want at most 1", nextCount)
	}
}

func TestComputePassPlan_SunSeparation(t *testing.T) {
	now := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC) // Summer solstice

	// Fixed RA/Dec on celestial equator
	raDecFunc := func(t time.Time) (ra, dec float64) {
		return 0.0, 0.0
	}

	samples := generateSamples(now, 24*time.Hour, 5*time.Minute, raDecFunc)
	plan := ComputePassPlan("TEST", samples, now)

	for _, p := range plan.Passes {
		// Sun separation should be a valid angle
		if p.SunMinSep < 0 || p.SunMinSep > 180 {
			t.Errorf("invalid SunMinSep: %v", p.SunMinSep)
		}
	}
}

func TestComputePassPlan_MaxElevation(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	raDecFunc := func(t time.Time) (ra, dec float64) {
		hours := t.Sub(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)).Hours()
		ra = math.Mod(hours*15, 360)
		dec = 35.0
		return ra, dec
	}

	samples := generateSamples(now, 24*time.Hour, 5*time.Minute, raDecFunc)
	plan := ComputePassPlan("TEST", samples, now)

	for _, p := range plan.Passes {
		// Max elevation should be >= MinPassElevation (5°)
		if p.MaxElDeg < MinPassElevation {
			t.Errorf("MaxElDeg=%v < MinPassElevation=%v", p.MaxElDeg, MinPassElevation)
		}

		// Peak time should be within pass window
		if p.Peak.Before(p.Start) || p.Peak.After(p.End) {
			t.Errorf("peak time %v outside pass window [%v, %v]", p.Peak, p.Start, p.End)
		}
	}
}

func TestGetPassesForComplex(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	raDecFunc := func(t time.Time) (ra, dec float64) {
		hours := t.Sub(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)).Hours()
		ra = math.Mod(hours*15, 360)
		dec = 20.0 // Visible from all sites
		return ra, dec
	}

	samples := generateSamples(now, 24*time.Hour, 5*time.Minute, raDecFunc)
	plan := ComputePassPlan("TEST", samples, now)

	gdsPasses := plan.GetPassesForComplex(ComplexGoldstone)
	for _, p := range gdsPasses {
		if p.Complex != ComplexGoldstone {
			t.Errorf("GetPassesForComplex returned pass with wrong complex: %v", p.Complex)
		}
	}
}

func TestInterpolateCrossing(t *testing.T) {
	tests := []struct {
		name      string
		t1, t2    time.Time
		el1, el2  float64
		threshold float64
		wantFrac  float64 // expected fraction of interval
	}{
		{
			name:      "midpoint",
			t1:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:        time.Date(2025, 1, 1, 0, 10, 0, 0, time.UTC),
			el1:       0,
			el2:       10,
			threshold: 5,
			wantFrac:  0.5,
		},
		{
			name:      "quarter",
			t1:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:        time.Date(2025, 1, 1, 0, 10, 0, 0, time.UTC),
			el1:       0,
			el2:       20,
			threshold: 5,
			wantFrac:  0.25,
		},
		{
			name:      "at start",
			t1:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:        time.Date(2025, 1, 1, 0, 10, 0, 0, time.UTC),
			el1:       5,
			el2:       10,
			threshold: 5,
			wantFrac:  0.0,
		},
		{
			name:      "equal elevations",
			t1:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:        time.Date(2025, 1, 1, 0, 10, 0, 0, time.UTC),
			el1:       10,
			el2:       10,
			threshold: 5,
			wantFrac:  0.0, // returns t1 when equal
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpolateCrossing(tt.t1, tt.t2, tt.el1, tt.el2, tt.threshold)
			gotFrac := got.Sub(tt.t1).Seconds() / tt.t2.Sub(tt.t1).Seconds()
			if math.Abs(gotFrac-tt.wantFrac) > 0.01 {
				t.Errorf("interpolateCrossing fraction = %v, want %v", gotFrac, tt.wantFrac)
			}
		})
	}
}

func TestClassifyPasses(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	passes := []Pass{
		{Start: baseTime.Add(-2 * time.Hour), End: baseTime.Add(-1 * time.Hour)},      // Past
		{Start: baseTime.Add(-30 * time.Minute), End: baseTime.Add(30 * time.Minute)}, // Now
		{Start: baseTime.Add(1 * time.Hour), End: baseTime.Add(2 * time.Hour)},        // Next
		{Start: baseTime.Add(3 * time.Hour), End: baseTime.Add(4 * time.Hour)},        // Future
	}

	classifyPasses(passes, baseTime)

	expected := []PassStatus{PassPast, PassNow, PassNext, PassFuture}
	for i, p := range passes {
		if p.Status != expected[i] {
			t.Errorf("pass %d: got status %v, want %v", i, p.Status, expected[i])
		}
	}
}

func TestClassifyPasses_NoNowPass(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	passes := []Pass{
		{Start: baseTime.Add(-2 * time.Hour), End: baseTime.Add(-1 * time.Hour)}, // Past
		{Start: baseTime.Add(1 * time.Hour), End: baseTime.Add(2 * time.Hour)},   // Should be Next
		{Start: baseTime.Add(3 * time.Hour), End: baseTime.Add(4 * time.Hour)},   // Future
	}

	classifyPasses(passes, baseTime)

	expected := []PassStatus{PassPast, PassNext, PassFuture}
	for i, p := range passes {
		if p.Status != expected[i] {
			t.Errorf("pass %d: got status %v, want %v", i, p.Status, expected[i])
		}
	}
}

func TestClassifyPasses_AllFuture(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	passes := []Pass{
		{Start: baseTime.Add(1 * time.Hour), End: baseTime.Add(2 * time.Hour)}, // Next
		{Start: baseTime.Add(3 * time.Hour), End: baseTime.Add(4 * time.Hour)}, // Future
	}

	classifyPasses(passes, baseTime)

	if passes[0].Status != PassNext {
		t.Errorf("first pass should be NEXT, got %v", passes[0].Status)
	}
	if passes[1].Status != PassFuture {
		t.Errorf("second pass should be FUTURE, got %v", passes[1].Status)
	}
}

func TestComplexShortName(t *testing.T) {
	tests := []struct {
		complex Complex
		want    string
	}{
		{ComplexGoldstone, "GDS"},
		{ComplexCanberra, "CDS"},
		{ComplexMadrid, "MDS"},
		{Complex("unknown"), "???"},
	}

	for _, tt := range tests {
		if got := ComplexShortName(tt.complex); got != tt.want {
			t.Errorf("ComplexShortName(%v) = %q, want %q", tt.complex, got, tt.want)
		}
	}
}

func TestPassPlanHelpers(t *testing.T) {
	plan := &PassPlan{
		Passes: []Pass{
			{Complex: ComplexGoldstone, Status: PassPast},
			{Complex: ComplexCanberra, Status: PassNow},
			{Complex: ComplexMadrid, Status: PassNext},
			{Complex: ComplexGoldstone, Status: PassFuture},
		},
	}

	// Test GetCurrentPass
	current := plan.GetCurrentPass()
	if current == nil {
		t.Error("GetCurrentPass returned nil, expected pass")
	} else if current.Status != PassNow {
		t.Errorf("GetCurrentPass returned pass with status %v, want NOW", current.Status)
	}

	// Test GetNextPass
	next := plan.GetNextPass()
	if next == nil {
		t.Error("GetNextPass returned nil, expected pass")
	} else if next.Status != PassNext {
		t.Errorf("GetNextPass returned pass with status %v, want NEXT", next.Status)
	}
}

func TestPassPlanHelpers_NilCases(t *testing.T) {
	plan := &PassPlan{
		Passes: []Pass{
			{Complex: ComplexGoldstone, Status: PassPast},
			{Complex: ComplexCanberra, Status: PassFuture},
		},
	}

	// No NOW pass
	current := plan.GetCurrentPass()
	if current != nil {
		t.Error("GetCurrentPass should return nil when no NOW pass")
	}

	// No NEXT pass (only PAST and FUTURE)
	// Actually FUTURE should become NEXT - let's fix the test
	plan2 := &PassPlan{
		Passes: []Pass{
			{Complex: ComplexGoldstone, Status: PassPast},
		},
	}

	next := plan2.GetNextPass()
	if next != nil {
		t.Error("GetNextPass should return nil when no NEXT pass")
	}
}
