package ui

import (
	"fmt"
	"strings"

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
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57"))

	complexActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46"))

	complexIdleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
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

func (m DashboardModel) renderComplexSummary() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("DSN Complex Status"))
	b.WriteString("\n")

	complexes := []dsn.Complex{dsn.ComplexGoldstone, dsn.ComplexCanberra, dsn.ComplexMadrid}

	for _, c := range complexes {
		info := dsn.KnownComplexes[c]
		load, ok := m.snapshot.ComplexLoads[c]

		name := fmt.Sprintf("%-10s", info.Name)
		var status string

		if !ok || load.TotalAntennas == 0 {
			status = complexIdleStyle.Render(name + " [offline]")
		} else {
			bar := m.renderUtilizationBar(load.Utilization, 10)
			links := fmt.Sprintf("%d links", load.ActiveLinks)
			if load.ActiveLinks > 0 {
				status = complexActiveStyle.Render(name) + " " + bar + " " + links
			} else {
				status = complexIdleStyle.Render(name) + " " + bar + " idle"
			}
		}

		b.WriteString("  " + status + "\n")
	}

	return b.String()
}

func (m DashboardModel) renderUtilizationBar(util float64, width int) string {
	filled := int(util * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	// Color based on utilization
	var style lipgloss.Style
	switch {
	case util >= 0.8:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	case util >= 0.5:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // yellow
	default:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // green
	}

	return "[" + style.Render(bar) + "]"
}

func (m DashboardModel) renderLinksTable() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Active Links"))
	b.WriteString("\n")

	// Table header
	header := fmt.Sprintf("%-10s %-8s %-16s %-4s %-10s %-10s %-8s",
		"Station", "Antenna", "Spacecraft", "Band", "Rate", "Distance", "Struggle")
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

		row := fmt.Sprintf("%-10s %-8s %-16s %-4s %-10s %-10s %s",
			truncate(string(link.Complex), 10),
			truncate(link.AntennaID, 8),
			truncate(link.Spacecraft, 16),
			link.Band,
			dsn.FormatDataRate(link.DataRate),
			dsn.FormatDistance(link.Distance),
			m.renderStruggleBar(struggle),
		)

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
