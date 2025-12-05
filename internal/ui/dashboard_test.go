package ui

import (
	"strings"
	"testing"
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
