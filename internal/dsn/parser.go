package dsn

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"time"
)

// XML structures matching the DSN feed format

// xmlDSN is the root element of the DSN XML feed.
// In the actual feed, stations and dishes are siblings (not nested).
// Dishes belong to the preceding station element.
type xmlDSN struct {
	XMLName   xml.Name     `xml:"dsn"`
	Stations  []xmlStation `xml:"station"`
	Dishes    []xmlDish    `xml:"dish"` // siblings of stations at top level
	Timestamp string       `xml:"timestamp"`
}

type xmlStation struct {
	Name         string `xml:"name,attr"`
	FriendlyName string `xml:"friendlyName,attr"`
	TimeUTC      string `xml:"timeUTC,attr"`
	TimeZone     string `xml:"timeZoneOffset,attr"`
}

type xmlDish struct {
	Name           string      `xml:"name,attr"`
	AzimuthAngle   string      `xml:"azimuthAngle,attr"`
	ElevationAngle string      `xml:"elevationAngle,attr"`
	WindSpeed      string      `xml:"windSpeed,attr"`
	IsMSPA         string      `xml:"isMSPA,attr"`
	IsArray        string      `xml:"isArray,attr"`
	IsDDOR         string      `xml:"isDDOR,attr"`
	Activity       string      `xml:"activity,attr"`
	Created        string      `xml:"created,attr"`
	Updated        string      `xml:"updated,attr"`
	Targets        []xmlTarget `xml:"target"`
	DownSignals    []xmlSignal `xml:"downSignal"`
	UpSignals      []xmlSignal `xml:"upSignal"`
}

type xmlTarget struct {
	Name         string `xml:"name,attr"`
	ID           string `xml:"id,attr"`
	DownlegRange string `xml:"downlegRange,attr"`
	UplegRange   string `xml:"uplegRange,attr"`
	RTLT         string `xml:"rtlt,attr"`
}

type xmlSignal struct {
	Active       string `xml:"active,attr"`
	SignalType   string `xml:"signalType,attr"`
	DataRate     string `xml:"dataRate,attr"`
	Frequency    string `xml:"frequency,attr"`
	Band         string `xml:"band,attr"`
	Power        string `xml:"power,attr"`
	Spacecraft   string `xml:"spacecraft,attr"`
	SpacecraftID string `xml:"spacecraftID,attr"` // capital ID in real feed
}

// Parse parses DSN XML data and returns a DSNData structure.
func Parse(data []byte) (*DSNData, error) {
	var raw xmlDSN
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal DSN XML: %w", err)
	}

	result := &DSNData{
		Timestamp: time.Now().UTC(),
		Stations:  make([]Station, 0, len(raw.Stations)),
		Links:     make([]Link, 0),
		Errors:    make([]string, 0),
	}

	// Parse timestamp if available
	if raw.Timestamp != "" {
		if ts, err := parseTimestamp(raw.Timestamp); err == nil {
			result.Timestamp = ts
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("parse timestamp: %v", err))
		}
	}

	// Build station map for dish association
	stationMap := make(map[Complex]*Station)
	for _, xmlStn := range raw.Stations {
		station := parseStationHeader(xmlStn)
		stationMap[station.Complex] = &station
		result.Stations = append(result.Stations, station)
	}

	// Associate dishes with stations by complex (inferred from antenna ID)
	for _, xmlDish := range raw.Dishes {
		complex := inferComplex(xmlDish.Name)
		antenna, links, errs := parseDish(xmlDish, complex, string(complex))
		result.Links = append(result.Links, links...)
		result.Errors = append(result.Errors, errs...)

		// Add antenna to corresponding station
		for i := range result.Stations {
			if result.Stations[i].Complex == complex {
				result.Stations[i].Antennas = append(result.Stations[i].Antennas, antenna)
				break
			}
		}
	}

	return result, nil
}

// parseStationHeader parses station metadata (without dishes, which are siblings).
func parseStationHeader(xmlStn xmlStation) Station {
	station := Station{
		Name:         xmlStn.Name,
		FriendlyName: xmlStn.FriendlyName,
		Complex:      Complex(xmlStn.Name), // station name IS the complex ID (gdscc, mdscc, cdscc)
		Antennas:     make([]Antenna, 0),
	}

	// Parse time
	if xmlStn.TimeUTC != "" {
		if t, err := parseTimestamp(xmlStn.TimeUTC); err == nil {
			station.TimeUTC = t
		}
	}

	// Parse timezone offset (comes as milliseconds in the feed)
	if xmlStn.TimeZone != "" {
		if tz, err := strconv.ParseFloat(xmlStn.TimeZone, 64); err == nil {
			station.TimeZone = int(tz / 1000) // convert ms to seconds
		}
	}

	return station
}

func parseDish(xmlDish xmlDish, complex Complex, stationName string) (Antenna, []Link, []string) {
	var errors []string

	antenna := Antenna{
		ID:       xmlDish.Name,
		Name:     xmlDish.Name,
		Activity: xmlDish.Activity,
	}

	// Parse numeric fields
	antenna.Azimuth = parseFloat(xmlDish.AzimuthAngle, &errors, "azimuth")
	antenna.Elevation = parseFloat(xmlDish.ElevationAngle, &errors, "elevation")
	antenna.WindSpeed = parseFloat(xmlDish.WindSpeed, &errors, "windSpeed")

	// Parse boolean flags
	antenna.IsMSPA = xmlDish.IsMSPA == "true"
	antenna.IsArray = xmlDish.IsArray == "true"
	antenna.IsDDOR = xmlDish.IsDDOR == "true"

	// Parse timestamps
	if xmlDish.Created != "" {
		if t, err := parseTimestamp(xmlDish.Created); err == nil {
			antenna.Created = t
		}
	}
	if xmlDish.Updated != "" {
		if t, err := parseTimestamp(xmlDish.Updated); err == nil {
			antenna.Updated = t
		}
	}

	// Parse targets
	for _, xmlTgt := range xmlDish.Targets {
		target := Target{
			Name:         xmlTgt.Name,
			DownlegRange: parseFloat(xmlTgt.DownlegRange, &errors, "downlegRange"),
			UplegRange:   parseFloat(xmlTgt.UplegRange, &errors, "uplegRange"),
			RTLT:         parseFloat(xmlTgt.RTLT, &errors, "rtlt"),
		}
		if xmlTgt.ID != "" {
			if id, err := strconv.Atoi(xmlTgt.ID); err == nil {
				target.ID = id
			}
		}
		antenna.Targets = append(antenna.Targets, target)
	}

	// Parse signals
	for _, xmlSig := range xmlDish.DownSignals {
		sig := parseSignal(xmlSig, &errors)
		antenna.DownSignals = append(antenna.DownSignals, sig)
	}
	for _, xmlSig := range xmlDish.UpSignals {
		sig := parseSignal(xmlSig, &errors)
		antenna.UpSignals = append(antenna.UpSignals, sig)
	}

	// Build links from targets and signals
	links := buildLinks(antenna, complex, stationName)

	return antenna, links, errors
}

func parseSignal(xmlSig xmlSignal, errors *[]string) Signal {
	sig := Signal{
		Active:     xmlSig.Active == "true",
		SignalType: xmlSig.SignalType,
		Band:       xmlSig.Band,
		Spacecraft: xmlSig.Spacecraft,
		DataRate:   parseFloat(xmlSig.DataRate, errors, "dataRate"),
		Frequency:  parseFloat(xmlSig.Frequency, errors, "frequency"),
		Power:      parseFloat(xmlSig.Power, errors, "power"),
	}
	if xmlSig.SpacecraftID != "" {
		// SpacecraftID can be negative (e.g., "-62")
		if id, err := strconv.Atoi(xmlSig.SpacecraftID); err == nil {
			sig.SpacecraftID = id
		}
	}
	return sig
}

func buildLinks(antenna Antenna, complex Complex, stationName string) []Link {
	var links []Link

	// Create a link for each target with signals
	for _, target := range antenna.Targets {
		link := Link{
			StationID:    stationName,
			AntennaID:    antenna.ID,
			Complex:      complex,
			SpacecraftID: target.ID,
			Spacecraft:   target.Name,
			RTLT:         target.RTLT,
			Distance:     DistanceFromRTLT(target.RTLT),
		}

		// Find matching signals for this target
		// Match by spacecraft name (ID in XML target is positive, signal ID is negative)
		for _, sig := range antenna.DownSignals {
			if sig.Spacecraft == target.Name {
				link.DownRate = sig.DataRate
				if sig.Band != "" {
					link.Band = sig.Band
				} else if sig.Frequency > 0 {
					link.Band = inferBand(sig.Frequency)
				}
				if sig.DataRate > link.DataRate {
					link.DataRate = sig.DataRate
				}
			}
		}
		for _, sig := range antenna.UpSignals {
			if sig.Spacecraft == target.Name {
				link.UpRate = sig.DataRate
				link.Power = sig.Power
				if link.Band == "" {
					if sig.Band != "" {
						link.Band = sig.Band
					} else if sig.Frequency > 0 {
						link.Band = inferBand(sig.Frequency)
					}
				}
				if sig.DataRate > link.DataRate {
					link.DataRate = sig.DataRate
				}
			}
		}

		links = append(links, link)
	}

	return links
}

func inferComplex(stationName string) Complex {
	// DSN station naming: DSSXX or DSS-XX where XX indicates complex
	// 1x, 2x = Goldstone, 3x, 4x = Canberra, 5x, 6x = Madrid
	var digit byte
	if len(stationName) >= 5 && stationName[:4] == "DSS-" {
		digit = stationName[4] // "DSS-14" -> '1'
	} else if len(stationName) >= 4 && stationName[:3] == "DSS" {
		digit = stationName[3] // "DSS14" or "DSS55" -> '1' or '5'
	}
	if digit != 0 {
		switch digit {
		case '1', '2':
			return ComplexGoldstone
		case '3', '4':
			return ComplexCanberra
		case '5', '6':
			return ComplexMadrid
		}
	}
	// Fallback: check for complex name patterns
	switch stationName {
	case "gdscc":
		return ComplexGoldstone
	case "cdscc":
		return ComplexCanberra
	case "mdscc":
		return ComplexMadrid
	}
	return ""
}

func inferBand(frequency float64) string {
	// DSN frequency bands (approximate ranges in Hz)
	switch {
	case frequency >= 31e9 && frequency <= 40e9:
		return "Ka"
	case frequency >= 8e9 && frequency <= 12e9:
		return "X"
	case frequency >= 2e9 && frequency <= 4e9:
		return "S"
	case frequency >= 0.3e9 && frequency <= 1e9:
		return "UHF"
	default:
		return ""
	}
}

func parseFloat(s string, errors *[]string, field string) float64 {
	if s == "" || s == "none" || s == "null" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Only log truly unexpected parse errors, not empty/null values
		*errors = append(*errors, fmt.Sprintf("parse %s: %v", field, err))
		return 0
	}
	return f
}

func parseTimestamp(s string) (time.Time, error) {
	// Try multiple timestamp formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"20060102150405", // compact format sometimes used
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	// Try parsing as Unix timestamp (milliseconds)
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.UnixMilli(ms), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format: %s", s)
}
