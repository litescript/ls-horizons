package ui

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/litescript/ls-horizons/internal/astro"
	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

// SolarSystemModel renders a top-down view of the solar system.
type SolarSystemModel struct {
	width     int
	height    int
	snapshot  state.Snapshot
	solarSnap dsn.SolarSystemSnapshot

	// View state
	focusIdx   int     // Index in bodies list (-1 = Sun)
	zoomLevel  int     // Index into zoomLevels
	panX       float64 // Pan offset in display units
	panY       float64
	scaleMode  astro.ScaleMode
	labelMode  LabelMode // Label display mode (reuses sky_view LabelMode)
	userPanned bool      // True if user has manually panned (disables auto-center on zoom)
	showStars  bool      // Whether to show background starfield
}

// Discrete zoom levels for clean stepping
var zoomLevels = []float64{0.25, 0.5, 0.75, 1.0, 1.5, 2.0, 3.0, 5.0, 10.0}

// NewSolarSystemModel creates a new solar system view model.
func NewSolarSystemModel() SolarSystemModel {
	return SolarSystemModel{
		focusIdx:  -1, // Start focused on Sun
		zoomLevel: 3,  // Index of 1.0 in zoomLevels
		scaleMode: astro.ScaleLogR,
		labelMode: LabelFocused, // Show focused body label by default
		showStars: true,         // Show starfield by default
	}
}

// scale returns the current zoom scale.
func (m SolarSystemModel) scale() float64 {
	if m.zoomLevel < 0 || m.zoomLevel >= len(zoomLevels) {
		return 1.0
	}
	return zoomLevels[m.zoomLevel]
}

// SetSize updates the viewport size.
func (m SolarSystemModel) SetSize(width, height int) SolarSystemModel {
	m.width = width
	m.height = height
	return m
}

// UpdateData updates the model with new data.
func (m SolarSystemModel) UpdateData(snapshot state.Snapshot, solarSnap dsn.SolarSystemSnapshot) SolarSystemModel {
	m.snapshot = snapshot
	m.solarSnap = solarSnap
	return m
}

// Update handles input messages.
func (m SolarSystemModel) Update(msg tea.Msg) (SolarSystemModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Focus navigation (j/k like other views, or [/])
		case "j", "[":
			m.focusPrev()
		case "k", "]":
			m.focusNext()
		// Spacecraft cycling (n/N since tab is global view switch)
		case "n":
			m.focusNextSpacecraft()
		case "N":
			m.focusPrevSpacecraft()

		// Viewport panning (arrow keys - no conflict with global keys)
		case "up":
			m.panY -= 0.1 / m.scale()
			m.userPanned = true
		case "down":
			m.panY += 0.1 / m.scale()
			m.userPanned = true
		case "left":
			m.panX -= 0.1 / m.scale()
			m.userPanned = true
		case "right":
			m.panX += 0.1 / m.scale()
			m.userPanned = true
		case "c":
			m.panX, m.panY = 0, 0 // Center on Sun
			m.userPanned = false

		// Find/focus - center on selected object
		case "f":
			m.centerOnFocused()
			m.userPanned = false

		// Zoom (discrete levels) - only auto-center if user hasn't panned
		case "+", "=":
			if m.zoomLevel < len(zoomLevels)-1 {
				m.zoomLevel++
				if !m.userPanned {
					m.centerOnFocused()
				}
			}
		case "-":
			if m.zoomLevel > 0 {
				m.zoomLevel--
				if !m.userPanned {
					m.centerOnFocused()
				}
			}
		case "0":
			// Reset to 1.0x zoom
			m.zoomLevel = 3
			if !m.userPanned {
				m.centerOnFocused()
			}

		// Scale mode toggle (z for "zoom mode" - no conflict)
		case "z":
			m.scaleMode = (m.scaleMode + 1) % 3
			if !m.userPanned {
				m.centerOnFocused()
			}

		// Label mode toggle
		case "l":
			m.labelMode = (m.labelMode + 1) % 3

		// Starfield toggle
		case "t":
			m.showStars = !m.showStars

		// Reset everything
		case "r":
			m.panX, m.panY = 0, 0
			m.zoomLevel = 3
			m.userPanned = false
		}
	}
	return m, nil
}

func (m *SolarSystemModel) focusNext() {
	bodies := m.solarSnap.Bodies
	if len(bodies) == 0 {
		return
	}
	m.focusIdx++
	if m.focusIdx >= len(bodies) {
		m.focusIdx = -1 // Wrap to Sun
	}
	m.centerOnFocused()
	m.userPanned = false
}

func (m *SolarSystemModel) focusPrev() {
	bodies := m.solarSnap.Bodies
	if len(bodies) == 0 {
		return
	}
	m.focusIdx--
	if m.focusIdx < -1 {
		m.focusIdx = len(bodies) - 1
	}
	m.centerOnFocused()
	m.userPanned = false
}

func (m *SolarSystemModel) focusNextSpacecraft() {
	bodies := m.solarSnap.Bodies
	start := m.focusIdx + 1
	for i := 0; i < len(bodies); i++ {
		idx := (start + i) % len(bodies)
		if bodies[idx].Kind == dsn.BodySpacecraft {
			m.focusIdx = idx
			m.centerOnFocused()
			m.userPanned = false
			return
		}
	}
}

func (m *SolarSystemModel) focusPrevSpacecraft() {
	bodies := m.solarSnap.Bodies
	start := m.focusIdx - 1
	if start < 0 {
		start = len(bodies) - 1
	}
	for i := 0; i < len(bodies); i++ {
		idx := start - i
		if idx < 0 {
			idx = len(bodies) + idx
		}
		if bodies[idx].Kind == dsn.BodySpacecraft {
			m.focusIdx = idx
			m.centerOnFocused()
			m.userPanned = false
			return
		}
	}
}

// centerOnFocused pans the view to center on the currently focused body.
func (m *SolarSystemModel) centerOnFocused() {
	if m.focusIdx < 0 || m.focusIdx >= len(m.solarSnap.Bodies) {
		// Sun is at origin, just reset pan
		m.panX, m.panY = 0, 0
		return
	}

	body := m.solarSnap.Bodies[m.focusIdx]
	cfg := astro.ProjectionConfig{
		Scale: m.scale(),
		Mode:  m.scaleMode,
	}

	// Get projected position
	proj := astro.ProjectEclipticTopDown(body.Pos, cfg)

	// Set pan to center on this body
	// panX = -proj.X and panY = -proj.Y centers the body on screen
	m.panX = -proj.X
	m.panY = -proj.Y
}

// View renders the solar system view.
func (m SolarSystemModel) View() string {
	if m.width < 40 || m.height < 10 {
		return "Terminal too small for solar system view"
	}

	// Build the view in a canvas
	canvas := m.buildCanvas()

	// Render focus HUD
	hud := m.renderHUD()

	// Combine canvas and HUD
	return lipgloss.JoinVertical(lipgloss.Left, canvas, hud)
}

// bodyPos tracks a body's screen position for label rendering.
type bodyPos struct {
	x, y      int
	name      string
	kind      dsn.BodyKind
	isFocused bool
}

// buildCanvas renders the solar system to a string canvas.
func (m SolarSystemModel) buildCanvas() string {
	// Reserve space for HUD (3 lines)
	canvasH := m.height - 5
	if canvasH < 5 {
		canvasH = 5
	}
	canvasW := m.width

	// Create character grid
	grid := make([][]rune, canvasH)
	for y := range grid {
		grid[y] = make([]rune, canvasW)
		for x := range grid[y] {
			grid[y][x] = ' '
		}
	}

	// Screen center
	screenCenterX := canvasW / 2
	screenCenterY := canvasH / 2

	scale := m.scale()
	cfg := astro.ProjectionConfig{
		Scale: scale,
		Mode:  m.scaleMode,
	}

	// Compute display scaling factor
	// Map log(30 AU + 1) ~ 1.5 to fit in half the canvas
	maxDisplayR := float64(min(screenCenterX, screenCenterY*2)) * 0.9
	displayScale := maxDisplayR / 1.5 * scale

	// Pan offset moves the solar system origin on screen
	// Positive panX moves origin right, positive panY moves origin up (screen Y is inverted)
	originX := screenCenterX + int(m.panX*displayScale)
	originY := screenCenterY - int(m.panY*displayScale)

	// Draw starfield background (before everything else)
	if m.showStars {
		m.drawStarfield(grid, originX, originY, displayScale, cfg)
	}

	// Draw orbit rings centered on the panned origin
	m.drawOrbitRings(grid, originX, originY, displayScale, cfg)

	// Track body positions for labels
	var positions []bodyPos

	// Draw bodies (except Sun - draw it last)
	for i, body := range m.solarSnap.Bodies {
		if body.Kind == dsn.BodySun {
			continue
		}

		proj := astro.ProjectEclipticTopDown(body.Pos, cfg)

		// Convert to screen coordinates relative to panned origin
		sx := originX + int(proj.X*displayScale)
		sy := originY - int(proj.Y*displayScale) // Y flipped for screen

		if sx < 0 || sx >= canvasW || sy < 0 || sy >= canvasH {
			continue
		}

		// Select glyph based on body type
		glyph := m.getBodyGlyph(body, i == m.focusIdx)
		grid[sy][sx] = glyph

		// Track position for labels
		positions = append(positions, bodyPos{
			x:         sx,
			y:         sy,
			name:      body.Name,
			kind:      body.Kind,
			isFocused: i == m.focusIdx,
		})
	}

	// Draw Sun at panned origin LAST so it's always visible
	if originX >= 0 && originX < canvasW && originY >= 0 && originY < canvasH {
		grid[originY][originX] = '☉'
		// Track Sun position for label
		positions = append(positions, bodyPos{
			x:         originX,
			y:         originY,
			name:      "Sun",
			kind:      dsn.BodySun,
			isFocused: m.focusIdx == -1,
		})
	}

	// Draw labels based on label mode
	m.renderLabels(grid, canvasW, canvasH, positions)

	// Convert grid to string with colors
	return m.renderGrid(grid, screenCenterX, screenCenterY, displayScale, cfg)
}

func (m SolarSystemModel) drawOrbitRings(grid [][]rune, cx, cy int, scale float64, cfg astro.ProjectionConfig) {
	// Draw reference orbit circles for key distances
	orbitAUs := []float64{1, 5, 10, 20, 30} // Earth, Jupiter, Saturn, Uranus, Neptune regions

	for _, au := range orbitAUs {
		// Project this distance
		proj := astro.ProjectEclipticTopDown(astro.Vec3{X: au, Y: 0, Z: 0}, cfg)
		r := proj.X * scale

		// Draw circle with ASCII
		m.drawCircle(grid, cx, cy, r)
	}
}

func (m SolarSystemModel) drawCircle(grid [][]rune, cx, cy int, r float64) {
	if r < 1 {
		return
	}

	h := len(grid)
	w := len(grid[0])

	// Draw circle using parametric equations
	steps := int(2 * math.Pi * r)
	if steps < 8 {
		steps = 8
	}
	if steps > 360 {
		steps = 360
	}

	for i := 0; i < steps; i++ {
		theta := 2 * math.Pi * float64(i) / float64(steps)
		x := cx + int(r*math.Cos(theta))
		y := cy - int(r*math.Sin(theta)*0.5) // Aspect ratio correction

		if x >= 0 && x < w && y >= 0 && y < h && grid[y][x] == ' ' {
			grid[y][x] = '·'
		}
	}
}

// drawStarfield renders background stars from the bright star catalog.
// Stars are projected to the same ecliptic top-down view as planets.
// The shell radius adapts to zoom level so stars remain visible as a
// stable background at all zoom levels.
func (m SolarSystemModel) drawStarfield(grid [][]rune, cx, cy int, displayScale float64, cfg astro.ProjectionConfig) {
	h := len(grid)
	w := len(grid[0])

	// Get the bright star catalog
	catalog := astro.DefaultStarCatalog()

	// Adaptive shell radius: scale inversely with zoom so stars stay
	// at the edge of the viewport regardless of zoom level.
	// At zoom 1.0x with log scale, 100 AU gives ~2.0 display units.
	// We want stars to appear at ~viewportR/displayScale in display space.
	shellRadius := astro.DefaultStarShellRadiusAU / cfg.Scale

	// Set up projection config for stars
	starCfg := astro.ProjectionConfig{
		Scale:             cfg.Scale,
		Mode:              cfg.Mode,
		StarShellRadiusAU: shellRadius,
	}

	for _, star := range catalog.Stars {
		// Project star to ecliptic top-down view
		proj := astro.ProjectStarEclipticTopDown(star.RAdeg, star.DecDeg, starCfg)

		// Convert to screen coordinates (same as planets)
		sx := cx + int(proj.X*displayScale)
		sy := cy - int(proj.Y*displayScale*0.5) // Aspect ratio correction

		// Bounds check
		if sx < 0 || sx >= w || sy < 0 || sy >= h {
			continue
		}

		// Only draw on empty cells (don't overwrite anything)
		if grid[sy][sx] != ' ' {
			continue
		}

		// Select glyph based on magnitude (brighter = lower magnitude)
		glyph := m.starGlyph(star.Mag)
		if glyph != ' ' {
			grid[sy][sx] = glyph
		}
	}
}

// starGlyph returns a subtle glyph based on star magnitude.
// Brighter stars (lower magnitude) get slightly more prominent glyphs.
func (m SolarSystemModel) starGlyph(mag float64) rune {
	switch {
	case mag <= 1.0:
		return '∗' // Bright stars: slightly more visible
	case mag <= 2.5:
		return '·' // Medium stars: standard dot
	case mag <= 3.5:
		return '˙' // Dim stars: small dot
	default:
		return ' ' // Very dim: skip to avoid clutter
	}
}

// renderLabels draws body labels on the canvas based on label mode.
func (m SolarSystemModel) renderLabels(grid [][]rune, width, height int, positions []bodyPos) {
	if m.labelMode == LabelNone || len(positions) == 0 {
		return
	}

	// Process focused bodies first (they get priority)
	for _, pos := range positions {
		showLabel := false
		switch m.labelMode {
		case LabelFocused:
			showLabel = pos.isFocused
		case LabelAll:
			showLabel = true
		}

		if !showLabel {
			continue
		}

		// Determine label position (to the right of the glyph, with 1 space gap)
		labelX := pos.x + 2
		labelY := pos.y

		// Skip if label would start off-screen
		if labelY < 0 || labelY >= height || labelX >= width {
			continue
		}

		// Build label text
		labelText := pos.name
		if pos.isFocused {
			labelText = "◄ " + pos.name
		}

		// Write label characters to grid
		for i, r := range labelText {
			x := labelX + i
			if x >= width {
				break
			}
			// Only write if position is empty or has orbit ring
			if grid[labelY][x] == ' ' || grid[labelY][x] == '·' {
				grid[labelY][x] = r
			}
		}
	}
}

func (m SolarSystemModel) getBodyGlyph(body dsn.EclipticBody, focused bool) rune {
	switch body.Kind {
	case dsn.BodyPlanet:
		if body.Class == dsn.ClassGiant {
			if focused {
				return '◉'
			}
			return '○'
		}
		if focused {
			return '●'
		}
		return '•'
	case dsn.BodySpacecraft:
		if focused {
			return '◆'
		}
		return '◇'
	default:
		return '?'
	}
}

func (m SolarSystemModel) renderGrid(grid [][]rune, cx, cy int, scale float64, cfg astro.ProjectionConfig) string {
	var b strings.Builder

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	starStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236")) // Very dim for stars
	sunStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	planetStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	giantStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	scStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	focusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("249"))

	for y, row := range grid {
		for x, ch := range row {
			var style lipgloss.Style

			switch ch {
			case ' ':
				b.WriteRune(ch)
				continue
			case '·':
				style = dimStyle
			case '∗', '˙': // Star glyphs
				style = starStyle
			case '☉':
				style = sunStyle
			case '•':
				style = planetStyle
			case '○':
				style = giantStyle
			case '◇':
				style = scStyle
			case '●', '◉', '◆':
				style = focusStyle
			case '◄':
				// Focus indicator arrow
				style = focusStyle
			default:
				// Label text characters
				style = labelStyle
			}

			b.WriteString(style.Render(string(ch)))
			_ = x // Avoid unused variable warning
			_ = y
		}
		b.WriteRune('\n')
	}

	return b.String()
}

func (m SolarSystemModel) renderHUD() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(12)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Get focused body
	var focused *dsn.EclipticBody
	if m.focusIdx >= 0 && m.focusIdx < len(m.solarSnap.Bodies) {
		focused = &m.solarSnap.Bodies[m.focusIdx]
	}

	// Header line with focus info
	if focused != nil {
		b.WriteString(headerStyle.Render(fmt.Sprintf("◆ %s", focused.Name)))
		b.WriteString("  ")
		b.WriteString(labelStyle.Render("Distance:"))
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.3f AU", focused.DistanceAU())))
		b.WriteString("  ")
		b.WriteString(labelStyle.Render("Light Time:"))
		b.WriteString(valueStyle.Render(astro.FormatLightTime(focused.LightTimeSec())))
	} else {
		b.WriteString(headerStyle.Render("☉ Sun"))
		b.WriteString("  ")
		b.WriteString(dimStyle.Render("(center of solar system)"))
	}
	b.WriteString("\n")

	// Second line: coordinates + scale info
	if focused != nil {
		b.WriteString(labelStyle.Render("Ecl Lon:"))
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f°", focused.EclipticLonDeg())))
		b.WriteString("  ")
		b.WriteString(labelStyle.Render("Ecl Lat:"))
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f°", focused.EclipticLatDeg())))
		b.WriteString("  ")
	}

	// Scale mode indicator
	modeName := ""
	switch m.scaleMode {
	case astro.ScaleLogR:
		modeName = "Log"
	case astro.ScaleInner:
		modeName = "Inner"
	case astro.ScaleOuter:
		modeName = "Outer"
	}

	// Label mode indicator
	labelName := ""
	switch m.labelMode {
	case LabelNone:
		labelName = "off"
	case LabelFocused:
		labelName = "focus"
	case LabelAll:
		labelName = "all"
	}

	// Stars indicator
	starsName := "off"
	if m.showStars {
		starsName = "on"
	}

	// Use consistent label/value styling
	b.WriteString(dimStyle.Render("Mode:"))
	b.WriteString(valueStyle.Render(modeName))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("Zoom:"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.2gx", m.scale())))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("Labels:"))
	b.WriteString(valueStyle.Render(labelName))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("Stars:"))
	b.WriteString(valueStyle.Render(starsName))

	return b.String()
}

// FocusedBody returns the currently focused body, or nil for Sun.
func (m SolarSystemModel) FocusedBody() *dsn.EclipticBody {
	if m.focusIdx >= 0 && m.focusIdx < len(m.solarSnap.Bodies) {
		return &m.solarSnap.Bodies[m.focusIdx]
	}
	return nil
}

// ShowStars returns whether the starfield is visible.
func (m SolarSystemModel) ShowStars() bool {
	return m.showStars
}

// SetFocusByCode sets focus to a body by its code.
func (m *SolarSystemModel) SetFocusByCode(code string) {
	for i, body := range m.solarSnap.Bodies {
		if body.Code == code {
			m.focusIdx = i
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
