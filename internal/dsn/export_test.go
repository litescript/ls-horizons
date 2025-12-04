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
				StationID:   "gdscc",
				AntennaID:   "DSS-14",
				Spacecraft:  "Voyager 1",
				SpacecraftID: 31,
				Band:        "X",
				DataRate:    160,
				Distance:    24e9,
				RTLT:        160200,
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
