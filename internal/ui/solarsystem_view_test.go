package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/litescript/ls-horizons/internal/astro"
	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

func TestSolarSystemModelInit(t *testing.T) {
	m := NewSolarSystemModel()

	if m.focusIdx != -1 {
		t.Errorf("expected focusIdx -1 (Sun), got %d", m.focusIdx)
	}
	if m.scale() != 1.0 {
		t.Errorf("expected scale 1.0, got %f", m.scale())
	}
	if m.scaleMode != astro.ScaleLogR {
		t.Errorf("expected ScaleLogR, got %d", m.scaleMode)
	}
}

func TestSolarSystemModelSetSize(t *testing.T) {
	m := NewSolarSystemModel()
	m = m.SetSize(120, 40)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestSolarSystemModelFocusNavigation(t *testing.T) {
	m := NewSolarSystemModel()

	// Create test snapshot with bodies
	solarSnap := dsn.SolarSystemSnapshot{
		Bodies: []dsn.EclipticBody{
			{Name: "Sun", Code: "SUN", Kind: dsn.BodySun, Pos: astro.Vec3{}},
			{Name: "Earth", Code: "EARTH", Kind: dsn.BodyPlanet, Pos: astro.Vec3{X: 1}},
			{Name: "Mars", Code: "MARS", Kind: dsn.BodyPlanet, Pos: astro.Vec3{X: 1.5}},
			{Name: "Voyager 1", Code: "VGR1", Kind: dsn.BodySpacecraft, Pos: astro.Vec3{X: 160}},
		},
	}

	m = m.UpdateData(state.Snapshot{}, solarSnap)

	// Focus starts on Sun (index -1)
	if m.focusIdx != -1 {
		t.Errorf("expected focusIdx -1, got %d", m.focusIdx)
	}

	// Navigate next (k)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.focusIdx != 0 {
		t.Errorf("after next, expected focusIdx 0, got %d", m.focusIdx)
	}

	// Navigate next again
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.focusIdx != 1 {
		t.Errorf("after next again, expected focusIdx 1, got %d", m.focusIdx)
	}

	// Navigate prev (j)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.focusIdx != 0 {
		t.Errorf("after prev, expected focusIdx 0, got %d", m.focusIdx)
	}
}

func TestSolarSystemModelZoom(t *testing.T) {
	m := NewSolarSystemModel()

	// Initial scale is 1.0
	if m.scale() != 1.0 {
		t.Errorf("expected initial scale 1.0, got %f", m.scale())
	}

	// Zoom in
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	if m.scale() != 1.5 {
		t.Errorf("expected scale 1.5 after zoom in, got %f", m.scale())
	}

	// Zoom out twice to get back to 1.0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if m.scale() != 1.0 {
		t.Errorf("expected scale 1.0 after zoom out, got %f", m.scale())
	}

	// Reset with 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	if m.scale() != 1.0 {
		t.Errorf("expected scale 1.0 after reset, got %f", m.scale())
	}
}

func TestSolarSystemModelPan(t *testing.T) {
	m := NewSolarSystemModel()

	// Pan right (arrow key)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.panX <= 0 {
		t.Errorf("expected panX > 0 after pan right, got %f", m.panX)
	}

	// Pan up (arrow key)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.panY >= 0 {
		t.Errorf("expected panY < 0 after pan up, got %f", m.panY)
	}

	// Center
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.panX != 0 || m.panY != 0 {
		t.Errorf("expected pan (0, 0) after center, got (%f, %f)", m.panX, m.panY)
	}

	// Reset with 'r' also centers
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m.panX != 0 || m.panY != 0 {
		t.Errorf("expected pan (0, 0) after reset, got (%f, %f)", m.panX, m.panY)
	}
}

func TestSolarSystemModelScaleMode(t *testing.T) {
	m := NewSolarSystemModel()

	// Initial mode
	if m.scaleMode != astro.ScaleLogR {
		t.Errorf("expected initial mode ScaleLogR, got %d", m.scaleMode)
	}

	// Toggle mode (z key now)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.scaleMode != astro.ScaleInner {
		t.Errorf("expected ScaleInner after toggle, got %d", m.scaleMode)
	}

	// Toggle again
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.scaleMode != astro.ScaleOuter {
		t.Errorf("expected ScaleOuter after second toggle, got %d", m.scaleMode)
	}

	// Toggle back to LogR
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m.scaleMode != astro.ScaleLogR {
		t.Errorf("expected ScaleLogR after third toggle, got %d", m.scaleMode)
	}
}

func TestSolarSystemModelView(t *testing.T) {
	m := NewSolarSystemModel()
	m = m.SetSize(80, 24)

	solarSnap := dsn.SolarSystemSnapshot{
		Bodies: []dsn.EclipticBody{
			{Name: "Sun", Code: "SUN", Kind: dsn.BodySun, Pos: astro.Vec3{}},
			{Name: "Earth", Code: "EARTH", Kind: dsn.BodyPlanet, Class: dsn.ClassInner, Pos: astro.Vec3{X: 1}},
		},
	}
	m = m.UpdateData(state.Snapshot{}, solarSnap)

	view := m.View()
	if len(view) == 0 {
		t.Error("expected non-empty view")
	}

	// Check that view contains expected elements
	if !containsRune(view, '☉') {
		t.Error("view should contain Sun glyph ☉")
	}
}

func TestSolarSystemModelFocusedBody(t *testing.T) {
	m := NewSolarSystemModel()

	solarSnap := dsn.SolarSystemSnapshot{
		Bodies: []dsn.EclipticBody{
			{Name: "Sun", Code: "SUN", Kind: dsn.BodySun},
			{Name: "Earth", Code: "EARTH", Kind: dsn.BodyPlanet},
		},
	}
	m = m.UpdateData(state.Snapshot{}, solarSnap)

	// Initially focused on Sun (nil because focusIdx = -1)
	if m.FocusedBody() != nil {
		t.Error("expected nil for Sun focus")
	}

	// Focus next (k key)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	focused := m.FocusedBody()
	if focused == nil || focused.Name != "Sun" {
		// Note: index 0 is Sun in bodies list
		t.Errorf("expected Sun at index 0, got %v", focused)
	}

	// Focus next again
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	focused = m.FocusedBody()
	if focused == nil || focused.Name != "Earth" {
		t.Errorf("expected Earth, got %v", focused)
	}
}

func TestSolarSystemModelSetFocusByCode(t *testing.T) {
	m := NewSolarSystemModel()

	solarSnap := dsn.SolarSystemSnapshot{
		Bodies: []dsn.EclipticBody{
			{Name: "Sun", Code: "SUN", Kind: dsn.BodySun},
			{Name: "Earth", Code: "EARTH", Kind: dsn.BodyPlanet},
			{Name: "Mars", Code: "MARS", Kind: dsn.BodyPlanet},
		},
	}
	m = m.UpdateData(state.Snapshot{}, solarSnap)

	m.SetFocusByCode("MARS")
	focused := m.FocusedBody()
	if focused == nil || focused.Code != "MARS" {
		t.Errorf("expected MARS after SetFocusByCode, got %v", focused)
	}
}

func TestSolarSystemModelLabelMode(t *testing.T) {
	m := NewSolarSystemModel()

	// Default should be LabelFocused
	if m.labelMode != LabelFocused {
		t.Errorf("initial labelMode = %d, want %d (LabelFocused)", m.labelMode, LabelFocused)
	}

	// Toggle with 'l' key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.labelMode != LabelAll {
		t.Errorf("after first toggle, labelMode = %d, want %d (LabelAll)", m.labelMode, LabelAll)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.labelMode != LabelNone {
		t.Errorf("after second toggle, labelMode = %d, want %d (LabelNone)", m.labelMode, LabelNone)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.labelMode != LabelFocused {
		t.Errorf("after third toggle, labelMode = %d, want %d (LabelFocused)", m.labelMode, LabelFocused)
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

func TestSolarSystemModelStarfieldToggle(t *testing.T) {
	m := NewSolarSystemModel()

	// Starfield should be on by default
	if !m.ShowStars() {
		t.Error("expected showStars true by default")
	}

	// Toggle off with 't' key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if m.ShowStars() {
		t.Error("expected showStars false after first toggle")
	}

	// Toggle back on
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !m.ShowStars() {
		t.Error("expected showStars true after second toggle")
	}
}

func TestSolarSystemModelStarfieldRenderNoPanic(t *testing.T) {
	m := NewSolarSystemModel()
	m = m.SetSize(80, 24)

	// With stars on
	m.showStars = true
	solarSnap := dsn.SolarSystemSnapshot{
		Bodies: []dsn.EclipticBody{
			{Name: "Sun", Code: "SUN", Kind: dsn.BodySun, Pos: astro.Vec3{}},
			{Name: "Earth", Code: "EARTH", Kind: dsn.BodyPlanet, Class: dsn.ClassInner, Pos: astro.Vec3{X: 1}},
		},
	}
	m = m.UpdateData(state.Snapshot{}, solarSnap)

	// Should not panic
	view := m.View()
	if len(view) == 0 {
		t.Error("expected non-empty view with stars on")
	}

	// Toggle stars off
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})

	// Should also not panic with stars off
	view = m.View()
	if len(view) == 0 {
		t.Error("expected non-empty view with stars off")
	}
}

func TestSolarSystemModelStarfieldHUD(t *testing.T) {
	m := NewSolarSystemModel()
	m = m.SetSize(120, 30)

	solarSnap := dsn.SolarSystemSnapshot{
		Bodies: []dsn.EclipticBody{
			{Name: "Sun", Code: "SUN", Kind: dsn.BodySun, Pos: astro.Vec3{}},
		},
	}
	m = m.UpdateData(state.Snapshot{}, solarSnap)

	// With stars on, HUD should show "Stars:on"
	view := m.View()
	if !strings.Contains(view, "Stars:") {
		t.Error("HUD should contain 'Stars:' indicator")
	}
}

func TestSolarSystemModelStarGlyph(t *testing.T) {
	m := NewSolarSystemModel()

	tests := []struct {
		mag       float64
		wantGlyph rune
	}{
		{-1.0, '∗'}, // Very bright (Sirius)
		{0.5, '∗'},  // Bright
		{1.0, '∗'},  // Threshold
		{1.5, '·'},  // Medium
		{2.5, '·'},  // Medium threshold
		{3.0, '˙'},  // Dim
		{3.5, '˙'},  // Dim threshold
		{4.0, ' '},  // Too dim
		{5.0, ' '},  // Very dim
	}

	for _, tt := range tests {
		got := m.starGlyph(tt.mag)
		if got != tt.wantGlyph {
			t.Errorf("starGlyph(%.1f) = %q, want %q", tt.mag, string(got), string(tt.wantGlyph))
		}
	}
}
