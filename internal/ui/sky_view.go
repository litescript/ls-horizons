package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/peter/ls-horizons/internal/dsn"
	"github.com/peter/ls-horizons/internal/state"
)

const (
	// Field of view in degrees
	fovAz = 120.0 // horizontal FOV
	fovEl = 60.0  // vertical FOV

	// Animation
	animDuration  = 400 * time.Millisecond
	animFrameRate = 30 * time.Millisecond
)

// SkyViewModel renders the sky dome with spacecraft positions.
type SkyViewModel struct {
	width  int
	height int

	// Camera position (center of view)
	camAz float64
	camEl float64

	// Animation state
	animating   bool
	animStartAz float64
	animStartEl float64
	animTargAz  float64
	animTargEl  float64
	animStart   time.Time

	// Focus
	focusIdx   int
	skyObjects []dsn.SkyObject

	// Selected complex filter (empty = all)
	complex dsn.Complex
}

// NewSkyViewModel creates a new sky view model.
func NewSkyViewModel() SkyViewModel {
	return SkyViewModel{
		camAz: 180,
		camEl: 45,
	}
}

// SetSize updates the viewport size.
func (m SkyViewModel) SetSize(width, height int) SkyViewModel {
	m.width = width
	m.height = height
	return m
}

// UpdateData updates with new data snapshot.
func (m SkyViewModel) UpdateData(snapshot state.Snapshot) SkyViewModel {
	m.skyObjects = snapshot.SkyObjects

	// If focus is out of bounds, reset
	if m.focusIdx >= len(m.skyObjects) {
		m.focusIdx = 0
	}

	// If not animating, snap camera to focused object
	if !m.animating && len(m.skyObjects) > 0 && m.focusIdx < len(m.skyObjects) {
		obj := m.skyObjects[m.focusIdx]
		m.camAz = obj.Azimuth
		m.camEl = obj.Elevation
	}

	return m
}

// SyncFromDashboard initializes sky view focus from dashboard selection.
func (m SkyViewModel) SyncFromDashboard(dash DashboardModel, snapshot state.Snapshot) SkyViewModel {
	m.skyObjects = snapshot.SkyObjects

	// Try to find the spacecraft selected in dashboard
	if link := dash.GetSelectedLink(); link != nil {
		for i, obj := range m.skyObjects {
			if obj.Spacecraft == link.Spacecraft && obj.AntennaID == link.AntennaID {
				m.focusIdx = i
				m.camAz = obj.Azimuth
				m.camEl = obj.Elevation
				return m
			}
		}
	}

	// Default to first object
	if len(m.skyObjects) > 0 {
		m.focusIdx = 0
		m.camAz = m.skyObjects[0].Azimuth
		m.camEl = m.skyObjects[0].Elevation
	}
	return m
}

// animTickMsg is sent during animation
type animTickMsg time.Time

func animTick() tea.Cmd {
	return tea.Tick(animFrameRate, func(t time.Time) tea.Msg {
		return animTickMsg(t)
	})
}

// Update handles messages.
func (m SkyViewModel) Update(msg tea.Msg) (SkyViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			return m.focusPrev()
		case "right", "l":
			return m.focusNext()
		case "c":
			// Cycle complex filter
			m = m.cycleComplex()
		}

	case animTickMsg:
		if m.animating {
			return m.updateAnimation()
		}
	}

	return m, nil
}

func (m SkyViewModel) focusNext() (SkyViewModel, tea.Cmd) {
	if len(m.skyObjects) == 0 {
		return m, nil
	}
	m.focusIdx = (m.focusIdx + 1) % len(m.skyObjects)
	return m.startAnimation()
}

func (m SkyViewModel) focusPrev() (SkyViewModel, tea.Cmd) {
	if len(m.skyObjects) == 0 {
		return m, nil
	}
	m.focusIdx--
	if m.focusIdx < 0 {
		m.focusIdx = len(m.skyObjects) - 1
	}
	return m.startAnimation()
}

func (m SkyViewModel) startAnimation() (SkyViewModel, tea.Cmd) {
	if len(m.skyObjects) == 0 || m.focusIdx >= len(m.skyObjects) {
		return m, nil
	}

	target := m.skyObjects[m.focusIdx]
	m.animating = true
	m.animStartAz = m.camAz
	m.animStartEl = m.camEl
	m.animTargAz = target.Azimuth
	m.animTargEl = target.Elevation
	m.animStart = time.Now()

	return m, animTick()
}

func (m SkyViewModel) updateAnimation() (SkyViewModel, tea.Cmd) {
	elapsed := time.Since(m.animStart)
	t := float64(elapsed) / float64(animDuration)

	if t >= 1.0 {
		// Animation complete
		m.animating = false
		m.camAz = m.animTargAz
		m.camEl = m.animTargEl
		return m, nil
	}

	// Ease-out cubic
	t = 1 - math.Pow(1-t, 3)

	// Interpolate azimuth with wrap-around handling
	m.camAz = lerpAngle(m.animStartAz, m.animTargAz, t)
	m.camEl = lerp(m.animStartEl, m.animTargEl, t)

	return m, animTick()
}

func (m SkyViewModel) cycleComplex() SkyViewModel {
	switch m.complex {
	case "":
		m.complex = dsn.ComplexGoldstone
	case dsn.ComplexGoldstone:
		m.complex = dsn.ComplexCanberra
	case dsn.ComplexCanberra:
		m.complex = dsn.ComplexMadrid
	case dsn.ComplexMadrid:
		m.complex = ""
	}
	return m
}

// View renders the sky view.
func (m SkyViewModel) View() string {
	if m.width < 20 || m.height < 10 {
		return "Sky view requires larger terminal"
	}

	// Reserve lines for header and status
	viewHeight := m.height - 4
	viewWidth := m.width

	// Create the sky canvas
	canvas := m.renderSkyCanvas(viewWidth, viewHeight)

	// Build output
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(canvas)
	b.WriteString("\n")
	b.WriteString(m.renderStatus())

	return b.String()
}

func (m SkyViewModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("135")) // violet
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("60"))               // muted purple

	title := titleStyle.Render("Sky View")

	complexFilter := "All Complexes"
	if m.complex != "" {
		info := dsn.KnownComplexes[m.complex]
		complexFilter = info.Name
	}

	compass := fmt.Sprintf("Az: %.0f° El: %.0f°", m.camAz, m.camEl)
	help := dimStyle.Render("←/→: focus | c: complex | d: dashboard | q: quit")

	return fmt.Sprintf("%s  |  %s  |  %s  |  %s", title, complexFilter, compass, help)
}

func (m SkyViewModel) renderStatus() string {
	if len(m.skyObjects) == 0 {
		return "No spacecraft in view"
	}

	if m.focusIdx >= len(m.skyObjects) {
		return ""
	}

	obj := m.skyObjects[m.focusIdx]
	status := fmt.Sprintf(">>> %s @ %s [%s] | Az:%.0f° El:%.0f° | %s | Struggle: %.0f%%",
		obj.Spacecraft,
		obj.AntennaID,
		obj.Band,
		obj.Azimuth,
		obj.Elevation,
		dsn.FormatDistance(obj.Distance),
		obj.StruggleIndex*100,
	)

	return lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Render(status)
}

func (m SkyViewModel) renderSkyCanvas(width, height int) string {
	// Initialize canvas with starfield (space colors: deep blues/purples)
	canvas := make([][]rune, height)
	colors := make([][]lipgloss.Color, height)
	for y := 0; y < height; y++ {
		canvas[y] = make([]rune, width)
		colors[y] = make([]lipgloss.Color, width)
		for x := 0; x < width; x++ {
			canvas[y][x] = m.starfieldChar(x, y)
			colors[y][x] = m.starfieldColor(x, y)
		}
	}

	// Draw horizon line (purple tint)
	horizonY := height - 2
	for x := 0; x < width; x++ {
		canvas[horizonY][x] = '─'
		colors[horizonY][x] = "60" // muted purple
	}

	// Draw cardinal directions on horizon
	m.drawCardinal(canvas, colors, width, height, "N", 0)
	m.drawCardinal(canvas, colors, width, height, "E", 90)
	m.drawCardinal(canvas, colors, width, height, "S", 180)
	m.drawCardinal(canvas, colors, width, height, "W", 270)

	// Draw spacecraft
	for i, obj := range m.skyObjects {
		// Filter by complex if set
		if m.complex != "" && obj.Complex != m.complex {
			continue
		}

		x, y, visible := m.projectToScreen(obj.Azimuth, obj.Elevation, width, height)
		if !visible {
			continue
		}

		// Clamp to canvas bounds
		if x < 0 || x >= width || y < 0 || y >= horizonY {
			continue
		}

		// Choose symbol and color (nebula palette)
		sym := '●'
		color := lipgloss.Color("69") // soft blue default

		switch obj.Band {
		case "X":
			color = "75" // sky blue
		case "S":
			color = "141" // soft purple
		case "Ka":
			color = "212" // pink
		}

		if i == m.focusIdx {
			sym = '◆'
			color = "229" // bright gold for focused
		}

		canvas[y][x] = sym
		colors[y][x] = color
	}

	// Draw station marker at bottom center
	stationX := width / 2
	stationY := height - 1
	if stationY >= 0 && stationX >= 0 && stationX < width {
		canvas[stationY][stationX] = '▲'
		colors[stationY][stationX] = "46"
	}

	// Render canvas to string
	var b strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			style := lipgloss.NewStyle().Foreground(colors[y][x])
			b.WriteString(style.Render(string(canvas[y][x])))
		}
		if y < height-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m SkyViewModel) starfieldChar(x, y int) rune {
	// Simple deterministic starfield
	hash := (x*31 + y*17) % 100
	switch {
	case hash < 2:
		return '·'
	case hash < 3:
		return '+'
	case hash < 4:
		return '*'
	default:
		return ' '
	}
}

func (m SkyViewModel) starfieldColor(x, y int) lipgloss.Color {
	hash := (x*31 + y*17) % 100
	switch {
	case hash < 2:
		return "63" // blue-purple
	case hash < 3:
		return "141" // lavender
	case hash < 4:
		return "183" // pink-white
	default:
		return "236" // very dark (background)
	}
}

func (m SkyViewModel) drawCardinal(canvas [][]rune, colors [][]lipgloss.Color, width, height int, label string, az float64) {
	x, _, visible := m.projectToScreen(az, 0, width, height)
	if !visible {
		return
	}
	y := height - 2 // horizon line

	if x >= 0 && x < width && y >= 0 && y < height {
		canvas[y][x] = rune(label[0])
		colors[y][x] = "252"
	}
}

// projectToScreen converts az/el to screen coordinates relative to camera
func (m SkyViewModel) projectToScreen(az, el float64, width, height int) (int, int, bool) {
	// Calculate angular offset from camera center
	dAz := normalizeAngle(az - m.camAz)
	dEl := el - m.camEl

	// Check if within FOV
	if dAz < -fovAz/2 || dAz > fovAz/2 {
		return 0, 0, false
	}
	if dEl < -fovEl/2 || dEl > fovEl/2 {
		return 0, 0, false
	}

	// Map to screen coordinates
	// X: -fovAz/2..+fovAz/2 -> 0..width
	// Y: +fovEl/2..-fovEl/2 -> 0..height (inverted, higher el = higher on screen)
	horizonY := height - 2

	x := int((dAz + fovAz/2) / fovAz * float64(width))
	y := int((fovEl/2 - dEl) / fovEl * float64(horizonY))

	return x, y, true
}

// normalizeAngle wraps angle to -180..+180 range
func normalizeAngle(a float64) float64 {
	for a > 180 {
		a -= 360
	}
	for a < -180 {
		a += 360
	}
	return a
}

// lerpAngle interpolates between angles, taking shortest path
func lerpAngle(a, b, t float64) float64 {
	diff := normalizeAngle(b - a)
	return a + diff*t
}

// lerp linear interpolation
func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// Exported for compatibility with existing UI structure

// Init returns nil cmd
func (m SkyViewModel) Init() tea.Cmd {
	return nil
}
