package dsn

import (
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

func TestComputeElevationTrace(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-ElevationTraceWindow)
	windowEnd := now.Add(ElevationTraceWindow)

	// Generate fake RA/Dec samples spanning the window
	// 4 hours at 5 minute intervals = 49 samples
	var samples []astro.RADecAtTime
	sampleTime := windowStart
	for sampleTime.Before(windowEnd) || sampleTime.Equal(windowEnd) {
		samples = append(samples, astro.RADecAtTime{
			Time:   sampleTime,
			RAdeg:  180.0, // Arbitrary RA
			DecDeg: 45.0,  // Arbitrary Dec - should give reasonable elevation
		})
		sampleTime = sampleTime.Add(ElevationTraceSampleInterval)
	}

	trace := ComputeElevationTrace("VGR1", ComplexGoldstone, samples, now)

	// Verify basic properties
	if trace.SpacecraftCode != "VGR1" {
		t.Errorf("SpacecraftCode = %q, want %q", trace.SpacecraftCode, "VGR1")
	}
	if trace.Complex != ComplexGoldstone {
		t.Errorf("Complex = %v, want %v", trace.Complex, ComplexGoldstone)
	}

	// Verify sample count (should be ~49 for 4h at 5min intervals)
	expectedSamples := 49
	if len(trace.Samples) != expectedSamples {
		t.Errorf("sample count = %d, want %d", len(trace.Samples), expectedSamples)
	}

	// Verify time ordering
	for i := 1; i < len(trace.Samples); i++ {
		if !trace.Samples[i].Time.After(trace.Samples[i-1].Time) {
			t.Errorf("samples not in chronological order at index %d", i)
		}
	}

	// Verify spacing (should be ~5 minutes between samples)
	if len(trace.Samples) >= 2 {
		delta := trace.Samples[1].Time.Sub(trace.Samples[0].Time)
		if delta != ElevationTraceSampleInterval {
			t.Errorf("sample spacing = %v, want %v", delta, ElevationTraceSampleInterval)
		}
	}

	// Verify elevations are reasonable (not NaN, within -90 to 90)
	for i, s := range trace.Samples {
		if s.Elevation < -90 || s.Elevation > 90 {
			t.Errorf("sample[%d] elevation = %f, out of valid range", i, s.Elevation)
		}
	}
}

func TestComputeElevationTrace_EmptySamples(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	trace := ComputeElevationTrace("VGR1", ComplexGoldstone, nil, now)

	if trace == nil {
		t.Fatal("expected non-nil trace for empty samples")
	}
	if len(trace.Samples) != 0 {
		t.Errorf("expected 0 samples, got %d", len(trace.Samples))
	}
}

func TestComputeElevationTrace_AllComplexes(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// Single sample at 'now'
	samples := []astro.RADecAtTime{
		{Time: now, RAdeg: 180.0, DecDeg: 30.0},
	}

	complexes := []Complex{ComplexGoldstone, ComplexCanberra, ComplexMadrid}

	for _, c := range complexes {
		trace := ComputeElevationTrace("TEST", c, samples, now)

		if trace.Complex != c {
			t.Errorf("for complex %v: got Complex = %v", c, trace.Complex)
		}
		if len(trace.Samples) != 1 {
			t.Errorf("for complex %v: got %d samples, want 1", c, len(trace.Samples))
		}
	}
}

func TestComputeElevationTrace_WindowFiltering(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// Samples outside the window should be filtered out
	samples := []astro.RADecAtTime{
		{Time: now.Add(-5 * time.Hour), RAdeg: 180.0, DecDeg: 30.0}, // Too early
		{Time: now.Add(-1 * time.Hour), RAdeg: 180.0, DecDeg: 30.0}, // In window
		{Time: now, RAdeg: 180.0, DecDeg: 30.0},                     // In window
		{Time: now.Add(1 * time.Hour), RAdeg: 180.0, DecDeg: 30.0},  // In window
		{Time: now.Add(5 * time.Hour), RAdeg: 180.0, DecDeg: 30.0},  // Too late
	}

	trace := ComputeElevationTrace("TEST", ComplexGoldstone, samples, now)

	// Should only have the 3 samples within ±2h
	if len(trace.Samples) != 3 {
		t.Errorf("sample count = %d, want 3 (filtered to window)", len(trace.Samples))
	}
}

func TestElevationTrace_CurrentElevation(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	trace := &ElevationTrace{
		Samples: []ElevationSample{
			{Time: now.Add(-30 * time.Minute), Elevation: 10.0},
			{Time: now.Add(-15 * time.Minute), Elevation: 20.0},
			{Time: now, Elevation: 30.0},
			{Time: now.Add(15 * time.Minute), Elevation: 40.0},
			{Time: now.Add(30 * time.Minute), Elevation: 50.0},
		},
	}

	// Query at exactly 'now' should return the 30.0 sample
	current := trace.CurrentElevation(now)
	if current == nil {
		t.Fatal("expected non-nil current elevation")
	}
	if current.Elevation != 30.0 {
		t.Errorf("elevation = %f, want 30.0", current.Elevation)
	}

	// Query slightly after 'now' should still return closest sample
	current = trace.CurrentElevation(now.Add(5 * time.Minute))
	if current == nil {
		t.Fatal("expected non-nil current elevation")
	}
	if current.Elevation != 30.0 {
		t.Errorf("elevation = %f, want 30.0 (closest)", current.Elevation)
	}

	// Query at +20 minutes should return the +15min sample (40.0)
	current = trace.CurrentElevation(now.Add(20 * time.Minute))
	if current == nil {
		t.Fatal("expected non-nil current elevation")
	}
	if current.Elevation != 40.0 {
		t.Errorf("elevation = %f, want 40.0 (closest)", current.Elevation)
	}
}

func TestElevationTrace_CurrentElevation_Empty(t *testing.T) {
	trace := &ElevationTrace{
		Samples: nil,
	}

	current := trace.CurrentElevation(time.Now())
	if current != nil {
		t.Errorf("expected nil for empty trace, got %+v", current)
	}
}
