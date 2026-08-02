// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package dsn

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestExportSolarSystemShape(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := BuildSolarSystemSnapshot(nil, at)
	export := ExportSolarSystem(snap)

	if export.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", export.SchemaVersion, SchemaVersion)
	}
	if export.Frame != "heliocentric-ecliptic-J2000" {
		t.Errorf("frame = %q, want heliocentric-ecliptic-J2000", export.Frame)
	}
	if export.Units != "AU" {
		t.Errorf("units = %q, want AU", export.Units)
	}

	// Sun plus eight planets.
	if len(export.Bodies) != 9 {
		t.Fatalf("got %d bodies, want 9 (Sun + 8 planets)", len(export.Bodies))
	}

	var sunSeen bool
	for _, b := range export.Bodies {
		switch b.Kind {
		case "sun":
			sunSeen = true
			if b.Source != SourceStatic {
				t.Errorf("Sun source = %q, want %q", b.Source, SourceStatic)
			}
			if b.DistanceAU != 0 {
				t.Errorf("Sun should sit at the origin, got %.6f AU", b.DistanceAU)
			}
		case "planet":
			if b.Source != SourceKeplerian {
				t.Errorf("%s source = %q, want %q", b.Name, b.Source, SourceKeplerian)
			}
			if b.Class != "inner" && b.Class != "giant" {
				t.Errorf("%s class = %q, want inner or giant", b.Name, b.Class)
			}
			if b.DistanceAU <= 0 {
				t.Errorf("%s has non-positive distance %.6f", b.Name, b.DistanceAU)
			}
			// Position magnitude must agree with the reported distance.
			mag := math.Sqrt(b.Position.X*b.Position.X +
				b.Position.Y*b.Position.Y + b.Position.Z*b.Position.Z)
			if math.Abs(mag-b.DistanceAU) > 1e-9 {
				t.Errorf("%s: |position| = %.9f but distance_au = %.9f", b.Name, mag, b.DistanceAU)
			}
			// Light time from the Sun must be consistent with distance.
			wantLT := b.DistanceAU * 499.004784
			if math.Abs(b.LightTimeSec-wantLT) > 1.0 {
				t.Errorf("%s: light_time_sec = %.2f, want ~%.2f", b.Name, b.LightTimeSec, wantLT)
			}
		}
	}
	if !sunSeen {
		t.Error("export is missing the Sun")
	}
}

func TestExportSolarSystemIncludesTrackedSpacecraft(t *testing.T) {
	data, err := Parse([]byte(realisticXML))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	snap := BuildSolarSystemSnapshot(data, time.Now())
	export := ExportSolarSystem(snap)

	var spacecraft int
	for _, b := range export.Bodies {
		if b.Kind == "spacecraft" {
			spacecraft++
			if b.Source != SourceDSN {
				t.Errorf("%s source = %q, want %q", b.Name, b.Source, SourceDSN)
			}
			if b.Class != "" {
				t.Errorf("%s should have no planet class, got %q", b.Name, b.Class)
			}
		}
	}
	if spacecraft == 0 {
		t.Error("expected tracked spacecraft to appear in the solar system export")
	}
}

func TestSolarSystemExportIsValidJSON(t *testing.T) {
	snap := BuildSolarSystemSnapshot(nil, time.Now())

	var buf bytes.Buffer
	if err := ExportSolarSystem(snap).WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var parsed struct {
		SchemaVersion string `json:"schema_version"`
		Frame         string `json:"frame"`
		Bodies        []struct {
			Name     string `json:"name"`
			Position struct {
				X, Y, Z float64
			} `json:"position"`
		} `json:"bodies"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed.SchemaVersion == "" || parsed.Frame == "" || len(parsed.Bodies) == 0 {
		t.Error("round-tripped JSON is missing required fields")
	}
}

// TestSnapshotExportUsesSnakeCaseThroughout guards the complex_loads regression:
// ComplexLoad had no struct tags, so it serialized with Go field names amid
// otherwise snake_case output.
func TestSnapshotExportUsesSnakeCaseThroughout(t *testing.T) {
	data, err := Parse([]byte(realisticXML))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	var buf bytes.Buffer
	if err := ExportSnapshot(data, time.Now()).WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	loads, ok := parsed["complex_loads"].([]any)
	if !ok || len(loads) == 0 {
		t.Fatal("complex_loads missing from export")
	}
	first, ok := loads[0].(map[string]any)
	if !ok {
		t.Fatal("complex_loads entry is not an object")
	}
	for _, want := range []string{"complex", "active_links", "total_antennas", "utilization"} {
		if _, present := first[want]; !present {
			t.Errorf("complex_loads entry missing snake_case key %q (has %v)", want, keysOf(first))
		}
	}
	for _, forbidden := range []string{"Complex", "ActiveLinks", "TotalAntennas", "Utilization"} {
		if _, present := first[forbidden]; present {
			t.Errorf("complex_loads entry still exposes Go field name %q", forbidden)
		}
	}
}

// TestUnknownRangeIsNullNotSentinel verifies that the DSN feed's -1 "no ranging
// solution" marker does not reach consumers as a negative light time.
func TestUnknownRangeIsNullNotSentinel(t *testing.T) {
	data := &DSNData{
		Timestamp: time.Now(),
		Links: []Link{
			{Spacecraft: "NO-RANGE", AntennaID: "DSS99", RTLT: -1, Distance: 0},
			{Spacecraft: "HAS-RANGE", AntennaID: "DSS98", RTLT: 1200, Distance: 1.8e8},
		},
	}

	var buf bytes.Buffer
	if err := ExportSnapshot(data, time.Now()).WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var parsed struct {
		Links []struct {
			Spacecraft string   `json:"spacecraft"`
			Distance   *float64 `json:"distance_km"`
			RTLT       *float64 `json:"rtlt_seconds"`
		} `json:"links"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	for _, l := range parsed.Links {
		switch l.Spacecraft {
		case "NO-RANGE":
			if l.RTLT != nil {
				t.Errorf("unknown RTLT exported as %v, want null", *l.RTLT)
			}
			if l.Distance != nil {
				t.Errorf("unknown distance exported as %v, want null", *l.Distance)
			}
		case "HAS-RANGE":
			if l.RTLT == nil || *l.RTLT != 1200 {
				t.Errorf("known RTLT should survive export, got %v", l.RTLT)
			}
			if l.Distance == nil {
				t.Error("known distance should survive export")
			}
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
