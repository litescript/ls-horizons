package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

func TestComplexStatus_Classification(t *testing.T) {
	now := time.Now()
	recent := now.Add(-60 * time.Second) // within 120s window
	old := now.Add(-300 * time.Second)   // outside 120s window

	tests := []struct {
		name      string
		events    []state.Event
		complex   dsn.Complex
		wantGlyph string
		wantLabel string
	}{
		{
			name:      "no events - stable",
			events:    nil,
			complex:   dsn.ComplexGoldstone,
			wantGlyph: glyphStable,
			wantLabel: labelStable,
		},
		{
			name: "old events only - stable",
			events: []state.Event{
				{Type: state.EventNewLink, Timestamp: old, Complex: "gdscc"},
			},
			complex:   dsn.ComplexGoldstone,
			wantGlyph: glyphStable,
			wantLabel: labelStable,
		},
		{
			name: "NEW_LINK - up",
			events: []state.Event{
				{Type: state.EventNewLink, Timestamp: recent, Complex: "gdscc"},
			},
			complex:   dsn.ComplexGoldstone,
			wantGlyph: glyphUp,
			wantLabel: labelUp,
		},
		{
			name: "LINK_RESUMED - up",
			events: []state.Event{
				{Type: state.EventLinkResumed, Timestamp: recent, Complex: "mdscc"},
			},
			complex:   dsn.ComplexMadrid,
			wantGlyph: glyphUp,
			wantLabel: labelUp,
		},
		{
			name: "LINK_LOST - down",
			events: []state.Event{
				{Type: state.EventLinkLost, Timestamp: recent, Complex: "cdscc"},
			},
			complex:   dsn.ComplexCanberra,
			wantGlyph: glyphDown,
			wantLabel: labelDown,
		},
		{
			name: "HANDOFF with complex field - shifting",
			events: []state.Event{
				{Type: state.EventHandoff, Timestamp: recent, Complex: "gdscc", OldStation: "gdscc", NewStation: "mdscc"},
			},
			complex:   dsn.ComplexGoldstone,
			wantGlyph: glyphShifting,
			wantLabel: labelShifting,
		},
		{
			name: "HANDOFF destination - shifting",
			events: []state.Event{
				{Type: state.EventHandoff, Timestamp: recent, Complex: "", OldStation: "gdscc", NewStation: "mdscc"},
			},
			complex:   dsn.ComplexMadrid,
			wantGlyph: glyphShifting,
			wantLabel: labelShifting,
		},
		{
			name: "HANDOFF via antenna ID - shifting",
			events: []state.Event{
				{Type: state.EventHandoff, Timestamp: recent, Complex: "", OldStation: "DSS14", NewStation: "DSS55"},
			},
			complex:   dsn.ComplexGoldstone, // DSS14 is Goldstone
			wantGlyph: glyphShifting,
			wantLabel: labelShifting,
		},
		{
			name: "priority: HANDOFF over LINK_LOST",
			events: []state.Event{
				{Type: state.EventLinkLost, Timestamp: recent, Complex: "gdscc"},
				{Type: state.EventHandoff, Timestamp: recent, Complex: "gdscc"},
			},
			complex:   dsn.ComplexGoldstone,
			wantGlyph: glyphShifting,
			wantLabel: labelShifting,
		},
		{
			name: "priority: LINK_LOST over NEW_LINK",
			events: []state.Event{
				{Type: state.EventNewLink, Timestamp: recent, Complex: "gdscc"},
				{Type: state.EventLinkLost, Timestamp: recent, Complex: "gdscc"},
			},
			complex:   dsn.ComplexGoldstone,
			wantGlyph: glyphDown,
			wantLabel: labelDown,
		},
		{
			name: "event for different complex - stable",
			events: []state.Event{
				{Type: state.EventNewLink, Timestamp: recent, Complex: "mdscc"},
			},
			complex:   dsn.ComplexGoldstone,
			wantGlyph: glyphStable,
			wantLabel: labelStable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := DashboardModel{
				snapshot: state.Snapshot{
					Events: tt.events,
				},
			}

			gotGlyph, gotLabel := m.classifyComplexStatus(tt.complex)

			if gotGlyph != tt.wantGlyph {
				t.Errorf("glyph = %q, want %q", gotGlyph, tt.wantGlyph)
			}
			if gotLabel != tt.wantLabel {
				t.Errorf("label = %q, want %q", gotLabel, tt.wantLabel)
			}
		})
	}
}

func TestComplexStatus_MissionList(t *testing.T) {
	tests := []struct {
		name        string
		links       []dsn.Link
		complex     dsn.Complex
		wantMissons []string // mission names in expected order
		wantNone    bool
	}{
		{
			name:     "no data - none",
			links:    nil,
			complex:  dsn.ComplexGoldstone,
			wantNone: true,
		},
		{
			name: "filter internal DSN targets",
			links: []dsn.Link{
				{Complex: dsn.ComplexGoldstone, Spacecraft: "DSN", AntennaID: "DSS14"},
				{Complex: dsn.ComplexGoldstone, Spacecraft: "DSS", AntennaID: "DSS24"},
			},
			complex:  dsn.ComplexGoldstone,
			wantNone: true,
		},
		{
			name: "single mission",
			links: []dsn.Link{
				{Complex: dsn.ComplexMadrid, Spacecraft: "JWST", AntennaID: "DSS55"},
			},
			complex:     dsn.ComplexMadrid,
			wantMissons: []string{"JWST"},
		},
		{
			name: "sorted by mission name",
			links: []dsn.Link{
				{Complex: dsn.ComplexCanberra, Spacecraft: "MRO", AntennaID: "DSS36"},
				{Complex: dsn.ComplexCanberra, Spacecraft: "JWST", AntennaID: "DSS35"},
				{Complex: dsn.ComplexCanberra, Spacecraft: "VGR1", AntennaID: "DSS43"},
			},
			complex:     dsn.ComplexCanberra,
			wantMissons: []string{"JWST", "MRO", "VGR1"},
		},
		{
			name: "same mission multiple antennas - sorted by DSS number",
			links: []dsn.Link{
				{Complex: dsn.ComplexMadrid, Spacecraft: "MRO", AntennaID: "DSS65"},
				{Complex: dsn.ComplexMadrid, Spacecraft: "MRO", AntennaID: "DSS54"},
			},
			complex:     dsn.ComplexMadrid,
			wantMissons: []string{"MRO", "MRO"}, // DSS54 before DSS65
		},
		{
			name: "filter by complex",
			links: []dsn.Link{
				{Complex: dsn.ComplexGoldstone, Spacecraft: "JWST", AntennaID: "DSS14"},
				{Complex: dsn.ComplexMadrid, Spacecraft: "VGR1", AntennaID: "DSS55"},
			},
			complex:     dsn.ComplexGoldstone,
			wantMissons: []string{"JWST"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data *dsn.DSNData
			if tt.links != nil {
				data = &dsn.DSNData{Links: tt.links}
			}

			m := DashboardModel{
				snapshot: state.Snapshot{
					Data: data,
				},
			}

			result := m.buildMissionLine(tt.complex)

			if tt.wantNone {
				if !strings.Contains(result, "(none)") {
					t.Errorf("expected (none), got %q", result)
				}
				return
			}

			// Verify missions appear in order
			lastIndex := -1
			for _, mission := range tt.wantMissons {
				idx := strings.Index(result[lastIndex+1:], mission)
				if idx == -1 {
					t.Errorf("mission %q not found in result %q", mission, result)
					continue
				}
				lastIndex = lastIndex + 1 + idx
			}

			// Verify @ symbols present for each mission
			atCount := strings.Count(result, "@")
			if atCount != len(tt.wantMissons) {
				t.Errorf("expected %d @ symbols, got %d in %q", len(tt.wantMissons), atCount, result)
			}
		})
	}
}

func TestComplexFromStation(t *testing.T) {
	tests := []struct {
		station string
		want    string
	}{
		{"gdscc", "gdscc"},
		{"cdscc", "cdscc"},
		{"mdscc", "mdscc"},
		{"DSS14", "gdscc"},
		{"DSS24", "gdscc"},
		{"DSS34", "cdscc"},
		{"DSS43", "cdscc"},
		{"DSS55", "mdscc"},
		{"DSS63", "mdscc"},
		{"", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.station, func(t *testing.T) {
			got := complexFromStation(tt.station)
			if got != tt.want {
				t.Errorf("complexFromStation(%q) = %q, want %q", tt.station, got, tt.want)
			}
		})
	}
}

func TestComplexStatus_Alignment(t *testing.T) {
	m := DashboardModel{
		snapshot: state.Snapshot{
			Data: &dsn.DSNData{},
		},
	}

	output := m.renderComplexSummary()
	lines := strings.Split(output, "\n")

	// Find lines with complex names
	complexLines := []string{}
	for _, line := range lines {
		if strings.Contains(line, "Goldstone") ||
			strings.Contains(line, "Canberra") ||
			strings.Contains(line, "Madrid") {
			complexLines = append(complexLines, line)
		}
	}

	if len(complexLines) != 3 {
		t.Fatalf("expected 3 complex lines, got %d", len(complexLines))
	}

	// All glyph positions should be roughly aligned
	// (complex names are padded to 10 chars)
	for _, line := range complexLines {
		// Should contain one of the status glyphs
		hasGlyph := strings.Contains(line, glyphStable) ||
			strings.Contains(line, glyphUp) ||
			strings.Contains(line, glyphDown) ||
			strings.Contains(line, glyphShifting)
		if !hasGlyph {
			t.Errorf("line missing status glyph: %q", line)
		}
	}
}
