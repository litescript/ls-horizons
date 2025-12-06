package dsn

import (
	"sync"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

const (
	// VisibilityCacheTTL is how long visibility windows remain valid.
	VisibilityCacheTTL = 5 * time.Minute

	// VisibilityWindowSpan is the time range for visibility calculation.
	VisibilityWindowSpan = 24 * time.Hour

	// VisibilitySampleStep is the sample interval for visibility calculation.
	VisibilitySampleStep = 15 * time.Minute
)

// VisibilityInfo holds visibility data for a spacecraft at one DSN complex.
type VisibilityInfo struct {
	Complex      Complex
	ComplexName  string
	Window       astro.VisibilityWindow
	CurrentElev  float64 // Current elevation in degrees
	ElevTier     astro.ElevationTier
	SunSep       float64 // Sun separation angle in degrees
	SunSepTier   astro.SunSeparationTier
	LastComputed time.Time
}

// VisibilityCache caches visibility windows for spacecraft at all DSN sites.
type VisibilityCache struct {
	mu sync.RWMutex

	// Cache keyed by spacecraft code -> complex -> visibility info
	cache map[string]map[Complex]*VisibilityInfo

	// Track which spacecraft is currently focused for refresh logic
	focusedCode string
}

// NewVisibilityCache creates a new visibility cache.
func NewVisibilityCache() *VisibilityCache {
	return &VisibilityCache{
		cache: make(map[string]map[Complex]*VisibilityInfo),
	}
}

// GetVisibility returns visibility info for a spacecraft at a complex.
// Returns nil if no data is available.
func (vc *VisibilityCache) GetVisibility(code string, complex Complex) *VisibilityInfo {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if complexMap, ok := vc.cache[code]; ok {
		return complexMap[complex]
	}
	return nil
}

// GetAllVisibility returns visibility info for a spacecraft at all complexes.
func (vc *VisibilityCache) GetAllVisibility(code string) map[Complex]*VisibilityInfo {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if complexMap, ok := vc.cache[code]; ok {
		// Return a copy to prevent concurrent map access
		result := make(map[Complex]*VisibilityInfo)
		for c, v := range complexMap {
			info := *v
			result[c] = &info
		}
		return result
	}
	return nil
}

// SetFocus updates the focused spacecraft and triggers refresh if changed.
func (vc *VisibilityCache) SetFocus(code string) bool {
	vc.mu.Lock()
	changed := vc.focusedCode != code
	vc.focusedCode = code
	vc.mu.Unlock()
	return changed
}

// GetFocus returns the currently focused spacecraft code.
func (vc *VisibilityCache) GetFocus() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.focusedCode
}

// NeedsRefresh checks if visibility data for a spacecraft needs refreshing.
func (vc *VisibilityCache) NeedsRefresh(code string) bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	complexMap, ok := vc.cache[code]
	if !ok || len(complexMap) == 0 {
		return true
	}

	// Check if any complex data is stale
	for _, info := range complexMap {
		if time.Since(info.LastComputed) > VisibilityCacheTTL {
			return true
		}
	}

	return false
}

// UpdateVisibility computes and caches visibility for a spacecraft at all complexes.
// This should be called asynchronously to avoid blocking the UI.
func (vc *VisibilityCache) UpdateVisibility(code string, raDeg, decDeg float64) error {
	now := time.Now()

	// Generate samples for visibility calculation
	samples := make([]astro.RADecAtTime, 0, int(VisibilityWindowSpan/VisibilitySampleStep)+1)
	for t := now; t.Before(now.Add(VisibilityWindowSpan)); t = t.Add(VisibilitySampleStep) {
		samples = append(samples, astro.RADecAtTime{
			Time:   t,
			RAdeg:  raDeg,
			DecDeg: decDeg,
		})
	}

	if len(samples) < 3 {
		return astro.ErrInsufficientSamples
	}

	// Compute sun separation (same for all complexes, based on RA/Dec)
	sunSep := astro.SunSeparation(raDeg, decDeg, now)
	sunSepTier := astro.GetSunSeparationTier(sunSep)

	// Compute visibility for each complex
	result := make(map[Complex]*VisibilityInfo)

	for complex, info := range KnownComplexes {
		obs := ObserverForComplex(complex)

		window, err := astro.RiseSet(obs, samples)
		if err != nil {
			// Still store result with invalid window
			result[complex] = &VisibilityInfo{
				Complex:      complex,
				ComplexName:  info.Name,
				Window:       astro.VisibilityWindow{Valid: false},
				CurrentElev:  astro.CurrentElevation(obs, raDeg, decDeg, now),
				SunSep:       sunSep,
				SunSepTier:   sunSepTier,
				LastComputed: now,
			}
			continue
		}

		currentElev := astro.CurrentElevation(obs, raDeg, decDeg, now)

		result[complex] = &VisibilityInfo{
			Complex:      complex,
			ComplexName:  info.Name,
			Window:       window,
			CurrentElev:  currentElev,
			ElevTier:     astro.GetElevationTier(currentElev),
			SunSep:       sunSep,
			SunSepTier:   sunSepTier,
			LastComputed: now,
		}
	}

	// Store results
	vc.mu.Lock()
	vc.cache[code] = result
	vc.mu.Unlock()

	return nil
}


// Clear removes all cached visibility data.
func (vc *VisibilityCache) Clear() {
	vc.mu.Lock()
	vc.cache = make(map[string]map[Complex]*VisibilityInfo)
	vc.mu.Unlock()
}

// ClearSpacecraft removes cached visibility data for a specific spacecraft.
func (vc *VisibilityCache) ClearSpacecraft(code string) {
	vc.mu.Lock()
	delete(vc.cache, code)
	vc.mu.Unlock()
}

// ComplexOrder returns the standard display order for complexes.
var ComplexOrder = []Complex{ComplexGoldstone, ComplexCanberra, ComplexMadrid}

// ShortName returns a short name for a complex (3 chars).
func ShortName(c Complex) string {
	switch c {
	case ComplexGoldstone:
		return "GDS"
	case ComplexCanberra:
		return "CDS"
	case ComplexMadrid:
		return "MDS"
	default:
		return "???"
	}
}
