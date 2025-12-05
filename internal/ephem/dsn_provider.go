package ephem

import (
	"fmt"
	"sync"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
	"github.com/litescript/ls-horizons/internal/dsn"
)

// DSNProvider uses DSN antenna pointing data for ephemeris.
// This is a fallback provider that derives positions from real-time
// DSN tracking data rather than computed ephemerides.
type DSNProvider struct {
	mu   sync.RWMutex
	data *dsn.DSNData

	// Cache spacecraft views
	views    []dsn.SpacecraftView
	viewsMap map[int]dsn.SpacecraftView // keyed by SpacecraftID
}

// NewDSNProvider creates a new DSN-based ephemeris provider.
func NewDSNProvider() *DSNProvider {
	return &DSNProvider{
		viewsMap: make(map[int]dsn.SpacecraftView),
	}
}

// UpdateData updates the provider with fresh DSN data.
func (p *DSNProvider) UpdateData(data *dsn.DSNData) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data = data
	if data == nil {
		p.views = nil
		p.viewsMap = make(map[int]dsn.SpacecraftView)
		return
	}

	elevMap := dsn.BuildElevationMap(data)
	p.views = dsn.BuildSpacecraftViews(data, elevMap)

	p.viewsMap = make(map[int]dsn.SpacecraftView, len(p.views))
	for _, sv := range p.views {
		p.viewsMap[sv.ID] = sv
	}
}

// Name implements Provider.
func (p *DSNProvider) Name() string {
	return "DSN"
}

// GetPosition implements Provider.
// Returns position derived from DSN antenna pointing.
func (p *DSNProvider) GetPosition(target TargetID, t time.Time, obs astro.Observer) (EphemerisPoint, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Find spacecraft by DSN ID
	scID := p.findSpacecraftID(target)
	if scID == 0 {
		return EphemerisPoint{Valid: false}, fmt.Errorf("target %d not found in DSN data", target)
	}

	sv, ok := p.viewsMap[scID]
	if !ok {
		return EphemerisPoint{Valid: false}, fmt.Errorf("spacecraft %d not currently tracked", scID)
	}

	coord := sv.Coord()

	return EphemerisPoint{
		Time:  t,
		Coord: coord,
		Valid: true,
	}, nil
}

// GetPath implements Provider.
// DSN provider cannot provide trajectory paths - it only has current position.
// Returns a path with a single point (current position).
func (p *DSNProvider) GetPath(target TargetID, start, end time.Time, step time.Duration, obs astro.Observer) (EphemerisPath, error) {
	point, err := p.GetPosition(target, time.Now(), obs)
	if err != nil {
		return EphemerisPath{}, err
	}

	return EphemerisPath{
		TargetID: target,
		Points:   []EphemerisPoint{point},
		Start:    start,
		End:      end,
	}, nil
}

// Available implements Provider.
func (p *DSNProvider) Available(target TargetID) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	scID := p.findSpacecraftID(target)
	if scID == 0 {
		return false
	}
	_, ok := p.viewsMap[scID]
	return ok
}

// findSpacecraftID maps a NAIF target ID to a DSN spacecraft ID.
// This requires the targets mapping.
func (p *DSNProvider) findSpacecraftID(target TargetID) int {
	// Use the global targets registry
	if info, ok := TargetsByNAIF[target]; ok {
		return info.DSNID
	}
	return 0
}

// GetSpacecraftViews returns the current spacecraft views.
func (p *DSNProvider) GetSpacecraftViews() []dsn.SpacecraftView {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.views
}
