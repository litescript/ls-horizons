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
