// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package astro

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

// Star represents a cataloged star with position, brightness, and color.
type Star struct {
	Name   string  // Proper name (e.g., "Sirius"); empty for the great majority
	RAdeg  float64 // Right Ascension in degrees (J2000)
	DecDeg float64 // Declination in degrees (J2000)
	Mag    float64 // Apparent visual magnitude (lower = brighter)

	// CatalogID is the stable identifier in the source catalog, e.g. "HR 2491".
	CatalogID string

	// Designation is the Bayer/Flamsteed designation the catalog carries,
	// e.g. "9Alp CMa". Most naked-eye stars have one of these even though
	// only a few hundred have a proper name.
	Designation string

	// BV is the B-V color index, nil where the catalog has no photometry.
	// A pointer rather than a sentinel: B-V is legitimately near zero for
	// A0 stars, so no numeric value can stand in for "unknown".
	BV *float64

	// SpectralType is the MK classification, e.g. "A1Vm".
	SpectralType string
}

// StarCatalog holds a collection of stars for rendering, along with the
// provenance and selection criteria a consumer needs to interpret it.
type StarCatalog struct {
	Stars []Star

	// MagnitudeLimit is the faintest apparent visual magnitude the catalog
	// was selected down to, describing the selection rather than whichever
	// star happens to be faintest.
	MagnitudeLimit float64

	// Source records where the data came from, so provenance travels with
	// the catalog into every export rather than living only in a comment.
	Source StarSource
}

// bsc5Table is the Bright Star Catalogue, filtered to the naked-eye limit and
// reduced to the columns this project uses.
//
// It is embedded rather than fetched so the binary stays self-contained: no
// runtime download, no cache directory, no failure mode where the sky is empty
// because a host is offline. Regenerate it with scripts/gen-starcatalog; the
// source catalog was finalized in 1991 and does not change.
//
//go:embed data/bsc5.tsv
var bsc5Table string

// CatalogMagnitudeLimit is the selection cut of the embedded catalog. 6.5 is
// the conventional naked-eye limit under dark skies and the completeness limit
// the Bright Star Catalogue is published to.
const CatalogMagnitudeLimit = 6.5

// TUIMagnitudeLimit is the cut applied for terminal rendering.
//
// The full catalog is roughly fifty times denser than an ASCII grid can
// resolve; drawing all of it would fill the canvas with overlapping glyphs and
// cost the sky view a per-frame coordinate transform for every star. Third
// magnitude is where the constellation figures live, and it happens to yield
// almost exactly the star count the hand-maintained table used to carry, so
// the views look as they did while now being complete across the whole sky
// rather than wherever entries had been added by hand.
const TUIMagnitudeLimit = 3.0

// bsc5Source is the provenance of the embedded catalog. It travels into every
// export so a consumer holding only the JSON can still cite it.
var bsc5Source = StarSource{
	Catalog:   "Bright Star Catalogue, 5th Revised Ed. (Preliminary Version)",
	Reference: "Hoffleit D., Warren Jr W.H., Astronomical Data Center, NSSDC/ADC (1991)",
	ID:        "VizieR V/50",
	URL:       "https://cdsarc.cds.unistra.fr/ftp/V/50/",
}

var (
	catalogOnce  sync.Once
	catalogStars []Star
)

// DefaultStarCatalog returns the full embedded catalog: every Bright Star
// Catalogue entry down to the naked-eye magnitude limit, J2000.
//
// Callers rendering to a terminal want TUIStarCatalog instead.
func DefaultStarCatalog() StarCatalog {
	catalogOnce.Do(func() {
		catalogStars = parseCatalog(bsc5Table)
	})

	return StarCatalog{
		Stars:          catalogStars,
		MagnitudeLimit: CatalogMagnitudeLimit,
		Source:         bsc5Source,
	}
}

// TUIStarCatalog returns the subset dense enough to look like a sky in a
// terminal without swamping it. See TUIMagnitudeLimit.
func TUIStarCatalog() StarCatalog {
	return DefaultStarCatalog().FilterByMagnitude(TUIMagnitudeLimit)
}

// FilterByMagnitude returns the stars at least as bright as limit, preserving
// order and provenance and narrowing the reported selection limit to match.
func (c StarCatalog) FilterByMagnitude(limit float64) StarCatalog {
	out := make([]Star, 0, len(c.Stars))
	for _, s := range c.Stars {
		if s.Mag <= limit {
			out = append(out, s)
		}
	}

	// Never claim a wider selection than was actually applied.
	if limit > c.MagnitudeLimit {
		limit = c.MagnitudeLimit
	}

	return StarCatalog{
		Stars:          out,
		MagnitudeLimit: limit,
		Source:         c.Source,
	}
}

// Column order of the embedded table, matching what scripts/gen-starcatalog
// writes.
const (
	colHR = iota
	colRA
	colDec
	colVmag
	colBV
	colSpectralType
	colDesignation
	colName
	colCount
)

// parseCatalog reads the embedded table.
//
// Malformed rows are skipped rather than fatal. The data is generated and
// committed, and the tests in this package assert the exact row count and the
// validity of every field, so a bad row means a corrupted binary rather than a
// bad input -- and a starfield missing one entry is a far better outcome for a
// running process than a panic on startup.
func parseCatalog(table string) []Star {
	lines := strings.Split(table, "\n")
	stars := make([]Star, 0, len(lines))

	for _, line := range lines {
		if line == "" || line[0] == '#' {
			continue
		}

		f := strings.Split(line, "\t")
		if len(f) != colCount {
			continue
		}

		ra, err := strconv.ParseFloat(f[colRA], 64)
		if err != nil {
			continue
		}
		dec, err := strconv.ParseFloat(f[colDec], 64)
		if err != nil {
			continue
		}
		mag, err := strconv.ParseFloat(f[colVmag], 64)
		if err != nil {
			continue
		}

		star := Star{
			Name:         f[colName],
			RAdeg:        ra,
			DecDeg:       dec,
			Mag:          mag,
			CatalogID:    "HR " + f[colHR],
			Designation:  f[colDesignation],
			SpectralType: f[colSpectralType],
		}

		// An empty B-V column means the catalog has no photometry for the
		// star, which is different from a B-V of zero.
		if f[colBV] != "" {
			if bv, err := strconv.ParseFloat(f[colBV], 64); err == nil {
				star.BV = &bv
			}
		}

		stars = append(stars, star)
	}

	return stars
}
