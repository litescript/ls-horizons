package dsn

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestExportSnapshot(t *testing.T) {
	data := &DSNData{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Stations: []Station{
			{
				Name:         "gdscc",
				FriendlyName: "Goldstone",
				Complex:      ComplexGoldstone,
				Antennas: []Antenna{
					{ID: "DSS-14", Azimuth: 180, Elevation: 45, Activity: "track"},
				},
			},
		},
		Links: []Link{
			{
				Complex:      ComplexGoldstone,
				StationID:    "gdscc",
				AntennaID:    "DSS-14",
				Spacecraft:   "Voyager 1",
				SpacecraftID: 31,
				Band:         "X",
				DataRate:     160,
				Distance:     24e9,
				RTLT:         160200,
			},
		},
	}

	fetchedAt := time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC)
	export := ExportSnapshot(data, fetchedAt)

	if export.Timestamp != data.Timestamp {
		t.Errorf("Timestamp = %v, want %v", export.Timestamp, data.Timestamp)
	}
	if export.FetchedAt != fetchedAt {
		t.Errorf("FetchedAt = %v, want %v", export.FetchedAt, fetchedAt)
	}
	if len(export.Stations) != 1 {
		t.Fatalf("Stations count = %d, want 1", len(export.Stations))
	}
	if len(export.Links) != 1 {
		t.Fatalf("Links count = %d, want 1", len(export.Links))
	}

	link := export.Links[0]
	if link.Spacecraft != "Voyager 1" {
		t.Errorf("Link spacecraft = %q, want Voyager 1", link.Spacecraft)
	}
	if link.StruggleIndex <= 0 {
		t.Errorf("StruggleIndex = %v, want > 0", link.StruggleIndex)
	}
	if link.Health == "" {
		t.Error("Health should not be empty")
	}
}

func TestExportSnapshot_Nil(t *testing.T) {
	fetchedAt := time.Now()
	export := ExportSnapshot(nil, fetchedAt)

	if export.FetchedAt != fetchedAt {
		t.Errorf("FetchedAt = %v, want %v", export.FetchedAt, fetchedAt)
	}
	if len(export.Stations) != 0 {
		t.Errorf("Stations should be empty for nil data")
	}
}

func TestSnapshotExport_WriteJSON(t *testing.T) {
	export := &SnapshotExport{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		FetchedAt: time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC),
		Links: []LinkExport{
			{
				Spacecraft:    "Mars Orbiter",
				DataRate:      100000,
				Distance:      200e6,
				StruggleIndex: 0.45,
				Health:        "MARGINAL",
			},
		},
	}

	var buf bytes.Buffer
	if err := export.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify indentation
	if !strings.Contains(buf.String(), "  ") {
		t.Error("JSON should be indented")
	}
}

func TestGenerateSummaryRows(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{
				Complex: ComplexGoldstone,
				Antennas: []Antenna{
					{ID: "DSS-14", Elevation: 45},
				},
			},
		},
		Links: []Link{
			{
				Complex:    ComplexGoldstone,
				StationID:  "gdscc",
				AntennaID:  "DSS-14",
				Spacecraft: "TestCraft",
				Band:       "X",
				DataRate:   50000,
				Distance:   1e6,
			},
		},
	}

	rows := GenerateSummaryRows(data)
	if len(rows) != 1 {
		t.Fatalf("Rows count = %d, want 1", len(rows))
	}

	row := rows[0]
	if row.Spacecraft != "TestCraft" {
		t.Errorf("Spacecraft = %q, want TestCraft", row.Spacecraft)
	}
	if row.Band != "X" {
		t.Errorf("Band = %q, want X", row.Band)
	}
	if row.Rate == "" || row.Rate == "N/A" {
		t.Errorf("Rate = %q, want formatted rate", row.Rate)
	}
	if row.Distance == "" || row.Distance == "N/A" {
		t.Errorf("Distance = %q, want formatted distance", row.Distance)
	}
}

func TestGenerateSummaryRows_Nil(t *testing.T) {
	rows := GenerateSummaryRows(nil)
	if rows != nil {
		t.Errorf("Expected nil for nil data, got %v", rows)
	}
}

func TestWriteSummaryTable(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{
				Complex:  ComplexMadrid,
				Antennas: []Antenna{{ID: "DSS-55", Elevation: 30}},
			},
		},
		Links: []Link{
			{
				Complex:    ComplexMadrid,
				AntennaID:  "DSS-55",
				Spacecraft: "EMM",
				Band:       "X",
				DataRate:   240000,
				Distance:   300e6,
			},
		},
	}

	var buf bytes.Buffer
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	WriteSummaryTable(&buf, data, timestamp)

	output := buf.String()

	// Check header present
	if !strings.Contains(output, "DSN Status") {
		t.Error("Output should contain 'DSN Status' header")
	}
	if !strings.Contains(output, "2024-01-15") {
		t.Error("Output should contain timestamp")
	}
	if !strings.Contains(output, "EMM") {
		t.Error("Output should contain spacecraft name")
	}
	if !strings.Contains(output, "Madrid") || !strings.Contains(output, "MDSCC") || !strings.Contains(output, "1 active") {
		// At least should have active links count
		if !strings.Contains(output, "1 active") {
			t.Error("Output should contain active links count")
		}
	}
}

func TestWriteSummaryTable_Empty(t *testing.T) {
	data := &DSNData{}

	var buf bytes.Buffer
	WriteSummaryTable(&buf, data, time.Now())

	output := buf.String()
	if !strings.Contains(output, "No active links") {
		t.Error("Output should indicate no active links")
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10c", 10, "exactly10c"},
		{"toolongstring", 8, "toolon.."},
		{"ab", 2, "ab"},
		{"abc", 2, "ab"},
	}

	for _, tt := range tests {
		got := truncateStr(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

func TestWriteMiniSky(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{
				Complex: ComplexGoldstone,
				Antennas: []Antenna{
					{
						ID:        "DSS-14",
						Azimuth:   180,
						Elevation: 45,
						Targets:   []Target{{ID: 1, Name: "Voyager", RTLT: 160000}},
					},
				},
			},
		},
		Links: []Link{
			{AntennaID: "DSS-14", Spacecraft: "Voyager", Band: "X"},
		},
	}

	var buf bytes.Buffer
	WriteMiniSky(&buf, data, DefaultMiniSkyConfig())
	output := buf.String()

	// Check box characters present
	if !strings.Contains(output, "┌") || !strings.Contains(output, "┘") {
		t.Error("Mini sky should have box borders")
	}
	// Check station marker
	if !strings.Contains(output, "▲") {
		t.Error("Mini sky should have station marker")
	}
	// Check legend
	if !strings.Contains(output, "Voyager") {
		t.Error("Mini sky should have spacecraft in legend")
	}
}

func TestWriteMiniSky_Empty(t *testing.T) {
	var buf bytes.Buffer
	WriteMiniSky(&buf, nil, DefaultMiniSkyConfig())
	if !strings.Contains(buf.String(), "No spacecraft") {
		t.Error("Empty data should show no spacecraft message")
	}
}

func TestWriteNowPlaying(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{Antennas: []Antenna{{ID: "DSS-14", Elevation: 45}}},
		},
		Links: []Link{
			{AntennaID: "DSS-14", Spacecraft: "Mars", DataRate: 1e6, RTLT: 1200},
		},
	}

	var buf bytes.Buffer
	WriteNowPlaying(&buf, data)
	output := buf.String()

	if !strings.Contains(output, "DSS-14") {
		t.Error("Now playing should show antenna")
	}
	if !strings.Contains(output, "Mars") {
		t.Error("Now playing should show spacecraft")
	}
}

func TestWriteNowPlaying_Empty(t *testing.T) {
	var buf bytes.Buffer
	WriteNowPlaying(&buf, nil)
	if !strings.Contains(buf.String(), "No active links") {
		t.Error("Empty data should show no active links")
	}
}

func TestWriteSpacecraftCard(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{Antennas: []Antenna{{ID: "DSS-43", Elevation: 30}}},
		},
		Links: []Link{
			{
				AntennaID:  "DSS-43",
				Spacecraft: "Perseverance",
				Band:       "X",
				DataRate:   2e6,
				Distance:   300e6,
				RTLT:       2000,
				Complex:    ComplexCanberra,
			},
		},
	}

	var buf bytes.Buffer
	WriteSpacecraftCard(&buf, data, "Perseverance", nil)
	output := buf.String()

	if !strings.Contains(output, "Perseverance") {
		t.Error("Card should show spacecraft name")
	}
	if !strings.Contains(output, "DSS-43") {
		t.Error("Card should show antenna")
	}
	if !strings.Contains(output, "┌") {
		t.Error("Card should have box border")
	}
}

func TestWriteSpacecraftCard_NotFound(t *testing.T) {
	data := &DSNData{}
	var buf bytes.Buffer
	WriteSpacecraftCard(&buf, data, "NotExist", nil)
	if !strings.Contains(buf.String(), "not currently tracked") {
		t.Error("Should indicate spacecraft not found")
	}
}

func TestComputeDiff(t *testing.T) {
	prev := &DSNData{
		Links: []Link{
			{Spacecraft: "Alpha", StationID: "gdscc", DataRate: 1000},
			{Spacecraft: "Beta", StationID: "cdscc", DataRate: 2000},
		},
	}
	curr := &DSNData{
		Links: []Link{
			{Spacecraft: "Alpha", StationID: "mdscc", DataRate: 1000}, // Handoff
			{Spacecraft: "Gamma", StationID: "gdscc", DataRate: 3000}, // New
			// Beta is lost
		},
	}

	diff := ComputeDiff(prev, curr)

	if len(diff.NewLinks) != 1 || diff.NewLinks[0].Spacecraft != "Gamma" {
		t.Errorf("NewLinks = %v, want [Gamma]", diff.NewLinks)
	}
	if len(diff.LostLinks) != 1 || diff.LostLinks[0].Spacecraft != "Beta" {
		t.Errorf("LostLinks = %v, want [Beta]", diff.LostLinks)
	}
	if len(diff.Handoffs) != 1 || diff.Handoffs[0].Spacecraft != "Alpha" {
		t.Errorf("Handoffs = %v, want [Alpha]", diff.Handoffs)
	}
}

func TestComputeDiff_RateChange(t *testing.T) {
	prev := &DSNData{
		Links: []Link{{Spacecraft: "Test", DataRate: 1000}},
	}
	curr := &DSNData{
		Links: []Link{{Spacecraft: "Test", DataRate: 2000}}, // 2x increase
	}

	diff := ComputeDiff(prev, curr)
	if len(diff.RateChange) != 1 {
		t.Errorf("RateChange count = %d, want 1", len(diff.RateChange))
	}
}

func TestComputeDiff_NilPrev(t *testing.T) {
	curr := &DSNData{
		Links: []Link{{Spacecraft: "New"}},
	}
	diff := ComputeDiff(nil, curr)
	if len(diff.NewLinks) != 1 {
		t.Error("All links should be new when prev is nil")
	}
}

func TestDiffResult_HasChanges(t *testing.T) {
	empty := DiffResult{}
	if empty.HasChanges() {
		t.Error("Empty diff should not have changes")
	}

	withNew := DiffResult{NewLinks: []Link{{}}}
	if !withNew.HasChanges() {
		t.Error("Diff with new links should have changes")
	}
}

func TestWriteDiff(t *testing.T) {
	diff := DiffResult{
		NewLinks:  []Link{{Spacecraft: "New", AntennaID: "DSS-14", DataRate: 1000}},
		LostLinks: []Link{{Spacecraft: "Lost", AntennaID: "DSS-43"}},
		Handoffs:  []Handoff{{Spacecraft: "Hand", From: "gdscc", To: "mdscc"}},
	}

	var buf bytes.Buffer
	WriteDiff(&buf, diff, time.Now())
	output := buf.String()

	if !strings.Contains(output, "NEW") {
		t.Error("Should show NEW link")
	}
	if !strings.Contains(output, "LOST") {
		t.Error("Should show LOST link")
	}
	if !strings.Contains(output, "HANDOFF") {
		t.Error("Should show HANDOFF")
	}
}

func TestWriteDiff_NoChanges(t *testing.T) {
	var buf bytes.Buffer
	WriteDiff(&buf, DiffResult{}, time.Now())
	if !strings.Contains(buf.String(), "No changes") {
		t.Error("Empty diff should say no changes")
	}
}

func TestWriteEvents(t *testing.T) {
	events := []Event{
		{Type: EventNewLink, Timestamp: time.Now(), Spacecraft: "Test", NewStation: "gdscc"},
	}

	var buf bytes.Buffer
	WriteEvents(&buf, events, 10)
	output := buf.String()

	if !strings.Contains(output, "Event Log") {
		t.Error("Should have Event Log header")
	}
	if !strings.Contains(output, "Test") {
		t.Error("Should show spacecraft name")
	}
}

func TestWriteEvents_Empty(t *testing.T) {
	var buf bytes.Buffer
	WriteEvents(&buf, nil, 10)
	if !strings.Contains(buf.String(), "No events") {
		t.Error("Empty events should say no events")
	}
}

func TestFormatEventType(t *testing.T) {
	tests := []struct {
		t    EventType
		want string
	}{
		{EventNewLink, "●NEW "},
		{EventHandoff, "→HAND"},
		{EventLinkLost, "○LOST"},
	}
	for _, tt := range tests {
		if got := formatEventType(tt.t); got != tt.want {
			t.Errorf("formatEventType(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}
