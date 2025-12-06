// Package ui provides the terminal user interface using Bubble Tea.
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
	"github.com/litescript/ls-horizons/internal/version"
)

// ViewMode represents the current UI view.
type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewMissionDetail
	ViewSky
	ViewSolarSystem
)

// Msg types for Bubble Tea
type (
	// TickMsg triggers periodic UI updates.
	TickMsg time.Time

	// AnimTickMsg triggers fast animation updates.
	AnimTickMsg time.Time

	// DataUpdateMsg signals new DSN data is available.
	DataUpdateMsg struct {
		Snapshot state.Snapshot
	}

	// ErrorMsg signals a fetch error.
	ErrorMsg struct {
		Error error
	}

	// updateCheckMsg contains result of version check.
	updateCheckMsg struct {
		info version.UpdateInfo
	}
)

// Model is the root Bubble Tea model.
type Model struct {
	// Dependencies
	state *state.Manager

	// UI state
	viewMode  ViewMode
	width     int
	height    int
	ready     bool
	statusMsg string // Status message for update checks, etc.
	animTick  int    // Animation tick for shimmer effects

	// Sub-models
	dashboard     DashboardModel
	missionDetail MissionDetailModel
	skyView       SkyViewModel
	solarSystem   SolarSystemModel

	// Data snapshot (updated on DataUpdateMsg)
	snapshot    state.Snapshot
	solarCache  *dsn.SolarSystemCache
}

// New creates a new root UI model.
func New(stateMgr *state.Manager, ephemProvider ephem.Provider) Model {
	skyView := NewSkyViewModel()
	if ephemProvider != nil {
		skyView = skyView.SetPathProvider(ephemProvider)
	}

	// Create solar system cache with Horizons provider if available
	var solarCache *dsn.SolarSystemCache
	if hp, ok := ephemProvider.(*ephem.HorizonsProvider); ok {
		solarCache = dsn.NewSolarSystemCache(hp)
	} else {
		solarCache = dsn.NewSolarSystemCache(nil)
	}

	return Model{
		state:         stateMgr,
		viewMode:      ViewDashboard,
		dashboard:     NewDashboardModel(),
		missionDetail: NewMissionDetailModel(),
		skyView:       skyView,
		solarSystem:   NewSolarSystemModel(),
		solarCache:    solarCache,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		animTickCmd(),
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
		case "4", "o":
			m.viewMode = ViewSolarSystem

		case "tab":
			// Cycle through views
			m.viewMode = (m.viewMode + 1) % 4

		case "u":
			m.statusMsg = "Checking for updates..."
			cmds = append(cmds, checkForUpdate())

		default:
			// Pass to active view
			cmds = append(cmds, m.updateActiveView(msg))
		}

	case updateCheckMsg:
		if msg.info.Error != nil {
			m.statusMsg = fmt.Sprintf("Update check failed: %v", msg.info.Error)
		} else if msg.info.UpdateAvailable {
			m.statusMsg = fmt.Sprintf("Update available: v%s → v%s",
				msg.info.CurrentVersion, msg.info.LatestVersion)
		} else {
			m.statusMsg = fmt.Sprintf("You're on the latest version (v%s)", msg.info.CurrentVersion)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Propagate to sub-models
		// Logo takes ~11 lines (added version line), footer ~2 lines
		contentHeight := msg.Height - 15
		m.dashboard = m.dashboard.SetSize(msg.Width, contentHeight)
		m.missionDetail = m.missionDetail.SetSize(msg.Width, contentHeight)
		m.skyView = m.skyView.SetSize(msg.Width, contentHeight)
		m.solarSystem = m.solarSystem.SetSize(msg.Width, contentHeight)

	case TickMsg:
		cmds = append(cmds, tickCmd())
		// Request fresh snapshot
		m.snapshot = m.state.Snapshot()

	case AnimTickMsg:
		cmds = append(cmds, animTickCmd())
		m.animTick++

	case DataUpdateMsg:
		m.snapshot = msg.Snapshot
		m.dashboard = m.dashboard.UpdateData(m.snapshot)
		m.missionDetail = m.missionDetail.UpdateData(m.snapshot)
		m.skyView = m.skyView.UpdateData(m.snapshot)

		// Update solar system cache with DSN data (async to avoid blocking UI)
		if m.solarCache != nil {
			// Spacecraft updates are fast (just uses DSN data)
			if m.solarCache.NeedsSpacecraftRefresh() {
				_ = m.solarCache.UpdateSpacecraft(m.snapshot.Data)
			}
			// Planet updates are slow (HTTP calls) - do async
			if m.solarCache.NeedsPlanetRefresh() {
				go m.solarCache.UpdatePlanets()
			}
			solarSnap := m.solarCache.GetSnapshot()
			m.solarSystem = m.solarSystem.UpdateData(m.snapshot, solarSnap)
		}

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
	case ViewSolarSystem:
		m.solarSystem, cmd = m.solarSystem.Update(msg)
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
	case ViewSolarSystem:
		content = m.solarSystem.View()
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
	// ASCII art with smooth truecolor gradient
	logo := []string{
		`  ██╗     ███████╗      ██╗  ██╗ ██████╗ ██████╗ ██╗███████╗ ██████╗ ███╗   ██╗███████╗`,
		`  ██║     ██╔════╝      ██║  ██║██╔═══██╗██╔══██╗██║╚══███╔╝██╔═══██╗████╗  ██║██╔════╝`,
		`  ██║     ███████╗█████╗███████║██║   ██║██████╔╝██║  ███╔╝ ██║   ██║██╔██╗ ██║███████╗`,
		`  ██║     ╚════██║╚════╝██╔══██║██║   ██║██╔══██╗██║ ███╔╝  ██║   ██║██║╚██╗██║╚════██║`,
		`  ███████╗███████║      ██║  ██║╚██████╔╝██║  ██║██║███████╗╚██████╔╝██║ ╚████║███████║`,
		`  ╚══════╝╚══════╝      ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝╚══════╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝`,
	}

	var b strings.Builder
	b.WriteString("\n")

	// Render each line with a horizontal truecolor gradient
	for row, line := range logo {
		runes := []rune(line)
		lineLen := len(runes)

		for col, r := range runes {
			// Create a smooth gradient based on position
			// Horizontal: purple -> pink -> cyan
			// Vertical: brighter at top, darker at bottom
			color := gradientColor(col, row, lineLen, len(logo))
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			b.WriteString(style.Render(string(r)))
		}
		b.WriteString("\n")
	}

	// Tagline
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))
	b.WriteString(muted.Render("  Deep Space Network · Real-time Visualization"))
	b.WriteString("\n")

	// Version/copyright line
	copyright := fmt.Sprintf("  (c) 2025 litescript.net | v%s | [u]check update", version.Version)
	b.WriteString(muted.Render(copyright))
	b.WriteString("\n\n")

	return b.String()
}

// gradientColor returns a hex color for a position in the logo gradient.
// Creates a vibrant nebula effect: blue -> purple -> magenta -> pink
func gradientColor(col, row, width, height int) string {
	// Normalize positions to 0-1
	xRatio := float64(col) / float64(width)
	yRatio := float64(row) / float64(height)

	// More dramatic horizontal gradient with higher saturation
	// Blue (#3B82F6) -> Purple (#8B5CF6) -> Magenta (#D946EF) -> Pink (#EC4899)
	var r, g, b float64

	if xRatio < 0.33 {
		// Blue to Purple
		t := xRatio / 0.33
		r = 59 + t*(139-59)
		g = 130 + t*(92-130)
		b = 246 + t*(246-246)
	} else if xRatio < 0.66 {
		// Purple to Magenta
		t := (xRatio - 0.33) / 0.33
		r = 139 + t*(217-139)
		g = 92 + t*(70-92)
		b = 246 + t*(239-246)
	} else {
		// Magenta to Pink
		t := (xRatio - 0.66) / 0.34
		r = 217 + t*(236-217)
		g = 70 + t*(72-70)
		b = 239 + t*(153-239)
	}

	// Vertical fade: brighter at top, darker toward bottom
	brightnessFactor := 1.0 - (yRatio * 0.5)
	r *= brightnessFactor
	g *= brightnessFactor
	b *= brightnessFactor

	// Clamp to valid range
	ri := int(r)
	gi := int(g)
	bi := int(b)
	if ri > 255 {
		ri = 255
	}
	if gi > 255 {
		gi = 255
	}
	if bi > 255 {
		bi = 255
	}
	if ri < 0 {
		ri = 0
	}
	if gi < 0 {
		gi = 0
	}
	if bi < 0 {
		bi = 0
	}

	return fmt.Sprintf("#%02X%02X%02X", ri, gi, bi)
}

func (m Model) renderStatusLine() string {
	tabs := m.renderTabs()
	return tabs + "\n"
}

func (m Model) renderTabs() string {
	tabs := []string{"[1] Dashboard", "[2] Mission", "[3] Sky", "[4] Orbit"}
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

	// Animated spinner frames
	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := spinnerFrames[m.animTick%len(spinnerFrames)]

	var status string
	if m.snapshot.LastError != nil {
		status = errorStyle.Render("ERROR: " + m.snapshot.LastError.Error())
	} else if !m.snapshot.LastFetch.IsZero() {
		// Show countdown to next refresh with spinner
		countdown := time.Until(m.snapshot.NextRefresh).Round(time.Second)
		if countdown < 0 {
			countdown = 0
		}
		status = accentStyle.Render(spinner) + dimStyle.Render(fmt.Sprintf(" refresh in %ds", int(countdown.Seconds())))
		if m.snapshot.FetchDuration > 0 {
			status += dimStyle.Render(" (" + m.snapshot.FetchDuration.Round(time.Millisecond).String() + ")")
		}
	} else {
		status = accentStyle.Render(spinner) + " " + m.renderShimmerText("Waiting for data...")
	}

	// View-specific help hints
	var help string
	switch m.viewMode {
	case ViewMissionDetail:
		help = dimStyle.Render("←/→: spacecraft | h: passes | ↑↓: scroll")
	case ViewSky:
		help = dimStyle.Render("j/k: focus | l: labels | c: complex | p: path | v: visibility")
	case ViewSolarSystem:
		help = dimStyle.Render("j/k: focus | n/N: spacecraft | +/-: zoom | arrows: pan | f: find | l: labels | z: mode")
	default:
		help = dimStyle.Render("↑↓: navigate | tab: switch view")
	}

	footer := "  " + status + "  " + dimStyle.Render("|") + "  " + help

	// Show update status message if present
	if m.statusMsg != "" {
		footer += "\n  " + dimStyle.Render(m.statusMsg)
	}

	return footer
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

func animTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return AnimTickMsg(t)
	})
}

func checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		info := version.CheckForUpdate()
		return updateCheckMsg{info: info}
	}
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

// renderShimmerText renders text with a subtle moving shine effect.
func (m Model) renderShimmerText(text string) string {
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

// Helper to get link count for status display
func countActiveLinks(data *dsn.DSNData) int {
	if data == nil {
		return 0
	}
	return len(data.Links)
}
