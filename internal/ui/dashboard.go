package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

// Styles for the dashboard
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Background(lipgloss.Color("235"))

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	// Complex status styles
	complexNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#d0c8ff"))

	statusGlyphStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#d0c8ff"))

	missionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d0c8ff"))

	stationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6a6a7a"))
)

// DashboardModel is the control room dashboard view.
type DashboardModel struct {
	width      int
	height     int
	cursor     int
	snapshot   state.Snapshot
	spacecraft []dsn.SpacecraftView // grouped spacecraft with their links
	lastErr    error
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel() DashboardModel {
	return DashboardModel{}
}

// Init implements the Bubble Tea model interface.
func (m DashboardModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the viewport size.
func (m DashboardModel) SetSize(width, height int) DashboardModel {
	m.width = width
	m.height = height
	return m
}

// UpdateData updates the model with new data.
func (m DashboardModel) UpdateData(snapshot state.Snapshot) DashboardModel {
	m.snapshot = snapshot

	// Build spacecraft views (grouped, filtered)
	elevMap := dsn.BuildElevationMap(snapshot.Data)
	m.spacecraft = dsn.BuildSpacecraftViews(snapshot.Data, elevMap)

	// Clamp cursor to valid range
	if m.cursor >= len(m.spacecraft) {
		m.cursor = max(0, len(m.spacecraft)-1)
	}

	return m
}

// SetError sets the last error for display.
func (m DashboardModel) SetError(err error) DashboardModel {
	m.lastErr = err
	return m
}

// Update handles messages.
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		scCount := len(m.spacecraft)

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < scCount-1 {
				m.cursor++
			}
		case "home":
			m.cursor = 0
		case "end":
			if scCount > 0 {
				m.cursor = scCount - 1
			}
		}
	}

	return m, nil
}

// View renders the dashboard.
func (m DashboardModel) View() string {
	var b strings.Builder

	// Show error state if present
	if m.lastErr != nil {
		b.WriteString(errorStyle.Render("Error: " + m.lastErr.Error()))
		b.WriteString("\n\n")
	}

	// Show loading state
	if m.snapshot.Data == nil && m.lastErr == nil {
		b.WriteString("Waiting for DSN data...\n")
		return b.String()
	}

	// Complex load summary
	b.WriteString(m.renderComplexSummary())
	b.WriteString("\n\n")

	// Active links table
	b.WriteString(m.renderLinksTable())

	return b.String()
}

// Complex status constants
const (
	statusLookbackWindow = 120 * time.Second

	glyphStable   = "◎"
	glyphUp       = "▲"
	glyphDown     = "▽"
	glyphShifting = "◆"

	labelStable   = "stable"
	labelUp       = "up"
	labelDown     = "down"
	labelShifting = "shifting"
)

func (m DashboardModel) renderComplexSummary() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("DSN Complex Status"))
	b.WriteString("\n")

	complexes := []dsn.Complex{dsn.ComplexGoldstone, dsn.ComplexCanberra, dsn.ComplexMadrid}

	for _, c := range complexes {
		info := dsn.KnownComplexes[c]

		// Get status glyph and label based on recent events
		glyph, label := m.classifyComplexStatus(c)

		// Format: "Goldstone   ◎ stable"
		name := fmt.Sprintf("%-10s", info.Name)
		statusLine := complexNameStyle.Render(name) + "  " +
			statusGlyphStyle.Render(glyph+" "+label)
		b.WriteString("  " + statusLine + "\n")

		// Format: "    → JWST@DSS26, MRO@DSS36"
		missionLine := m.buildMissionLine(c)
		b.WriteString("    " + stationStyle.Render("→") + " " + missionLine + "\n")
	}

	return b.String()
}

// classifyComplexStatus determines the status glyph and label for a complex
// based on recent events within the lookback window.
// Priority: shifting (HANDOFF) > down (LINK_LOST) > up (NEW_LINK/LINK_RESUMED) > stable
func (m DashboardModel) classifyComplexStatus(c dsn.Complex) (glyph, label string) {
	cutoff := time.Now().Add(-statusLookbackWindow)
	complexID := string(c)

	hasHandoff := false
	hasLinkLost := false
	hasLinkUp := false

	for _, event := range m.snapshot.Events {
		if event.Timestamp.Before(cutoff) {
			continue
		}

		// Check if this event involves the complex
		eventComplex := event.Complex
		involvesComplex := eventComplex == complexID

		// For handoffs, check both old and new stations
		if event.Type == state.EventHandoff {
			if eventComplex == complexID ||
				complexFromStation(event.OldStation) == complexID ||
				complexFromStation(event.NewStation) == complexID {
				hasHandoff = true
			}
			continue
		}

		if !involvesComplex {
			continue
		}

		switch event.Type {
		case state.EventLinkLost:
			hasLinkLost = true
		case state.EventNewLink, state.EventLinkResumed:
			hasLinkUp = true
		}
	}

	// Priority order
	switch {
	case hasHandoff:
		return glyphShifting, labelShifting
	case hasLinkLost:
		return glyphDown, labelDown
	case hasLinkUp:
		return glyphUp, labelUp
	default:
		return glyphStable, labelStable
	}
}

// complexFromStation extracts the complex ID from a station name like "mdscc" or antenna ID like "DSS55"
func complexFromStation(station string) string {
	if station == "" {
		return ""
	}
	// Direct complex name
	switch station {
	case "gdscc", "cdscc", "mdscc":
		return station
	}
	// Antenna ID like DSS55 -> mdscc (5x, 6x = Madrid)
	if strings.HasPrefix(station, "DSS") && len(station) >= 4 {
		digit := station[3]
		switch digit {
		case '1', '2':
			return "gdscc"
		case '3', '4':
			return "cdscc"
		case '5', '6':
			return "mdscc"
		}
	}
	return ""
}

// missionEntry represents a mission@station pair for sorting
type missionEntry struct {
	mission   string
	antennaID string
	dssNum    int
}

// buildMissionLine creates the formatted "Mission@DSSxx, ..." line for a complex
func (m DashboardModel) buildMissionLine(c dsn.Complex) string {
	if m.snapshot.Data == nil {
		return stationStyle.Render("(none)")
	}

	var entries []missionEntry

	for _, link := range m.snapshot.Data.Links {
		if link.Complex != c {
			continue
		}
		if !dsn.IsRealSpacecraft(link.Spacecraft) {
			continue
		}

		// Extract DSS number for sorting
		dssNum := 0
		if strings.HasPrefix(link.AntennaID, "DSS") {
			if num, err := strconv.Atoi(link.AntennaID[3:]); err == nil {
				dssNum = num
			}
		}

		entries = append(entries, missionEntry{
			mission:   link.Spacecraft,
			antennaID: link.AntennaID,
			dssNum:    dssNum,
		})
	}

	if len(entries) == 0 {
		return stationStyle.Render("(none)")
	}

	// Sort by mission name, then by DSS number
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].mission != entries[j].mission {
			return entries[i].mission < entries[j].mission
		}
		return entries[i].dssNum < entries[j].dssNum
	})

	// Build formatted string
	var parts []string
	for _, e := range entries {
		part := missionStyle.Render(e.mission) + stationStyle.Render("@"+e.antennaID)
		parts = append(parts, part)
	}

	return strings.Join(parts, stationStyle.Render(", "))
}

func (m DashboardModel) renderUtilizationBar(util float64, width int) string {
	filled := int(util * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	// Subtle purple activity bar (matches logo palette)
	fillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5A189A"))  // royal purple
	emptyStyle := lipgloss.NewStyle().Background(lipgloss.Color("#10002B")) // near black purple

	filledPart := fillStyle.Render(strings.Repeat("█", filled))
	emptyPart := emptyStyle.Render(strings.Repeat(" ", empty))

	return "[" + filledPart + emptyPart + "]"
}

// Column widths for table alignment
const (
	colAntenna  = 7
	colBand     = 4
	colRate     = 10
	colDistance = 11
	colStruggle = 8
)

// renderColumnHeader renders the column labels for the antenna detail rows.
func (m DashboardModel) renderColumnHeader() string {
	// Align with bullet rows: "  • " prefix (4 chars) then columns
	line := fmt.Sprintf("    %s  %s  %s  %s  %s",
		pad("Station", colAntenna),
		pad("Band", colBand),
		pad("Rate", colRate),
		pad("Distance", colDistance),
		"Struggle",
	)
	return headerStyle.Render(line)
}

func (m DashboardModel) renderLinksTable() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Active Spacecraft"))
	b.WriteString("\n")

	if len(m.spacecraft) == 0 {
		b.WriteString("  No active spacecraft\n")
		return b.String()
	}

	// Column header row
	b.WriteString(m.renderColumnHeader())
	b.WriteString("\n")

	// Calculate visible spacecraft based on height
	// Each spacecraft takes 1 header line + N link lines
	maxSpacecraft := m.height - 10
	if maxSpacecraft < 3 {
		maxSpacecraft = 3
	}

	startIdx := 0
	if m.cursor >= maxSpacecraft {
		startIdx = m.cursor - maxSpacecraft + 1
	}

	endIdx := startIdx + maxSpacecraft
	if endIdx > len(m.spacecraft) {
		endIdx = len(m.spacecraft)
	}

	for i := startIdx; i < endIdx; i++ {
		sc := m.spacecraft[i]
		isSelected := i == m.cursor

		// Spacecraft header row
		headerLine := m.renderSpacecraftHeader(sc, isSelected)
		b.WriteString(headerLine)
		b.WriteString("\n")

		// Per-antenna detail lines
		for _, link := range sc.Links {
			detailLine := m.renderLinkDetail(link, isSelected)
			b.WriteString(detailLine)
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(m.spacecraft) > maxSpacecraft {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
		b.WriteString(dimStyle.Render(fmt.Sprintf("\n  Showing %d-%d of %d spacecraft", startIdx+1, endIdx, len(m.spacecraft))))
	}

	return b.String()
}

// renderSpacecraftHeader renders the header line for a spacecraft.
func (m DashboardModel) renderSpacecraftHeader(sc dsn.SpacecraftView, selected bool) string {
	// Format: "VGR2  Voyager 2" or just "JWST  James Webb Space Telescope"
	name := sc.Name
	if name == sc.Code {
		name = "" // Don't repeat if same
	}

	var line string
	if name != "" {
		line = fmt.Sprintf("%s  %s", sc.Code, name)
	} else {
		line = sc.Code
	}

	if selected {
		return selectedRowStyle.Render("▶ " + line)
	}
	return missionStyle.Render("  " + line)
}

// renderLinkDetail renders a single antenna link line.
func (m DashboardModel) renderLinkDetail(link dsn.LinkView, selected bool) string {
	band := link.Band
	if band == "" {
		band = "-"
	}

	// Format: "  • DSS34   X   344 bps   21.3 B km   ▃▃▃▃▃"
	line := fmt.Sprintf("  • %s  %s  %s  %s  %s",
		pad(link.Station, colAntenna),
		pad(band, colBand),
		pad(dsn.FormatDataRate(link.Rate), colRate),
		pad(dsn.FormatDistance(link.DistanceKm), colDistance),
		m.renderStruggleBar(link.Struggle),
	)

	if selected {
		// Slightly dimmer than header but still highlighted
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("223"))
		return style.Render(line)
	}
	return stationStyle.Render(line)
}

func (m DashboardModel) buildElevationMap() map[string]float64 {
	elevMap := make(map[string]float64)
	if m.snapshot.Data == nil {
		return elevMap
	}
	for _, station := range m.snapshot.Data.Stations {
		for _, ant := range station.Antennas {
			elevMap[ant.ID] = ant.Elevation
		}
	}
	return elevMap
}

func (m DashboardModel) renderStruggleBar(struggle float64) string {
	// 5-char fill-style bar: ███░░
	const barWidth = 5
	filled := int(struggle * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	empty := barWidth - filled

	fillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#d0c8ff"))  // light purple
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3a3a4a")) // dim gray

	filledPart := fillStyle.Render(strings.Repeat("█", filled))
	emptyPart := emptyStyle.Render(strings.Repeat("░", empty))

	return filledPart + emptyPart
}

// GetSelectedSpacecraft returns the currently selected spacecraft, if any.
func (m DashboardModel) GetSelectedSpacecraft() *dsn.SpacecraftView {
	if len(m.spacecraft) == 0 {
		return nil
	}
	if m.cursor < 0 || m.cursor >= len(m.spacecraft) {
		return nil
	}
	return &m.spacecraft[m.cursor]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// pad truncates or pads a string to exactly the given width.
func pad(s string, width int) string {
	if len(s) > width {
		if width <= 3 {
			return s[:width]
		}
		return s[:width-3] + "..."
	}
	return s + strings.Repeat(" ", width-len(s))
}
