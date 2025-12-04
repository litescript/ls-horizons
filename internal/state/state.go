// Package state provides thread-safe state management for the application.
package state

import (
	"sync"
	"time"

	"github.com/peter/ls-horizons/internal/dsn"
)

// EventType represents the type of state change event.
type EventType string

const (
	EventNewLink     EventType = "NEW_LINK"
	EventHandoff     EventType = "HANDOFF"
	EventLinkLost    EventType = "LINK_LOST"
	EventLinkResumed EventType = "LINK_RESUMED"
)

// Event represents a state change in the DSN network.
type Event struct {
	Type       EventType `json:"type"`
	Timestamp  time.Time `json:"timestamp"`
	Spacecraft string    `json:"spacecraft"`
	OldStation string    `json:"old_station,omitempty"`
	NewStation string    `json:"new_station,omitempty"`
	AntennaID  string    `json:"antenna_id,omitempty"`
	Complex    string    `json:"complex,omitempty"`
}

// HistoryEntry represents a single point in the history buffer.
type HistoryEntry struct {
	Timestamp time.Time
	Data      *dsn.DSNData
}

// SpacecraftHistory tracks historical metrics for a spacecraft.
type SpacecraftHistory struct {
	SpacecraftID   int
	SpacecraftName string
	RTLTHistory    []TimeSeries
	RateHistory    []TimeSeries
}

// TimeSeries is a single data point with timestamp.
type TimeSeries struct {
	Timestamp time.Time
	Value     float64
}

// linkKey uniquely identifies a spacecraft link.
type linkKey struct {
	spacecraft string
	stationID  string
}

// Manager handles all shared application state with thread-safe access.
type Manager struct {
	mu sync.RWMutex

	// Current state
	current       *dsn.DSNData
	lastFetch     time.Time
	lastError     error
	fetchDuration time.Duration

	// Previous links for event detection
	prevLinks map[linkKey]dsn.Link

	// History buffers
	history           []HistoryEntry
	maxHistoryLen     int
	spacecraftHistory map[int]*SpacecraftHistory
	maxSpacecraftHist int

	// Event log (ring buffer)
	events       []Event
	maxEvents    int
	eventWriteAt int

	// Derived/cached data
	complexLoads map[dsn.Complex]dsn.ComplexLoad
	spacecraft   []dsn.Spacecraft

	// Configuration
	refreshInterval time.Duration
}

// Config holds configuration for the state manager.
type Config struct {
	MaxHistoryLen     int
	MaxSpacecraftHist int
	MaxEvents         int
	RefreshInterval   time.Duration
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxHistoryLen:     60,  // Keep ~1 hour at 1 fetch/min
		MaxSpacecraftHist: 120, // 2 hours of per-spacecraft data
		MaxEvents:         50,  // Last 50 events
		RefreshInterval:   5 * time.Second,
	}
}

// NewManager creates a new state manager.
func NewManager(cfg Config) *Manager {
	maxEvents := cfg.MaxEvents
	if maxEvents <= 0 {
		maxEvents = 50
	}
	return &Manager{
		maxHistoryLen:     cfg.MaxHistoryLen,
		maxSpacecraftHist: cfg.MaxSpacecraftHist,
		maxEvents:         maxEvents,
		events:            make([]Event, 0, maxEvents),
		refreshInterval:   cfg.RefreshInterval,
		spacecraftHistory: make(map[int]*SpacecraftHistory),
		complexLoads:      make(map[dsn.Complex]dsn.ComplexLoad),
		prevLinks:         make(map[linkKey]dsn.Link),
	}
}

// Update atomically updates the state with new DSN data.
func (m *Manager) Update(data *dsn.DSNData, fetchDuration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastFetch = time.Now()
	m.lastError = err
	m.fetchDuration = fetchDuration

	if data == nil {
		return
	}

	// Detect events before updating current state
	m.detectEvents(data)

	m.current = data

	// Add to history
	entry := HistoryEntry{
		Timestamp: data.Timestamp,
		Data:      data,
	}
	m.history = append(m.history, entry)
	if len(m.history) > m.maxHistoryLen {
		m.history = m.history[1:]
	}

	// Update derived data
	m.complexLoads = dsn.ComplexUtilization(data)
	m.spacecraft = dsn.AggregateSpacecraft(data)

	// Update per-spacecraft history
	m.updateSpacecraftHistory(data)

	// Update prevLinks for next comparison
	m.prevLinks = make(map[linkKey]dsn.Link)
	for _, link := range data.Links {
		key := linkKey{spacecraft: link.Spacecraft, stationID: link.StationID}
		m.prevLinks[key] = link
	}
}

// detectEvents compares new data with previous state and generates events.
func (m *Manager) detectEvents(newData *dsn.DSNData) {
	now := time.Now()

	// Build current links map
	newLinks := make(map[linkKey]dsn.Link)
	newBySpacecraft := make(map[string]dsn.Link)
	for _, link := range newData.Links {
		key := linkKey{spacecraft: link.Spacecraft, stationID: link.StationID}
		newLinks[key] = link
		newBySpacecraft[link.Spacecraft] = link
	}

	// Build previous by spacecraft
	prevBySpacecraft := make(map[string]dsn.Link)
	for key, link := range m.prevLinks {
		prevBySpacecraft[key.spacecraft] = link
	}

	// Check for new links and handoffs
	for sc, newLink := range newBySpacecraft {
		prevLink, wasPrev := prevBySpacecraft[sc]

		if !wasPrev {
			// NEW_LINK: spacecraft wasn't tracked before
			m.addEvent(Event{
				Type:       EventNewLink,
				Timestamp:  now,
				Spacecraft: sc,
				NewStation: newLink.StationID,
				AntennaID:  newLink.AntennaID,
				Complex:    string(newLink.Complex),
			})
		} else if prevLink.StationID != newLink.StationID {
			// HANDOFF: station changed
			m.addEvent(Event{
				Type:       EventHandoff,
				Timestamp:  now,
				Spacecraft: sc,
				OldStation: prevLink.StationID,
				NewStation: newLink.StationID,
				AntennaID:  newLink.AntennaID,
				Complex:    string(newLink.Complex),
			})
		}
	}

	// Check for lost links
	for sc, prevLink := range prevBySpacecraft {
		if _, exists := newBySpacecraft[sc]; !exists {
			m.addEvent(Event{
				Type:       EventLinkLost,
				Timestamp:  now,
				Spacecraft: sc,
				OldStation: prevLink.StationID,
				Complex:    string(prevLink.Complex),
			})
		}
	}
}

// addEvent adds an event to the ring buffer.
func (m *Manager) addEvent(e Event) {
	if len(m.events) < m.maxEvents {
		m.events = append(m.events, e)
	} else {
		m.events[m.eventWriteAt] = e
		m.eventWriteAt = (m.eventWriteAt + 1) % m.maxEvents
	}
}

func (m *Manager) updateSpacecraftHistory(data *dsn.DSNData) {
	for _, link := range data.Links {
		hist, ok := m.spacecraftHistory[link.SpacecraftID]
		if !ok {
			hist = &SpacecraftHistory{
				SpacecraftID:   link.SpacecraftID,
				SpacecraftName: link.Spacecraft,
				RTLTHistory:    make([]TimeSeries, 0, m.maxSpacecraftHist),
				RateHistory:    make([]TimeSeries, 0, m.maxSpacecraftHist),
			}
			m.spacecraftHistory[link.SpacecraftID] = hist
		}

		ts := data.Timestamp

		// Add RTLT data point
		if link.RTLT > 0 {
			hist.RTLTHistory = append(hist.RTLTHistory, TimeSeries{Timestamp: ts, Value: link.RTLT})
			if len(hist.RTLTHistory) > m.maxSpacecraftHist {
				hist.RTLTHistory = hist.RTLTHistory[1:]
			}
		}

		// Add data rate point
		if link.DataRate > 0 {
			hist.RateHistory = append(hist.RateHistory, TimeSeries{Timestamp: ts, Value: link.DataRate})
			if len(hist.RateHistory) > m.maxSpacecraftHist {
				hist.RateHistory = hist.RateHistory[1:]
			}
		}
	}
}

// Snapshot represents an immutable snapshot of current state.
type Snapshot struct {
	Data          *dsn.DSNData
	LastFetch     time.Time
	LastError     error
	FetchDuration time.Duration
	ComplexLoads  map[dsn.Complex]dsn.ComplexLoad
	Spacecraft    []dsn.Spacecraft
	SkyObjects    []dsn.SkyObject
	Events        []Event
}

// Snapshot returns a consistent snapshot of current state.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copy complex loads map
	loads := make(map[dsn.Complex]dsn.ComplexLoad, len(m.complexLoads))
	for k, v := range m.complexLoads {
		loads[k] = v
	}

	// Copy spacecraft slice
	sc := make([]dsn.Spacecraft, len(m.spacecraft))
	copy(sc, m.spacecraft)

	// Get sky objects
	var skyObjs []dsn.SkyObject
	if m.current != nil {
		skyObjs = m.current.SkyObjects()
	}

	// Copy events in chronological order
	events := m.getEventsOrdered()

	return Snapshot{
		Data:          m.current,
		LastFetch:     m.lastFetch,
		LastError:     m.lastError,
		FetchDuration: m.fetchDuration,
		ComplexLoads:  loads,
		Spacecraft:    sc,
		SkyObjects:    skyObjs,
		Events:        events,
	}
}

// getEventsOrdered returns events in chronological order.
func (m *Manager) getEventsOrdered() []Event {
	if len(m.events) == 0 {
		return nil
	}

	// If buffer isn't full yet, just copy
	if len(m.events) < m.maxEvents {
		result := make([]Event, len(m.events))
		copy(result, m.events)
		return result
	}

	// Ring buffer is full, reorder from oldest to newest
	result := make([]Event, m.maxEvents)
	for i := 0; i < m.maxEvents; i++ {
		idx := (m.eventWriteAt + i) % m.maxEvents
		result[i] = m.events[idx]
	}
	return result
}

// RecentEvents returns the last n events.
func (m *Manager) RecentEvents(n int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := m.getEventsOrdered()
	if len(all) <= n {
		return all
	}
	return all[len(all)-n:]
}

// GetSpacecraftHistory returns history for a specific spacecraft.
func (m *Manager) GetSpacecraftHistory(spacecraftID int) *SpacecraftHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hist, ok := m.spacecraftHistory[spacecraftID]
	if !ok {
		return nil
	}

	// Return a copy
	copyHist := &SpacecraftHistory{
		SpacecraftID:   hist.SpacecraftID,
		SpacecraftName: hist.SpacecraftName,
		RTLTHistory:    make([]TimeSeries, len(hist.RTLTHistory)),
		RateHistory:    make([]TimeSeries, len(hist.RateHistory)),
	}
	copy(copyHist.RTLTHistory, hist.RTLTHistory)
	copy(copyHist.RateHistory, hist.RateHistory)

	return copyHist
}

// EstimateVelocity calculates velocity estimate for a spacecraft from history.
func (m *Manager) EstimateVelocity(spacecraftID int) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hist, ok := m.spacecraftHistory[spacecraftID]
	if !ok || len(hist.RTLTHistory) < 2 {
		return 0
	}

	// Use last two points for velocity estimate
	n := len(hist.RTLTHistory)
	p1 := hist.RTLTHistory[n-2]
	p2 := hist.RTLTHistory[n-1]

	deltaTime := p2.Timestamp.Sub(p1.Timestamp).Seconds()
	if deltaTime <= 0 {
		return 0
	}

	return dsn.VelocityFromRTLTDelta(p1.Value, p2.Value, deltaTime)
}

// RefreshInterval returns the configured refresh interval.
func (m *Manager) RefreshInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.refreshInterval
}

// SetRefreshInterval updates the refresh interval.
func (m *Manager) SetRefreshInterval(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshInterval = d
}

// HasData returns true if we have received at least one successful fetch.
func (m *Manager) HasData() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current != nil
}
