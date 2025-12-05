package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

// MissionDetailModel shows detailed info for a selected spacecraft.
type MissionDetailModel struct {
	width      int
	height     int
	selectedID int
	snapshot   state.Snapshot
	scrollY    int
}

// NewMissionDetailModel creates a new mission detail model.
func NewMissionDetailModel() MissionDetailModel {
	return MissionDetailModel{
		selectedID: -1,
	}
}

// SetSize updates the viewport size.
func (m MissionDetailModel) SetSize(width, height int) MissionDetailModel {
	m.width = width
	m.height = height
	return m
}

// UpdateData updates with new data snapshot.
func (m MissionDetailModel) UpdateData(snapshot state.Snapshot) MissionDetailModel {
	m.snapshot = snapshot

	// Auto-select first spacecraft if none selected
	if m.selectedID < 0 && len(snapshot.Spacecraft) > 0 {
		m.selectedID = snapshot.Spacecraft[0].ID
	}

	return m
}

// Update handles messages.
func (m MissionDetailModel) Update(msg tea.Msg) (MissionDetailModel, tea.Cmd) {
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
		case "left", "h":
			m.selectPrevSpacecraft()
		case "right", "l":
			m.selectNextSpacecraft()
		}
	}
	return m, nil
}

func (m *MissionDetailModel) selectNextSpacecraft() {
	if len(m.snapshot.Spacecraft) == 0 {
		return
	}
	for i, sc := range m.snapshot.Spacecraft {
		if sc.ID == m.selectedID && i < len(m.snapshot.Spacecraft)-1 {
			m.selectedID = m.snapshot.Spacecraft[i+1].ID
			m.scrollY = 0
			return
		}
	}
}

func (m *MissionDetailModel) selectPrevSpacecraft() {
	if len(m.snapshot.Spacecraft) == 0 {
		return
	}
	for i, sc := range m.snapshot.Spacecraft {
		if sc.ID == m.selectedID && i > 0 {
			m.selectedID = m.snapshot.Spacecraft[i-1].ID
			m.scrollY = 0
			return
		}
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

	// Spacecraft details
	b.WriteString(m.renderSpacecraftDetails(selected))

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

	// Name header
	b.WriteString(headerStyle.Render(sc.Name))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", len(sc.Name)+4))
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
		}
	}

	// Sparkline placeholders for history
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Signal History"))
	b.WriteString("\n")
	b.WriteString(m.renderSparkline("Distance", 30))
	b.WriteString("\n")
	b.WriteString(m.renderSparkline("Data Rate", 30))
	b.WriteString("\n")

	return b.String()
}

// renderSparkline renders a simple text-based sparkline (placeholder).
func (m MissionDetailModel) renderSparkline(label string, width int) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Width(12)

	// Placeholder sparkline characters: ▁▂▃▄▅▆▇█
	// For now, just show a placeholder pattern
	sparkChars := "▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇▆▅▄▃▂▁▂▃▄"
	if len(sparkChars) > width {
		sparkChars = sparkChars[:width]
	}

	sparkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	return labelStyle.Render(label+":") + " " + sparkStyle.Render(sparkChars)
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
