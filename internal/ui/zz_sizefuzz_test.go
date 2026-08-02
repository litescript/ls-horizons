package ui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/state"
)

// fuzzData builds a realistic multi-complex DSN snapshot.
func fuzzData() *dsn.DSNData {
	now := time.Now()
	d := &dsn.DSNData{Timestamp: now}

	for _, c := range []dsn.Complex{dsn.ComplexGoldstone, dsn.ComplexCanberra, dsn.ComplexMadrid} {
		st := dsn.Station{
			Complex:      c,
			Name:         string(c),
			FriendlyName: dsn.KnownComplexes[c].Name,
			TimeUTC:      now,
		}
		for i := 0; i < 4; i++ {
			st.Antennas = append(st.Antennas, dsn.Antenna{
				ID:        fmt.Sprintf("DSS%d%d", len(string(c)), i),
				Azimuth:   float64(i) * 73.0,
				Elevation: float64(10 + i*17),
				Activity:  "Spacecraft Telemetry, Tracking, and Command",
			})
		}
		d.Stations = append(d.Stations, st)
	}

	names := []string{"Voyager 1", "MRO", "ACE", "SOHO", "Parker Solar Probe", "JWST"}
	for i, n := range names {
		c := []dsn.Complex{dsn.ComplexGoldstone, dsn.ComplexCanberra, dsn.ComplexMadrid}[i%3]
		d.Links = append(d.Links, dsn.Link{
			StationID:    string(c),
			AntennaID:    fmt.Sprintf("DSS%d%d", len(string(c)), i%4),
			Complex:      c,
			SpacecraftID: 100 + i,
			Spacecraft:   n,
			Band:         []string{"X", "S", "Ka"}[i%3],
			DataRate:     float64(i+1) * 1234.5,
			DownRate:     float64(i+1) * 1234.5,
			RTLT:         float64(i) * 3600.0,
			Distance:     float64(i) * 1.5e8,
		})
	}
	return d
}

func fuzzSnapshot() state.Snapshot {
	data := fuzzData()
	snap := state.Snapshot{
		Data:          data,
		LastFetch:     time.Now(),
		NextRefresh:   time.Now().Add(5 * time.Second),
		FetchDuration: 240 * time.Millisecond,
	}
	seen := map[int]bool{}
	for _, l := range data.Links {
		if seen[l.SpacecraftID] {
			continue
		}
		seen[l.SpacecraftID] = true
		snap.Spacecraft = append(snap.Spacecraft, dsn.Spacecraft{
			ID:    l.SpacecraftID,
			Name:  l.Spacecraft,
			Links: []dsn.Link{l},
		})
	}
	return snap
}

// TestViewSizeRobustness drives the root model across a wide range of terminal
// geometries in every view mode and asserts that rendering never panics.
func TestViewSizeRobustness(t *testing.T) {
	widths := []int{0, 1, 2, 5, 10, 20, 40, 60, 80, 120, 200, 400}
	heights := []int{0, 1, 2, 3, 5, 8, 14, 15, 16, 24, 50, 200}

	snap := fuzzSnapshot()

	for _, w := range widths {
		for _, h := range heights {
			for view := 0; view < 4; view++ {
				name := fmt.Sprintf("w%d_h%d_view%d", w, h, view)
				t.Run(name, func(t *testing.T) {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("panic at %dx%d view=%d: %v", w, h, view, r)
						}
					}()

					m := New(state.NewManager(state.DefaultConfig()), nil)
					tm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
					m = tm.(Model)
					tm, _ = m.Update(DataUpdateMsg{Snapshot: snap})
					m = tm.(Model)
					m.viewMode = ViewMode(view)
					_ = m.View()
				})
			}
		}
	}
}

// TestEmptyDataRobustness renders every view with nil/empty data.
func TestEmptyDataRobustness(t *testing.T) {
	cases := map[string]state.Snapshot{
		"zero":      {},
		"nil-data":  {Data: nil, LastFetch: time.Now()},
		"no-links":  {Data: &dsn.DSNData{Timestamp: time.Now()}, LastFetch: time.Now()},
		"err-state": {Data: nil, LastError: fmt.Errorf("synthetic fetch failure")},
	}

	for name, snap := range cases {
		for view := 0; view < 4; view++ {
			t.Run(fmt.Sprintf("%s_view%d", name, view), func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic %s view=%d: %v", name, view, r)
					}
				}()
				m := New(state.NewManager(state.DefaultConfig()), nil)
				tm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
				m = tm.(Model)
				tm, _ = m.Update(DataUpdateMsg{Snapshot: snap})
				m = tm.(Model)
				m.viewMode = ViewMode(view)
				_ = m.View()
			})
		}
	}
}
