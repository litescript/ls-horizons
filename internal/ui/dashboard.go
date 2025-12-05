package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/peter/ls-horizons/internal/dsn"
	"github.com/peter/ls-horizons/internal/state"
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
	width    int
	height   int
	cursor   int
	snapshot state.Snapshot
	lastErr  error
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
		linkCount := 0
		if m.snapshot.Data != nil {
			linkCount = len(m.snapshot.Data.Links)
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < linkCount-1 {
				m.cursor++
			}
		case "home":
			m.cursor = 0
		case "end":
			if linkCount > 0 {
				m.cursor = linkCount - 1
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
	colStation    = 8
	colAntenna    = 7
	colSpacecraft = 12
	colBand       = 6
	colRate       = 10
	colDistance   = 11
	colStruggle   = 8
)

func (m DashboardModel) renderLinksTable() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Active Links"))
	b.WriteString("\n")

	// Table header
	header := pad("Station", colStation) + " " +
		pad("Antenna", colAntenna) + " " +
		pad("Spacecraft", colSpacecraft) + " " +
		pad("Band", colBand) + " " +
		pad("Rate", colRate) + " " +
		pad("Distance", colDistance) + " " +
		pad("Struggle", colStruggle)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	if m.snapshot.Data == nil || len(m.snapshot.Data.Links) == 0 {
		b.WriteString("  No active links\n")
		return b.String()
	}

	// Build antenna elevation map for struggle index
	elevationMap := m.buildElevationMap()

	// Calculate visible rows based on height
	maxRows := m.height - 10 // Leave room for header and summary
	if maxRows < 5 {
		maxRows = 5
	}

	links := m.snapshot.Data.Links
	startIdx := 0
	if m.cursor >= maxRows {
		startIdx = m.cursor - maxRows + 1
	}

	endIdx := startIdx + maxRows
	if endIdx > len(links) {
		endIdx = len(links)
	}

	for i := startIdx; i < endIdx; i++ {
		link := links[i]

		// Get elevation for struggle index
		elevation := elevationMap[link.AntennaID]
		struggle := dsn.StruggleIndex(link, elevation)

		band := link.Band
		if band == "" {
			band = "-"
		}

		row := pad(string(link.Complex), colStation) + " " +
			pad(link.AntennaID, colAntenna) + " " +
			pad(link.Spacecraft, colSpacecraft) + " " +
			pad(band, colBand) + " " +
			pad(dsn.FormatDataRate(link.DataRate), colRate) + " " +
			pad(dsn.FormatDistance(link.Distance), colDistance) + " " +
			m.renderStruggleBar(struggle)

		if i == m.cursor {
			b.WriteString(selectedRowStyle.Render(row))
		} else {
			b.WriteString(rowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(links) > maxRows {
		b.WriteString(fmt.Sprintf("\n  Showing %d-%d of %d links", startIdx+1, endIdx, len(links)))
	}

	return b.String()
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
	// 5-char bar: ▁▂▃▄▅▆▇█
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	idx := int(struggle * float64(len(chars)-1))
	if idx >= len(chars) {
		idx = len(chars) - 1
	}
	if idx < 0 {
		idx = 0
	}

	// Color based on struggle
	var style lipgloss.Style
	switch {
	case struggle >= 0.7:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	case struggle >= 0.4:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // yellow
	default:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // green
	}

	bar := strings.Repeat(string(chars[idx]), 5)
	return style.Render(bar)
}

// GetSelectedLink returns the currently selected link, if any.
func (m DashboardModel) GetSelectedLink() *dsn.Link {
	if m.snapshot.Data == nil || len(m.snapshot.Data.Links) == 0 {
		return nil
	}
	if m.cursor < 0 || m.cursor >= len(m.snapshot.Data.Links) {
		return nil
	}
	link := m.snapshot.Data.Links[m.cursor]
	return &link
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
