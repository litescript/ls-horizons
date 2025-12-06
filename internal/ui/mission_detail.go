package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/ephem"
	"github.com/litescript/ls-horizons/internal/state"
)

// MissionDetailModel shows detailed info for a selected spacecraft.
type MissionDetailModel struct {
	width         int
	height        int
	selectedID    int
	snapshot      state.Snapshot
	scrollY       int
	showPassPanel bool
	passPlan      *dsn.PassPlan
	animTick      int // Animation tick for shimmer effects
}

// NewMissionDetailModel creates a new mission detail model.
func NewMissionDetailModel() MissionDetailModel {
	return MissionDetailModel{
		selectedID:    -1,
		showPassPanel: true, // Default ON per spec
	}
}

// SetSize updates the viewport size.
func (m MissionDetailModel) SetSize(width, height int) MissionDetailModel {
	m.width = width
	m.height = height
	return m
}

// SetAnimTick updates the animation tick for shimmer effects.
func (m MissionDetailModel) SetAnimTick(tick int) MissionDetailModel {
	m.animTick = tick
	return m
}

// UpdateData updates with new data snapshot.
func (m MissionDetailModel) UpdateData(snapshot state.Snapshot) MissionDetailModel {
	m.snapshot = snapshot

	// Auto-select first valid spacecraft if none selected (skip stations like DSS)
	if m.selectedID < 0 && len(snapshot.Spacecraft) > 0 {
		for _, sc := range snapshot.Spacecraft {
			if !isStationNotSpacecraft(sc.Name) {
				m.selectedID = sc.ID
				break
			}
		}
	}

	return m
}

// SpacecraftChangedMsg signals the selected spacecraft changed.
type SpacecraftChangedMsg struct {
	SpacecraftID int
}

// Update handles messages.
func (m MissionDetailModel) Update(msg tea.Msg) (MissionDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.scrollY--
			if m.scrollY < 0 {
				m.scrollY = 0
			}
		case "down", "j":
			m.scrollY++
		case "left", "[":
			oldID := m.selectedID
			m.selectPrevSpacecraft()
			if m.selectedID != oldID {
				newID := m.selectedID // Capture value explicitly for closure
				cmd = func() tea.Msg {
					return SpacecraftChangedMsg{SpacecraftID: newID}
				}
			}
		case "right", "]":
			oldID := m.selectedID
			m.selectNextSpacecraft()
			if m.selectedID != oldID {
				newID := m.selectedID // Capture value explicitly for closure
				cmd = func() tea.Msg {
					return SpacecraftChangedMsg{SpacecraftID: newID}
				}
			}
		case "h":
			m.showPassPanel = !m.showPassPanel
		}
	}
	return m, cmd
}

func (m *MissionDetailModel) selectNextSpacecraft() {
	if len(m.snapshot.Spacecraft) == 0 {
		return
	}
	// Find current index, then find next valid (non-station) spacecraft
	foundCurrent := false
	for _, sc := range m.snapshot.Spacecraft {
		if isStationNotSpacecraft(sc.Name) {
			continue
		}
		if foundCurrent {
			m.selectedID = sc.ID
			m.scrollY = 0
			return
		}
		if sc.ID == m.selectedID {
			foundCurrent = true
		}
	}
}

func (m *MissionDetailModel) selectPrevSpacecraft() {
	if len(m.snapshot.Spacecraft) == 0 {
		return
	}
	// Find previous valid (non-station) spacecraft
	var prevID int
	for _, sc := range m.snapshot.Spacecraft {
		if isStationNotSpacecraft(sc.Name) {
			continue
		}
		if sc.ID == m.selectedID {
			if prevID != 0 {
				m.selectedID = prevID
				m.scrollY = 0
			}
			return
		}
		prevID = sc.ID
	}
}

// View renders the mission detail view.
func (m MissionDetailModel) View() string {
	var b strings.Builder

	// Spacecraft selector
	b.WriteString(m.renderSpacecraftSelector())
	b.WriteString("\n\n")

	// Find selected spacecraft
	var selected *dsn.Spacecraft
	for i := range m.snapshot.Spacecraft {
		if m.snapshot.Spacecraft[i].ID == m.selectedID {
			selected = &m.snapshot.Spacecraft[i]
			break
		}
	}

	if selected == nil {
		b.WriteString("  No spacecraft selected. Use ←/→ to select.\n")
		return b.String()
	}

	// Spacecraft details first
	b.WriteString(m.renderSpacecraftDetails(selected))

	// Pass panel below details (if enabled)
	if m.showPassPanel {
		b.WriteString("\n")
		b.WriteString(m.renderPassPanel())
	}

	return b.String()
}

func (m MissionDetailModel) renderSpacecraftSelector() string {
	var b strings.Builder

	selectorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Padding(0, 1)

	b.WriteString(selectorStyle.Render("Spacecraft: "))
	b.WriteString("← ")

	for _, sc := range m.snapshot.Spacecraft {
		// Skip station entries (DSS) - they're not spacecraft
		if isStationNotSpacecraft(sc.Name) {
			continue
		}
		if sc.ID == m.selectedID {
			b.WriteString(selectedStyle.Render(sc.Name))
		} else {
			b.WriteString(unselectedStyle.Render(sc.Name))
		}
		b.WriteString(" ")
	}

	b.WriteString("→")

	return b.String()
}

func (m MissionDetailModel) renderSpacecraftDetails(sc *dsn.Spacecraft) string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Width(16)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	// Name header - use full name from registry if available
	displayName := sc.Name
	if target, ok := ephem.GetTargetByName(sc.Name); ok {
		displayName = target.Name
	}
	b.WriteString(headerStyle.Render(displayName))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", len(displayName)+4))
	b.WriteString("\n\n")

	// Core metrics
	b.WriteString(labelStyle.Render("Distance:"))
	b.WriteString(valueStyle.Render(dsn.FormatDistance(sc.Distance)))
	b.WriteString("\n")

	// Active links count
	b.WriteString(labelStyle.Render("Active Links:"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", len(sc.Links))))
	b.WriteString("\n\n")

	// Link details
	if len(sc.Links) > 0 {
		b.WriteString(headerStyle.Render("Link Details"))
		b.WriteString("\n")

		for i, link := range sc.Links {
			b.WriteString(fmt.Sprintf("\n  Link %d: %s @ %s\n", i+1, link.AntennaID, link.Complex))

			b.WriteString("    ")
			b.WriteString(labelStyle.Render("Band:"))
			b.WriteString(valueStyle.Render(link.Band))
			b.WriteString("\n")

			b.WriteString("    ")
			b.WriteString(labelStyle.Render("RTLT:"))
			b.WriteString(valueStyle.Render(dsn.FormatRTLT(link.RTLT)))
			b.WriteString("\n")

			b.WriteString("    ")
			b.WriteString(labelStyle.Render("Down Rate:"))
			b.WriteString(valueStyle.Render(dsn.FormatDataRate(link.DownRate)))
			b.WriteString("\n")

			b.WriteString("    ")
			b.WriteString(labelStyle.Render("Up Rate:"))
			b.WriteString(valueStyle.Render(dsn.FormatDataRate(link.UpRate)))
			b.WriteString("\n")

			// Doppler modeling (based on carrier frequency)
			b.WriteString("    ")
			b.WriteString(labelStyle.Render("Doppler:"))
			b.WriteString(valueStyle.Render(m.renderDopplerInfo(link.Band, sc.Distance)))
			b.WriteString("\n")
		}
	}

	// Elevation sparkline
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Elevation"))
	b.WriteString("\n")
	b.WriteString(m.renderElevationSparkline())
	b.WriteString("\n")

	return b.String()
}

// renderDopplerInfo renders Doppler information for a link.
// Since we don't have measured Doppler from DSN, we show model parameters.
func (m MissionDetailModel) renderDopplerInfo(band string, distanceKm float64) string {
	if distanceKm <= 0 {
		return "N/A"
	}

	freq := dsn.GetBandFrequency(band)
	if freq <= 0 {
		return "N/A"
	}

	// Without range rate data, we can only show the carrier frequency
	// Real implementation would compute Doppler from range rate
	return fmt.Sprintf("Model: %s @ %.0f MHz", band, freq)
}

// SparklineWidth is the fixed width of the elevation sparkline.
const SparklineWidth = 48

// sparklineBlocks are the Unicode block characters for sparkline (0 = lowest, 7 = highest).
var sparklineBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// elevColorLow is the color for low elevation (dark blue).
var elevColorLow = [3]uint8{0x1b, 0x2b, 0x4b}

// elevColorMid is the color for mid elevation (blue).
var elevColorMid = [3]uint8{0x34, 0x78, 0xc0}

// elevColorHigh is the color for high elevation (cyan).
var elevColorHigh = [3]uint8{0x8b, 0xe9, 0xff}

// renderElevationSparkline renders the elevation trace as a sparkline.
func (m MissionDetailModel) renderElevationSparkline() string {
	// Check if we have elevation trace data
	if m.snapshot.ElevationTraceLoading {
		return m.renderShimmerSparkline("Loading elevation data...")
	}

	if m.snapshot.ElevationTraceError != nil {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return dimStyle.Render("Error: " + m.snapshot.ElevationTraceError.Error())
	}

	trace := m.snapshot.ElevationTrace
	if trace == nil || len(trace.Samples) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return dimStyle.Render("No DSN geometry available")
	}

	// Resample to fixed width
	samples := resampleElevation(trace.Samples, SparklineWidth)
	if len(samples) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		return dimStyle.Render("No DSN geometry available")
	}

	// Build sparkline with per-cell coloring
	var sb strings.Builder

	// Complex label prefix
	complexLabel := string(m.snapshot.ElevationTraceComplex)
	if complexLabel != "" {
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		sb.WriteString(labelStyle.Render(complexLabel))
		sb.WriteString(" ")
	}

	for _, elev := range samples {
		// Clamp to valid range
		if elev < 0 {
			elev = 0
		}
		if elev > 90 {
			elev = 90
		}

		// Normalize to 0-1 for color (0° = 0, 90° = 1)
		t := elev / 90.0

		// Map to block character (0° = lowest block, 90° = highest)
		blockIdx := int(t * 7.0)
		if blockIdx > 7 {
			blockIdx = 7
		}
		blockChar := sparklineBlocks[blockIdx]

		// Compute color via linear interpolation
		r, g, b := interpolateElevColor(t)
		color := fmt.Sprintf("#%02x%02x%02x", r, g, b)

		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(string(blockChar)))
	}

	// Add current elevation marker and value
	now := time.Now()
	if currentSample := trace.CurrentElevation(now); currentSample != nil {
		nowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		sb.WriteString(nowStyle.Render(fmt.Sprintf(" now: %.0f°", currentSample.Elevation)))
	}

	return sb.String()
}

// renderShimmerSparkline renders a loading animation sparkline.
func (m MissionDetailModel) renderShimmerSparkline(msg string) string {
	var sb strings.Builder

	// Create shimmer effect using animTick
	offset := m.animTick % SparklineWidth
	for i := 0; i < SparklineWidth; i++ {
		// Calculate brightness based on position relative to shimmer wave
		dist := (i - offset + SparklineWidth) % SparklineWidth
		var gray int
		if dist < 8 {
			gray = 60 + dist*8
		} else {
			gray = 60
		}
		color := fmt.Sprintf("#%02x%02x%02x", gray, gray, gray)
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("▄"))
	}

	// Append message
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(" ")
	sb.WriteString(dimStyle.Render(msg))

	return sb.String()
}

// interpolateElevColor returns RGB color for elevation value t in [0, 1].
// Gradient: low (dark blue) → mid (blue) → high (cyan).
func interpolateElevColor(t float64) (uint8, uint8, uint8) {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	var r, g, b uint8
	if t < 0.5 {
		// Interpolate from low to mid
		s := t * 2 // Scale to 0-1
		r = uint8(float64(elevColorLow[0])*(1-s) + float64(elevColorMid[0])*s)
		g = uint8(float64(elevColorLow[1])*(1-s) + float64(elevColorMid[1])*s)
		b = uint8(float64(elevColorLow[2])*(1-s) + float64(elevColorMid[2])*s)
	} else {
		// Interpolate from mid to high
		s := (t - 0.5) * 2 // Scale to 0-1
		r = uint8(float64(elevColorMid[0])*(1-s) + float64(elevColorHigh[0])*s)
		g = uint8(float64(elevColorMid[1])*(1-s) + float64(elevColorHigh[1])*s)
		b = uint8(float64(elevColorMid[2])*(1-s) + float64(elevColorHigh[2])*s)
	}

	return r, g, b
}

// resampleElevation resamples elevation samples to a fixed number of buckets.
func resampleElevation(samples []dsn.ElevationSample, width int) []float64 {
	if len(samples) == 0 || width <= 0 {
		return nil
	}

	result := make([]float64, width)
	samplesPerBucket := float64(len(samples)) / float64(width)

	for i := 0; i < width; i++ {
		// Average samples in this bucket
		startIdx := int(float64(i) * samplesPerBucket)
		endIdx := int(float64(i+1) * samplesPerBucket)
		if endIdx > len(samples) {
			endIdx = len(samples)
		}
		if startIdx >= endIdx {
			startIdx = endIdx - 1
		}
		if startIdx < 0 {
			startIdx = 0
		}

		sum := 0.0
		count := 0
		for j := startIdx; j < endIdx; j++ {
			sum += samples[j].Elevation
			count++
		}
		if count > 0 {
			result[i] = sum / float64(count)
		}
	}

	return result
}

// SelectedSpacecraftID returns the currently selected spacecraft ID.
func (m MissionDetailModel) SelectedSpacecraftID() int {
	return m.selectedID
}

// SetSelectedSpacecraft sets the selected spacecraft by ID.
func (m *MissionDetailModel) SetSelectedSpacecraft(id int) {
	m.selectedID = id
	m.scrollY = 0
}

// UpdatePassPlan updates the pass plan data.
func (m MissionDetailModel) UpdatePassPlan(plan *dsn.PassPlan) MissionDetailModel {
	m.passPlan = plan
	return m
}

// ShowPassPanel returns whether the pass panel is visible.
func (m MissionDetailModel) ShowPassPanel() bool {
	return m.showPassPanel
}

// renderPassPanel renders the pass & handoff panel.
func (m MissionDetailModel) renderPassPanel() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	nowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true)

	nextStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229"))

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("208"))

	// Find selected spacecraft name
	scName := "Unknown"
	for _, sc := range m.snapshot.Spacecraft {
		if sc.ID == m.selectedID {
			scName = sc.Name
			break
		}
	}

	b.WriteString(headerStyle.Render(fmt.Sprintf("PASSES — %s (next 24h)", scName)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n\n")

	// Use pass plan from snapshot (centralized state)
	passPlan := m.snapshot.PassPlan
	if passPlan == nil || len(passPlan.Passes) == 0 {
		if m.snapshot.PassPlanError != nil {
			errStr := m.snapshot.PassPlanError.Error()
			var msg string
			if strings.Contains(errStr, "unknown spacecraft") {
				msg = "  Ephemeris data not available for this mission"
			} else {
				msg = fmt.Sprintf("  %v", m.snapshot.PassPlanError)
			}
			b.WriteString(dimStyle.Render(msg))
		} else if m.snapshot.PassPlanLoading {
			// Show shimmer animation while loading
			b.WriteString("  ")
			b.WriteString(m.renderShimmerText("Computing pass schedule..."))
		} else {
			b.WriteString(dimStyle.Render("  Computing pass schedule..."))
		}
		b.WriteString("\n")
		return b.String()
	}

	// Column headers
	b.WriteString(labelStyle.Render("  COMPLEX   START      PEAK EL   END        SUN SEP   STATUS"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  " + strings.Repeat("─", 58)))
	b.WriteString("\n")

	// Group passes by complex for cleaner display
	complexes := []dsn.Complex{dsn.ComplexGoldstone, dsn.ComplexCanberra, dsn.ComplexMadrid}

	for _, c := range complexes {
		passes := passPlan.GetPassesForComplex(c)
		shortName := dsn.ComplexShortName(c)

		if len(passes) == 0 {
			b.WriteString(fmt.Sprintf("  %-8s  ", shortName))
			b.WriteString(dimStyle.Render("-- no passes --"))
			b.WriteString("\n")
			continue
		}

		for i, p := range passes {
			// Skip past passes for cleaner display (show max 1 past)
			if p.Status == dsn.PassPast && i > 0 {
				continue
			}

			// Complex name (only show for first pass of this complex)
			if i == 0 {
				b.WriteString(fmt.Sprintf("  %-8s  ", shortName))
			} else {
				b.WriteString("            ")
			}

			// Start time
			b.WriteString(valueStyle.Render(p.Start.UTC().Format("15:04")))
			b.WriteString("      ")

			// Peak elevation
			elStr := fmt.Sprintf("%2.0f°", p.MaxElDeg)
			b.WriteString(valueStyle.Render(elStr))
			b.WriteString("       ")

			// End time
			b.WriteString(valueStyle.Render(p.End.UTC().Format("15:04")))
			b.WriteString("      ")

			// Sun separation
			sunStr := fmt.Sprintf("%3.0f°", p.SunMinSep)
			if p.SunMinSep < 10 {
				b.WriteString(warningStyle.Render(sunStr))
			} else {
				b.WriteString(valueStyle.Render(sunStr))
			}
			b.WriteString("      ")

			// Status
			switch p.Status {
			case dsn.PassNow:
				b.WriteString(nowStyle.Render("NOW"))
			case dsn.PassNext:
				b.WriteString(nextStyle.Render("NEXT"))
			case dsn.PassPast:
				b.WriteString(dimStyle.Render("PAST"))
			default:
				b.WriteString(dimStyle.Render("—"))
			}

			b.WriteString("\n")
		}
	}

	// Show next pass summary
	b.WriteString("\n")
	if current := passPlan.GetCurrentPass(); current != nil {
		remaining := time.Until(current.End)
		b.WriteString(nowStyle.Render(fmt.Sprintf("  ▶ Active: %s pass ends in %s",
			dsn.ComplexShortName(current.Complex),
			formatDuration(remaining))))
		b.WriteString("\n")
	}

	if next := passPlan.GetNextPass(); next != nil {
		until := time.Until(next.Start)
		b.WriteString(nextStyle.Render(fmt.Sprintf("  ▷ Next: %s pass in %s",
			dsn.ComplexShortName(next.Complex),
			formatDuration(until))))
		b.WriteString("\n")
	}

	return b.String()
}

// isStationNotSpacecraft returns true if the name is a station designator, not a spacecraft.
func isStationNotSpacecraft(name string) bool {
	// DSS (Deep Space Station) entries are stations, not spacecraft
	// They sometimes appear in DSN data but aren't useful for pass planning
	upper := strings.ToUpper(name)
	return upper == "DSS" || strings.HasPrefix(upper, "DSS-") || strings.HasPrefix(upper, "DSS ")
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// renderShimmerText renders text with a subtle moving shine effect.
func (m MissionDetailModel) renderShimmerText(text string) string {
	runes := []rune(text)
	textLen := len(runes)
	if textLen == 0 {
		return ""
	}

	// Shimmer sweeps smoothly across
	pos := m.animTick % (textLen + 8) // A bit of padding for smooth entry/exit

	var result strings.Builder

	for i, r := range runes {
		// Distance from shimmer center
		dist := i - pos + 4
		if dist < 0 {
			dist = -dist
		}

		// Subtle purple gradient - gentle highlight that fades smoothly
		// Base is dim purple, highlight is brighter lavender
		var r8, g8, b8 int
		if dist <= 1 {
			// Soft highlight - light lavender
			r8, g8, b8 = 180, 160, 220
		} else if dist <= 3 {
			// Mid transition
			r8, g8, b8 = 140, 120, 180
		} else if dist <= 5 {
			// Fading
			r8, g8, b8 = 110, 90, 150
		} else {
			// Base dim purple
			r8, g8, b8 = 80, 70, 120
		}

		hexColor := fmt.Sprintf("#%02X%02X%02X", r8, g8, b8)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor))
		result.WriteString(style.Render(string(r)))
	}

	return result.String()
}
