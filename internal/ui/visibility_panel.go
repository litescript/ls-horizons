package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/litescript/ls-horizons/internal/astro"
	"github.com/litescript/ls-horizons/internal/dsn"
)

// VisibilityMode controls whether visibility data is displayed.
type VisibilityMode int

const (
	VisibilityOff VisibilityMode = iota // No visibility display
	VisibilityOn                        // Show visibility data
)

// Visibility display colors
const (
	colorVisHigh   = "#7CFC00" // Lawn green - high elevation
	colorVisMedium = "#FFD700" // Gold - medium elevation
	colorVisLow    = "#FF6347" // Tomato - low elevation
	colorVisNone   = "#444444" // Dark gray - below horizon

	// Sun separation colors
	colorSunSafe    = "#7CFC00" // Green - safe (>=20°)
	colorSunCaution = "#FFD700" // Gold - caution (10-20°)
	colorSunWarning = "#FF4500" // Orange-red - warning (<10°)
)

// RenderVisibilityPanel renders the visibility information for all DSN complexes.
// Format:
//
//	GDS   Rise 22:14   Peak 23:02 @ 58°   Set 23:49
//	CDS   Below horizon
//	MDS   Rise 02:15   Peak 03:01 @ 12°   Set 03:48
func RenderVisibilityPanel(visibility map[dsn.Complex]*dsn.VisibilityInfo) string {
	if len(visibility) == 0 {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("135")).Bold(true)

	var lines []string

	for _, complex := range dsn.ComplexOrder {
		info, ok := visibility[complex]
		if !ok || info == nil {
			continue
		}

		shortName := dsn.ShortName(complex)
		line := labelStyle.Render(fmt.Sprintf("%-5s", shortName))

		if !info.Window.Valid {
			line += dimStyle.Render("No data")
			lines = append(lines, line)
			continue
		}

		if info.Window.NeverVisible {
			line += dimStyle.Render("Below horizon")
			lines = append(lines, line)
			continue
		}

		if info.Window.AlwaysVisible {
			line += colorByTier(info.ElevTier, fmt.Sprintf("Always visible @ %.0f°", info.CurrentElev))
			lines = append(lines, line)
			continue
		}

		// Normal rise/transit/set window
		var parts []string

		if !info.Window.Rise.IsZero() {
			parts = append(parts, fmt.Sprintf("Rise %s", info.Window.Rise.Local().Format("15:04")))
		}

		if !info.Window.Transit.IsZero() {
			parts = append(parts, fmt.Sprintf("Peak %s @ %.0f°",
				info.Window.Transit.Local().Format("15:04"),
				info.Window.MaxElevation))
		}

		if !info.Window.Set.IsZero() {
			parts = append(parts, fmt.Sprintf("Set %s", info.Window.Set.Local().Format("15:04")))
		}

		if len(parts) > 0 {
			line += colorByTier(info.ElevTier, strings.Join(parts, "   "))
		} else {
			line += dimStyle.Render("Calculating...")
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// RenderVisibilityBar renders a compact horizontal bar showing visibility at all sites.
// Format: GDS ████   CDS ░░░░   MDS ██░░
func RenderVisibilityBar(visibility map[dsn.Complex]*dsn.VisibilityInfo) string {
	if len(visibility) == 0 {
		return ""
	}

	var parts []string

	for _, complex := range dsn.ComplexOrder {
		info, ok := visibility[complex]
		if !ok || info == nil {
			parts = append(parts, renderBarSegment(dsn.ShortName(complex), astro.ElevationNone, false))
			continue
		}

		parts = append(parts, renderBarSegment(dsn.ShortName(complex), info.ElevTier, info.Window.Valid))
	}

	return strings.Join(parts, "   ")
}

// renderBarSegment renders one complex's visibility bar segment.
func renderBarSegment(name string, tier astro.ElevationTier, valid bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	label := labelStyle.Render(name + " ")

	if !valid {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return label + dimStyle.Render("····")
	}

	bar := tierToBar(tier)
	color := tierToColor(tier)
	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

	return label + barStyle.Render(bar)
}

// tierToBar converts elevation tier to a 4-character bar representation.
func tierToBar(tier astro.ElevationTier) string {
	switch tier {
	case astro.ElevationHigh:
		return "████"
	case astro.ElevationMedium:
		return "██░░"
	case astro.ElevationLow:
		return "█░░░"
	default:
		return "░░░░"
	}
}

// tierToColor returns the color for an elevation tier.
func tierToColor(tier astro.ElevationTier) string {
	switch tier {
	case astro.ElevationHigh:
		return colorVisHigh
	case astro.ElevationMedium:
		return colorVisMedium
	case astro.ElevationLow:
		return colorVisLow
	default:
		return colorVisNone
	}
}

// colorByTier applies tier-based coloring to text.
func colorByTier(tier astro.ElevationTier, text string) string {
	color := tierToColor(tier)
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return style.Render(text)
}

// RenderCurrentElevation renders the current elevation status for one complex.
func RenderCurrentElevation(info *dsn.VisibilityInfo) string {
	if info == nil || !info.Window.Valid {
		return ""
	}

	color := tierToColor(info.ElevTier)
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

	if info.CurrentElev <= 0 {
		return style.Render("Below horizon")
	}

	return style.Render(fmt.Sprintf("%.0f°", info.CurrentElev))
}

// RenderSunSeparation renders the sun separation angle with appropriate styling.
func RenderSunSeparation(visibility map[dsn.Complex]*dsn.VisibilityInfo) string {
	// Get sun separation from first available visibility info (same for all complexes)
	var sunSep float64
	var sunTier astro.SunSeparationTier
	for _, info := range visibility {
		if info != nil {
			sunSep = info.SunSep
			sunTier = info.SunSepTier
			break
		}
	}

	color := sunTierToColor(sunTier)
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

	label := "sun-sep: "
	value := fmt.Sprintf("%.1f°", sunSep)

	var status string
	switch sunTier {
	case astro.SunSepWarning:
		status = " (warning)"
	case astro.SunSepCaution:
		status = " (caution)"
	default:
		status = ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	return dimStyle.Render(label) + style.Render(value+status)
}

// sunTierToColor returns the color for a sun separation tier.
func sunTierToColor(tier astro.SunSeparationTier) string {
	switch tier {
	case astro.SunSepWarning:
		return colorSunWarning
	case astro.SunSepCaution:
		return colorSunCaution
	default:
		return colorSunSafe
	}
}
