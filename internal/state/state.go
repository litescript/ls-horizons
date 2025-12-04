// Package state provides thread-safe state management for the application.
package state

import (
	"sync"
	"time"

	"github.com/peter/ls-horizons/internal/dsn"
)

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

// Manager handles all shared application state with thread-safe access.
type Manager struct {
	mu sync.RWMutex

	// Current state
	current       *dsn.DSNData
	lastFetch     time.Time
	lastError     error
	fetchDuration time.Duration

	// History buffers
	history            []HistoryEntry
	maxHistoryLen      int
	spacecraftHistory  map[int]*SpacecraftHistory
	maxSpacecraftHist  int

	// Derived/cached data
	complexLoads map[dsn.Complex]dsn.ComplexLoad
	spacecraft   []dsn.Spacecraft

	// Configuration
	refreshInterval time.Duration
}

// Config holds configuration for the state manager.
type Config struct {
	MaxHistoryLen       int
	MaxSpacecraftHist   int
	RefreshInterval     time.Duration
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxHistoryLen:     60,  // Keep ~1 hour at 1 fetch/min
		MaxSpacecraftHist: 120, // 2 hours of per-spacecraft data
		RefreshInterval:   5 * time.Second,
	}
}

// NewManager creates a new state manager.
func NewManager(cfg Config) *Manager {
	return &Manager{
		maxHistoryLen:     cfg.MaxHistoryLen,
		maxSpacecraftHist: cfg.MaxSpacecraftHist,
		refreshInterval:   cfg.RefreshInterval,
		spacecraftHistory: make(map[int]*SpacecraftHistory),
		complexLoads:      make(map[dsn.Complex]dsn.ComplexLoad),
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

	return Snapshot{
		Data:          m.current,
		LastFetch:     m.lastFetch,
		LastError:     m.lastError,
		FetchDuration: m.fetchDuration,
		ComplexLoads:  loads,
		Spacecraft:    sc,
		SkyObjects:    skyObjs,
	}
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
