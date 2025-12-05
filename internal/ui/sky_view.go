package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/litescript/ls-horizons/internal/astro"
	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

const (
	// Field of view in degrees
	fovAz = 120.0 // horizontal FOV
	fovEl = 60.0  // vertical FOV

	// Animation
	animDuration  = 400 * time.Millisecond
	animFrameRate = 30 * time.Millisecond

	// Spacecraft glyphs
	glyphSpacecraft        = '✦'
	glyphSpacecraftFocused = '◆'

	// Spacecraft colors
	colorSpacecraft        = "#d0c8ff"
	colorSpacecraftFocused = "229" // bright gold

	// Star glyphs by magnitude
	glyphStarBright   = '✶' // mag < 1.5
	glyphStarMedium   = '✸' // mag 1.5-3.0
	glyphStarDim      = '·' // mag 3.0-4.0
	glyphStarVeryDim  = '·' // mag > 4.0

	// Star colors (grayscale to not compete with spacecraft)
	colorStarBright  = "255" // bright white
	colorStarMedium  = "250" // medium gray
	colorStarDim     = "244" // dim gray
	colorStarVeryDim = "240" // very dim gray
)

// LabelMode controls how spacecraft labels are displayed.
type LabelMode int

const (
	LabelNone    LabelMode = iota // No labels
	LabelFocused                  // Only focused spacecraft
	LabelAll                      // All spacecraft
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

	// Focus - now operates on spacecraft, not individual links
	focusIdx   int
	spacecraft []dsn.SpacecraftView // grouped spacecraft with their links

	// Selected complex filter (empty = all)
	complex dsn.Complex

	// Label display mode
	labelMode LabelMode

	// Star catalog (loaded once)
	starCatalog astro.StarCatalog
}

// NewSkyViewModel creates a new sky view model.
func NewSkyViewModel() SkyViewModel {
	return SkyViewModel{
		camAz:       180,
		camEl:       45,
		labelMode:   LabelFocused, // default to showing focused spacecraft label
		starCatalog: astro.DefaultStarCatalog(),
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
	// Build spacecraft views (grouped, filtered)
	elevMap := dsn.BuildElevationMap(snapshot.Data)
	m.spacecraft = dsn.BuildSpacecraftViews(snapshot.Data, elevMap)

	// If focus is out of bounds, reset
	if m.focusIdx >= len(m.spacecraft) {
		m.focusIdx = 0
	}

	// If not animating, snap camera to focused spacecraft
	if !m.animating && len(m.spacecraft) > 0 && m.focusIdx < len(m.spacecraft) {
		coord := m.spacecraft[m.focusIdx].Coord()
		m.camAz = coord.AzDeg
		m.camEl = coord.ElDeg
	}

	return m
}

// SyncFromDashboard initializes sky view focus from dashboard selection.
func (m SkyViewModel) SyncFromDashboard(dash DashboardModel, snapshot state.Snapshot) SkyViewModel {
	// Build spacecraft views (grouped, filtered)
	elevMap := dsn.BuildElevationMap(snapshot.Data)
	m.spacecraft = dsn.BuildSpacecraftViews(snapshot.Data, elevMap)

	// Try to find the spacecraft selected in dashboard
	if sv := dash.GetSelectedSpacecraft(); sv != nil {
		for i, sc := range m.spacecraft {
			if sc.Code == sv.Code {
				m.focusIdx = i
				coord := sc.Coord()
				m.camAz = coord.AzDeg
				m.camEl = coord.ElDeg
				return m
			}
		}
	}

	// Default to first spacecraft
	if len(m.spacecraft) > 0 {
		m.focusIdx = 0
		coord := m.spacecraft[0].Coord()
		m.camAz = coord.AzDeg
		m.camEl = coord.ElDeg
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
		case "up", "k":
			return m.focusPrev()
		case "down", "j":
			return m.focusNext()
		case "l":
			// Cycle label mode
			m = m.cycleLabelMode()
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

func (m SkyViewModel) cycleLabelMode() SkyViewModel {
	m.labelMode = (m.labelMode + 1) % 3
	return m
}

func (m SkyViewModel) focusNext() (SkyViewModel, tea.Cmd) {
	if len(m.spacecraft) == 0 {
		return m, nil
	}
	m.focusIdx = (m.focusIdx + 1) % len(m.spacecraft)
	return m.startAnimation()
}

func (m SkyViewModel) focusPrev() (SkyViewModel, tea.Cmd) {
	if len(m.spacecraft) == 0 {
		return m, nil
	}
	m.focusIdx--
	if m.focusIdx < 0 {
		m.focusIdx = len(m.spacecraft) - 1
	}
	return m.startAnimation()
}

func (m SkyViewModel) startAnimation() (SkyViewModel, tea.Cmd) {
	if len(m.spacecraft) == 0 || m.focusIdx >= len(m.spacecraft) {
		return m, nil
	}

	coord := m.spacecraft[m.focusIdx].Coord()
	m.animating = true
	m.animStartAz = m.camAz
	m.animStartEl = m.camEl
	m.animTargAz = coord.AzDeg
	m.animTargEl = coord.ElDeg
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
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSpacecraft)) // soft purple

	title := titleStyle.Render("Sky View")

	// Complex filter
	complexStr := ""
	if m.complex == "" {
		complexStr = dimStyle.Render("All Complexes")
	} else {
		info := dsn.KnownComplexes[m.complex]
		complexStr = accentStyle.Render(info.Name)
	}

	// Label mode indicator
	var labelStr string
	switch m.labelMode {
	case LabelNone:
		labelStr = dimStyle.Render("Labels: off")
	case LabelFocused:
		labelStr = accentStyle.Render("Labels: focus")
	case LabelAll:
		labelStr = accentStyle.Render("Labels: all")
	}

	compass := dimStyle.Render(fmt.Sprintf("Az:%.0f° El:%.0f°", m.camAz, m.camEl))

	return fmt.Sprintf("%s | %s | %s | %s", title, complexStr, labelStr, compass)
}

func (m SkyViewModel) renderStatus() string {
	if len(m.spacecraft) == 0 {
		return "No spacecraft in view"
	}

	if m.focusIdx >= len(m.spacecraft) {
		return ""
	}

	sc := m.spacecraft[m.focusIdx]
	coord := sc.Coord()
	primary := sc.PrimaryLink

	// Build antenna list (e.g., "DSS34+DSS36")
	antennaList := sc.AntennaList()

	// First line: code @ antenna(s) with signal info from primary link
	band := primary.Band
	if band == "" {
		band = "-"
	}

	line1 := fmt.Sprintf(">>> %s @ %s [%s] | Az:%.0f° El:%.0f° | %s | Struggle: %.0f%%",
		sc.Code,
		antennaList,
		band,
		coord.AzDeg,
		coord.ElDeg,
		dsn.FormatDistance(primary.DistanceKm),
		primary.Struggle*100,
	)

	// Style the first line in gold
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	status := accentStyle.Render(line1)

	// Second line: full mission name (if known and different from code)
	if sc.Name != sc.Code {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorSpacecraft))
		status += "\n" + dimStyle.Render("    "+sc.Name)
	}

	return status
}

// spacecraftPos tracks spacecraft position for label rendering
type spacecraftPos struct {
	x, y       int
	name       string
	isFocused  bool
	labelStart int // calculated label start position
	labelEnd   int // calculated label end position
}

// getObserver returns the observer location based on the focused spacecraft's complex.
// Defaults to Goldstone if no spacecraft is focused.
func (m SkyViewModel) getObserver() astro.Observer {
	if len(m.spacecraft) > 0 && m.focusIdx < len(m.spacecraft) {
		primary := m.spacecraft[m.focusIdx].PrimaryLink
		return dsn.ObserverForComplex(primary.Complex)
	}
	return dsn.ObserverForComplex(dsn.ComplexGoldstone)
}

func (m SkyViewModel) renderSkyCanvas(width, height int) string {
	// Initialize canvas with empty space (very dark background)
	canvas := make([][]rune, height)
	colors := make([][]lipgloss.Color, height)
	for y := 0; y < height; y++ {
		canvas[y] = make([]rune, width)
		colors[y] = make([]lipgloss.Color, width)
		for x := 0; x < width; x++ {
			canvas[y][x] = ' '
			colors[y][x] = "236" // very dark background
		}
	}

	// Draw real stars from catalog
	horizonY := height - 2
	observer := m.getObserver()
	now := time.Now()

	for _, star := range m.starCatalog.Stars {
		// Convert RA/Dec to Az/El for current observer and time
		eq := astro.SkyCoord{RAdeg: star.RAdeg, DecDeg: star.DecDeg}
		horiz := astro.EquatorialToHorizontal(eq, observer, now)

		// Skip stars below horizon
		if horiz.ElDeg <= 0 {
			continue
		}

		// Project to screen
		x, y, visible := m.projectToScreen(horiz.AzDeg, horiz.ElDeg, width, height)
		if !visible {
			continue
		}

		// Clamp to canvas bounds (above horizon line)
		if x < 0 || x >= width || y < 0 || y >= horizonY {
			continue
		}

		// Choose glyph and color based on magnitude
		glyph, color := m.starGlyph(star.Mag)
		canvas[y][x] = glyph
		colors[y][x] = color
	}

	// Draw horizon line (purple tint)
	for x := 0; x < width; x++ {
		canvas[horizonY][x] = '─'
		colors[horizonY][x] = "60" // muted purple
	}

	// Draw cardinal directions on horizon
	m.drawCardinal(canvas, colors, width, height, "N", 0)
	m.drawCardinal(canvas, colors, width, height, "E", 90)
	m.drawCardinal(canvas, colors, width, height, "S", 180)
	m.drawCardinal(canvas, colors, width, height, "W", 270)

	// Collect spacecraft positions for label rendering
	var positions []spacecraftPos

	// Draw spacecraft (one glyph per spacecraft, using primary link position)
	for i, sc := range m.spacecraft {
		// Filter by complex if set (check primary link's complex)
		if m.complex != "" && sc.PrimaryLink.Complex != m.complex {
			continue
		}

		coord := sc.Coord()
		x, y, visible := m.projectToScreenCoord(coord, width, height)
		if !visible {
			continue
		}

		// Clamp to canvas bounds
		if x < 0 || x >= width || y < 0 || y >= horizonY {
			continue
		}

		isFocused := i == m.focusIdx

		// Choose symbol and color
		sym := glyphSpacecraft
		color := lipgloss.Color(colorSpacecraft)

		if isFocused {
			sym = glyphSpacecraftFocused
			color = colorSpacecraftFocused
		}

		canvas[y][x] = sym
		colors[y][x] = color

		// Track position for labels
		positions = append(positions, spacecraftPos{
			x:         x,
			y:         y,
			name:      sc.Code,
			isFocused: isFocused,
		})
	}

	// Draw labels based on label mode
	m.renderLabels(canvas, colors, width, horizonY, positions)

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

// renderLabels draws spacecraft labels on the canvas based on label mode.
// Focused spacecraft labels take priority in overlapping regions.
func (m SkyViewModel) renderLabels(canvas [][]rune, colors [][]lipgloss.Color, width, horizonY int, positions []spacecraftPos) {
	if m.labelMode == LabelNone || len(positions) == 0 {
		return
	}

	// Calculate label positions (to the right of spacecraft glyph with 1-char gap)
	for i := range positions {
		pos := &positions[i]
		// Label starts 2 chars after glyph (1 space gap)
		pos.labelStart = pos.x + 2
		// Focused labels have "► " prefix (2 extra chars)
		labelLen := len(pos.name)
		if pos.isFocused {
			labelLen += 2
		}
		pos.labelEnd = pos.labelStart + labelLen
	}

	// Track which x positions on each row are claimed by focused labels
	// This allows focused labels to "win" over non-focused ones
	focusedClaims := make(map[int]map[int]bool) // y -> x -> claimed

	// First pass: mark positions claimed by focused spacecraft
	for _, pos := range positions {
		if !pos.isFocused {
			continue
		}
		if focusedClaims[pos.y] == nil {
			focusedClaims[pos.y] = make(map[int]bool)
		}
		for x := pos.labelStart; x < pos.labelEnd; x++ {
			focusedClaims[pos.y][x] = true
		}
	}

	// Second pass: render labels
	for _, pos := range positions {
		// Check if we should render this label
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

		// Render label character by character
		labelColor := lipgloss.Color(colorSpacecraft)
		if pos.isFocused {
			labelColor = colorSpacecraftFocused
		}

		// Add arrow prefix for focused spacecraft: "◄ NAME" (points to spacecraft)
		labelText := pos.name
		if pos.isFocused {
			labelText = "◄ " + pos.name
		}

		labelRunes := []rune(labelText)
		for i, r := range labelRunes {
			x := pos.labelStart + i

			// Skip if out of bounds
			if x < 0 || x >= width || pos.y < 0 || pos.y >= horizonY {
				continue
			}

			// For non-focused labels, skip if position is claimed by focused
			if !pos.isFocused && focusedClaims[pos.y][x] {
				continue
			}

			canvas[pos.y][x] = r
			colors[pos.y][x] = labelColor
		}
	}
}

// starGlyph returns the appropriate glyph and color for a star based on its magnitude.
// Brighter stars (lower magnitude) get more prominent symbols.
func (m SkyViewModel) starGlyph(mag float64) (rune, lipgloss.Color) {
	switch {
	case mag < 1.5:
		return glyphStarBright, colorStarBright
	case mag < 3.0:
		return glyphStarMedium, colorStarMedium
	case mag < 4.0:
		return glyphStarDim, colorStarDim
	default:
		return glyphStarVeryDim, colorStarVeryDim
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

// projectToScreenCoord converts a SkyCoord to screen coordinates.
// This is the primary projection interface - takes SkyCoord for future-proofing
// when we add JPL Horizons support.
func (m SkyViewModel) projectToScreenCoord(coord dsn.SkyCoord, width, height int) (int, int, bool) {
	return m.projectToScreen(coord.AzDeg, coord.ElDeg, width, height)
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
