package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

func TestRenderUtilizationBar(t *testing.T) {
	m := DashboardModel{}

	tests := []struct {
		name       string
		util       float64
		width      int
		wantFilled int
	}{
		{"empty", 0.0, 10, 0},
		{"full", 1.0, 10, 10},
		{"half", 0.5, 10, 5},
		{"quarter", 0.25, 8, 2},
		{"over 100%", 1.5, 10, 10}, // capped at width
		{"small fraction", 0.1, 10, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := m.renderUtilizationBar(tt.util, tt.width)

			// Check bar has brackets
			if !strings.HasPrefix(bar, "[") || !strings.HasSuffix(bar, "]") {
				t.Errorf("bar should have brackets, got %q", bar)
			}

			// Count filled characters (█)
			filledCount := strings.Count(bar, "█")
			if filledCount != tt.wantFilled {
				t.Errorf("filled count = %d, want %d", filledCount, tt.wantFilled)
			}
		})
	}
}

func TestRenderUtilizationBar_NoTrafficLightColors(t *testing.T) {
	m := DashboardModel{}

	// Test various utilization levels that would have triggered different colors
	utils := []float64{0.0, 0.3, 0.5, 0.7, 0.8, 0.9, 1.0}

	// ANSI color codes for red (196), yellow (226), green (46)
	forbiddenCodes := []string{
		"38;5;196", // red foreground
		"38;5;226", // yellow foreground
		"38;5;46",  // green foreground
	}

	for _, util := range utils {
		bar := m.renderUtilizationBar(util, 10)

		for _, code := range forbiddenCodes {
			if strings.Contains(bar, code) {
				t.Errorf("bar at util=%.1f contains forbidden color code %q", util, code)
			}
		}
	}
}

func TestDashboardEnterOpensMission(t *testing.T) {
	// Create a dashboard with spacecraft
	m := NewDashboardModel()
	m = m.SetSize(80, 24)

	// Create a snapshot with spacecraft
	snapshot := state.Snapshot{
		Spacecraft: []dsn.Spacecraft{
			{ID: 123, Name: "Voyager 1"},
			{ID: 456, Name: "Voyager 2"},
		},
	}

	// Build spacecraft views directly
	m.spacecraft = []dsn.SpacecraftView{
		{ID: 123, Code: "VGR1", Name: "Voyager 1"},
		{ID: 456, Code: "VGR2", Name: "Voyager 2"},
	}
	m.snapshot = snapshot
	m.cursor = 0 // Select first spacecraft

	// Press Enter
	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify cursor hasn't changed
	if updatedModel.cursor != 0 {
		t.Errorf("cursor changed unexpectedly: got %d, want 0", updatedModel.cursor)
	}

	// Verify a command was returned
	if cmd == nil {
		t.Fatal("expected a command to be returned, got nil")
	}

	// Execute the command and check the message
	msg := cmd()
	openMsg, ok := msg.(DashboardOpenMissionMsg)
	if !ok {
		t.Fatalf("expected DashboardOpenMissionMsg, got %T", msg)
	}

	if openMsg.SpacecraftID != 123 {
		t.Errorf("spacecraft ID = %d, want 123", openMsg.SpacecraftID)
	}
}

func TestDashboardEnterNoSpacecraft(t *testing.T) {
	// Create an empty dashboard
	m := NewDashboardModel()
	m = m.SetSize(80, 24)

	// Press Enter with no spacecraft
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should not emit a command
	if cmd != nil {
		t.Error("expected no command when no spacecraft selected")
	}
}

func TestDashboardNavigationAndEnter(t *testing.T) {
	m := NewDashboardModel()
	m.spacecraft = []dsn.SpacecraftView{
		{ID: 100, Code: "SC1", Name: "Spacecraft 1"},
		{ID: 200, Code: "SC2", Name: "Spacecraft 2"},
		{ID: 300, Code: "SC3", Name: "Spacecraft 3"},
	}
	m.cursor = 0

	// Navigate down twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if m.cursor != 2 {
		t.Errorf("cursor after down x2 = %d, want 2", m.cursor)
	}

	// Press Enter
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command after Enter")
	}

	msg := cmd()
	openMsg, ok := msg.(DashboardOpenMissionMsg)
	if !ok {
		t.Fatalf("expected DashboardOpenMissionMsg, got %T", msg)
	}

	// Should be the third spacecraft (ID: 300)
	if openMsg.SpacecraftID != 300 {
		t.Errorf("spacecraft ID = %d, want 300", openMsg.SpacecraftID)
	}
}
