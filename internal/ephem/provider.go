// Package ephem provides ephemeris data for spacecraft positions.
package ephem

import (
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

// TargetID is a NAIF SPICE ID for a spacecraft or body.
type TargetID int

// EphemerisPoint represents a spacecraft position at a specific time.
type EphemerisPoint struct {
	Time  time.Time
	Coord astro.SkyCoord // RA/Dec and optionally Az/El if computed
	Valid bool           // Whether this point has valid data
}

// EphemerisPath represents a trajectory arc over time.
type EphemerisPath struct {
	TargetID TargetID
	Points   []EphemerisPoint
	Start    time.Time
	End      time.Time
}

// Provider defines the interface for ephemeris data sources.
type Provider interface {
	// Name returns the provider name for display/logging.
	Name() string

	// GetPosition returns the current position for a target.
	// Returns an invalid point if the target is unknown or unavailable.
	GetPosition(target TargetID, t time.Time, obs astro.Observer) (EphemerisPoint, error)

	// GetPath returns a trajectory arc for a target over a time range.
	// step is the time between points (e.g., 1 minute).
	GetPath(target TargetID, start, end time.Time, step time.Duration, obs astro.Observer) (EphemerisPath, error)

	// Available returns true if this provider can supply data for the target.
	Available(target TargetID) bool
}

// Mode represents which ephemeris source to use.
type Mode int

const (
	ModeHorizons Mode = iota // Use JPL Horizons (default)
	ModeDSN                  // Use DSN-derived geometry only
	ModeAuto                 // Try Horizons, fall back to DSN
)

// String returns the mode name.
func (m Mode) String() string {
	switch m {
	case ModeHorizons:
		return "horizons"
	case ModeDSN:
		return "dsn"
	case ModeAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// ParseMode parses a mode string.
func ParseMode(s string) Mode {
	switch s {
	case "horizons":
		return ModeHorizons
	case "dsn":
		return ModeDSN
	case "auto":
		return ModeAuto
	default:
		return ModeAuto
	}
}
