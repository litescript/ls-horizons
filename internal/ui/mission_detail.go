package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/ephem"
	"github.com/litescript/ls-horizons/internal/missions"
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

	// Spotlight phase tracking (for transient highlight on phase change)
	spotlightPhase     string // Last observed phase name
	spotlightHighlight int    // Ticks remaining for phase-change highlight

	// Ephemeris estimate cache (for range/light-time when DSN data unavailable)
	ephemPoint      ephem.EphemerisPoint
	ephemForID      int         // Spacecraft ID this estimate is for
	ephemForComplex dsn.Complex // Observer complex used for estimate
	ephemFetchedAt  time.Time   // When the estimate was fetched
	ephemErr        error       // Error from last fetch attempt
	ephemLoading    bool        // True if fetch is in progress
}

// EphemCacheTTL is how long to use cached ephemeris data before refetching.
const EphemCacheTTL = 60 * time.Second

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

// SpotlightHighlightTicks is how many animation ticks (80ms each) a phase-change
// highlight lasts. 25 ticks ≈ 2 seconds.
const SpotlightHighlightTicks = 25

// SetAnimTick updates the animation tick for shimmer effects and spotlight tracking.
func (m MissionDetailModel) SetAnimTick(tick int) MissionDetailModel {
	m.animTick = tick

	// Decrement spotlight highlight counter
	if m.spotlightHighlight > 0 {
		m.spotlightHighlight--
	}

	// Track phase changes for the selected spacecraft
	for i := range m.snapshot.Spacecraft {
		if m.snapshot.Spacecraft[i].ID == m.selectedID {
			if st := missions.BuildSpotlightState(time.Now(), &m.snapshot.Spacecraft[i]); st != nil {
				if m.spotlightPhase != "" && st.CurrentPhase != m.spotlightPhase {
					m.spotlightHighlight = SpotlightHighlightTicks
				}
				m.spotlightPhase = st.CurrentPhase
			}
			break
		}
	}

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
	// If no valid selection, select first valid spacecraft
	if m.selectedID < 0 {
		for _, sc := range m.snapshot.Spacecraft {
			if !isStationNotSpacecraft(sc.Name) {
				m.selectedID = sc.ID
				m.scrollY = 0
				return
			}
		}
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
	// If no valid selection, select first valid spacecraft
	if m.selectedID < 0 {
		for _, sc := range m.snapshot.Spacecraft {
			if !isStationNotSpacecraft(sc.Name) {
				m.selectedID = sc.ID
				m.scrollY = 0
				return
			}
		}
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

	// Mission spotlight (if this spacecraft has a curated profile)
	if spotlight := m.renderMissionSpotlight(selected); spotlight != "" {
		b.WriteString(spotlight)
		b.WriteString("\n")
	}

	// Spacecraft details
	b.WriteString(m.renderSpacecraftDetails(selected))

	// Propagation delay visualizer
	if propPanel := m.renderPropagationPanel(selected, m.width); propPanel != "" {
		b.WriteString("\n")
		b.WriteString(propPanel)
	}

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

// renderMissionSpotlight renders the mission spotlight panel for a spacecraft
// with a curated mission profile. Returns "" if no profile matches.
func (m MissionDetailModel) renderMissionSpotlight(sc *dsn.Spacecraft) string {
	st := missions.BuildSpotlightState(time.Now(), sc)
	if st == nil {
		return ""
	}

	var b strings.Builder
	p := st.Profile
	w := m.width
	if w <= 0 {
		w = 80
	}

	accentStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(p.Accent.Primary))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Accent.Secondary))

	dimAccent := lipgloss.NewStyle().
		Foreground(lipgloss.Color(p.Accent.Dim))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	// ── Header bar: ━━ MISSION NAME ★ ━━━━━━━━━━━━━━━━
	headerText := " " + p.DisplayName + " "
	if p.Crewed {
		headerText += missions.CrewBadge(true) + " "
	}
	barRemain := w - len(headerText) - 4
	if barRemain < 2 {
		barRemain = 2
	}
	bar := "━━" + headerText + strings.Repeat("━", barRemain)
	b.WriteString(accentStyle.Render(bar))
	b.WriteString("\n")

	// ── Subtitle
	b.WriteString("  ")
	b.WriteString(subtitleStyle.Render(truncate(p.Subtitle, w-4)))
	b.WriteString("\n")

	// ── Crew (if present and terminal wide enough)
	if len(p.Crew) > 0 && w >= 50 {
		crewStr := "Crew: " + strings.Join(p.Crew, " \u00b7 ")
		b.WriteString("  ")
		b.WriteString(dimAccent.Render(truncate(crewStr, w-4)))
		b.WriteString("\n")
	}

	// ── Hero text (only if wide enough)
	if p.HeroText != "" && w >= 70 {
		b.WriteString("  ")
		b.WriteString(dimAccent.Render(truncate(p.HeroText, w-4)))
		b.WriteString("\n")
	}

	// ── Phase line with optional highlight on phase change
	phaseStyle := valueStyle
	if m.spotlightHighlight > 0 {
		// Fade from bright accent to normal over the highlight period
		if m.spotlightHighlight > SpotlightHighlightTicks/2 {
			phaseStyle = accentStyle
		} else {
			phaseStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.Accent.Primary))
		}
	}

	b.WriteString("  ")
	b.WriteString(labelStyle.Render("Phase: "))
	b.WriteString(phaseStyle.Render(st.CurrentPhase))

	// MET or countdown on same line (right side)
	if !st.IsPreLaunch && !st.IsComplete {
		b.WriteString("  ")
		b.WriteString(dimAccent.Render(missions.FormatMET(st.MET)))
	} else if st.IsComplete {
		b.WriteString("  ")
		b.WriteString(dimAccent.Render(missions.FormatMET(st.MET)))
	}
	b.WriteString("\n")

	// ── Next event line with countdown threshold coloring
	if st.NextEvent != nil {
		countdownStyle := m.countdownStyle(st.Countdown, p)
		b.WriteString("  ")
		b.WriteString(labelStyle.Render("Next:  "))
		b.WriteString(valueStyle.Render(st.NextEvent.Name))
		b.WriteString("  ")
		b.WriteString(countdownStyle.Render(missions.FormatCountdown(st.Countdown)))
		b.WriteString("\n")
	}

	// ── Timeline rail
	b.WriteString("  ")
	b.WriteString(m.renderTimelineRail(st, w-4))
	b.WriteString("\n")

	// ── Provenance indicator
	if st.Provenance == missions.ProvenanceCurated {
		provenanceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("239")).
			Italic(true)
		b.WriteString("  ")
		b.WriteString(provenanceStyle.Render(missions.ProvenanceLabel(st.Provenance)))
		b.WriteString("\n")
	}

	return b.String()
}

// countdownStyle returns a style for the countdown based on threshold proximity.
func (m MissionDetailModel) countdownStyle(d time.Duration, p *missions.MissionProfile) lipgloss.Style {
	switch {
	case d < 10*time.Minute:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.Accent.Primary))
	case d < time.Hour:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(p.Accent.Primary))
	case d < 24*time.Hour:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(p.Accent.Secondary))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(p.Accent.Dim))
	}
}

// renderTimelineRail renders a compact horizontal timeline of mission events.
func (m MissionDetailModel) renderTimelineRail(st *missions.SpotlightState, maxWidth int) string {
	if len(st.Timeline) == 0 {
		return ""
	}

	p := st.Profile
	pastStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Accent.Dim))
	currentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.Accent.Primary))
	futureStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	connPast := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Accent.Dim))
	connFuture := lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	var b strings.Builder
	for i, item := range st.Timeline {
		if i > 0 {
			// Connector style matches the segment we're entering
			conn := connFuture
			if item.Status != missions.TimelineFuture {
				conn = connPast
			}
			b.WriteString(conn.Render(" \u2500\u2500 "))
		}

		var glyph string
		var style lipgloss.Style
		switch item.Status {
		case missions.TimelinePast:
			glyph = "\u25cf" // ●
			style = pastStyle
		case missions.TimelineCurrent:
			glyph = "\u25c9" // ◉
			style = currentStyle
		default:
			glyph = "\u25cb" // ○
			style = futureStyle
		}

		key := item.Key
		// Abbreviate keys for narrow terminals
		if maxWidth < 60 && len(key) > 3 {
			key = key[:3]
		}

		b.WriteString(style.Render(glyph + " " + key))
	}

	return b.String()
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	ephemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("139")) // Muted purple for ephemeris data

	// Core metrics
	b.WriteString(labelStyle.Render("Distance:"))
	if sc.Distance > 0 {
		b.WriteString(valueStyle.Render(dsn.FormatDistance(sc.Distance)))
	} else {
		b.WriteString(dimStyle.Render("Awaiting DSN range data..."))
		// Show ephemeris estimate if available
		if ephemPoint, ephemComplex, ok := m.GetEphemEstimate(sc.ID); ok {
			b.WriteString("\n")
			b.WriteString(labelStyle.Render(""))
			b.WriteString(ephemStyle.Render(fmt.Sprintf("Horizons est. (%s): %s",
				dsn.ComplexShortName(ephemComplex),
				dsn.FormatDistance(ephemPoint.RangeKm))))
		}
	}
	b.WriteString("\n")

	// Active links count
	b.WriteString(labelStyle.Render("Active Links:"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", len(sc.Links))))
	b.WriteString("\n\n")

	// Link details (compact: 2 lines per link)
	if len(sc.Links) > 0 {
		b.WriteString(headerStyle.Render("Link Details"))
		b.WriteString("\n")

		compactLabel := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

		for i, link := range sc.Links {
			b.WriteString(fmt.Sprintf("\n  Link %d: %s @ %s\n", i+1, link.AntennaID, link.Complex))
			b.WriteString("    ")
			b.WriteString(compactLabel.Render("Band: "))
			b.WriteString(valueStyle.Render(link.Band))
			b.WriteString("  ")
			b.WriteString(compactLabel.Render("RTLT: "))
			b.WriteString(valueStyle.Render(dsn.FormatRTLT(link.RTLT)))
			b.WriteString("  ")
			b.WriteString(compactLabel.Render("Down: "))
			b.WriteString(valueStyle.Render(dsn.FormatDataRate(link.DownRate)))
			b.WriteString("  ")
			b.WriteString(compactLabel.Render("Up: "))
			b.WriteString(valueStyle.Render(dsn.FormatDataRate(link.UpRate)))
			b.WriteString("  ")
			b.WriteString(compactLabel.Render("Doppler: "))
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
		if m.selectedSpacecraftIsCurated() {
			return dimStyle.Render("Elevation data not available for this mission profile")
		}
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

// selectedSpacecraftIsCurated returns true if the currently selected spacecraft
// has a curated mission profile, indicating it may not have full ephemeris support.
func (m MissionDetailModel) selectedSpacecraftIsCurated() bool {
	for _, sc := range m.snapshot.Spacecraft {
		if sc.ID == m.selectedID {
			return missions.HasSpotlight(sc.Name)
		}
	}
	return false
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

// UpdateEphemEstimate updates the cached ephemeris estimate.
func (m MissionDetailModel) UpdateEphemEstimate(spacecraftID int, complex dsn.Complex, point ephem.EphemerisPoint, fetchedAt time.Time, err error) MissionDetailModel {
	m.ephemForID = spacecraftID
	m.ephemForComplex = complex
	m.ephemPoint = point
	m.ephemFetchedAt = fetchedAt
	m.ephemErr = err
	m.ephemLoading = false
	return m
}

// SetEphemLoading sets the loading state for ephemeris fetch.
func (m MissionDetailModel) SetEphemLoading(loading bool) MissionDetailModel {
	m.ephemLoading = loading
	return m
}

// NeedsEphemRefresh returns true if we should fetch new ephemeris data.
// Conditions: different spacecraft/complex, or cache expired, and not currently loading.
func (m MissionDetailModel) NeedsEphemRefresh(spacecraftID int, complex dsn.Complex) bool {
	if m.ephemLoading {
		return false
	}
	if m.ephemForID != spacecraftID || m.ephemForComplex != complex {
		return true
	}
	if time.Since(m.ephemFetchedAt) > EphemCacheTTL {
		return true
	}
	return false
}

// GetEphemEstimate returns the cached ephemeris estimate if valid for the given spacecraft.
func (m MissionDetailModel) GetEphemEstimate(spacecraftID int) (ephem.EphemerisPoint, dsn.Complex, bool) {
	if m.ephemForID != spacecraftID {
		return ephem.EphemerisPoint{}, "", false
	}
	if !m.ephemPoint.Valid || !m.ephemPoint.HasRangeData {
		return ephem.EphemerisPoint{}, "", false
	}
	return m.ephemPoint, m.ephemForComplex, true
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
			var msg string
			if m.selectedSpacecraftIsCurated() {
				msg = "  Pass predictions not available for this mission profile"
			} else {
				errStr := m.snapshot.PassPlanError.Error()
				if strings.Contains(errStr, "unknown spacecraft") {
					msg = "  Ephemeris data not available for this mission"
				} else {
					msg = fmt.Sprintf("  %v", m.snapshot.PassPlanError)
				}
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
		if until <= 0 {
			// Pass is starting now
			b.WriteString(nextStyle.Render(fmt.Sprintf("  ▷ Next: %s pass starting now",
				dsn.ComplexShortName(next.Complex))))
		} else {
			b.WriteString(nextStyle.Render(fmt.Sprintf("  ▷ Next: %s pass in %s",
				dsn.ComplexShortName(next.Complex),
				formatDuration(until))))
		}
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

// PropagationAnimPeriod is the number of animation ticks for one full pulse traversal.
// At 80ms per tick, 31 ticks ≈ 2.5 seconds.
const PropagationAnimPeriod = 31

// renderPropagationPanel renders the signal propagation delay visualizer.
// It shows one-way and round-trip light times with animated pulses traveling
// between Earth and the spacecraft.
func (m MissionDetailModel) renderPropagationPanel(sc *dsn.Spacecraft, width int) string {
	if sc == nil {
		return ""
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Find the best link (highest DataRate, fallback to first)
	var bestLink *dsn.Link
	for i := range sc.Links {
		if bestLink == nil || sc.Links[i].DataRate > bestLink.DataRate {
			bestLink = &sc.Links[i]
		}
	}

	// Try to get RTLT: first from link, then calculate from distance
	var rtlt float64
	var usingEphemeris bool
	var ephemComplex dsn.Complex

	if bestLink != nil && bestLink.RTLT > 0 {
		rtlt = bestLink.RTLT
	} else if sc.Distance > 0 {
		// Calculate RTLT from distance: rtlt = 2 * distance / speed_of_light
		rtlt = (sc.Distance / dsn.SpeedOfLight) * 2
	} else if bestLink != nil && bestLink.Distance > 0 {
		rtlt = (bestLink.Distance / dsn.SpeedOfLight) * 2
	}

	// If no DSN data, try ephemeris fallback
	if rtlt <= 0 {
		if ephemPoint, complex, ok := m.GetEphemEstimate(sc.ID); ok {
			// Use ephemeris one-way light time (in minutes), convert to RTLT in seconds
			rtlt = ephemPoint.OneWayLTMin * 60 * 2 // Convert one-way minutes to round-trip seconds
			usingEphemeris = true
			ephemComplex = complex
		}
	}

	// Show header with appropriate message if no data available
	if bestLink == nil {
		var b strings.Builder
		b.WriteString(headerStyle.Render("Propagation"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  No active link"))
		b.WriteString("\n")
		return b.String()
	}

	if rtlt <= 0 {
		var b strings.Builder
		b.WriteString(headerStyle.Render("Propagation"))
		b.WriteString("\n")
		if m.ephemLoading {
			b.WriteString(dimStyle.Render("  Loading ephemeris..."))
		} else {
			b.WriteString(dimStyle.Render("  Awaiting range data..."))
		}
		b.WriteString("\n")
		return b.String()
	}

	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	earthStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	scStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)

	upPulseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true)

	downPulseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	// Calculate light times (rtlt already computed above)
	oneWay := rtlt / 2 // One-way light time
	oneWayDur := time.Duration(oneWay * float64(time.Second))
	rtltDur := time.Duration(rtlt * float64(time.Second))

	// Use snapshot timestamp as "now" for consistency
	now := time.Now()
	if m.snapshot.Data != nil && !m.snapshot.Data.Timestamp.IsZero() {
		now = m.snapshot.Data.Timestamp
	}

	ephemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("139")) // Muted purple for ephemeris data

	// Header
	b.WriteString(headerStyle.Render("Propagation"))
	if usingEphemeris {
		b.WriteString(ephemStyle.Render(fmt.Sprintf(" (Horizons est. via %s)", dsn.ComplexShortName(ephemComplex))))
	}
	b.WriteString("\n")

	// Time displays
	b.WriteString(labelStyle.Render("  One-way:     "))
	if usingEphemeris {
		b.WriteString(ephemStyle.Render(formatLightTime(oneWayDur)))
	} else {
		b.WriteString(valueStyle.Render(formatLightTime(oneWayDur)))
	}
	b.WriteString(labelStyle.Render("    Round-trip: "))
	if usingEphemeris {
		b.WriteString(ephemStyle.Render(formatLightTime(rtltDur)))
	} else {
		b.WriteString(valueStyle.Render(formatLightTime(rtltDur)))
	}
	b.WriteString("\n")

	// Arrival/reception times
	arrivalTime := now.Add(oneWayDur)
	telemetryTime := now.Add(-oneWayDur)

	b.WriteString(labelStyle.Render("  Transmit now → arrives: "))
	if usingEphemeris {
		b.WriteString(ephemStyle.Render("~" + arrivalTime.Format("15:04:05")))
	} else {
		b.WriteString(valueStyle.Render(arrivalTime.Format("15:04:05")))
	}
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("  Seeing telemetry from:  "))
	if usingEphemeris {
		b.WriteString(ephemStyle.Render("~" + telemetryTime.Format("15:04:05")))
	} else {
		b.WriteString(valueStyle.Render(telemetryTime.Format("15:04:05")))
	}
	b.WriteString("\n\n")

	// Calculate bar width: clamp to 20..60, leave room for labels
	barWidth := width - 20 // Account for "Earth [" and "] SC" labels
	if barWidth < 20 {
		barWidth = 20
	}
	if barWidth > 60 {
		barWidth = 60
	}

	// Animation position (0.0 to 1.0)
	animPos := float64(m.animTick%PropagationAnimPeriod) / float64(PropagationAnimPeriod)

	// Render uplink bar (Earth → Spacecraft, dot moves left to right)
	b.WriteString("  ")
	b.WriteString(earthStyle.Render("Earth"))
	b.WriteString(" ")
	b.WriteString(dimStyle.Render("["))
	b.WriteString(renderPulseBar(barWidth, animPos, true, upPulseStyle, dimStyle))
	b.WriteString(dimStyle.Render("]"))
	b.WriteString(" ")
	b.WriteString(scStyle.Render("SC"))
	b.WriteString(labelStyle.Render("  ↑ cmd"))
	b.WriteString("\n")

	// Render downlink bar (Spacecraft → Earth, dot moves right to left)
	// Offset downlink animation by half period for visual interest
	downPos := animPos + 0.5
	if downPos >= 1.0 {
		downPos -= 1.0
	}
	b.WriteString("  ")
	b.WriteString(earthStyle.Render("Earth"))
	b.WriteString(" ")
	b.WriteString(dimStyle.Render("["))
	b.WriteString(renderPulseBar(barWidth, downPos, false, downPulseStyle, dimStyle))
	b.WriteString(dimStyle.Render("]"))
	b.WriteString(" ")
	b.WriteString(scStyle.Render("SC"))
	b.WriteString(labelStyle.Render("  ↓ tlm"))
	b.WriteString("\n")

	return b.String()
}

// renderPulseBar renders an ASCII bar with a moving pulse dot.
// If leftToRight is true, the dot moves from left to right; otherwise right to left.
func renderPulseBar(width int, pos float64, leftToRight bool, pulseStyle, dimStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}

	// Calculate dot position
	dotPos := int(pos * float64(width))
	if !leftToRight {
		dotPos = width - 1 - dotPos
	}

	// Clamp dot position
	if dotPos < 0 {
		dotPos = 0
	}
	if dotPos >= width {
		dotPos = width - 1
	}

	var b strings.Builder
	for i := 0; i < width; i++ {
		if i == dotPos {
			b.WriteString(pulseStyle.Render("●"))
		} else if i == dotPos-1 || i == dotPos+1 {
			// Subtle trail/lead effect
			b.WriteString(dimStyle.Render("·"))
		} else {
			b.WriteString(dimStyle.Render("─"))
		}
	}

	return b.String()
}

// formatLightTime formats a duration as mm:ss or hh:mm:ss for light time display.
func formatLightTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	totalSec := int(d.Seconds())
	hours := totalSec / 3600
	mins := (totalSec % 3600) / 60
	secs := totalSec % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d", mins, secs)
}
