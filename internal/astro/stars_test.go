// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package astro

import (
	"math"
	"strings"
	"testing"
)

// The catalog is imported and generated rather than hand-written, so these
// tests validate the import instead of spot-checking entries someone typed.
// The failure mode they exist to catch is a regenerated or re-parsed table
// that is subtly wrong everywhere, which no amount of checking famous stars
// would reveal.

// minExpectedStars guards against the catalog silently collapsing. The Bright
// Star Catalogue yields a little over eight thousand entries at mag <= 6.5; a
// parser change that dropped most rows would still leave a plausible-looking
// sky, so the floor is set close to the real figure rather than at zero.
const minExpectedStars = 8000

func TestCatalog_SizeHasNotCollapsed(t *testing.T) {
	cat := DefaultStarCatalog()

	if len(cat.Stars) < minExpectedStars {
		t.Errorf("catalog has %d stars, expected at least %d -- did the import or parser drop rows?",
			len(cat.Stars), minExpectedStars)
	}
	if cat.MagnitudeLimit != CatalogMagnitudeLimit {
		t.Errorf("MagnitudeLimit = %v, want %v", cat.MagnitudeLimit, CatalogMagnitudeLimit)
	}
}

// TestCatalog_ParserConsumesEveryRow is the guard on the lenient parser.
// parseCatalog skips malformed rows so a running process degrades instead of
// panicking, which means a corrupted table would go unnoticed at runtime.
// Here, every data row in the embedded file must survive.
func TestCatalog_ParserConsumesEveryRow(t *testing.T) {
	var rows int
	for _, line := range strings.Split(bsc5Table, "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		rows++
	}

	if got := len(DefaultStarCatalog().Stars); got != rows {
		t.Errorf("parsed %d stars from %d data rows -- the parser is skipping rows", got, rows)
	}
}

func TestCatalog_NoDuplicateCatalogIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range DefaultStarCatalog().Stars {
		if s.CatalogID == "" {
			t.Errorf("star %q (%v, %v) has no catalog ID", s.Name, s.RAdeg, s.DecDeg)
			continue
		}
		if seen[s.CatalogID] {
			t.Errorf("duplicate catalog ID: %s", s.CatalogID)
		}
		seen[s.CatalogID] = true
	}
}

// A proper name identifying two different stars is the exact defect that hid
// in the hand-maintained table, where one star appeared under two names.
func TestCatalog_NoDuplicateProperNames(t *testing.T) {
	seen := make(map[string]string)
	for _, s := range DefaultStarCatalog().Stars {
		if s.Name == "" {
			continue
		}
		if prev, dup := seen[s.Name]; dup {
			t.Errorf("proper name %q used by both %s and %s", s.Name, prev, s.CatalogID)
		}
		seen[s.Name] = s.CatalogID
	}
}

func TestCatalog_CoordinatesArePossible(t *testing.T) {
	for _, s := range DefaultStarCatalog().Stars {
		if math.IsNaN(s.RAdeg) || math.IsInf(s.RAdeg, 0) || s.RAdeg < 0 || s.RAdeg >= 360 {
			t.Errorf("%s has impossible RA: %v", s.CatalogID, s.RAdeg)
		}
		if math.IsNaN(s.DecDeg) || math.IsInf(s.DecDeg, 0) || s.DecDeg < -90 || s.DecDeg > 90 {
			t.Errorf("%s has impossible Dec: %v", s.CatalogID, s.DecDeg)
		}
	}
}

func TestCatalog_MagnitudesAreValid(t *testing.T) {
	// Sirius at -1.46 is the brightest star there is, so nothing in a star
	// catalog has any business being brighter than -2.
	const brightestPossible = -2.0

	for _, s := range DefaultStarCatalog().Stars {
		if math.IsNaN(s.Mag) || math.IsInf(s.Mag, 0) {
			t.Errorf("%s has non-finite magnitude: %v", s.CatalogID, s.Mag)
			continue
		}
		if s.Mag < brightestPossible {
			t.Errorf("%s magnitude %v is brighter than any real star", s.CatalogID, s.Mag)
		}
		if s.Mag > CatalogMagnitudeLimit {
			t.Errorf("%s magnitude %v exceeds the selection limit %v",
				s.CatalogID, s.Mag, CatalogMagnitudeLimit)
		}
	}
}

func TestCatalog_ColorDataIsPlausible(t *testing.T) {
	// Real B-V runs from about -0.4 for the hottest O stars to about +4 for
	// the reddest carbon stars. Anything outside that is a parsing error --
	// most likely a misread column boundary.
	const minBV, maxBV = -0.5, 4.0

	cat := DefaultStarCatalog()
	var withColor int

	for _, s := range cat.Stars {
		if s.BV == nil {
			continue
		}
		withColor++

		if math.IsNaN(*s.BV) || math.IsInf(*s.BV, 0) {
			t.Errorf("%s has non-finite B-V: %v", s.CatalogID, *s.BV)
			continue
		}
		if *s.BV < minBV || *s.BV > maxBV {
			t.Errorf("%s has implausible B-V: %v", s.CatalogID, *s.BV)
		}
	}

	// The whole point of importing this catalog was colour. If coverage
	// collapses, the starfield silently turns uniformly white.
	if ratio := float64(withColor) / float64(len(cat.Stars)); ratio < 0.90 {
		t.Errorf("only %.1f%% of stars have B-V, expected over 90%%", ratio*100)
	}
}

// Close binaries the catalog resolves genuinely share a rounded position, so
// coincident coordinates are not by themselves a defect. Some of these pairs
// are photometric twins listed with identical magnitude, colour, and spectral
// type -- epsilon Arietis and alpha Comae both do this -- so a value clash is
// not evidence of copying either. What would be a defect is the same entry
// appearing twice, or the count of coincidences growing beyond the handful of
// real pairs the catalog contains.
func TestCatalog_CoincidentPositionsAreRealPairs(t *testing.T) {
	const maxCoincident = 25

	type key struct{ ra, dec float64 }
	seen := make(map[key]Star)
	var coincident int

	for _, s := range DefaultStarCatalog().Stars {
		k := key{math.Round(s.RAdeg*1e5) / 1e5, math.Round(s.DecDeg*1e5) / 1e5}

		if prev, dup := seen[k]; dup {
			coincident++
			if prev.CatalogID == s.CatalogID {
				t.Errorf("%s appears twice at the same position", s.CatalogID)
			}
			continue
		}
		seen[k] = s
	}

	if coincident > maxCoincident {
		t.Errorf("%d coincident positions, expected at most %d", coincident, maxCoincident)
	}
}

// Sorted order is part of the contract: consumers that want only the brightest
// N stars can truncate rather than sort, and the TUI filter relies on it too.
func TestCatalog_SortedBrightestFirst(t *testing.T) {
	stars := DefaultStarCatalog().Stars

	for i := 1; i < len(stars); i++ {
		if stars[i].Mag < stars[i-1].Mag {
			t.Fatalf("catalog is not sorted by magnitude: %s (%v) follows %s (%v)",
				stars[i].CatalogID, stars[i].Mag, stars[i-1].CatalogID, stars[i-1].Mag)
		}
	}

	if len(stars) > 0 && stars[0].Name != "Sirius" {
		t.Errorf("brightest star is %q (%s), want Sirius", stars[0].Name, stars[0].CatalogID)
	}
}

// A handful of stars whose identity, position, and colour are not going to be
// revised. These catch a wholesale column misread that the range checks above
// would pass.
func TestCatalog_KnownStars(t *testing.T) {
	want := map[string]struct {
		catalogID      string
		ra, dec        float64
		mag            float64
		bv             float64
		spectralPrefix string
	}{
		"Sirius":     {"HR 2491", 101.287, -16.716, -1.46, 0.00, "A1"},
		"Vega":       {"HR 7001", 279.234, 38.784, 0.03, 0.00, "A0"},
		"Betelgeuse": {"HR 2061", 88.793, 7.407, 0.50, 1.85, "M1"},
		"Polaris":    {"HR 424", 37.955, 89.264, 2.02, 0.60, "F7"},
		"Acrux":      {"HR 4730", 186.650, -63.099, 1.33, -0.24, "B0.5"},
		"Castor":     {"HR 2891", 113.650, 31.888, 1.98, 0.03, "A1"},
	}

	byName := make(map[string]Star)
	for _, s := range DefaultStarCatalog().Stars {
		if s.Name != "" {
			byName[s.Name] = s
		}
	}

	for name, w := range want {
		got, found := byName[name]
		if !found {
			t.Errorf("%s is missing from the catalog", name)
			continue
		}

		if got.CatalogID != w.catalogID {
			t.Errorf("%s catalog ID = %q, want %q", name, got.CatalogID, w.catalogID)
		}
		if math.Abs(got.RAdeg-w.ra) > 0.01 {
			t.Errorf("%s RA = %v, want %v", name, got.RAdeg, w.ra)
		}
		if math.Abs(got.DecDeg-w.dec) > 0.01 {
			t.Errorf("%s Dec = %v, want %v", name, got.DecDeg, w.dec)
		}
		if math.Abs(got.Mag-w.mag) > 0.02 {
			t.Errorf("%s Mag = %v, want %v", name, got.Mag, w.mag)
		}
		if got.BV == nil {
			t.Errorf("%s has no B-V", name)
		} else if math.Abs(*got.BV-w.bv) > 0.02 {
			t.Errorf("%s B-V = %v, want %v", name, *got.BV, w.bv)
		}
		if !strings.HasPrefix(got.SpectralType, w.spectralPrefix) {
			t.Errorf("%s spectral type = %q, want prefix %q", name, got.SpectralType, w.spectralPrefix)
		}
	}
}

// Betelgeuse is red and Rigel is blue. If a B-V column were misaligned by a
// field, this ordering would invert while every value stayed in range.
func TestCatalog_ColorOrderingIsPhysical(t *testing.T) {
	byID := make(map[string]Star)
	for _, s := range DefaultStarCatalog().Stars {
		byID[s.CatalogID] = s
	}

	betelgeuse, rigel := byID["HR 2061"], byID["HR 1713"]
	if betelgeuse.BV == nil || rigel.BV == nil {
		t.Fatal("Betelgeuse or Rigel is missing B-V")
	}

	if *betelgeuse.BV <= *rigel.BV {
		t.Errorf("Betelgeuse B-V %v should be far redder than Rigel's %v",
			*betelgeuse.BV, *rigel.BV)
	}
}

// Names must be real or absent. An empty-looking placeholder would defeat the
// omitempty in the export and publish a star named "".
func TestCatalog_NamesAreRealOrAbsent(t *testing.T) {
	cat := DefaultStarCatalog()
	var named int

	for _, s := range cat.Stars {
		if s.Name == "" {
			continue
		}
		named++
		if strings.TrimSpace(s.Name) != s.Name {
			t.Errorf("%s has a padded name: %q", s.CatalogID, s.Name)
		}
	}

	// Only a few hundred stars have proper names at all, so this is a
	// sanity floor rather than a target.
	if named < 100 {
		t.Errorf("only %d stars carry a proper name, expected at least 100", named)
	}
	if named > len(cat.Stars) {
		t.Fatalf("named count %d exceeds catalog size %d", named, len(cat.Stars))
	}
}

func TestCatalog_DeterministicAcrossCalls(t *testing.T) {
	a, b := DefaultStarCatalog(), DefaultStarCatalog()

	if len(a.Stars) != len(b.Stars) {
		t.Fatalf("catalog length differs between calls: %d vs %d", len(a.Stars), len(b.Stars))
	}
	for i := range a.Stars {
		if a.Stars[i].CatalogID != b.Stars[i].CatalogID {
			t.Fatalf("order differs at %d: %s vs %s", i, a.Stars[i].CatalogID, b.Stars[i].CatalogID)
		}
	}
}

func TestCatalog_ProvenanceIsRecorded(t *testing.T) {
	src := DefaultStarCatalog().Source

	if !strings.Contains(src.Catalog, "Bright Star Catalogue") {
		t.Errorf("Source.Catalog = %q", src.Catalog)
	}
	if !strings.Contains(src.Reference, "Hoffleit") {
		t.Errorf("Source.Reference = %q", src.Reference)
	}
	if src.ID == "" || src.URL == "" {
		t.Errorf("Source is missing an identifier or URL: %+v", src)
	}
}

func TestTUIStarCatalog_PreservesTerminalDensity(t *testing.T) {
	tui := TUIStarCatalog()

	// The hand-maintained table the TUI used to render carried 167 stars.
	// This subset should be in the same neighbourhood -- enough to look like
	// a sky, not enough to fill the canvas.
	if len(tui.Stars) < 120 || len(tui.Stars) > 260 {
		t.Errorf("TUI catalog has %d stars, expected roughly the ~167 the views were built around",
			len(tui.Stars))
	}

	for _, s := range tui.Stars {
		if s.Mag > TUIMagnitudeLimit {
			t.Errorf("%s at magnitude %v is fainter than the TUI limit %v",
				s.CatalogID, s.Mag, TUIMagnitudeLimit)
		}
	}

	if tui.MagnitudeLimit != TUIMagnitudeLimit {
		t.Errorf("MagnitudeLimit = %v, want %v", tui.MagnitudeLimit, TUIMagnitudeLimit)
	}
	if tui.Source.Catalog == "" {
		t.Error("filtering dropped the provenance")
	}
}

func TestFilterByMagnitude(t *testing.T) {
	full := DefaultStarCatalog()

	bright := full.FilterByMagnitude(1.0)
	for _, s := range bright.Stars {
		if s.Mag > 1.0 {
			t.Errorf("%s at %v survived a 1.0 filter", s.CatalogID, s.Mag)
		}
	}
	if len(bright.Stars) == 0 {
		t.Error("filtering to magnitude 1.0 left nothing")
	}

	// A filter wider than the catalog must not advertise data that is not
	// there: the reported limit stays at what was actually selected.
	wide := full.FilterByMagnitude(30)
	if len(wide.Stars) != len(full.Stars) {
		t.Errorf("a wider filter changed the star count: %d vs %d", len(wide.Stars), len(full.Stars))
	}
	if wide.MagnitudeLimit != full.MagnitudeLimit {
		t.Errorf("MagnitudeLimit = %v, want %v", wide.MagnitudeLimit, full.MagnitudeLimit)
	}

	// Filtering must not disturb the brightest-first ordering.
	mid := full.FilterByMagnitude(4.0)
	for i := 1; i < len(mid.Stars); i++ {
		if mid.Stars[i].Mag < mid.Stars[i-1].Mag {
			t.Fatalf("filter broke magnitude ordering at index %d", i)
		}
	}
}
