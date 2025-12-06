// Package state provides thread-safe state management for the application.
package state

import (
	"sync"
	"time"

	"github.com/litescript/ls-horizons/internal/dsn"
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

// CachedPassPlan stores a pass plan with metadata.
type CachedPassPlan struct {
	Plan      *dsn.PassPlan
	UpdatedAt time.Time
	Error     error
	Loading   bool // True if currently being fetched
}

// CachedElevationTrace stores an elevation trace with metadata.
type CachedElevationTrace struct {
	Trace     *dsn.ElevationTrace
	UpdatedAt time.Time
	Error     error
	Loading   bool        // True if currently being fetched
	Complex   dsn.Complex // Which complex this trace was computed for
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
	nextRefresh   time.Time
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

	// Pass planning state
	focusedSpacecraftID int // Currently focused spacecraft for pass planning

	// Pass plan cache - stores plans for ALL spacecraft, not just focused
	passPlanCache map[int]*CachedPassPlan

	// Elevation trace cache - stores traces for ALL spacecraft
	elevTraceCache map[int]*CachedElevationTrace

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
		passPlanCache:     make(map[int]*CachedPassPlan),
		elevTraceCache:    make(map[int]*CachedElevationTrace),
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
	NextRefresh   time.Time // When the next fetch is scheduled
	LastError     error
	FetchDuration time.Duration
	ComplexLoads  map[dsn.Complex]dsn.ComplexLoad
	Spacecraft    []dsn.Spacecraft
	SkyObjects    []dsn.SkyObject
	Events        []Event

	// Pass planning state for focused spacecraft
	PassPlan            *dsn.PassPlan
	PassPlanUpdatedAt   time.Time
	PassPlanError       error
	PassPlanLoading     bool
	FocusedSpacecraftID int

	// Elevation trace state for focused spacecraft
	ElevationTrace          *dsn.ElevationTrace
	ElevationTraceUpdatedAt time.Time
	ElevationTraceError     error
	ElevationTraceLoading   bool
	ElevationTraceComplex   dsn.Complex
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

	// Get pass plan for focused spacecraft from cache
	var passPlan *dsn.PassPlan
	var passPlanUpdatedAt time.Time
	var passPlanError error
	var passPlanLoading bool
	if cached, ok := m.passPlanCache[m.focusedSpacecraftID]; ok {
		passPlan = cached.Plan
		passPlanUpdatedAt = cached.UpdatedAt
		passPlanError = cached.Error
		passPlanLoading = cached.Loading
	}

	// Get elevation trace for focused spacecraft from cache
	var elevTrace *dsn.ElevationTrace
	var elevTraceUpdatedAt time.Time
	var elevTraceError error
	var elevTraceLoading bool
	var elevTraceComplex dsn.Complex
	if cached, ok := m.elevTraceCache[m.focusedSpacecraftID]; ok {
		elevTrace = cached.Trace
		elevTraceUpdatedAt = cached.UpdatedAt
		elevTraceError = cached.Error
		elevTraceLoading = cached.Loading
		elevTraceComplex = cached.Complex
	}

	return Snapshot{
		Data:                    m.current,
		LastFetch:               m.lastFetch,
		NextRefresh:             m.nextRefresh,
		LastError:               m.lastError,
		FetchDuration:           m.fetchDuration,
		ComplexLoads:            loads,
		Spacecraft:              sc,
		SkyObjects:              skyObjs,
		Events:                  events,
		PassPlan:                passPlan,
		PassPlanUpdatedAt:       passPlanUpdatedAt,
		PassPlanError:           passPlanError,
		PassPlanLoading:         passPlanLoading,
		FocusedSpacecraftID:     m.focusedSpacecraftID,
		ElevationTrace:          elevTrace,
		ElevationTraceUpdatedAt: elevTraceUpdatedAt,
		ElevationTraceError:     elevTraceError,
		ElevationTraceLoading:   elevTraceLoading,
		ElevationTraceComplex:   elevTraceComplex,
	}
}

// SetNextRefresh updates the next scheduled refresh time.
func (m *Manager) SetNextRefresh(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextRefresh = t
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

// PassPlanTTL is how long a computed pass plan remains valid.
const PassPlanTTL = 5 * time.Minute

// ElevationTraceTTL is how long a computed elevation trace remains valid.
const ElevationTraceTTL = 2 * time.Minute

// SetFocusedSpacecraft updates the focused spacecraft for pass planning.
func (m *Manager) SetFocusedSpacecraft(spacecraftID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.focusedSpacecraftID = spacecraftID
}

// GetFocusedSpacecraftID returns the currently focused spacecraft ID.
func (m *Manager) GetFocusedSpacecraftID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.focusedSpacecraftID
}

// SetPassPlanLoading marks a spacecraft's pass plan as loading.
func (m *Manager) SetPassPlanLoading(spacecraftID int, loading bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cached, ok := m.passPlanCache[spacecraftID]
	if !ok {
		cached = &CachedPassPlan{}
		m.passPlanCache[spacecraftID] = cached
	}
	cached.Loading = loading
}

// UpdatePassPlan sets the cached pass plan for a spacecraft.
func (m *Manager) UpdatePassPlan(spacecraftID int, plan *dsn.PassPlan, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.passPlanCache[spacecraftID] = &CachedPassPlan{
		Plan:      plan,
		UpdatedAt: time.Now(),
		Error:     err,
		Loading:   false,
	}
}

// GetCachedPassPlan returns the cached pass plan for a spacecraft.
func (m *Manager) GetCachedPassPlan(spacecraftID int) *CachedPassPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cached, ok := m.passPlanCache[spacecraftID]; ok {
		// Return a copy
		return &CachedPassPlan{
			Plan:      cached.Plan,
			UpdatedAt: cached.UpdatedAt,
			Error:     cached.Error,
			Loading:   cached.Loading,
		}
	}
	return nil
}

// NeedsPassPlanRefresh returns true if a spacecraft's pass plan should be recomputed.
func (m *Manager) NeedsPassPlanRefresh(spacecraftID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if spacecraftID == 0 {
		return false
	}

	cached, ok := m.passPlanCache[spacecraftID]
	if !ok {
		return true // No cache entry
	}

	if cached.Loading {
		return false // Already loading
	}

	if cached.Plan == nil && cached.Error == nil {
		return true // No plan yet
	}

	if time.Since(cached.UpdatedAt) > PassPlanTTL {
		return true // TTL expired
	}

	return false
}

// GetAllSpacecraftIDs returns IDs of all known spacecraft.
func (m *Manager) GetAllSpacecraftIDs() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]int, 0, len(m.spacecraft))
	for _, sc := range m.spacecraft {
		ids = append(ids, sc.ID)
	}
	return ids
}

// SetElevationTraceLoading marks a spacecraft's elevation trace as loading.
func (m *Manager) SetElevationTraceLoading(spacecraftID int, loading bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cached, ok := m.elevTraceCache[spacecraftID]
	if !ok {
		cached = &CachedElevationTrace{}
		m.elevTraceCache[spacecraftID] = cached
	}
	cached.Loading = loading
}

// UpdateElevationTrace sets the cached elevation trace for a spacecraft.
func (m *Manager) UpdateElevationTrace(spacecraftID int, trace *dsn.ElevationTrace, complex dsn.Complex, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.elevTraceCache[spacecraftID] = &CachedElevationTrace{
		Trace:     trace,
		UpdatedAt: time.Now(),
		Error:     err,
		Loading:   false,
		Complex:   complex,
	}
}

// GetCachedElevationTrace returns the cached elevation trace for a spacecraft.
func (m *Manager) GetCachedElevationTrace(spacecraftID int) *CachedElevationTrace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cached, ok := m.elevTraceCache[spacecraftID]; ok {
		// Return a copy
		return &CachedElevationTrace{
			Trace:     cached.Trace,
			UpdatedAt: cached.UpdatedAt,
			Error:     cached.Error,
			Loading:   cached.Loading,
			Complex:   cached.Complex,
		}
	}
	return nil
}

// NeedsElevationTraceRefresh returns true if a spacecraft's elevation trace should be recomputed.
// It also checks if the target complex has changed.
func (m *Manager) NeedsElevationTraceRefresh(spacecraftID int, targetComplex dsn.Complex) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if spacecraftID == 0 {
		return false
	}

	cached, ok := m.elevTraceCache[spacecraftID]
	if !ok {
		return true // No cache entry
	}

	if cached.Loading {
		return false // Already loading
	}

	if cached.Trace == nil && cached.Error == nil {
		return true // No trace yet
	}

	// If complex changed, we need to recompute
	if targetComplex != "" && cached.Complex != targetComplex {
		return true
	}

	if time.Since(cached.UpdatedAt) > ElevationTraceTTL {
		return true // TTL expired
	}

	return false
}
