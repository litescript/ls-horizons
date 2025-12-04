// Package dsn provides types and functions for working with NASA Deep Space Network data.
package dsn

import "time"

// Complex represents a DSN complex (Goldstone, Canberra, Madrid).
type Complex string

const (
	ComplexGoldstone Complex = "gdscc" // Goldstone Deep Space Communications Complex
	ComplexCanberra  Complex = "cdscc" // Canberra Deep Space Communication Complex
	ComplexMadrid    Complex = "mdscc" // Madrid Deep Space Communications Complex
)

// ComplexInfo contains metadata about a DSN complex.
type ComplexInfo struct {
	ID        Complex
	Name      string
	Latitude  float64
	Longitude float64
}

// KnownComplexes maps complex IDs to their full information.
var KnownComplexes = map[Complex]ComplexInfo{
	ComplexGoldstone: {ID: ComplexGoldstone, Name: "Goldstone", Latitude: 35.4267, Longitude: -116.8900},
	ComplexCanberra:  {ID: ComplexCanberra, Name: "Canberra", Latitude: -35.4014, Longitude: 148.9817},
	ComplexMadrid:    {ID: ComplexMadrid, Name: "Madrid", Latitude: 40.4314, Longitude: -4.2481},
}

// Station represents a DSN station within a complex.
type Station struct {
	Complex      Complex
	Name         string
	FriendlyName string
	Antennas     []Antenna
	TimeUTC      time.Time
	TimeZone     int // offset in seconds
}

// Antenna represents an individual DSN antenna.
type Antenna struct {
	ID          string  // e.g., "DSS14" or "DSS-14"
	Name        string
	Diameter    float64 // meters
	Azimuth     float64 // degrees, 0-360
	Elevation   float64 // degrees, 0-90
	WindSpeed   float64 // km/h
	IsMSPA      bool    // Multiple Spacecraft Per Aperture
	IsArray     bool    // Part of antenna array
	IsDDOR      bool    // Delta-Differential One-way Ranging
	Activity    string  // e.g., "Spacecraft Telemetry, Tracking, and Command"
	Created     time.Time
	Updated     time.Time
	Targets     []Target
	DownSignals []Signal
	UpSignals   []Signal
}

// Target represents a spacecraft being tracked by an antenna.
type Target struct {
	ID           int
	Name         string
	DownlegRange float64 // km
	UplegRange   float64 // km
	RTLT         float64 // Round-Trip Light Time in seconds
}

// Signal represents an up or down signal on an antenna.
type Signal struct {
	Active       bool
	SignalType   string  // e.g., "data", "carrier", "ranging"
	DataRate     float64 // bits per second
	Frequency    float64 // Hz
	Band         string  // e.g., "X", "S", "Ka"
	Power        float64 // dBm or kW depending on direction
	SpacecraftID int
	Spacecraft   string
}

// Spacecraft represents a spacecraft entity with aggregated info.
type Spacecraft struct {
	ID       int
	Name     string
	Links    []Link // Active links to this spacecraft
	Distance float64 // Estimated distance in km (derived)
	Velocity float64 // Estimated velocity in km/s (derived)
}

// Link represents an active communication link between an antenna and spacecraft.
type Link struct {
	StationID    string
	AntennaID    string
	Complex      Complex
	SpacecraftID int
	Spacecraft   string

	// Signal characteristics
	Band     string  // e.g., "X", "S", "Ka"
	DataRate float64 // bits per second (highest of up/down)
	DownRate float64 // downlink rate bps
	UpRate   float64 // uplink rate bps
	Power    float64 // signal power

	// Timing
	RTLT      float64   // Round-Trip Light Time in seconds
	PassStart time.Time // estimated pass start
	PassEnd   time.Time // estimated pass end

	// Derived
	Distance      float64 // km, derived from RTLT
	SignalQuality float64 // 0-1 quality indicator
}

// DSNData represents a complete snapshot of DSN state at a point in time.
type DSNData struct {
	Timestamp time.Time
	Stations  []Station
	Links     []Link      // All active links (flattened view)
	Errors    []string    // Any parse warnings/errors
}

// ComplexLoad represents utilization metrics for a complex.
type ComplexLoad struct {
	Complex       Complex
	ActiveLinks   int
	TotalAntennas int
	Utilization   float64 // 0-1
}

// SkyObject represents a spacecraft visible in the sky from a DSN complex.
type SkyObject struct {
	Complex       Complex
	StationID     string
	AntennaID     string
	SpacecraftID  int
	Spacecraft    string
	Azimuth       float64 // degrees 0-360
	Elevation     float64 // degrees 0-90
	Band          string
	Distance      float64 // km
	DataRate      float64 // bps
	StruggleIndex float64 // 0-1
}

// SkyObjects extracts all visible spacecraft from DSN data for sky view rendering.
func (d *DSNData) SkyObjects() []SkyObject {
	if d == nil {
		return nil
	}
	var objects []SkyObject
	for _, station := range d.Stations {
		for _, ant := range station.Antennas {
			for _, target := range ant.Targets {
				obj := SkyObject{
					Complex:      station.Complex,
					StationID:    station.Name,
					AntennaID:    ant.ID,
					SpacecraftID: target.ID,
					Spacecraft:   target.Name,
					Azimuth:      ant.Azimuth,
					Elevation:    ant.Elevation,
					Distance:     DistanceFromRTLT(target.RTLT),
				}
				// Find matching link for band/rate
				for _, link := range d.Links {
					if link.Spacecraft == target.Name && link.AntennaID == ant.ID {
						obj.Band = link.Band
						obj.DataRate = link.DataRate
						obj.StruggleIndex = StruggleIndex(link, ant.Elevation)
						break
					}
				}
				objects = append(objects, obj)
			}
		}
	}
	return objects
}
