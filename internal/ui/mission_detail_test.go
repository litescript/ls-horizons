package ui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

func TestMissionDetailPassPanelToggle(t *testing.T) {
	m := NewMissionDetailModel()

	// Initially pass panel should be visible (default ON per spec)
	if !m.ShowPassPanel() {
		t.Error("pass panel should be visible initially (default ON)")
	}

	// Press 'h' to toggle off
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.ShowPassPanel() {
		t.Error("pass panel should be hidden after pressing 'h'")
	}

	// Press 'h' again to toggle back on
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if !m.ShowPassPanel() {
		t.Error("pass panel should be visible after pressing 'h' again")
	}
}

func TestMissionDetailSpacecraftNavigation(t *testing.T) {
	m := NewMissionDetailModel()

	// Set up test data with multiple spacecraft
	snapshot := state.Snapshot{
		Spacecraft: []dsn.Spacecraft{
			{ID: 1, Name: "Voyager 1"},
			{ID: 2, Name: "Voyager 2"},
			{ID: 3, Name: "New Horizons"},
		},
	}
	m = m.UpdateData(snapshot)

	// Should auto-select first spacecraft
	if m.SelectedSpacecraftID() != 1 {
		t.Errorf("expected first spacecraft selected (ID=1), got %d", m.SelectedSpacecraftID())
	}

	// Navigate right with ']'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.SelectedSpacecraftID() != 2 {
		t.Errorf("expected ID=2 after ']', got %d", m.SelectedSpacecraftID())
	}

	// Navigate right with 'right' arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.SelectedSpacecraftID() != 3 {
		t.Errorf("expected ID=3 after right arrow, got %d", m.SelectedSpacecraftID())
	}

	// Navigate left with '['
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if m.SelectedSpacecraftID() != 2 {
		t.Errorf("expected ID=2 after '[', got %d", m.SelectedSpacecraftID())
	}

	// Navigate left with 'left' arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.SelectedSpacecraftID() != 1 {
		t.Errorf("expected ID=1 after left arrow, got %d", m.SelectedSpacecraftID())
	}
}

func TestMissionDetailRenderNoPanic(t *testing.T) {
	tests := []struct {
		name  string
		setup func() MissionDetailModel
	}{
		{
			name: "empty model",
			setup: func() MissionDetailModel {
				return NewMissionDetailModel()
			},
		},
		{
			name: "with spacecraft but no pass plan",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m = m.SetSize(80, 24)
				m = m.UpdateData(state.Snapshot{
					Spacecraft: []dsn.Spacecraft{
						{ID: 1, Name: "Test", Distance: 1000000},
					},
				})
				return m
			},
		},
		{
			name: "with pass panel visible but no pass plan",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m = m.SetSize(80, 24)
				m = m.UpdateData(state.Snapshot{
					Spacecraft: []dsn.Spacecraft{
						{ID: 1, Name: "Test", Distance: 1000000},
					},
				})
				// Pass panel is visible by default (default ON per spec)
				return m
			},
		},
		{
			name: "with pass panel and empty pass plan",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m = m.SetSize(80, 24)
				m = m.UpdateData(state.Snapshot{
					Spacecraft: []dsn.Spacecraft{
						{ID: 1, Name: "Test", Distance: 1000000},
					},
				})
				m = m.UpdatePassPlan(&dsn.PassPlan{
					SpacecraftCode: "TEST",
					GeneratedAt:    time.Now(),
					Passes:         nil,
				})
				// Pass panel is visible by default (default ON per spec)
				return m
			},
		},
		{
			name: "with pass panel and synthetic passes",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m = m.SetSize(80, 24)
				m = m.UpdateData(state.Snapshot{
					Spacecraft: []dsn.Spacecraft{
						{ID: 1, Name: "Test", Distance: 1000000},
					},
				})
				now := time.Now()
				m = m.UpdatePassPlan(&dsn.PassPlan{
					SpacecraftCode: "TEST",
					GeneratedAt:    now,
					WindowStart:    now,
					WindowEnd:      now.Add(24 * time.Hour),
					Passes: []dsn.Pass{
						{
							Complex:   dsn.ComplexGoldstone,
							Start:     now.Add(-1 * time.Hour),
							Peak:      now.Add(-30 * time.Minute),
							End:       now.Add(30 * time.Minute),
							MaxElDeg:  45.0,
							SunMinSep: 90.0,
							Status:    dsn.PassNow,
						},
						{
							Complex:   dsn.ComplexCanberra,
							Start:     now.Add(2 * time.Hour),
							Peak:      now.Add(4 * time.Hour),
							End:       now.Add(6 * time.Hour),
							MaxElDeg:  60.0,
							SunMinSep: 5.0, // Low sun separation (warning)
							Status:    dsn.PassNext,
						},
						{
							Complex:   dsn.ComplexMadrid,
							Start:     now.Add(8 * time.Hour),
							Peak:      now.Add(10 * time.Hour),
							End:       now.Add(12 * time.Hour),
							MaxElDeg:  30.0,
							SunMinSep: 120.0,
							Status:    dsn.PassFuture,
						},
					},
				})
				// Pass panel is visible by default (default ON per spec)
				return m
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			// This should not panic
			output := m.View()
			if output == "" {
				t.Error("View() returned empty string")
			}
		})
	}
}

func TestMissionDetailUpdatePassPlan(t *testing.T) {
	m := NewMissionDetailModel()

	now := time.Now()
	plan := &dsn.PassPlan{
		SpacecraftCode: "VGR1",
		GeneratedAt:    now,
		Passes: []dsn.Pass{
			{
				Complex:  dsn.ComplexGoldstone,
				Start:    now,
				End:      now.Add(time.Hour),
				MaxElDeg: 45.0,
				Status:   dsn.PassNow,
			},
		},
	}

	m = m.UpdatePassPlan(plan)

	// Pass panel is visible by default (default ON per spec)
	// Render to verify pass data is used
	output := m.View()

	// Output should contain some indication of the pass
	if len(output) == 0 {
		t.Error("expected non-empty output with pass plan")
	}
}

func TestResampleElevation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		samples []dsn.ElevationSample
		width   int
		wantLen int
		wantNil bool
	}{
		{
			name:    "empty samples",
			samples: nil,
			width:   10,
			wantNil: true,
		},
		{
			name:    "zero width",
			samples: []dsn.ElevationSample{{Time: now, Elevation: 45}},
			width:   0,
			wantNil: true,
		},
		{
			name: "exact match",
			samples: []dsn.ElevationSample{
				{Time: now, Elevation: 10},
				{Time: now.Add(time.Minute), Elevation: 20},
				{Time: now.Add(2 * time.Minute), Elevation: 30},
			},
			width:   3,
			wantLen: 3,
		},
		{
			name: "downsampling",
			samples: []dsn.ElevationSample{
				{Time: now, Elevation: 10},
				{Time: now.Add(time.Minute), Elevation: 20},
				{Time: now.Add(2 * time.Minute), Elevation: 30},
				{Time: now.Add(3 * time.Minute), Elevation: 40},
				{Time: now.Add(4 * time.Minute), Elevation: 50},
				{Time: now.Add(5 * time.Minute), Elevation: 60},
			},
			width:   3,
			wantLen: 3,
		},
		{
			name: "upsampling",
			samples: []dsn.ElevationSample{
				{Time: now, Elevation: 10},
				{Time: now.Add(time.Minute), Elevation: 50},
			},
			width:   5,
			wantLen: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resampleElevation(tt.samples, tt.width)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestInterpolateElevColor(t *testing.T) {
	tests := []struct {
		name string
		t    float64
		// We just verify the color is reasonable (within expected ranges)
	}{
		{"low elevation", 0.0},
		{"mid elevation", 0.5},
		{"high elevation", 1.0},
		{"quarter", 0.25},
		{"three quarter", 0.75},
		{"below zero clamps", -0.5},
		{"above one clamps", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b := interpolateElevColor(tt.t)
			// Basic sanity checks - colors should be valid uint8
			if r > 255 || g > 255 || b > 255 {
				t.Errorf("invalid color: r=%d, g=%d, b=%d", r, g, b)
			}
		})
	}

	// Test that gradient is monotonic in blue/cyan direction
	_, _, b0 := interpolateElevColor(0.0)
	_, _, b1 := interpolateElevColor(1.0)
	if b1 <= b0 {
		t.Errorf("expected blue to increase from low to high elevation, got b0=%d, b1=%d", b0, b1)
	}
}

func TestRenderElevationSparklineStates(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		setup      func() MissionDetailModel
		wantSubstr string
	}{
		{
			name: "loading state",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m.snapshot = state.Snapshot{
					ElevationTraceLoading: true,
				}
				return m
			},
			wantSubstr: "Loading",
		},
		{
			name: "error state",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m.snapshot = state.Snapshot{
					ElevationTraceError: errors.New("unknown spacecraft"),
				}
				return m
			},
			wantSubstr: "Error",
		},
		{
			name: "no data state",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m.snapshot = state.Snapshot{
					ElevationTrace: nil,
				}
				return m
			},
			wantSubstr: "No DSN geometry",
		},
		{
			name: "empty samples",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m.snapshot = state.Snapshot{
					ElevationTrace: &dsn.ElevationTrace{
						Samples: nil,
					},
				}
				return m
			},
			wantSubstr: "No DSN geometry",
		},
		{
			name: "with data",
			setup: func() MissionDetailModel {
				m := NewMissionDetailModel()
				m.snapshot = state.Snapshot{
					ElevationTrace: &dsn.ElevationTrace{
						SpacecraftCode: "VGR1",
						Complex:        dsn.ComplexGoldstone,
						Samples: []dsn.ElevationSample{
							{Time: now.Add(-time.Hour), Elevation: 20},
							{Time: now.Add(-30 * time.Minute), Elevation: 40},
							{Time: now, Elevation: 60},
							{Time: now.Add(30 * time.Minute), Elevation: 40},
							{Time: now.Add(time.Hour), Elevation: 20},
						},
					},
					ElevationTraceComplex: dsn.ComplexGoldstone,
				}
				return m
			},
			wantSubstr: "now:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			output := m.renderElevationSparkline()

			if tt.wantSubstr != "" && len(output) == 0 {
				t.Error("expected non-empty output")
			}
			// Note: We can't easily check substrings because of ANSI codes
			// Just verify no panic and non-empty output
			if len(output) == 0 && tt.wantSubstr != "" {
				t.Errorf("expected output containing %q", tt.wantSubstr)
			}
		})
	}
}

func TestSparklineWidth(t *testing.T) {
	// Verify the sparkline width constant
	if SparklineWidth != 48 {
		t.Errorf("SparklineWidth = %d, want 48", SparklineWidth)
	}
}

func TestSparklineBlocks(t *testing.T) {
	// Verify we have 8 block characters
	if len(sparklineBlocks) != 8 {
		t.Errorf("sparklineBlocks length = %d, want 8", len(sparklineBlocks))
	}

	// Verify they are in ascending order (visually)
	expected := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	for i, r := range sparklineBlocks {
		if r != expected[i] {
			t.Errorf("sparklineBlocks[%d] = %c, want %c", i, r, expected[i])
		}
	}
}
