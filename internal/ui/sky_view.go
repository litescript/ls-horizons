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
	"github.com/litescript/ls-horizons/internal/ephem"
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
	glyphStarBright  = '✶' // mag < 1.5
	glyphStarMedium  = '✸' // mag 1.5-3.0
	glyphStarDim     = '·' // mag 3.0-4.0
	glyphStarVeryDim = '·' // mag > 4.0

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

// PathMode controls whether trajectory path is displayed.
type PathMode int

const (
	PathOff PathMode = iota // No path display
	PathOn                  // Show trajectory arc
)

// Path display constants
const (
	// Path colors - gradient from past to future
	colorPathPast   = "#6B5B7A" // muted purple for past
	colorPathNow    = "#B794F6" // bright purple for current position
	colorPathFuture = "#9D7CD8" // medium purple for future

	// Path refresh interval
	pathRefreshInterval = 5 * time.Minute
)

// Braille dot positions for 2x4 subpixel rendering
// Each braille character is 2 dots wide, 4 dots tall
// Bit positions:
//
//	0x01 0x08
//	0x02 0x10
//	0x04 0x20
//	0x40 0x80
var brailleDots = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

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

	// Path display mode and data
	pathMode         PathMode
	pathProvider     ephem.Provider // nil = paths disabled
	currentPath      ephem.EphemerisPath
	pathFocusTarget  ephem.TargetID // NAIF ID of focused target for path
	pathLastFetch    time.Time
	pathFetchPending bool

	// Star catalog (loaded once)
	starCatalog astro.StarCatalog
}

// NewSkyViewModel creates a new sky view model.
func NewSkyViewModel() SkyViewModel {
	return SkyViewModel{
		camAz:       180,
		camEl:       45,
		labelMode:   LabelFocused, // default to showing focused spacecraft label
		pathMode:    PathOff,      // paths off by default until provider is set
		starCatalog: astro.DefaultStarCatalog(),
	}
}

// SetPathProvider sets the ephemeris provider for trajectory paths.
func (m SkyViewModel) SetPathProvider(provider ephem.Provider) SkyViewModel {
	m.pathProvider = provider
	return m
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

// pathFetchMsg is sent when a path fetch completes
type pathFetchMsg struct {
	path ephem.EphemerisPath
	err  error
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
		case "p":
			// Toggle path mode
			return m.togglePathMode()
		}

	case animTickMsg:
		if m.animating {
			return m.updateAnimation()
		}

	case pathFetchMsg:
		m.pathFetchPending = false
		if msg.err == nil {
			m.currentPath = msg.path
			m.pathLastFetch = time.Now()
		}
	}

	return m, nil
}

func (m SkyViewModel) cycleLabelMode() SkyViewModel {
	m.labelMode = (m.labelMode + 1) % 3
	return m
}

func (m SkyViewModel) togglePathMode() (SkyViewModel, tea.Cmd) {
	// Can only enable path mode if provider is available
	if m.pathProvider == nil {
		return m, nil
	}

	if m.pathMode == PathOff {
		m.pathMode = PathOn
		// Trigger path fetch for focused spacecraft
		return m.fetchPathForFocus()
	}

	m.pathMode = PathOff
	m.currentPath = ephem.EphemerisPath{}
	return m, nil
}

func (m SkyViewModel) fetchPathForFocus() (SkyViewModel, tea.Cmd) {
	if m.pathProvider == nil || len(m.spacecraft) == 0 || m.focusIdx >= len(m.spacecraft) {
		return m, nil
	}

	sc := m.spacecraft[m.focusIdx]
	naifID := ephem.GetNAIFID(sc.Code)
	if naifID == 0 {
		// Unknown spacecraft - can't fetch path
		return m, nil
	}

	// Check if we already have a recent path for this target
	if m.pathFocusTarget == naifID && time.Since(m.pathLastFetch) < pathRefreshInterval {
		return m, nil
	}

	m.pathFocusTarget = naifID
	m.pathFetchPending = true

	// Get observer for the focused spacecraft's complex
	obs := m.getObserver()

	// Create async fetch command
	provider := m.pathProvider
	now := time.Now()
	// Fetch ±6 hours with 5-minute steps for smooth arcs
	start := now.Add(-6 * time.Hour)
	end := now.Add(6 * time.Hour)
	step := 5 * time.Minute

	return m, func() tea.Msg {
		path, err := provider.GetPath(naifID, start, end, step, obs)
		return pathFetchMsg{path: path, err: err}
	}
}

func (m SkyViewModel) focusNext() (SkyViewModel, tea.Cmd) {
	if len(m.spacecraft) == 0 {
		return m, nil
	}
	m.focusIdx = (m.focusIdx + 1) % len(m.spacecraft)
	m, animCmd := m.startAnimation()

	// If path mode is on, fetch path for new focus
	var pathCmd tea.Cmd
	if m.pathMode == PathOn {
		m, pathCmd = m.fetchPathForFocus()
	}

	return m, tea.Batch(animCmd, pathCmd)
}

func (m SkyViewModel) focusPrev() (SkyViewModel, tea.Cmd) {
	if len(m.spacecraft) == 0 {
		return m, nil
	}
	m.focusIdx--
	if m.focusIdx < 0 {
		m.focusIdx = len(m.spacecraft) - 1
	}
	m, animCmd := m.startAnimation()

	// If path mode is on, fetch path for new focus
	var pathCmd tea.Cmd
	if m.pathMode == PathOn {
		m, pathCmd = m.fetchPathForFocus()
	}

	return m, tea.Batch(animCmd, pathCmd)
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

	// Path mode indicator
	var pathStr string
	if m.pathProvider == nil {
		pathStr = dimStyle.Render("Path: n/a")
	} else if m.pathMode == PathOff {
		pathStr = dimStyle.Render("Path: off")
	} else if m.pathFetchPending {
		pathStr = accentStyle.Render("Path: loading...")
	} else {
		pathStr = accentStyle.Render("Path: on")
	}

	compass := dimStyle.Render(fmt.Sprintf("Az:%.0f° El:%.0f°", m.camAz, m.camEl))

	return fmt.Sprintf("%s | %s | %s | %s | %s", title, complexStr, labelStr, pathStr, compass)
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

	// Draw trajectory path if enabled and available
	if m.pathMode == PathOn && len(m.currentPath.Points) > 0 {
		m.renderPath(canvas, colors, width, horizonY, now)
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

// brailleCanvas holds subpixel data for smooth arc rendering.
// Each cell maps to a 2x4 grid of braille dots.
type brailleCanvas struct {
	width, height int
	dots          [][]rune           // accumulated braille patterns
	colors        [][]lipgloss.Color // color per cell
}

func newBrailleCanvas(width, height int) *brailleCanvas {
	bc := &brailleCanvas{
		width:  width,
		height: height,
		dots:   make([][]rune, height),
		colors: make([][]lipgloss.Color, height),
	}
	for y := 0; y < height; y++ {
		bc.dots[y] = make([]rune, width)
		bc.colors[y] = make([]lipgloss.Color, width)
		for x := 0; x < width; x++ {
			bc.colors[y][x] = colorPathFuture // default color
		}
	}
	return bc
}

// setDot sets a subpixel dot at high-resolution coordinates.
// subX is 0-1 within the cell, subY is 0-3 within the cell.
func (bc *brailleCanvas) setDot(cellX, cellY, subX, subY int, color lipgloss.Color) {
	if cellX < 0 || cellX >= bc.width || cellY < 0 || cellY >= bc.height {
		return
	}
	if subX < 0 || subX > 1 || subY < 0 || subY > 3 {
		return
	}
	bc.dots[cellY][cellX] |= brailleDots[subY][subX]
	bc.colors[cellY][cellX] = color
}

// setPixel sets a dot using floating-point coordinates with 2x4 subpixel resolution.
func (bc *brailleCanvas) setPixel(fx, fy float64, color lipgloss.Color) {
	// Scale to subpixel grid (2x horizontal, 4x vertical)
	subX := fx * 2
	subY := fy * 4

	cellX := int(subX) / 2
	cellY := int(subY) / 4
	dotX := int(subX) % 2
	dotY := int(subY) % 4

	bc.setDot(cellX, cellY, dotX, dotY, color)
}

// render composites the braille canvas onto the main canvas.
func (bc *brailleCanvas) render(canvas [][]rune, colors [][]lipgloss.Color) {
	for y := 0; y < bc.height && y < len(canvas); y++ {
		for x := 0; x < bc.width && x < len(canvas[y]); x++ {
			if bc.dots[y][x] != 0 {
				// Only draw on empty cells or other braille
				if canvas[y][x] == ' ' || (canvas[y][x] >= 0x2800 && canvas[y][x] <= 0x28FF) {
					if canvas[y][x] >= 0x2800 && canvas[y][x] <= 0x28FF {
						// Merge with existing braille
						canvas[y][x] = 0x2800 | (canvas[y][x] - 0x2800) | bc.dots[y][x]
					} else {
						canvas[y][x] = 0x2800 | bc.dots[y][x]
					}
					colors[y][x] = bc.colors[y][x]
				}
			}
		}
	}
}

// renderPath draws the trajectory path arc using braille subpixels for smooth curves.
func (m SkyViewModel) renderPath(canvas [][]rune, colors [][]lipgloss.Color, width, horizonY int, now time.Time) {
	if len(m.currentPath.Points) == 0 {
		return
	}

	bc := newBrailleCanvas(width, horizonY)

	// Collect visible points with screen coordinates
	type screenPoint struct {
		x, y   float64
		isPast bool
	}
	var points []screenPoint

	for _, point := range m.currentPath.Points {
		if !point.Valid {
			continue
		}

		// Project to screen (use float for subpixel precision)
		fx, fy, visible := m.projectToScreenFloat(point.Coord.AzDeg, point.Coord.ElDeg, width, horizonY)
		if !visible {
			continue
		}

		// Skip points outside bounds
		if fx < 0 || fx >= float64(width) || fy < 0 || fy >= float64(horizonY) {
			continue
		}

		points = append(points, screenPoint{
			x:      fx,
			y:      fy,
			isPast: point.Time.Before(now),
		})
	}

	if len(points) < 2 {
		return
	}

	// Draw connected line segments between points
	for i := 0; i < len(points)-1; i++ {
		p0 := points[i]
		p1 := points[i+1]

		// Choose color based on time
		var color lipgloss.Color
		if p0.isPast && p1.isPast {
			color = colorPathPast
		} else if !p0.isPast && !p1.isPast {
			color = colorPathFuture
		} else {
			color = colorPathNow // transition point
		}

		// Draw line using Bresenham-style interpolation with subpixel precision
		drawBrailleLine(bc, p0.x, p0.y, p1.x, p1.y, color)
	}

	// Composite onto main canvas
	bc.render(canvas, colors)
}

// drawBrailleLine draws a line between two points with subpixel precision.
func drawBrailleLine(bc *brailleCanvas, x0, y0, x1, y1 float64, color lipgloss.Color) {
	dx := x1 - x0
	dy := y1 - y0
	dist := math.Sqrt(dx*dx + dy*dy)

	if dist < 0.1 {
		bc.setPixel(x0, y0, color)
		return
	}

	// Step size for smooth curves (smaller = smoother but more dots)
	steps := int(dist*4) + 1
	if steps > 200 {
		steps = 200
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := x0 + dx*t
		y := y0 + dy*t
		bc.setPixel(x, y, color)
	}
}

// projectToScreenFloat is like projectToScreen but returns float coordinates.
func (m SkyViewModel) projectToScreenFloat(az, el float64, width, height int) (float64, float64, bool) {
	dAz := normalizeAngle(az - m.camAz)
	dEl := el - m.camEl

	if dAz < -fovAz/2 || dAz > fovAz/2 {
		return 0, 0, false
	}
	if dEl < -fovEl/2 || dEl > fovEl/2 {
		return 0, 0, false
	}

	x := (dAz + fovAz/2) / fovAz * float64(width)
	y := (fovEl/2 - dEl) / fovEl * float64(height)

	return x, y, true
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
