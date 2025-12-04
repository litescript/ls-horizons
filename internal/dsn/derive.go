package dsn

import (
	"math"
	"sort"
)

const (
	// SpeedOfLight in km/s (vacuum).
	SpeedOfLight = 299792.458
)

// DistanceFromRTLT calculates one-way distance in km from round-trip light time in seconds.
func DistanceFromRTLT(rtlt float64) float64 {
	if rtlt <= 0 {
		return 0
	}
	// RTLT is round-trip, so divide by 2 for one-way
	return (rtlt / 2) * SpeedOfLight
}

// VelocityFromRTLTDelta estimates radial velocity from two RTLT measurements.
// Returns velocity in km/s. Positive = moving away, negative = moving closer.
func VelocityFromRTLTDelta(rtlt1, rtlt2 float64, deltaTime float64) float64 {
	if deltaTime <= 0 {
		return 0
	}
	dist1 := DistanceFromRTLT(rtlt1)
	dist2 := DistanceFromRTLT(rtlt2)
	return (dist2 - dist1) / deltaTime
}

// Health represents link health classification.
type Health string

const (
	HealthGood     Health = "GOOD"
	HealthMarginal Health = "MARGINAL"
	HealthPoor     Health = "POOR"
)

// StruggleIndex calculates a difficulty metric for a communication link.
// Returns a value from 0 (easy) to 1 (difficult).
//
// Factors and weights:
//   - Distance (40%): log scale from 100k km (0) to 10B km (1)
//   - Data rate (30%): log scale from 1 Mbps (0) to 100 bps (1)
//   - Elevation (20%): 45°+ is easy (0), 0° is hard (1)
//   - Signal quality (10%): if available, inverted 0-1 scale
func StruggleIndex(link Link, elevation float64) float64 {
	var score float64

	// Distance factor: farther = harder
	// Use log scale since distances vary enormously (Moon vs Voyager)
	if link.Distance > 0 {
		// Normalize: Moon ~384,400 km = 0.1, Mars ~225M km = 0.5, Voyager ~24B km = 1.0
		logDist := math.Log10(link.Distance)
		distFactor := clamp((logDist-5)/(10-5), 0, 1) // log(100k) to log(10B)
		score += distFactor * 0.4
	}

	// Data rate factor: lower rate often indicates struggling link
	if link.DataRate > 0 {
		// Normalize: 1 Mbps+ = easy, 100 bps = hard
		logRate := math.Log10(link.DataRate)
		rateFactor := 1 - clamp((logRate-2)/(6-2), 0, 1) // log(100) to log(1M)
		score += rateFactor * 0.3
	}

	// Elevation factor: low elevation = harder (more atmosphere)
	if elevation >= 0 {
		elevFactor := 1 - clamp(elevation/45, 0, 1) // 0-45 degrees
		score += elevFactor * 0.2
	}

	// Signal quality factor (if available)
	if link.SignalQuality > 0 {
		score += (1 - link.SignalQuality) * 0.1
	} else {
		// Default medium difficulty if no signal quality data
		score += 0.05
	}

	return clamp(score, 0, 1)
}

// ClassifyHealth converts a struggle index to a health classification.
//
// Thresholds:
//   - GOOD: struggle < 0.3 (strong signal, close, high rate, good elevation)
//   - MARGINAL: 0.3 <= struggle < 0.6 (moderate conditions)
//   - POOR: struggle >= 0.6 (weak signal, far, low rate, low elevation)
func ClassifyHealth(struggle float64) Health {
	switch {
	case struggle < 0.3:
		return HealthGood
	case struggle < 0.6:
		return HealthMarginal
	default:
		return HealthPoor
	}
}

// LinkHealth computes struggle index and health for a link.
func LinkHealth(link Link, elevation float64) (float64, Health) {
	struggle := StruggleIndex(link, elevation)
	return struggle, ClassifyHealth(struggle)
}

// ComplexUtilization calculates load metrics for each DSN complex.
func ComplexUtilization(data *DSNData) map[Complex]ComplexLoad {
	loads := make(map[Complex]ComplexLoad)

	// Initialize all known complexes
	for c := range KnownComplexes {
		loads[c] = ComplexLoad{Complex: c}
	}

	// Count antennas and active links per complex
	antennaCount := make(map[Complex]int)
	activeLinks := make(map[Complex]int)

	for _, station := range data.Stations {
		complex := station.Complex
		if complex == "" {
			continue
		}
		antennaCount[complex] += len(station.Antennas)

		for _, ant := range station.Antennas {
			if len(ant.Targets) > 0 {
				activeLinks[complex] += len(ant.Targets)
			}
		}
	}

	// Calculate utilization
	for c := range loads {
		load := loads[c]
		load.TotalAntennas = antennaCount[c]
		load.ActiveLinks = activeLinks[c]
		if load.TotalAntennas > 0 {
			// Simple utilization: ratio of active links to antennas
			// Cap at 1.0 (MSPA means one antenna can have multiple links)
			load.Utilization = math.Min(float64(load.ActiveLinks)/float64(load.TotalAntennas), 1.0)
		}
		loads[c] = load
	}

	return loads
}

// AggregateSpacecraft creates a spacecraft summary from DSN data.
func AggregateSpacecraft(data *DSNData) []Spacecraft {
	scMap := make(map[int]*Spacecraft)

	for _, link := range data.Links {
		sc, ok := scMap[link.SpacecraftID]
		if !ok {
			sc = &Spacecraft{
				ID:   link.SpacecraftID,
				Name: link.Spacecraft,
			}
			scMap[link.SpacecraftID] = sc
		}
		sc.Links = append(sc.Links, link)

		// Use the most recent/reliable distance
		if link.Distance > 0 && (sc.Distance == 0 || link.Distance < sc.Distance) {
			sc.Distance = link.Distance
		}
	}

	// Convert map to sorted slice
	spacecraft := make([]Spacecraft, 0, len(scMap))
	for _, sc := range scMap {
		spacecraft = append(spacecraft, *sc)
	}
	sort.Slice(spacecraft, func(i, j int) bool {
		return spacecraft[i].Name < spacecraft[j].Name
	})

	return spacecraft
}

// NextHandoffPrediction attempts to predict which complex will next track a spacecraft.
// This is a simple heuristic based on Earth rotation and complex positions.
func NextHandoffPrediction(currentComplex Complex, elevation float64) Complex {
	// Very simple heuristic: as Earth rotates west-to-east,
	// tracking moves Goldstone -> Canberra -> Madrid -> Goldstone
	// Predict handoff when elevation drops below threshold
	if elevation < 15 { // Getting low, handoff likely soon
		switch currentComplex {
		case ComplexGoldstone:
			return ComplexCanberra
		case ComplexCanberra:
			return ComplexMadrid
		case ComplexMadrid:
			return ComplexGoldstone
		}
	}
	return "" // No imminent handoff predicted
}

// FormatDistance returns a human-readable distance string.
func FormatDistance(km float64) string {
	switch {
	case km <= 0:
		return "N/A"
	case km < 1e6:
		return formatWithUnit(km, "km")
	case km < 1e9:
		return formatWithUnit(km/1e6, "M km")
	case km < 1e12:
		return formatWithUnit(km/1e9, "B km")
	default:
		// Convert to AU for very large distances
		au := km / 1.496e8
		return formatWithUnit(au, "AU")
	}
}

// FormatDataRate returns a human-readable data rate string.
func FormatDataRate(bps float64) string {
	switch {
	case bps <= 0:
		return "N/A"
	case bps < 1e3:
		return formatWithUnit(bps, "bps")
	case bps < 1e6:
		return formatWithUnit(bps/1e3, "kbps")
	case bps < 1e9:
		return formatWithUnit(bps/1e6, "Mbps")
	default:
		return formatWithUnit(bps/1e9, "Gbps")
	}
}

// FormatRTLT returns a human-readable round-trip light time string.
func FormatRTLT(seconds float64) string {
	switch {
	case seconds <= 0:
		return "N/A"
	case seconds < 60:
		return formatWithUnit(seconds, "s")
	case seconds < 3600:
		return formatWithUnit(seconds/60, "min")
	default:
		return formatWithUnit(seconds/3600, "hr")
	}
}

func formatWithUnit(value float64, unit string) string {
	if value < 10 {
		return floatToString(value, 2) + " " + unit
	} else if value < 100 {
		return floatToString(value, 1) + " " + unit
	}
	return floatToString(value, 0) + " " + unit
}

func floatToString(f float64, precision int) string {
	format := "%." + string(rune('0'+precision)) + "f"
	return sprintf(format, f)
}

// sprintf is a simple float formatter to avoid importing fmt for this small use.
func sprintf(format string, f float64) string {
	// Use a simple approach - in real code we'd use strconv or fmt
	// But to avoid circular deps and keep this minimal:
	switch format {
	case "%.0f":
		return strconvFormatFloat(f, 0)
	case "%.1f":
		return strconvFormatFloat(f, 1)
	case "%.2f":
		return strconvFormatFloat(f, 2)
	default:
		return strconvFormatFloat(f, 2)
	}
}

func strconvFormatFloat(f float64, prec int) string {
	// Manual float formatting
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "+Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}

	negative := f < 0
	if negative {
		f = -f
	}

	// Round to precision
	mult := math.Pow(10, float64(prec))
	rounded := math.Round(f*mult) / mult

	// Integer part
	intPart := int64(rounded)
	fracPart := rounded - float64(intPart)

	result := ""
	if negative {
		result = "-"
	}

	// Format integer part
	if intPart == 0 {
		result += "0"
	} else {
		digits := ""
		for intPart > 0 {
			digits = string(rune('0'+intPart%10)) + digits
			intPart /= 10
		}
		result += digits
	}

	// Format fractional part
	if prec > 0 {
		result += "."
		fracPart *= mult
		fracInt := int64(math.Round(fracPart))
		fracStr := ""
		for i := 0; i < prec; i++ {
			fracStr = string(rune('0'+fracInt%10)) + fracStr
			fracInt /= 10
		}
		result += fracStr
	}

	return result
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
