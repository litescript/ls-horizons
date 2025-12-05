// Package dsn spacecraft view abstraction for grouping links by spacecraft.
package dsn

import (
	"sort"
	"strings"

	"github.com/litescript/ls-horizons/internal/astro"
)

// SkyCoord is an alias to astro.SkyCoord for backward compatibility.
// Use astro.SkyCoord directly in new code.
type SkyCoord = astro.SkyCoord

// LinkView represents a single antenna-to-spacecraft link.
type LinkView struct {
	Station    string  // e.g., "DSS34"
	Complex    Complex // e.g., ComplexCanberra
	Band       string  // e.g., "X", "S", "Ka"
	Rate       float64 // Data rate in bps
	DistanceKm float64 // Distance in km
	Struggle   float64 // Struggle index 0-1 (lower = healthier)
	AzDeg      float64 // Azimuth from this antenna
	ElDeg      float64 // Elevation from this antenna
}

// SpacecraftView represents a single spacecraft with all its active links.
// This is the primary entity for UI rendering - one spacecraft, potentially
// tracked by multiple antennas (arraying).
type SpacecraftView struct {
	Code        string     // Short code, e.g., "VGR2"
	Name        string     // Full name, e.g., "Voyager 2"
	ID          int        // Spacecraft ID from DSN
	Links       []LinkView // All active links for this spacecraft
	PrimaryLink LinkView   // The link used for summary/position (highest priority)
}

// Coord returns the sky coordinates for this spacecraft.
// Currently derived from PrimaryLink az/el; future implementations
// may use JPL Horizons or other ephemeris sources.
func (sv SpacecraftView) Coord() SkyCoord {
	return SkyCoord{
		AzDeg:   sv.PrimaryLink.AzDeg,
		ElDeg:   sv.PrimaryLink.ElDeg,
		RangeKm: sv.PrimaryLink.DistanceKm,
	}
}

// AntennaList returns a sorted, "+"-joined list of station IDs.
// e.g., "DSS34+DSS36" for arrayed tracking.
func (sv SpacecraftView) AntennaList() string {
	if len(sv.Links) == 0 {
		return ""
	}
	if len(sv.Links) == 1 {
		return sv.Links[0].Station
	}

	stations := make([]string, len(sv.Links))
	for i, link := range sv.Links {
		stations[i] = link.Station
	}
	sort.Strings(stations)
	return strings.Join(stations, "+")
}

// IsArrayed returns true if multiple antennas are tracking this spacecraft.
func (sv SpacecraftView) IsArrayed() bool {
	return len(sv.Links) > 1
}

// BuildSpacecraftViews groups DSN links by spacecraft and returns a slice
// of SpacecraftView with primary link selection.
//
// Filtering:
//   - Excludes internal DSN/DSS targets (uses IsRealSpacecraft)
//   - Excludes links with elevation < 0 (below horizon)
//
// Primary link selection priority:
//  1. Highest elevation (better signal geometry)
//  2. Lowest struggle (healthier link)
//  3. Lowest station ID (deterministic tiebreaker)
func BuildSpacecraftViews(data *DSNData, elevationMap map[string]float64) []SpacecraftView {
	if data == nil {
		return nil
	}

	// Group links by spacecraft
	groups := make(map[int]*SpacecraftView)

	for _, link := range data.Links {
		// Skip internal DSN/DSS targets
		if !IsRealSpacecraft(link.Spacecraft) {
			continue
		}

		// Get elevation for this antenna
		elevation := elevationMap[link.AntennaID]

		// Skip below horizon
		if elevation < 0 {
			continue
		}

		// Calculate struggle index
		struggle := StruggleIndex(link, elevation)

		// Create LinkView
		lv := LinkView{
			Station:    link.AntennaID,
			Complex:    link.Complex,
			Band:       link.Band,
			Rate:       link.DataRate,
			DistanceKm: link.Distance,
			Struggle:   struggle,
			AzDeg:      elevation, // Will be set from antenna data
			ElDeg:      elevation,
		}

		// Get azimuth from antenna data if available
		for _, station := range data.Stations {
			for _, ant := range station.Antennas {
				if ant.ID == link.AntennaID {
					lv.AzDeg = ant.Azimuth
					lv.ElDeg = ant.Elevation
					break
				}
			}
		}

		// Add to group
		sv, exists := groups[link.SpacecraftID]
		if !exists {
			sv = &SpacecraftView{
				Code: link.Spacecraft,
				Name: GetSpacecraftName(link.Spacecraft),
				ID:   link.SpacecraftID,
			}
			groups[link.SpacecraftID] = sv
		}
		sv.Links = append(sv.Links, lv)
	}

	// Convert map to slice and select primary links
	result := make([]SpacecraftView, 0, len(groups))
	for _, sv := range groups {
		// Sort links for deterministic order
		sortLinks(sv.Links)

		// Select primary link (first after sorting by priority)
		sv.PrimaryLink = selectPrimaryLink(sv.Links)

		result = append(result, *sv)
	}

	// Sort spacecraft by name for consistent display
	sort.Slice(result, func(i, j int) bool {
		return result[i].Code < result[j].Code
	})

	return result
}

// sortLinks sorts links by station ID for consistent ordering.
func sortLinks(links []LinkView) {
	sort.Slice(links, func(i, j int) bool {
		return links[i].Station < links[j].Station
	})
}

// selectPrimaryLink chooses the best link from a slice.
// Priority: highest elevation > lowest struggle > lowest station ID.
func selectPrimaryLink(links []LinkView) LinkView {
	if len(links) == 0 {
		return LinkView{}
	}
	if len(links) == 1 {
		return links[0]
	}

	best := links[0]
	for _, link := range links[1:] {
		if linkBetter(link, best) {
			best = link
		}
	}
	return best
}

// linkBetter returns true if a should be preferred over b as primary link.
func linkBetter(a, b LinkView) bool {
	// 1. Higher elevation wins
	if a.ElDeg != b.ElDeg {
		return a.ElDeg > b.ElDeg
	}
	// 2. Lower struggle wins (healthier)
	if a.Struggle != b.Struggle {
		return a.Struggle < b.Struggle
	}
	// 3. Lower station ID wins (deterministic)
	return a.Station < b.Station
}

// BuildElevationMap creates a map of antenna ID to elevation from DSN data.
func BuildElevationMap(data *DSNData) map[string]float64 {
	elevMap := make(map[string]float64)
	if data == nil {
		return elevMap
	}
	for _, station := range data.Stations {
		for _, ant := range station.Antennas {
			elevMap[ant.ID] = ant.Elevation
		}
	}
	return elevMap
}
