package ui

import (
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
