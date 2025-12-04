package dsn

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// SnapshotExport is the JSON-serializable representation of DSN state.
type SnapshotExport struct {
	Timestamp    time.Time       `json:"timestamp"`
	FetchedAt    time.Time       `json:"fetched_at"`
	Stations     []StationExport `json:"stations"`
	Links        []LinkExport    `json:"links"`
	ComplexLoads []ComplexLoad   `json:"complex_loads"`
}

// StationExport is a JSON-friendly station representation.
type StationExport struct {
	Name         string          `json:"name"`
	FriendlyName string          `json:"friendly_name"`
	Complex      string          `json:"complex"`
	Antennas     []AntennaExport `json:"antennas"`
}

// AntennaExport is a JSON-friendly antenna representation.
type AntennaExport struct {
	ID        string  `json:"id"`
	Azimuth   float64 `json:"azimuth"`
	Elevation float64 `json:"elevation"`
	Activity  string  `json:"activity,omitempty"`
}

// LinkExport is a JSON-friendly link with derived fields.
type LinkExport struct {
	Complex       string  `json:"complex"`
	StationID     string  `json:"station_id"`
	AntennaID     string  `json:"antenna_id"`
	Spacecraft    string  `json:"spacecraft"`
	SpacecraftID  int     `json:"spacecraft_id"`
	Band          string  `json:"band"`
	DataRate      float64 `json:"data_rate_bps"`
	Distance      float64 `json:"distance_km"`
	RTLT          float64 `json:"rtlt_seconds"`
	Elevation     float64 `json:"elevation"`
	StruggleIndex float64 `json:"struggle_index"`
	Health        string  `json:"health"`
}

// ExportSnapshot converts DSNData to an exportable format.
func ExportSnapshot(data *DSNData, fetchedAt time.Time) *SnapshotExport {
	if data == nil {
		return &SnapshotExport{FetchedAt: fetchedAt}
	}

	export := &SnapshotExport{
		Timestamp: data.Timestamp,
		FetchedAt: fetchedAt,
	}

	// Build elevation map for struggle calculations
	elevMap := make(map[string]float64)
	for _, station := range data.Stations {
		stn := StationExport{
			Name:         station.Name,
			FriendlyName: station.FriendlyName,
			Complex:      string(station.Complex),
		}
		for _, ant := range station.Antennas {
			stn.Antennas = append(stn.Antennas, AntennaExport{
				ID:        ant.ID,
				Azimuth:   ant.Azimuth,
				Elevation: ant.Elevation,
				Activity:  ant.Activity,
			})
			elevMap[ant.ID] = ant.Elevation
		}
		export.Stations = append(export.Stations, stn)
	}

	// Export links with derived metrics
	for _, link := range data.Links {
		elev := elevMap[link.AntennaID]
		struggle, health := LinkHealth(link, elev)
		export.Links = append(export.Links, LinkExport{
			Complex:       string(link.Complex),
			StationID:     link.StationID,
			AntennaID:     link.AntennaID,
			Spacecraft:    link.Spacecraft,
			SpacecraftID:  link.SpacecraftID,
			Band:          link.Band,
			DataRate:      link.DataRate,
			Distance:      link.Distance,
			RTLT:          link.RTLT,
			Elevation:     elev,
			StruggleIndex: struggle,
			Health:        string(health),
		})
	}

	// Add complex loads
	loads := ComplexUtilization(data)
	for _, load := range loads {
		export.ComplexLoads = append(export.ComplexLoads, load)
	}

	return export
}

// WriteJSON writes the snapshot as JSON to the given writer.
func (s *SnapshotExport) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// SummaryRow represents one row in the summary table.
type SummaryRow struct {
	Complex    string
	Station    string
	Antenna    string
	Spacecraft string
	Band       string
	Rate       string
	Distance   string
	Struggle   float64
	Health     Health
}

// GenerateSummaryRows creates summary rows from DSN data.
func GenerateSummaryRows(data *DSNData) []SummaryRow {
	if data == nil {
		return nil
	}

	// Build elevation map
	elevMap := make(map[string]float64)
	for _, station := range data.Stations {
		for _, ant := range station.Antennas {
			elevMap[ant.ID] = ant.Elevation
		}
	}

	var rows []SummaryRow
	for _, link := range data.Links {
		elev := elevMap[link.AntennaID]
		struggle, health := LinkHealth(link, elev)

		rows = append(rows, SummaryRow{
			Complex:    string(link.Complex),
			Station:    link.StationID,
			Antenna:    link.AntennaID,
			Spacecraft: link.Spacecraft,
			Band:       link.Band,
			Rate:       FormatDataRate(link.DataRate),
			Distance:   FormatDistance(link.Distance),
			Struggle:   struggle,
			Health:     health,
		})
	}
	return rows
}

// WriteSummaryTable writes a text table to the given writer.
func WriteSummaryTable(w io.Writer, data *DSNData, timestamp time.Time) {
	rows := GenerateSummaryRows(data)

	fmt.Fprintf(w, "DSN Status @ %s\n", timestamp.Format(time.RFC3339))
	fmt.Fprintln(w, strings.Repeat("─", 90))

	if len(rows) == 0 {
		fmt.Fprintln(w, "No active links")
		return
	}

	// Header
	fmt.Fprintf(w, "%-8s %-8s %-8s %-14s %-4s %-10s %-12s %-6s %-8s\n",
		"Complex", "Station", "Antenna", "Spacecraft", "Band", "Rate", "Distance", "Strug", "Health")
	fmt.Fprintln(w, strings.Repeat("─", 90))

	// Rows
	for _, r := range rows {
		fmt.Fprintf(w, "%-8s %-8s %-8s %-14s %-4s %-10s %-12s %5.0f%% %-8s\n",
			truncateStr(r.Complex, 8),
			truncateStr(r.Station, 8),
			truncateStr(r.Antenna, 8),
			truncateStr(r.Spacecraft, 14),
			r.Band,
			r.Rate,
			r.Distance,
			r.Struggle*100,
			r.Health,
		)
	}

	fmt.Fprintf(w, "\nTotal: %d active links\n", len(rows))
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}

// MiniSkyConfig configures the mini sky view.
type MiniSkyConfig struct {
	Width  int
	Height int
}

// DefaultMiniSkyConfig returns default mini sky dimensions.
func DefaultMiniSkyConfig() MiniSkyConfig {
	return MiniSkyConfig{Width: 60, Height: 5}
}

// WriteMiniSky renders a simple ASCII sky view.
func WriteMiniSky(w io.Writer, data *DSNData, cfg MiniSkyConfig) {
	if data == nil || len(data.Links) == 0 {
		fmt.Fprintln(w, "No spacecraft in view")
		return
	}

	// Initialize canvas
	canvas := make([][]rune, cfg.Height)
	for y := range canvas {
		canvas[y] = make([]rune, cfg.Width)
		for x := range canvas[y] {
			// Simple seeded starfield
			if (x*7+y*13)%23 == 0 {
				canvas[y][x] = '·'
			} else if (x*11+y*17)%37 == 0 {
				canvas[y][x] = '.'
			} else {
				canvas[y][x] = ' '
			}
		}
	}

	// Horizon line
	for x := 0; x < cfg.Width; x++ {
		canvas[cfg.Height-1][x] = '─'
	}

	// Get sky objects and place them
	objs := data.SkyObjects()
	for _, obj := range objs {
		// Map azimuth (0-360) to x (0-width)
		x := int((obj.Azimuth / 360.0) * float64(cfg.Width))
		if x >= cfg.Width {
			x = cfg.Width - 1
		}
		// Map elevation (0-90) to y (height-2 to 0, higher = lower y)
		y := cfg.Height - 2 - int((obj.Elevation/90.0)*float64(cfg.Height-2))
		if y < 0 {
			y = 0
		}
		if y >= cfg.Height-1 {
			y = cfg.Height - 2
		}

		// Place spacecraft marker
		if x >= 0 && x < cfg.Width && y >= 0 && y < cfg.Height {
			canvas[y][x] = '◆'
		}
	}

	// Place station marker at bottom center
	canvas[cfg.Height-1][cfg.Width/2] = '▲'

	// Render
	fmt.Fprintln(w, "┌"+strings.Repeat("─", cfg.Width)+"┐")
	for _, row := range canvas {
		fmt.Fprintf(w, "│%s│\n", string(row))
	}
	fmt.Fprintln(w, "└"+strings.Repeat("─", cfg.Width)+"┘")

	// Legend
	if len(objs) > 0 {
		var legend []string
		for _, obj := range objs {
			legend = append(legend, fmt.Sprintf("◆%s", truncateStr(obj.Spacecraft, 8)))
		}
		fmt.Fprintf(w, " %s\n", strings.Join(legend, " "))
	}
}

// WriteNowPlaying prints a single-line status for all active links.
func WriteNowPlaying(w io.Writer, data *DSNData) {
	if data == nil || len(data.Links) == 0 {
		fmt.Fprintln(w, "◌ No active links")
		return
	}

	// Build elevation map
	elevMap := make(map[string]float64)
	for _, station := range data.Stations {
		for _, ant := range station.Antennas {
			elevMap[ant.ID] = ant.Elevation
		}
	}

	var parts []string
	for _, link := range data.Links {
		elev := elevMap[link.AntennaID]
		_, health := LinkHealth(link, elev)
		healthIcon := "●"
		switch health {
		case HealthMarginal:
			healthIcon = "◐"
		case HealthPoor:
			healthIcon = "○"
		}
		parts = append(parts, fmt.Sprintf("%s %s→%s %s %s",
			healthIcon,
			link.AntennaID,
			truncateStr(link.Spacecraft, 10),
			FormatRTLT(link.RTLT),
			FormatDataRate(link.DataRate),
		))
	}
	fmt.Fprintln(w, strings.Join(parts, " | "))
}

// SpacecraftCard holds data for a single spacecraft card view.
type SpacecraftCard struct {
	Name     string
	Links    []Link
	Distance string
	RTLT     string
	Rate     string
	Health   Health
	Struggle float64
	Band     string
	Antenna  string
	Complex  string
}

// WriteSpacecraftCard prints a vertical card for a single spacecraft.
func WriteSpacecraftCard(w io.Writer, data *DSNData, name string, events []Event) {
	if data == nil {
		fmt.Fprintf(w, "Spacecraft %q not found\n", name)
		return
	}

	// Find links for this spacecraft
	var card *SpacecraftCard
	elevMap := make(map[string]float64)
	for _, station := range data.Stations {
		for _, ant := range station.Antennas {
			elevMap[ant.ID] = ant.Elevation
		}
	}

	for _, link := range data.Links {
		if strings.EqualFold(link.Spacecraft, name) {
			elev := elevMap[link.AntennaID]
			struggle, health := LinkHealth(link, elev)
			card = &SpacecraftCard{
				Name:     link.Spacecraft,
				Distance: FormatDistance(link.Distance),
				RTLT:     FormatRTLT(link.RTLT),
				Rate:     FormatDataRate(link.DataRate),
				Health:   health,
				Struggle: struggle,
				Band:     link.Band,
				Antenna:  link.AntennaID,
				Complex:  string(link.Complex),
			}
			break
		}
	}

	if card == nil {
		fmt.Fprintf(w, "Spacecraft %q not currently tracked\n", name)
		return
	}

	// Render card
	fmt.Fprintln(w, "┌────────────────────────┐")
	fmt.Fprintf(w, "│ %-22s │\n", truncateStr(card.Name, 22))
	fmt.Fprintln(w, "├────────────────────────┤")
	fmt.Fprintf(w, "│ Distance: %-12s │\n", card.Distance)
	fmt.Fprintf(w, "│ RTT:      %-12s │\n", card.RTLT)
	fmt.Fprintf(w, "│ Rate:     %-12s │\n", card.Rate)
	fmt.Fprintf(w, "│ Band:     %-12s │\n", card.Band)
	fmt.Fprintf(w, "│ Health:   %-12s │\n", card.Health)
	fmt.Fprintf(w, "│ Antenna:  %-12s │\n", card.Antenna)
	fmt.Fprintf(w, "│ Complex:  %-12s │\n", card.Complex)
	fmt.Fprintln(w, "└────────────────────────┘")

	// Recent events
	if len(events) > 0 {
		fmt.Fprintln(w, "Recent events:")
		count := 0
		for i := len(events) - 1; i >= 0 && count < 5; i-- {
			e := events[i]
			if strings.EqualFold(e.Spacecraft, name) {
				fmt.Fprintf(w, "  %s %s\n", formatEventType(e.Type), relativeTime(e.Timestamp))
				count++
			}
		}
		if count == 0 {
			fmt.Fprintln(w, "  (none)")
		}
	}
}

// DiffResult holds changes between two snapshots.
type DiffResult struct {
	NewLinks   []Link
	LostLinks  []Link
	Handoffs   []Handoff
	RateChange []RateChange
}

// Handoff represents a station change.
type Handoff struct {
	Spacecraft string
	From       string
	To         string
}

// RateChange represents a significant data rate change.
type RateChange struct {
	Spacecraft string
	OldRate    float64
	NewRate    float64
}

// ComputeDiff compares two snapshots and returns changes.
func ComputeDiff(prev, curr *DSNData) DiffResult {
	var result DiffResult
	if prev == nil || curr == nil {
		if curr != nil {
			result.NewLinks = curr.Links
		}
		return result
	}

	// Build maps
	prevByName := make(map[string]Link)
	for _, l := range prev.Links {
		prevByName[l.Spacecraft] = l
	}
	currByName := make(map[string]Link)
	for _, l := range curr.Links {
		currByName[l.Spacecraft] = l
	}

	// Find new and changed
	for name, curr := range currByName {
		prev, existed := prevByName[name]
		if !existed {
			result.NewLinks = append(result.NewLinks, curr)
		} else {
			// Check for handoff
			if prev.StationID != curr.StationID {
				result.Handoffs = append(result.Handoffs, Handoff{
					Spacecraft: name,
					From:       prev.StationID,
					To:         curr.StationID,
				})
			}
			// Check for significant rate change (>50%)
			if prev.DataRate > 0 && curr.DataRate > 0 {
				ratio := curr.DataRate / prev.DataRate
				if ratio > 1.5 || ratio < 0.67 {
					result.RateChange = append(result.RateChange, RateChange{
						Spacecraft: name,
						OldRate:    prev.DataRate,
						NewRate:    curr.DataRate,
					})
				}
			}
		}
	}

	// Find lost
	for name, prev := range prevByName {
		if _, exists := currByName[name]; !exists {
			result.LostLinks = append(result.LostLinks, prev)
		}
	}

	return result
}

// WriteDiff prints diff results.
func WriteDiff(w io.Writer, diff DiffResult, timestamp time.Time) {
	if len(diff.NewLinks) == 0 && len(diff.LostLinks) == 0 &&
		len(diff.Handoffs) == 0 && len(diff.RateChange) == 0 {
		fmt.Fprintf(w, "[%s] No changes\n", timestamp.Format("15:04:05"))
		return
	}

	fmt.Fprintf(w, "[%s] Changes:\n", timestamp.Format("15:04:05"))

	for _, l := range diff.NewLinks {
		fmt.Fprintf(w, "  + NEW: %s on %s (%s)\n", l.Spacecraft, l.AntennaID, FormatDataRate(l.DataRate))
	}
	for _, l := range diff.LostLinks {
		fmt.Fprintf(w, "  - LOST: %s (was on %s)\n", l.Spacecraft, l.AntennaID)
	}
	for _, h := range diff.Handoffs {
		fmt.Fprintf(w, "  → HANDOFF: %s %s→%s\n", h.Spacecraft, h.From, h.To)
	}
	for _, r := range diff.RateChange {
		fmt.Fprintf(w, "  Δ RATE: %s %s→%s\n", r.Spacecraft, FormatDataRate(r.OldRate), FormatDataRate(r.NewRate))
	}
}

// HasChanges returns true if diff contains any changes.
func (d DiffResult) HasChanges() bool {
	return len(d.NewLinks) > 0 || len(d.LostLinks) > 0 ||
		len(d.Handoffs) > 0 || len(d.RateChange) > 0
}

// WriteEvents prints formatted event log.
func WriteEvents(w io.Writer, events []Event, limit int) {
	if len(events) == 0 {
		fmt.Fprintln(w, "No events")
		return
	}

	fmt.Fprintln(w, "Event Log:")
	count := 0
	for i := len(events) - 1; i >= 0 && count < limit; i-- {
		e := events[i]
		fmt.Fprintf(w, "  %s %-10s %-14s %s\n",
			formatEventType(e.Type),
			relativeTime(e.Timestamp),
			truncateStr(e.Spacecraft, 14),
			formatEventDetail(e),
		)
		count++
	}
}

func formatEventType(t EventType) string {
	switch t {
	case EventNewLink:
		return "●NEW "
	case EventHandoff:
		return "→HAND"
	case EventLinkLost:
		return "○LOST"
	case EventLinkResumed:
		return "◐RESU"
	default:
		return "?    "
	}
}

func formatEventDetail(e Event) string {
	switch e.Type {
	case EventNewLink:
		return fmt.Sprintf("on %s", e.NewStation)
	case EventHandoff:
		return fmt.Sprintf("%s→%s", e.OldStation, e.NewStation)
	case EventLinkLost:
		return fmt.Sprintf("was %s", e.OldStation)
	case EventLinkResumed:
		return fmt.Sprintf("on %s", e.NewStation)
	default:
		return ""
	}
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

// Event types (mirrored from state package to avoid import cycle)
type EventType string

const (
	EventNewLink     EventType = "NEW_LINK"
	EventHandoff     EventType = "HANDOFF"
	EventLinkLost    EventType = "LINK_LOST"
	EventLinkResumed EventType = "LINK_RESUMED"
)

// Event represents a state change event.
type Event struct {
	Type       EventType
	Timestamp  time.Time
	Spacecraft string
	OldStation string
	NewStation string
	AntennaID  string
	Complex    string
}
