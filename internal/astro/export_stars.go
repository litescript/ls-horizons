// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package astro

import (
	"encoding/json"
	"io"
	"math"

	"github.com/litescript/ls-horizons/internal/version"
)

// The star export lives in package astro rather than alongside the DSN and
// solar system exporters because it has no DSN coupling whatsoever. It is a
// pure function of the star catalog and the frame math, both of which are
// here. Putting it in package dsn would have made that package the home for
// every export regardless of subject.

// StarsSchemaID names the payload so a consumer can tell it apart from the
// other endpoints without inspecting its shape.
const StarsSchemaID = "ls-horizons/stars"

// StarsSchemaVersion is versioned independently of the DSN and solar system
// exports. Those describe live state and evolve with the upstream feed; this
// describes a static catalog and will change on a completely different
// schedule. Sharing one constant across all three would force a version bump
// on payloads that had not changed.
//
// Bump the major component on any breaking change to field names or types;
// bump the minor component when adding fields.
const StarsSchemaVersion = "1.0"

// StarEpoch is the equinox and epoch of every coordinate in the export.
const StarEpoch = "J2000"

// Frame names for the two unit vectors carried by each record.
const (
	// EquatorialFrameJ2000 is right-handed: +X toward the vernal equinox
	// (RA 0h, Dec 0), +Z toward the north celestial pole (Dec +90), and
	// +Y completing the triad at RA 6h, Dec 0.
	EquatorialFrameJ2000 = "equatorial-J2000"

	// EclipticFrameJ2000 is right-handed and shares its +X axis with the
	// equatorial frame. +Z points at the north ecliptic pole and +Y
	// completes the triad. It is the equatorial frame rotated about +X by
	// the J2000 obliquity, which is exactly what EquatorialToEcliptic does.
	EclipticFrameJ2000 = "ecliptic-J2000"
)

// Rounding applied to exported values.
//
// Every figure below is finer than the underlying catalog's own precision, so
// rounding discards no real information. It exists to keep the payload
// compact: at several thousand stars, full float64 formatting roughly doubles
// the file for digits that describe nothing.
const (
	// raDecDecimals gives about 4 milliarcseconds.
	raDecDecimals = 6
	// vectorDecimals gives about 0.2 arcseconds on the unit sphere.
	vectorDecimals = 6
	// magDecimals matches the two decimal places catalogs publish.
	magDecimals = 2
	// colorDecimals leaves one digit of headroom over published B-V.
	colorDecimals = 3
)

// StarSource records where the catalog came from. Provenance travels with the
// data rather than living only in a source comment, so a consumer holding
// nothing but the JSON can still cite it and check it for staleness.
type StarSource struct {
	Catalog   string `json:"catalog"`
	Reference string `json:"reference,omitempty"`
	ID        string `json:"id,omitempty"`
	URL       string `json:"url,omitempty"`
}

// StarFieldExport is the JSON-serializable representation of the star catalog:
// a celestial-sphere direction catalog for rendering a starfield.
//
// This payload is static. It carries no timestamp, which is deliberate --
// every other field is a pure function of the catalog and the binary's
// version, so two runs of the same binary produce byte-identical output. That
// keeps ETags stable across restarts and makes the file safe to cache
// indefinitely. A generated_at field would be the one thing churning in an
// otherwise unchanging document.
type StarFieldExport struct {
	Schema        string `json:"schema"`
	SchemaVersion string `json:"schema_version"`
	Generator     string `json:"generator"`

	// Epoch is the equinox and epoch of every coordinate here.
	Epoch string `json:"epoch"`

	// Frames names the reference frame of each unit vector in a record.
	Frames StarFrames `json:"frames"`

	// MagnitudeLimit is the faintest apparent visual magnitude the catalog
	// was selected down to. It describes the selection, not the faintest
	// star that happens to be present.
	MagnitudeLimit float64 `json:"magnitude_limit"`

	Count  int          `json:"count"`
	Source StarSource   `json:"source"`
	Stars  []StarRecord `json:"stars"`
}

// StarFrames names the frame of each vector field on a star record.
type StarFrames struct {
	Equatorial string `json:"equatorial"`
	Ecliptic   string `json:"ecliptic"`
}

// StarVector is a dimensionless unit vector on the celestial sphere.
//
// It is a direction, not a position. Stars carry no distance in this payload
// deliberately: the catalog is a sphere of directions, and scaling these to
// any radius the renderer likes is the intended use.
type StarVector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// StarRecord is one star as published.
type StarRecord struct {
	// Name is the common or proper name, omitted entirely when the star has
	// none. Most naked-eye stars do not have one, and inventing a label for
	// them would put fabricated data in an otherwise sourced catalog.
	Name string `json:"name,omitempty"`

	// CatalogID is a stable identifier in the source catalog, so a consumer
	// can join against other astronomical data or diff two releases of this
	// file by something more durable than array position.
	CatalogID string `json:"catalog_id,omitempty"`

	RAdeg  float64 `json:"ra_deg"`
	DecDeg float64 `json:"dec_deg"`

	// Mag is apparent visual magnitude. Lower is brighter.
	Mag float64 `json:"mag"`

	// BV is the B-V color index, or null where the catalog has no
	// photometry for the star. It is the raw astronomical datum rather than
	// a display color: mapping B-V to RGB depends on the renderer's color
	// space, exposure, and how much saturation the scene wants, all of
	// which are presentation choices that do not belong in a data contract.
	// The README carries a worked conversion for consumers that want one.
	BV *float64 `json:"bv"`

	// SpectralType is the MK classification where the catalog gives one. It
	// is a coarse fallback color cue for the minority of stars with no B-V.
	SpectralType string `json:"spectral_type,omitempty"`

	// Equatorial and Ecliptic are the same direction in the two frames named
	// by the top-level Frames field. Both are supplied because the
	// conversion is a fixed rotation that costs nothing to precompute here
	// and saves every consumer from reimplementing it.
	Equatorial StarVector `json:"equatorial"`
	Ecliptic   StarVector `json:"ecliptic"`
}

// ExportStars converts a star catalog to its wire representation.
func ExportStars(cat StarCatalog) *StarFieldExport {
	export := &StarFieldExport{
		Schema:        StarsSchemaID,
		SchemaVersion: StarsSchemaVersion,
		Generator:     "ls-horizons/" + version.Version,
		Epoch:         StarEpoch,
		Frames: StarFrames{
			Equatorial: EquatorialFrameJ2000,
			Ecliptic:   EclipticFrameJ2000,
		},
		MagnitudeLimit: cat.MagnitudeLimit,
		Count:          len(cat.Stars),
		Source:         cat.Source,
		Stars:          make([]StarRecord, 0, len(cat.Stars)),
	}

	for _, s := range cat.Stars {
		export.Stars = append(export.Stars, exportStar(s))
	}

	return export
}

// exportStar builds one record, deriving both unit vectors from RA/Dec.
func exportStar(s Star) StarRecord {
	eq := StarUnitVector(s.RAdeg, s.DecDeg)
	ecl := EquatorialToEcliptic(eq)

	rec := StarRecord{
		Name:         s.Name,
		CatalogID:    s.CatalogID,
		RAdeg:        round(s.RAdeg, raDecDecimals),
		DecDeg:       round(s.DecDeg, raDecDecimals),
		Mag:          round(s.Mag, magDecimals),
		SpectralType: s.SpectralType,
		Equatorial:   roundVector(eq),
		Ecliptic:     roundVector(ecl),
	}

	if s.BV != nil {
		bv := round(*s.BV, colorDecimals)
		rec.BV = &bv
	}

	return rec
}

// StarUnitVector converts J2000 RA/Dec in degrees to an equatorial unit vector.
//
// This is the same construction ProjectStarEclipticTopDown performs before
// scaling to its display shell; naming it lets the exporter and the TUI share
// one definition of which axis points where.
func StarUnitVector(raDeg, decDeg float64) Vec3 {
	raRad := degToRad(raDeg)
	decRad := degToRad(decDeg)

	return Vec3{
		X: math.Cos(decRad) * math.Cos(raRad),
		Y: math.Cos(decRad) * math.Sin(raRad),
		Z: math.Sin(decRad),
	}
}

// roundVector rounds each component to the exported vector precision.
func roundVector(v Vec3) StarVector {
	return StarVector{
		X: round(v.X, vectorDecimals),
		Y: round(v.Y, vectorDecimals),
		Z: round(v.Z, vectorDecimals),
	}
}

// round returns v rounded to the given number of decimal places.
//
// The zero normalization matters: math.Round can return negative zero, which
// encoding/json faithfully writes as "-0". That is valid JSON and parses to 0
// everywhere, but it makes byte-identical output depend on the sign of inputs
// that rounded to nothing, which is a needless way to lose determinism.
func round(v float64, decimals int) float64 {
	scale := math.Pow(10, float64(decimals))
	r := math.Round(v*scale) / scale
	if r == 0 {
		return 0
	}
	return r
}

// WriteJSON writes the star field as JSON to the given writer.
func (e *StarFieldExport) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(e)
}
