package astro

import (
	"testing"
)

func TestDefaultStarCatalog_NonEmpty(t *testing.T) {
	cat := DefaultStarCatalog()

	if len(cat.Stars) == 0 {
		t.Error("DefaultStarCatalog() returned empty catalog")
	}

	// Should have roughly 100 stars
	if len(cat.Stars) < 50 {
		t.Errorf("Expected at least 50 stars, got %d", len(cat.Stars))
	}
}

func TestDefaultStarCatalog_KnownStars(t *testing.T) {
	cat := DefaultStarCatalog()

	// Check for some well-known bright stars
	knownStars := map[string]struct {
		minRA, maxRA   float64
		minDec, maxDec float64
		maxMag         float64
	}{
		"Sirius":     {100, 103, -18, -15, 0}, // brightest star
		"Vega":       {278, 281, 37, 40, 0.5}, // summer triangle
		"Polaris":    {35, 40, 88, 90, 2.5},   // north star
		"Canopus":    {94, 98, -54, -51, 0},   // second brightest
		"Arcturus":   {212, 215, 18, 21, 0.5}, // bright orange star
		"Betelgeuse": {87, 90, 6, 9, 1.0},     // Orion's shoulder
	}

	starMap := make(map[string]Star)
	for _, s := range cat.Stars {
		starMap[s.Name] = s
	}

	for name, expected := range knownStars {
		star, found := starMap[name]
		if !found {
			t.Errorf("Expected star %s not in catalog", name)
			continue
		}

		if star.RAdeg < expected.minRA || star.RAdeg > expected.maxRA {
			t.Errorf("%s RA=%v, expected %v-%v", name, star.RAdeg, expected.minRA, expected.maxRA)
		}

		if star.DecDeg < expected.minDec || star.DecDeg > expected.maxDec {
			t.Errorf("%s Dec=%v, expected %v-%v", name, star.DecDeg, expected.minDec, expected.maxDec)
		}

		if star.Mag > expected.maxMag {
			t.Errorf("%s Mag=%v, expected < %v", name, star.Mag, expected.maxMag)
		}
	}
}

func TestDefaultStarCatalog_ValidCoordinates(t *testing.T) {
	cat := DefaultStarCatalog()

	for _, star := range cat.Stars {
		// RA should be 0-360
		if star.RAdeg < 0 || star.RAdeg >= 360 {
			t.Errorf("Star %s has invalid RA: %v", star.Name, star.RAdeg)
		}

		// Dec should be -90 to +90
		if star.DecDeg < -90 || star.DecDeg > 90 {
			t.Errorf("Star %s has invalid Dec: %v", star.Name, star.DecDeg)
		}

		// Magnitude should be reasonable (brightest stars ~ -1.5, faintest in catalog ~ 4)
		if star.Mag < -2 || star.Mag > 5 {
			t.Errorf("Star %s has unusual magnitude: %v", star.Name, star.Mag)
		}

		// Name should not be empty
		if star.Name == "" {
			t.Error("Found star with empty name")
		}
	}
}

func TestDefaultStarCatalog_NoDuplicates(t *testing.T) {
	cat := DefaultStarCatalog()

	seen := make(map[string]bool)
	for _, star := range cat.Stars {
		if seen[star.Name] {
			t.Errorf("Duplicate star name: %s", star.Name)
		}
		seen[star.Name] = true
	}
}

func TestDefaultStarCatalog_DeterministicOrder(t *testing.T) {
	// Calling DefaultStarCatalog() twice should return same order
	cat1 := DefaultStarCatalog()
	cat2 := DefaultStarCatalog()

	if len(cat1.Stars) != len(cat2.Stars) {
		t.Fatal("Catalog length differs between calls")
	}

	for i := range cat1.Stars {
		if cat1.Stars[i].Name != cat2.Stars[i].Name {
			t.Errorf("Star order differs at index %d: %s vs %s",
				i, cat1.Stars[i].Name, cat2.Stars[i].Name)
		}
	}
}

func TestDefaultStarCatalog_BrightestFirst(t *testing.T) {
	cat := DefaultStarCatalog()

	// First star should be Sirius (brightest)
	if len(cat.Stars) > 0 && cat.Stars[0].Name != "Sirius" {
		t.Errorf("First star should be Sirius (brightest), got %s", cat.Stars[0].Name)
	}

	// First 10 stars should all be mag < 1.0
	for i := 0; i < 10 && i < len(cat.Stars); i++ {
		if cat.Stars[i].Mag > 1.0 {
			t.Errorf("Star %d (%s) has mag %v, expected < 1.0 for brightest stars",
				i, cat.Stars[i].Name, cat.Stars[i].Mag)
		}
	}
}
