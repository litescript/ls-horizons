// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

// Command gen-starcatalog converts the Yale Bright Star Catalogue into the
// compact table embedded in package astro.
//
// It is run by hand, not at build time, and its output is committed. The
// source catalog was finalized in 1991 and will not change; regenerating on
// every build would add a network dependency to the build for data that is
// frozen.
//
// Usage:
//
//	curl -O https://cdsarc.cds.unistra.fr/ftp/V/50/catalog.gz
//	gunzip catalog.gz
//	go run ./scripts/gen-starcatalog \
//	    -catalog catalog \
//	    -names scripts/gen-starcatalog/propernames.tsv \
//	    -out internal/astro/data/bsc5.tsv
//
// Provenance of the input is recorded in THIRD-PARTY-NOTICES and in the
// header of the generated file.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// magnitudeLimit is the selection cut. The Bright Star Catalogue is billed as
// complete to V=6.5, which is conventionally the naked-eye limit under dark
// skies, so this takes the catalog exactly as far as it is trustworthy.
const magnitudeLimit = 6.5

// Field positions in the V/50 fixed-width `catalog` file, as 1-based inclusive
// byte ranges from its ReadMe. Kept as named constants so a misread column is
// a visible edit rather than a magic number.
const (
	colHRStart, colHREnd     = 1, 4
	colNameStart, colNameEnd = 5, 14
	colRAhStart, colRAhEnd   = 76, 77
	colRAmStart, colRAmEnd   = 78, 79
	colRAsStart, colRAsEnd   = 80, 83
	colDESign                = 84
	colDEdStart, colDEdEnd   = 85, 86
	colDEmStart, colDEmEnd   = 87, 88
	colDEsStart, colDEsEnd   = 89, 90
	colVmagStart, colVmagEnd = 103, 107
	colBVStart, colBVEnd     = 110, 114
	colSpStart, colSpEnd     = 128, 147
)

// properName is one entry in the curated naming layer.
type properName struct {
	name     string
	raDeg    float64
	decDeg   float64
	mag      float64
	usedBy   int
	conflict bool
}

// star is one accepted catalog record.
type star struct {
	hr          int
	raDeg       float64
	decDeg      float64
	vmag        float64
	bv          string
	spType      string
	designation string
	name        string
}

func main() {
	catalogPath := flag.String("catalog", "catalog", "path to the V/50 `catalog` file")
	namesPath := flag.String("names", "", "path to the curated proper-name TSV")
	outPath := flag.String("out", "", "path to write the embedded table to")
	flag.Parse()

	if *outPath == "" {
		fatal("-out is required")
	}

	names, err := loadProperNames(*namesPath)
	if err != nil {
		fatal("load proper names: %v", err)
	}

	stars, skipped, err := loadCatalog(*catalogPath)
	if err != nil {
		fatal("load catalog: %v", err)
	}

	assignNames(stars, names)

	if err := writeTable(*outPath, stars); err != nil {
		fatal("write table: %v", err)
	}

	report(stars, names, skipped)
}

// loadCatalog parses the fixed-width catalog and applies the magnitude cut.
func loadCatalog(path string) ([]*star, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var (
		stars   []*star
		skipped int
	)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0

	for sc.Scan() {
		line++
		raw := sc.Text()

		hr, err := strconv.Atoi(strings.TrimSpace(field(raw, colHRStart, colHREnd)))
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: bad HR number: %w", line, err)
		}

		// Entries retained only to preserve numbering have blank coordinate
		// and photometry fields. The ReadMe calls these out explicitly; they
		// are novae and extragalactic objects, not stars.
		vmagRaw := strings.TrimSpace(field(raw, colVmagStart, colVmagEnd))
		if vmagRaw == "" || strings.TrimSpace(field(raw, colRAhStart, colRAhEnd)) == "" {
			skipped++
			continue
		}

		vmag, err := strconv.ParseFloat(vmagRaw, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("line %d (HR %d): bad Vmag %q: %w", line, hr, vmagRaw, err)
		}
		if vmag > magnitudeLimit {
			skipped++
			continue
		}

		raDeg, err := parseRA(raw, line, hr)
		if err != nil {
			return nil, 0, err
		}
		decDeg, err := parseDec(raw, line, hr)
		if err != nil {
			return nil, 0, err
		}

		s := &star{
			hr:          hr,
			raDeg:       raDeg,
			decDeg:      decDeg,
			vmag:        vmag,
			bv:          strings.TrimSpace(field(raw, colBVStart, colBVEnd)),
			spType:      normalizeSpace(field(raw, colSpStart, colSpEnd)),
			designation: normalizeSpace(field(raw, colNameStart, colNameEnd)),
		}
		stars = append(stars, s)
	}
	if err := sc.Err(); err != nil {
		return nil, 0, err
	}

	sort.Slice(stars, func(i, j int) bool {
		if stars[i].vmag != stars[j].vmag {
			return stars[i].vmag < stars[j].vmag
		}
		return stars[i].hr < stars[j].hr
	})

	return stars, skipped, nil
}

// parseRA converts the sexagesimal J2000 right ascension to degrees.
func parseRA(raw string, line, hr int) (float64, error) {
	h, err1 := strconv.ParseFloat(strings.TrimSpace(field(raw, colRAhStart, colRAhEnd)), 64)
	m, err2 := strconv.ParseFloat(strings.TrimSpace(field(raw, colRAmStart, colRAmEnd)), 64)
	s, err3 := strconv.ParseFloat(strings.TrimSpace(field(raw, colRAsStart, colRAsEnd)), 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, fmt.Errorf("line %d (HR %d): bad RA fields", line, hr)
	}
	// 24 hours of right ascension span the full 360 degrees.
	return (h + m/60 + s/3600) * 15, nil
}

// parseDec converts the signed sexagesimal J2000 declination to degrees.
func parseDec(raw string, line, hr int) (float64, error) {
	d, err1 := strconv.ParseFloat(strings.TrimSpace(field(raw, colDEdStart, colDEdEnd)), 64)
	m, err2 := strconv.ParseFloat(strings.TrimSpace(field(raw, colDEmStart, colDEmEnd)), 64)
	s, err3 := strconv.ParseFloat(strings.TrimSpace(field(raw, colDEsStart, colDEsEnd)), 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, fmt.Errorf("line %d (HR %d): bad Dec fields", line, hr)
	}

	dec := d + m/60 + s/3600
	// The sign lives in its own column, so it has to be applied to the whole
	// value rather than read off the degrees field.
	if field(raw, colDESign, colDESign) == "-" {
		dec = -dec
	}
	return dec, nil
}

// loadProperNames reads the curated naming layer. An empty path is allowed:
// the catalog is perfectly usable with no proper names at all.
func loadProperNames(path string) ([]*properName, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var names []*properName
	sc := bufio.NewScanner(f)
	line := 0

	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		parts := strings.Split(text, "\t")
		if len(parts) != 4 {
			return nil, fmt.Errorf("%s line %d: want 4 tab-separated fields, got %d", path, line, len(parts))
		}

		ra, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: bad RA: %w", path, line, err)
		}
		dec, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: bad Dec: %w", path, line, err)
		}
		mag, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: bad magnitude: %w", path, line, err)
		}

		names = append(names, &properName{name: parts[0], raDeg: ra, decDeg: dec, mag: mag})
	}
	return names, sc.Err()
}

// Matching tolerances for the naming cross-match.
//
// These are deliberately tight. A proper name attached to the wrong star is
// far worse than a star left unnamed: the export promises that names are real,
// and an unnamed star is an honest gap. Anything that does not match cleanly
// is dropped and reported.
const (
	// matchRadiusDeg is about three arcminutes, far larger than the rounding
	// in the curated table and far smaller than the separation between any
	// two naked-eye stars that could be confused for each other.
	matchRadiusDeg = 0.05

	// matchMagTolerance guards against a position that happens to land near
	// the wrong star. Curated magnitudes came from the same lineage as the
	// catalog, so a real match agrees closely.
	matchMagTolerance = 0.35
)

// assignNames attaches curated proper names to catalog entries by position.
//
// A name is only applied when exactly one catalog star lies within the match
// radius and its magnitude agrees. Duplicated names in the curated table, and
// names whose coordinates were wrong, therefore drop out on their own instead
// of having to be found and patched by hand.
func assignNames(stars []*star, names []*properName) {
	for _, n := range names {
		var (
			best      *star
			bestSep   = math.MaxFloat64
			matches   int
			cosDecRef = math.Cos(n.decDeg * math.Pi / 180)
		)

		// nearby holds every catalog entry inside the match radius,
		// regardless of magnitude, so a resolved multiple star can be
		// recognized as a group below.
		var nearby []*star

		for _, s := range stars {
			// Small-angle separation is plenty at these tolerances, but the
			// RA difference still has to be scaled by cos(dec) or the match
			// radius balloons near the poles.
			dRA := angleDiff(s.raDeg, n.raDeg) * cosDecRef
			dDec := s.decDeg - n.decDeg
			sep := math.Hypot(dRA, dDec)

			if sep > matchRadiusDeg {
				continue
			}
			nearby = append(nearby, s)

			if math.Abs(s.vmag-n.mag) > matchMagTolerance {
				continue
			}

			matches++
			if sep < bestSep {
				best, bestSep = s, sep
			}
		}

		// A curated entry naming an unresolved system will not match any
		// single component, because the catalog resolves the pair while the
		// curated magnitude is the combined brightness of both. Acrux and
		// Castor are the obvious cases. Summing the flux of everything in the
		// radius recovers them without loosening the tolerance for anything
		// else, and the name goes to the brightest component.
		if matches == 0 && len(nearby) > 1 &&
			math.Abs(combinedMagnitude(nearby)-n.mag) <= matchMagTolerance {
			matches = 1
			best = brightest(nearby)
		}

		switch {
		case matches == 0:
			// No candidate: the curated coordinates do not describe a star
			// in the catalog at that magnitude.
		case matches > 1:
			n.conflict = true
		case best.name != "":
			// Two curated names claim the same catalog star.
			n.conflict = true
		default:
			best.name = n.name
			n.usedBy = best.hr
		}
	}
}

// combinedMagnitude returns the apparent magnitude of several stars seen as
// one unresolved point of light. Magnitudes are logarithmic, so they combine
// by summing flux rather than by averaging.
func combinedMagnitude(stars []*star) float64 {
	var flux float64
	for _, s := range stars {
		flux += math.Pow(10, -0.4*s.vmag)
	}
	if flux <= 0 {
		return math.MaxFloat64
	}
	return -2.5 * math.Log10(flux)
}

// brightest returns the lowest-magnitude star in the slice.
func brightest(stars []*star) *star {
	best := stars[0]
	for _, s := range stars[1:] {
		if s.vmag < best.vmag {
			best = s
		}
	}
	return best
}

// angleDiff returns the signed shortest difference between two angles in
// degrees, so a pair straddling 0/360 is not treated as a full turn apart.
func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b+540, 360) - 180
	return d
}

// writeTable emits the embedded catalog.
func writeTable(path string, stars []*star) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	fmt.Fprintln(w, "# Bright Star Catalogue, 5th Revised Ed. (Preliminary Version)")
	fmt.Fprintln(w, "# Hoffleit D., Warren Jr W.H., Astronomical Data Center, NSSDC/ADC (1991)")
	fmt.Fprintln(w, "# VizieR catalogue V/50 -- https://cdsarc.cds.unistra.fr/ftp/V/50/")
	fmt.Fprintln(w, "#")
	fmt.Fprintln(w, "# Generated by scripts/gen-starcatalog. Do not edit by hand.")
	fmt.Fprintf(w, "# Selection: apparent visual magnitude <= %.1f. Equinox and epoch J2000.\n", magnitudeLimit)
	fmt.Fprintln(w, "#")
	fmt.Fprintln(w, "# hr\tra_deg\tdec_deg\tvmag\tbv\tspectral_type\tdesignation\tname")

	for _, s := range stars {
		fmt.Fprintf(w, "%d\t%.5f\t%.5f\t%.2f\t%s\t%s\t%s\t%s\n",
			s.hr, s.raDeg, s.decDeg, s.vmag, s.bv, s.spType, s.designation, s.name)
	}

	return w.Flush()
}

// report prints a generation summary to stderr. The counts are the check that
// a silent parsing change has not quietly halved the catalog.
func report(stars []*star, names []*properName, skipped int) {
	var withBV, withSp, named int
	for _, s := range stars {
		if s.bv != "" {
			withBV++
		}
		if s.spType != "" {
			withSp++
		}
		if s.name != "" {
			named++
		}
	}

	fmt.Fprintf(os.Stderr, "accepted %d stars (mag <= %.1f), skipped %d\n", len(stars), magnitudeLimit, skipped)
	fmt.Fprintf(os.Stderr, "  with B-V:          %d (%.1f%%)\n", withBV, pct(withBV, len(stars)))
	fmt.Fprintf(os.Stderr, "  with spectral type: %d (%.1f%%)\n", withSp, pct(withSp, len(stars)))
	fmt.Fprintf(os.Stderr, "  with proper name:   %d\n", named)

	var unmatched []string
	for _, n := range names {
		if n.usedBy == 0 {
			reason := "no match"
			if n.conflict {
				reason = "ambiguous"
			}
			unmatched = append(unmatched, fmt.Sprintf("%s (%s)", n.name, reason))
		}
	}
	if len(unmatched) > 0 {
		sort.Strings(unmatched)
		fmt.Fprintf(os.Stderr, "  unmatched names (%d): %s\n", len(unmatched), strings.Join(unmatched, ", "))
	}
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

// field returns the 1-based inclusive byte range from a fixed-width record,
// tolerating records that stop short of it.
func field(s string, start, end int) string {
	if start > len(s) {
		return ""
	}
	if end > len(s) {
		end = len(s)
	}
	return s[start-1 : end]
}

// normalizeSpace trims a field and collapses internal runs of whitespace, so
// column padding inside the source does not survive into the output.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen-starcatalog: "+format+"\n", args...)
	os.Exit(1)
}
