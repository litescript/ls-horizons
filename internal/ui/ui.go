// Package ui provides the terminal user interface using Bubble Tea.
package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/peter/ls-horizons/internal/dsn"
	"github.com/peter/ls-horizons/internal/state"
)

// ViewMode represents the current UI view.
type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewMissionDetail
	ViewSky
)

// Msg types for Bubble Tea
type (
	// TickMsg triggers periodic UI updates.
	TickMsg time.Time

	// DataUpdateMsg signals new DSN data is available.
	DataUpdateMsg struct {
		Snapshot state.Snapshot
	}

	// ErrorMsg signals a fetch error.
	ErrorMsg struct {
		Error error
	}
)

// Model is the root Bubble Tea model.
type Model struct {
	// Dependencies
	state *state.Manager

	// UI state
	viewMode          ViewMode
	width             int
	height            int
	ready             bool

	// Sub-models
	dashboard     DashboardModel
	missionDetail MissionDetailModel
	skyView       SkyViewModel

	// Data snapshot (updated on DataUpdateMsg)
	snapshot state.Snapshot
}

// New creates a new root UI model.
func New(stateMgr *state.Manager) Model {
	return Model{
		state:         stateMgr,
		viewMode:      ViewDashboard,
		dashboard:     NewDashboardModel(),
		missionDetail: NewMissionDetailModel(),
		skyView:       NewSkyViewModel(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.dashboard.Init(),
	)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "1", "d":
			m.viewMode = ViewDashboard
		case "2", "m":
			m.viewMode = ViewMissionDetail
		case "3", "s":
			// Enter Sky View, sync focus from dashboard if available
			if m.viewMode != ViewSky {
				m.skyView = m.skyView.SyncFromDashboard(m.dashboard, m.snapshot)
			}
			m.viewMode = ViewSky

		case "tab":
			// Cycle through views
			m.viewMode = (m.viewMode + 1) % 3

		default:
			// Pass to active view
			cmds = append(cmds, m.updateActiveView(msg))
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Propagate to sub-models
		// Logo takes ~10 lines, footer ~2 lines
		contentHeight := msg.Height - 14
		m.dashboard = m.dashboard.SetSize(msg.Width, contentHeight)
		m.missionDetail = m.missionDetail.SetSize(msg.Width, contentHeight)
		m.skyView = m.skyView.SetSize(msg.Width, contentHeight)

	case TickMsg:
		cmds = append(cmds, tickCmd())
		// Request fresh snapshot
		m.snapshot = m.state.Snapshot()

	case DataUpdateMsg:
		m.snapshot = msg.Snapshot
		m.dashboard = m.dashboard.UpdateData(m.snapshot)
		m.missionDetail = m.missionDetail.UpdateData(m.snapshot)
		m.skyView = m.skyView.UpdateData(m.snapshot)

	case ErrorMsg:
		// Could display error in status bar
		m.dashboard = m.dashboard.SetError(msg.Error)

	default:
		cmds = append(cmds, m.updateActiveView(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateActiveView(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.viewMode {
	case ViewDashboard:
		m.dashboard, cmd = m.dashboard.Update(msg)
	case ViewMissionDetail:
		m.missionDetail, cmd = m.missionDetail.Update(msg)
	case ViewSky:
		m.skyView, cmd = m.skyView.Update(msg)
	}
	return cmd
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var content string
	switch m.viewMode {
	case ViewDashboard:
		content = m.dashboard.View()
	case ViewMissionDetail:
		content = m.missionDetail.View()
	case ViewSky:
		content = m.skyView.View()
	}

	return m.renderFrame(content)
}

func (m Model) renderFrame(content string) string {
	header := m.renderHeader()
	footer := m.renderFooter()

	return header + "\n" + content + "\n" + footer
}

func (m Model) renderHeader() string {
	return m.renderLogo() + m.renderStatusLine()
}

func (m Model) renderLogo() string {
	// ASCII art with nebula/space gradient coloring
	logo := []string{
		`  ██╗     ███████╗      ██╗  ██╗ ██████╗ ██████╗ ██╗███████╗ ██████╗ ███╗   ██╗███████╗`,
		`  ██║     ██╔════╝      ██║  ██║██╔═══██╗██╔══██╗██║╚══███╔╝██╔═══██╗████╗  ██║██╔════╝`,
		`  ██║     ███████╗█████╗███████║██║   ██║██████╔╝██║  ███╔╝ ██║   ██║██╔██╗ ██║███████╗`,
		`  ██║     ╚════██║╚════╝██╔══██║██║   ██║██╔══██╗██║ ███╔╝  ██║   ██║██║╚██╗██║╚════██║`,
		`  ███████╗███████║      ██║  ██║╚██████╔╝██║  ██║██║███████╗╚██████╔╝██║ ╚████║███████║`,
		`  ╚══════╝╚══════╝      ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝╚══════╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝`,
	}

	// Space/nebula gradient - deep purple to bright blue/pink
	colors := []string{
		"#9D4EDD", // Vibrant purple
		"#7B2CBF", // Deep purple
		"#5A189A", // Royal purple
		"#3C096C", // Dark purple
		"#240046", // Deep violet
		"#10002B", // Near black purple
	}

	var b strings.Builder
	b.WriteString("\n")

	for i, line := range logo {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[i]))
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	// Tagline
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	b.WriteString(muted.Render("  Deep Space Network · Real-time Visualization"))
	b.WriteString("\n\n")

	return b.String()
}

func (m Model) renderStatusLine() string {
	tabs := m.renderTabs()
	return tabs + "\n"
}

func (m Model) renderTabs() string {
	tabs := []string{"[1] Dashboard", "[2] Mission", "[3] Sky"}
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9D4EDD")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))

	var parts []string
	for i, tab := range tabs {
		if ViewMode(i) == m.viewMode {
			parts = append(parts, activeStyle.Render("▶ "+tab))
		} else {
			parts = append(parts, dimStyle.Render("  "+tab))
		}
	}
	return "  " + strings.Join(parts, "  ")
}

func (m Model) renderFooter() string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E84A27"))
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7B2CBF"))

	var status string
	if m.snapshot.LastError != nil {
		status = errorStyle.Render("ERROR: " + m.snapshot.LastError.Error())
	} else if !m.snapshot.LastFetch.IsZero() {
		ago := time.Since(m.snapshot.LastFetch).Round(time.Second)
		status = accentStyle.Render("●") + dimStyle.Render(" "+ago.String()+" ago")
		if m.snapshot.FetchDuration > 0 {
			status += dimStyle.Render(" (" + m.snapshot.FetchDuration.Round(time.Millisecond).String() + ")")
		}
	} else {
		status = dimStyle.Render("◌ Waiting for data...")
	}

	help := dimStyle.Render("q: quit | tab: switch view | ↑↓: navigate")
	return "  " + status + "  " + dimStyle.Render("|") + "  " + help
}

// GetSelectedSpacecraft returns the currently selected spacecraft ID (for mission detail).
func (m Model) GetSelectedSpacecraft() int {
	return m.missionDetail.selectedID
}

// SetSelectedSpacecraft sets the selected spacecraft for mission detail view.
func (m *Model) SetSelectedSpacecraft(id int) {
	m.missionDetail.selectedID = id
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// SendDataUpdate creates a command that sends a data update message.
func SendDataUpdate(snapshot state.Snapshot) tea.Cmd {
	return func() tea.Msg {
		return DataUpdateMsg{Snapshot: snapshot}
	}
}

// SendError creates a command that sends an error message.
func SendError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Error: err}
	}
}

// Helper to get link count for status display
func countActiveLinks(data *dsn.DSNData) int {
	if data == nil {
		return 0
	}
	return len(data.Links)
}
