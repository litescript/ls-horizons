package state

import (
	"sync"
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/dsn"
)

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.RefreshInterval() != cfg.RefreshInterval {
		t.Errorf("RefreshInterval = %v, want %v", m.RefreshInterval(), cfg.RefreshInterval)
	}

	if m.HasData() {
		t.Error("HasData should be false initially")
	}
}

func TestManager_Update(t *testing.T) {
	m := NewManager(DefaultConfig())

	data := &dsn.DSNData{
		Timestamp: time.Now(),
		Stations: []dsn.Station{
			{
				Complex: dsn.ComplexGoldstone,
				Antennas: []dsn.Antenna{
					{ID: "DSS-14"},
				},
			},
		},
		Links: []dsn.Link{
			{SpacecraftID: 1, Spacecraft: "Test", RTLT: 100},
		},
	}

	m.Update(data, 100*time.Millisecond, nil)

	if !m.HasData() {
		t.Error("HasData should be true after Update")
	}

	snap := m.Snapshot()

	if snap.Data != data {
		t.Error("Snapshot Data doesn't match")
	}

	if snap.FetchDuration != 100*time.Millisecond {
		t.Errorf("FetchDuration = %v, want 100ms", snap.FetchDuration)
	}

	if snap.LastError != nil {
		t.Errorf("LastError = %v, want nil", snap.LastError)
	}
}

func TestManager_UpdateWithError(t *testing.T) {
	m := NewManager(DefaultConfig())

	testErr := &testError{msg: "fetch failed"}
	m.Update(nil, 50*time.Millisecond, testErr)

	snap := m.Snapshot()

	if snap.Data != nil {
		t.Error("Data should be nil on error")
	}

	if snap.LastError != testErr {
		t.Errorf("LastError = %v, want %v", snap.LastError, testErr)
	}
}

func TestManager_HistoryBuffer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxHistoryLen = 3
	m := NewManager(cfg)

	// Add 5 updates
	for i := 0; i < 5; i++ {
		data := &dsn.DSNData{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		}
		m.Update(data, 0, nil)
	}

	// History should only have last 3 entries
	m.mu.RLock()
	histLen := len(m.history)
	m.mu.RUnlock()

	if histLen != 3 {
		t.Errorf("history length = %d, want 3", histLen)
	}
}

func TestManager_SpacecraftHistory(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxSpacecraftHist = 5
	m := NewManager(cfg)

	// Add updates with incrementing RTLT
	for i := 0; i < 10; i++ {
		data := &dsn.DSNData{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Links: []dsn.Link{
				{SpacecraftID: 42, Spacecraft: "TestCraft", RTLT: float64(100 + i), DataRate: 1000},
			},
		}
		m.Update(data, 0, nil)
	}

	hist := m.GetSpacecraftHistory(42)
	if hist == nil {
		t.Fatal("GetSpacecraftHistory returned nil")
	}

	if hist.SpacecraftID != 42 {
		t.Errorf("SpacecraftID = %d, want 42", hist.SpacecraftID)
	}

	// Should only have last 5 entries
	if len(hist.RTLTHistory) != 5 {
		t.Errorf("RTLTHistory length = %d, want 5", len(hist.RTLTHistory))
	}

	// First entry should be RTLT 105 (10 updates - 5 max = start at index 5)
	if hist.RTLTHistory[0].Value != 105 {
		t.Errorf("First RTLT = %v, want 105", hist.RTLTHistory[0].Value)
	}
}

func TestManager_EstimateVelocity(t *testing.T) {
	m := NewManager(DefaultConfig())

	// First update - no velocity possible
	data1 := &dsn.DSNData{
		Timestamp: time.Now(),
		Links: []dsn.Link{
			{SpacecraftID: 1, RTLT: 1000},
		},
	}
	m.Update(data1, 0, nil)

	v1 := m.EstimateVelocity(1)
	if v1 != 0 {
		t.Errorf("Velocity with single point = %v, want 0", v1)
	}

	// Second update - RTLT increased (moving away)
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference
	data2 := &dsn.DSNData{
		Timestamp: time.Now(),
		Links: []dsn.Link{
			{SpacecraftID: 1, RTLT: 1010}, // 10 seconds more
		},
	}
	m.Update(data2, 0, nil)

	v2 := m.EstimateVelocity(1)
	if v2 <= 0 {
		t.Errorf("Velocity for receding spacecraft = %v, want positive", v2)
	}
}

func TestManager_ComplexLoads(t *testing.T) {
	m := NewManager(DefaultConfig())

	data := &dsn.DSNData{
		Stations: []dsn.Station{
			{
				Complex: dsn.ComplexGoldstone,
				Antennas: []dsn.Antenna{
					{ID: "DSS-14", Targets: []dsn.Target{{Name: "A"}}},
					{ID: "DSS-24"},
				},
			},
		},
	}
	m.Update(data, 0, nil)

	snap := m.Snapshot()

	load, ok := snap.ComplexLoads[dsn.ComplexGoldstone]
	if !ok {
		t.Fatal("Goldstone load not found")
	}

	if load.TotalAntennas != 2 {
		t.Errorf("TotalAntennas = %d, want 2", load.TotalAntennas)
	}

	if load.ActiveLinks != 1 {
		t.Errorf("ActiveLinks = %d, want 1", load.ActiveLinks)
	}
}

func TestManager_Snapshot_IsCopy(t *testing.T) {
	m := NewManager(DefaultConfig())

	data := &dsn.DSNData{
		Links: []dsn.Link{
			{SpacecraftID: 1, Spacecraft: "Test"},
		},
	}
	m.Update(data, 0, nil)

	snap := m.Snapshot()

	// Modify the snapshot's map
	snap.ComplexLoads[dsn.ComplexGoldstone] = dsn.ComplexLoad{ActiveLinks: 999}

	// Get another snapshot
	snap2 := m.Snapshot()

	// Original state should be unchanged
	if snap2.ComplexLoads[dsn.ComplexGoldstone].ActiveLinks == 999 {
		t.Error("Snapshot modification affected manager state")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager(DefaultConfig())

	var wg sync.WaitGroup
	iterations := 100

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			data := &dsn.DSNData{
				Timestamp: time.Now(),
				Links: []dsn.Link{
					{SpacecraftID: i, RTLT: float64(i)},
				},
			}
			m.Update(data, time.Duration(i)*time.Millisecond, nil)
		}
	}()

	// Reader goroutines
	for r := 0; r < 5; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = m.Snapshot()
				_ = m.HasData()
				_ = m.RefreshInterval()
				_ = m.GetSpacecraftHistory(i)
				_ = m.EstimateVelocity(i)
			}
		}()
	}

	wg.Wait()
}

func TestManager_SetRefreshInterval(t *testing.T) {
	m := NewManager(DefaultConfig())

	newInterval := 30 * time.Second
	m.SetRefreshInterval(newInterval)

	if m.RefreshInterval() != newInterval {
		t.Errorf("RefreshInterval = %v, want %v", m.RefreshInterval(), newInterval)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestManager_EventDetection_NewLink(t *testing.T) {
	m := NewManager(DefaultConfig())

	// First update with a spacecraft
	data1 := &dsn.DSNData{
		Timestamp: time.Now(),
		Links: []dsn.Link{
			{SpacecraftID: 1, Spacecraft: "Voyager", StationID: "gdscc", AntennaID: "DSS-14", Complex: dsn.ComplexGoldstone},
		},
	}
	m.Update(data1, 0, nil)

	events := m.RecentEvents(10)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventNewLink {
		t.Errorf("event type = %q, want NEW_LINK", events[0].Type)
	}
	if events[0].Spacecraft != "Voyager" {
		t.Errorf("spacecraft = %q, want Voyager", events[0].Spacecraft)
	}
	if events[0].NewStation != "gdscc" {
		t.Errorf("new station = %q, want gdscc", events[0].NewStation)
	}
}

func TestManager_EventDetection_Handoff(t *testing.T) {
	m := NewManager(DefaultConfig())

	// Initial link at Goldstone
	data1 := &dsn.DSNData{
		Timestamp: time.Now(),
		Links: []dsn.Link{
			{SpacecraftID: 1, Spacecraft: "Mars Orbiter", StationID: "gdscc", AntennaID: "DSS-14", Complex: dsn.ComplexGoldstone},
		},
	}
	m.Update(data1, 0, nil)

	// Handoff to Canberra
	data2 := &dsn.DSNData{
		Timestamp: time.Now().Add(time.Minute),
		Links: []dsn.Link{
			{SpacecraftID: 1, Spacecraft: "Mars Orbiter", StationID: "cdscc", AntennaID: "DSS-34", Complex: dsn.ComplexCanberra},
		},
	}
	m.Update(data2, 0, nil)

	events := m.RecentEvents(10)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Find the handoff event
	var handoff *Event
	for i := range events {
		if events[i].Type == EventHandoff {
			handoff = &events[i]
			break
		}
	}

	if handoff == nil {
		t.Fatal("no HANDOFF event found")
	}
	if handoff.Spacecraft != "Mars Orbiter" {
		t.Errorf("spacecraft = %q, want Mars Orbiter", handoff.Spacecraft)
	}
	if handoff.OldStation != "gdscc" {
		t.Errorf("old station = %q, want gdscc", handoff.OldStation)
	}
	if handoff.NewStation != "cdscc" {
		t.Errorf("new station = %q, want cdscc", handoff.NewStation)
	}
}

func TestManager_EventDetection_LinkLost(t *testing.T) {
	m := NewManager(DefaultConfig())

	// Initial link
	data1 := &dsn.DSNData{
		Timestamp: time.Now(),
		Links: []dsn.Link{
			{SpacecraftID: 1, Spacecraft: "TestCraft", StationID: "mdscc", Complex: dsn.ComplexMadrid},
		},
	}
	m.Update(data1, 0, nil)

	// Link lost (no links)
	data2 := &dsn.DSNData{
		Timestamp: time.Now().Add(time.Minute),
		Links:     []dsn.Link{},
	}
	m.Update(data2, 0, nil)

	events := m.RecentEvents(10)

	var linkLost *Event
	for i := range events {
		if events[i].Type == EventLinkLost {
			linkLost = &events[i]
			break
		}
	}

	if linkLost == nil {
		t.Fatal("no LINK_LOST event found")
	}
	if linkLost.Spacecraft != "TestCraft" {
		t.Errorf("spacecraft = %q, want TestCraft", linkLost.Spacecraft)
	}
	if linkLost.OldStation != "mdscc" {
		t.Errorf("old station = %q, want mdscc", linkLost.OldStation)
	}
}

func TestManager_EventRingBuffer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxEvents = 5
	m := NewManager(cfg)

	// Generate more events than buffer size
	// Each update removes previous spacecraft (LINK_LOST) and adds new one (NEW_LINK)
	for i := 0; i < 10; i++ {
		data := &dsn.DSNData{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Links: []dsn.Link{
				{SpacecraftID: i, Spacecraft: "SC" + string(rune('A'+i)), StationID: "gdscc", Complex: dsn.ComplexGoldstone},
			},
		}
		m.Update(data, 0, nil)
	}

	events := m.RecentEvents(100)
	if len(events) != 5 {
		t.Errorf("events count = %d, want 5 (max)", len(events))
	}

	// Should have recent events (could be NEW_LINK or LINK_LOST)
	// Verify ring buffer doesn't exceed max
	if len(events) > cfg.MaxEvents {
		t.Errorf("events exceeded max: got %d, max %d", len(events), cfg.MaxEvents)
	}

	// Verify events are ordered chronologically
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Errorf("events not in chronological order at index %d", i)
		}
	}
}

func TestManager_Snapshot_IncludesEvents(t *testing.T) {
	m := NewManager(DefaultConfig())

	data := &dsn.DSNData{
		Timestamp: time.Now(),
		Links: []dsn.Link{
			{SpacecraftID: 1, Spacecraft: "Test", StationID: "gdscc", Complex: dsn.ComplexGoldstone},
		},
	}
	m.Update(data, 0, nil)

	snap := m.Snapshot()
	if len(snap.Events) == 0 {
		t.Error("Snapshot should include events")
	}
	if snap.Events[0].Type != EventNewLink {
		t.Errorf("event type = %q, want NEW_LINK", snap.Events[0].Type)
	}
}
