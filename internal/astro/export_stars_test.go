// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package astro

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

// synthCatalog is a fixture with known-good values. Tests that assert on the
// shape of the export use this rather than the real catalog, so they keep
// passing when the catalog behind DefaultStarCatalog is replaced.
func synthCatalog() StarCatalog {
	bv := 0.0
	return StarCatalog{
		MagnitudeLimit: 6.5,
		Source: StarSource{
			Catalog: "test fixture",
			ID:      "fixture/1",
		},
		Stars: []Star{
			// Vernal equinox: equatorial and ecliptic vectors coincide.
			{Name: "Origin", CatalogID: "T1", RAdeg: 0, DecDeg: 0, Mag: 1.0, BV: &bv},
			// RA 6h on the celestial equator: the obliquity rotation is
			// fully visible in Y and Z here.
			{Name: "Quarter", CatalogID: "T2", RAdeg: 90, DecDeg: 0, Mag: 2.0},
			// North celestial pole.
			{Name: "Pole", CatalogID: "T3", RAdeg: 0, DecDeg: 90, Mag: 3.0},
			// Unnamed star, which must not acquire a name in the export.
			{CatalogID: "T4", RAdeg: 200.5, DecDeg: -45.25, Mag: 6.25, SpectralType: "K0III"},
		},
	}
}

func TestExportStars_SchemaMetadata(t *testing.T) {
	e := ExportStars(synthCatalog())

	if e.Schema != StarsSchemaID {
		t.Errorf("Schema = %q, want %q", e.Schema, StarsSchemaID)
	}
	if e.SchemaVersion != StarsSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", e.SchemaVersion, StarsSchemaVersion)
	}
	if e.Epoch != StarEpoch {
		t.Errorf("Epoch = %q, want %q", e.Epoch, StarEpoch)
	}
	if e.Frames.Equatorial != EquatorialFrameJ2000 {
		t.Errorf("Frames.Equatorial = %q", e.Frames.Equatorial)
	}
	if e.Frames.Ecliptic != EclipticFrameJ2000 {
		t.Errorf("Frames.Ecliptic = %q", e.Frames.Ecliptic)
	}
	if e.MagnitudeLimit != 6.5 {
		t.Errorf("MagnitudeLimit = %v, want 6.5", e.MagnitudeLimit)
	}
	if e.Source.Catalog != "test fixture" || e.Source.ID != "fixture/1" {
		t.Errorf("Source not carried through: %+v", e.Source)
	}
	if e.Generator == "" {
		t.Error("Generator is empty")
	}
}

// The star export is versioned on its own lifecycle. If someone later wires it
// to the DSN export's constant, the two payloads start bumping each other's
// versions for changes that did not touch them.
func TestExportStars_SchemaIDIsDistinct(t *testing.T) {
	if StarsSchemaID != "ls-horizons/stars" {
		t.Errorf("StarsSchemaID = %q, want ls-horizons/stars", StarsSchemaID)
	}
}

func TestExportStars_CountMatchesCatalog(t *testing.T) {
	cat := synthCatalog()
	e := ExportStars(cat)

	if e.Count != len(cat.Stars) {
		t.Errorf("Count = %d, want %d", e.Count, len(cat.Stars))
	}
	if len(e.Stars) != e.Count {
		t.Errorf("len(Stars) = %d, but Count = %d", len(e.Stars), e.Count)
	}
}

func TestExportStars_PreservesRADecAndMagnitude(t *testing.T) {
	cat := synthCatalog()
	e := ExportStars(cat)

	for i, want := range cat.Stars {
		got := e.Stars[i]
		if math.Abs(got.RAdeg-want.RAdeg) > 1e-6 {
			t.Errorf("star %d RA = %v, want %v", i, got.RAdeg, want.RAdeg)
		}
		if math.Abs(got.DecDeg-want.DecDeg) > 1e-6 {
			t.Errorf("star %d Dec = %v, want %v", i, got.DecDeg, want.DecDeg)
		}
		if math.Abs(got.Mag-want.Mag) > 1e-9 {
			t.Errorf("star %d Mag = %v, want %v", i, got.Mag, want.Mag)
		}
		if got.CatalogID != want.CatalogID {
			t.Errorf("star %d CatalogID = %q, want %q", i, got.CatalogID, want.CatalogID)
		}
	}
}

// Sirius is the one entry worth pinning by name: it is the brightest star in
// the sky, so it is present in any catalog this export could plausibly be
// pointed at, and its coordinates are not going to be revised.
func TestExportStars_Sirius(t *testing.T) {
	e := ExportStars(DefaultStarCatalog())

	var sirius *StarRecord
	for i := range e.Stars {
		if e.Stars[i].Name == "Sirius" {
			sirius = &e.Stars[i]
			break
		}
	}
	if sirius == nil {
		t.Fatal("Sirius not found in export")
	}

	if sirius.RAdeg < 101.0 || sirius.RAdeg > 101.6 {
		t.Errorf("Sirius RA = %v, want ~101.29", sirius.RAdeg)
	}
	if sirius.DecDeg < -17.0 || sirius.DecDeg > -16.4 {
		t.Errorf("Sirius Dec = %v, want ~-16.72", sirius.DecDeg)
	}
	if sirius.Mag > -1.0 {
		t.Errorf("Sirius Mag = %v, want brighter than -1.0", sirius.Mag)
	}

	// Sirius sits south of the celestial equator and south of the ecliptic,
	// so both Z components must be negative. This catches an axis swap or a
	// sign error in the obliquity rotation that a norm check would not.
	if sirius.Equatorial.Z >= 0 {
		t.Errorf("Sirius equatorial Z = %v, want negative", sirius.Equatorial.Z)
	}
	if sirius.Ecliptic.Z >= 0 {
		t.Errorf("Sirius ecliptic Z = %v, want negative", sirius.Ecliptic.Z)
	}
}

func TestExportStars_VectorsAreUnitLength(t *testing.T) {
	e := ExportStars(DefaultStarCatalog())

	// Tolerance is set by the export's own rounding, not by the math.
	const tol = 1e-5

	for _, s := range e.Stars {
		eqNorm := math.Sqrt(s.Equatorial.X*s.Equatorial.X +
			s.Equatorial.Y*s.Equatorial.Y + s.Equatorial.Z*s.Equatorial.Z)
		if math.Abs(eqNorm-1) > tol {
			t.Errorf("%s equatorial norm = %v, want 1", s.Name, eqNorm)
		}

		eclNorm := math.Sqrt(s.Ecliptic.X*s.Ecliptic.X +
			s.Ecliptic.Y*s.Ecliptic.Y + s.Ecliptic.Z*s.Ecliptic.Z)
		if math.Abs(eclNorm-1) > tol {
			t.Errorf("%s ecliptic norm = %v, want 1", s.Name, eclNorm)
		}
	}
}

// The exported ecliptic vector must be exactly what the shared frame math
// produces. This is the guard against the export growing its own private copy
// of the obliquity rotation.
func TestExportStars_EclipticMatchesFrameMath(t *testing.T) {
	cat := DefaultStarCatalog()
	e := ExportStars(cat)

	for i, src := range cat.Stars {
		want := EquatorialToEcliptic(StarUnitVector(src.RAdeg, src.DecDeg))
		got := e.Stars[i].Ecliptic

		if math.Abs(got.X-want.X) > 1e-5 ||
			math.Abs(got.Y-want.Y) > 1e-5 ||
			math.Abs(got.Z-want.Z) > 1e-5 {
			t.Errorf("%s ecliptic = %+v, frame math gives %+v", src.Name, got, want)
		}
	}
}

// Known closed-form values, so a future refactor of the frame math cannot
// silently change what this endpoint publishes.
func TestExportStars_KnownFrameGeometry(t *testing.T) {
	e := ExportStars(synthCatalog())

	const tol = 1e-5
	sinE, cosE := math.Sincos(23.439291 * math.Pi / 180)

	// Vernal equinox: shared +X axis, so both frames agree exactly.
	origin := e.Stars[0]
	if math.Abs(origin.Equatorial.X-1) > tol || math.Abs(origin.Ecliptic.X-1) > tol {
		t.Errorf("vernal equinox should be +X in both frames, got eq=%+v ecl=%+v",
			origin.Equatorial, origin.Ecliptic)
	}
	if math.Abs(origin.Ecliptic.Y) > tol || math.Abs(origin.Ecliptic.Z) > tol {
		t.Errorf("vernal equinox ecliptic Y/Z should vanish, got %+v", origin.Ecliptic)
	}

	// RA 6h on the equator rotates by the full obliquity.
	quarter := e.Stars[1]
	if math.Abs(quarter.Equatorial.Y-1) > tol {
		t.Errorf("RA 6h equatorial = %+v, want +Y", quarter.Equatorial)
	}
	if math.Abs(quarter.Ecliptic.Y-cosE) > tol || math.Abs(quarter.Ecliptic.Z+sinE) > tol {
		t.Errorf("RA 6h ecliptic = %+v, want Y=%v Z=%v", quarter.Ecliptic, cosE, -sinE)
	}

	// The north celestial pole sits one obliquity off the north ecliptic pole.
	pole := e.Stars[2]
	if math.Abs(pole.Equatorial.Z-1) > tol {
		t.Errorf("NCP equatorial = %+v, want +Z", pole.Equatorial)
	}
	if math.Abs(pole.Ecliptic.Y-sinE) > tol || math.Abs(pole.Ecliptic.Z-cosE) > tol {
		t.Errorf("NCP ecliptic = %+v, want Y=%v Z=%v", pole.Ecliptic, sinE, cosE)
	}
}

func TestExportStars_UnnamedStarsHaveNoName(t *testing.T) {
	var buf bytes.Buffer
	if err := ExportStars(synthCatalog()).WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var doc struct {
		Stars []map[string]any `json:"stars"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The fourth fixture star is unnamed: the key must be absent, not empty.
	if _, present := doc.Stars[3]["name"]; present {
		t.Errorf("unnamed star carries a name key: %v", doc.Stars[3]["name"])
	}
	// A named star must still emit one.
	if doc.Stars[0]["name"] != "Origin" {
		t.Errorf("named star lost its name: %v", doc.Stars[0]["name"])
	}
	if doc.Stars[3]["spectral_type"] != "K0III" {
		t.Errorf("spectral_type = %v, want K0III", doc.Stars[3]["spectral_type"])
	}
}

// B-V must serialize as null rather than 0 when unknown: 0.0 is a real B-V
// (an A0 star), so a zero value would be indistinguishable from missing data.
func TestExportStars_MissingColorIsNull(t *testing.T) {
	var buf bytes.Buffer
	if err := ExportStars(synthCatalog()).WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var doc struct {
		Stars []map[string]any `json:"stars"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	raw, present := doc.Stars[1]["bv"]
	if !present {
		t.Fatal("bv key missing entirely; it should be present and null")
	}
	if raw != nil {
		t.Errorf("bv = %v for a star with no photometry, want null", raw)
	}

	// The fixture's first star has a real B-V of exactly 0.0, which must
	// survive as a number rather than collapsing to null.
	zero, present := doc.Stars[0]["bv"]
	if !present || zero == nil {
		t.Errorf("bv = %v for a star with B-V 0.0, want 0", zero)
	}
}

// The payload is static, so two exports of the same catalog must produce
// identical bytes. Anything time-dependent creeping into the schema would
// break caching for a file that never actually changes.
func TestExportStars_DeterministicJSON(t *testing.T) {
	var first, second bytes.Buffer

	if err := ExportStars(DefaultStarCatalog()).WriteJSON(&first); err != nil {
		t.Fatalf("first WriteJSON: %v", err)
	}
	if err := ExportStars(DefaultStarCatalog()).WriteJSON(&second); err != nil {
		t.Fatalf("second WriteJSON: %v", err)
	}

	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Error("two exports of the same catalog produced different bytes")
	}
}

func TestExportStars_JSONIsValidAndComplete(t *testing.T) {
	var buf bytes.Buffer
	if err := ExportStars(DefaultStarCatalog()).WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var round StarFieldExport
	if err := json.Unmarshal(buf.Bytes(), &round); err != nil {
		t.Fatalf("exported JSON does not parse: %v", err)
	}

	if round.Count != len(round.Stars) {
		t.Errorf("round-tripped count = %d, len(stars) = %d", round.Count, len(round.Stars))
	}
	if round.Count == 0 {
		t.Error("round-tripped export has no stars")
	}
	if round.Schema != StarsSchemaID {
		t.Errorf("round-tripped schema = %q", round.Schema)
	}
}

func TestExportStars_EmptyCatalog(t *testing.T) {
	e := ExportStars(StarCatalog{})

	if e.Count != 0 {
		t.Errorf("Count = %d, want 0", e.Count)
	}

	// An empty list must serialize as [] rather than null, so a consumer can
	// iterate it without a nil check.
	var buf bytes.Buffer
	if err := e.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"stars": []`)) {
		t.Errorf("empty catalog did not emit an empty array:\n%s", buf.String())
	}
}

func TestRound_NormalizesNegativeZero(t *testing.T) {
	// math.Round(-0.0000001 * 1e6)/1e6 yields negative zero, which json
	// writes as "-0". Harmless to parse, but it makes byte-for-byte output
	// depend on the sign of a value that rounded away to nothing.
	got := round(-1e-9, 6)
	if math.Signbit(got) {
		t.Errorf("round(-1e-9, 6) = %v, want positive zero", got)
	}
}

func TestStarUnitVector_MatchesProjectionConstruction(t *testing.T) {
	// StarUnitVector was factored out of ProjectStarEclipticTopDown. If the
	// two ever disagree, the TUI and the export are drawing different skies.
	for _, tc := range []struct{ ra, dec float64 }{
		{0, 0}, {90, 0}, {180, 45}, {270, -30}, {45, 89},
	} {
		v := StarUnitVector(tc.ra, tc.dec)
		if math.Abs(v.Norm()-1) > 1e-12 {
			t.Errorf("StarUnitVector(%v, %v) norm = %v", tc.ra, tc.dec, v.Norm())
		}

		// Recover RA/Dec from the vector.
		ra := math.Atan2(v.Y, v.X) * 180 / math.Pi
		if ra < 0 {
			ra += 360
		}
		dec := math.Asin(v.Z) * 180 / math.Pi

		if math.Abs(dec-tc.dec) > 1e-9 {
			t.Errorf("Dec round-trip: got %v, want %v", dec, tc.dec)
		}
		// RA is degenerate at the poles, so only check it away from them.
		if math.Abs(tc.dec) < 89 && math.Abs(ra-tc.ra) > 1e-9 {
			t.Errorf("RA round-trip: got %v, want %v", ra, tc.ra)
		}
	}
}
