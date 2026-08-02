// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package dsn

import (
	"encoding/json"
	"io"
	"time"
)

// SolarSystemExport is the JSON-serializable representation of solar system
// state: every body's heliocentric position, ready to drop into a 3D scene.
//
// Positions are J2000 heliocentric ecliptic coordinates in AU. The ecliptic
// plane is the XY plane and the Sun sits at the origin, so a renderer can use
// these as scene coordinates directly, scaled by whatever AU-to-unit factor it
// likes, without any frame conversion.
type SolarSystemExport struct {
	SchemaVersion string       `json:"schema_version"`
	Generator     string       `json:"generator"`
	GeneratedAt   time.Time    `json:"generated_at"`
	Frame         string       `json:"frame"`
	Units         string       `json:"units"`
	Bodies        []BodyExport `json:"bodies"`
}

// BodyExport is a single celestial body positioned in the ecliptic frame.
type BodyExport struct {
	Name string `json:"name"`
	Code string `json:"code"`
	Kind string `json:"kind"` // sun, planet, or spacecraft

	// Class distinguishes inner planets from giants; empty for non-planets.
	Class string `json:"class,omitempty"`

	Position PositionExport `json:"position"`

	// DistanceAU is the heliocentric range, provided so consumers don't have to
	// recompute a magnitude they almost always want.
	DistanceAU float64 `json:"distance_au"`

	// EclipticLonDeg and EclipticLatDeg give the same position in angular form,
	// which is more convenient for labelling and orbital-plane work.
	EclipticLonDeg float64 `json:"ecliptic_lon_deg"`
	EclipticLatDeg float64 `json:"ecliptic_lat_deg"`

	// LightTimeSec is one-way light time from the Sun to this body.
	LightTimeSec float64 `json:"light_time_sec"`

	// Source records how the position was derived, so a consumer can tell
	// locally-propagated orbits from live-tracked positions.
	Source string `json:"source"`

	Meta map[string]string `json:"meta,omitempty"`
}

// PositionExport is a cartesian position in AU.
type PositionExport struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Position source labels.
const (
	// SourceKeplerian marks a position propagated from orbital elements in
	// process, with no network dependency.
	SourceKeplerian = "keplerian"

	// SourceDSN marks a position derived from live DSN tracking data.
	SourceDSN = "dsn"

	// SourceStatic marks a fixed position, such as the Sun at the origin.
	SourceStatic = "static"
)

// ExportSolarSystem converts a solar system snapshot to its wire representation.
func ExportSolarSystem(snap SolarSystemSnapshot) *SolarSystemExport {
	export := &SolarSystemExport{
		SchemaVersion: SchemaVersion,
		Generator:     generatorID(),
		GeneratedAt:   snap.GeneratedAt,
		Frame:         "heliocentric-ecliptic-J2000",
		Units:         "AU",
		Bodies:        make([]BodyExport, 0, len(snap.Bodies)),
	}

	if export.GeneratedAt.IsZero() {
		export.GeneratedAt = time.Now()
	}

	for _, b := range snap.Bodies {
		export.Bodies = append(export.Bodies, BodyExport{
			Name: b.Name,
			Code: b.Code,
			Kind: b.Kind.String(),
			Class: func() string {
				if b.Kind != BodyPlanet {
					return ""
				}
				if b.Class == ClassGiant {
					return "giant"
				}
				return "inner"
			}(),
			Position: PositionExport{
				X: b.Pos.X,
				Y: b.Pos.Y,
				Z: b.Pos.Z,
			},
			DistanceAU:     b.DistanceAU(),
			EclipticLonDeg: b.EclipticLonDeg(),
			EclipticLatDeg: b.EclipticLatDeg(),
			LightTimeSec:   b.LightTimeSec(),
			Source:         sourceForKind(b.Kind),
			Meta:           b.Meta,
		})
	}

	return export
}

// sourceForKind reports how a body's position was obtained.
func sourceForKind(kind BodyKind) string {
	switch kind {
	case BodySun:
		return SourceStatic
	case BodyPlanet:
		return SourceKeplerian
	case BodySpacecraft:
		return SourceDSN
	default:
		return ""
	}
}

// WriteJSON writes the solar system export as JSON to the given writer.
func (s *SolarSystemExport) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// BuildSolarSystemSnapshot assembles a complete solar system snapshot from DSN
// data without needing a long-lived cache. Planets come from local orbital
// element propagation; spacecraft come from whatever DSN is currently tracking.
func BuildSolarSystemSnapshot(data *DSNData, at time.Time) SolarSystemSnapshot {
	cache := NewSolarSystemCache()
	cache.UpdatePlanets()
	if data != nil {
		_ = cache.UpdateSpacecraft(data)
	}
	snap := cache.GetSnapshot()
	snap.GeneratedAt = at
	return snap
}
