package ephem

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/litescript/ls-horizons/internal/astro"
)

const (
	// HorizonsAPIURL is the JPL Horizons JSON API endpoint.
	HorizonsAPIURL = "https://ssd.jpl.nasa.gov/api/horizons.api"

	// DefaultPathDuration is the default time span for trajectory paths.
	DefaultPathDuration = 24 * time.Hour

	// DefaultPathStep is the default step between path points.
	DefaultPathStep = 10 * time.Minute

	// PathCacheTTL is how long to cache path data before regenerating.
	PathCacheTTL = 5 * time.Minute

	// RequestTimeout is the HTTP request timeout.
	RequestTimeout = 30 * time.Second
)

// HorizonsProvider queries JPL Horizons for spacecraft ephemerides.
type HorizonsProvider struct {
	client *http.Client

	// Path cache
	mu        sync.RWMutex
	pathCache map[TargetID]*cachedPath
}

// cachedPath stores a cached trajectory.
type cachedPath struct {
	path      EphemerisPath
	observer  astro.Observer
	fetchedAt time.Time
}

// NewHorizonsProvider creates a new Horizons API client.
func NewHorizonsProvider() *HorizonsProvider {
	return &HorizonsProvider{
		client: &http.Client{
			Timeout: RequestTimeout,
		},
		pathCache: make(map[TargetID]*cachedPath),
	}
}

// Name implements Provider.
func (p *HorizonsProvider) Name() string {
	return "Horizons"
}

// GetPosition implements Provider.
// Queries Horizons for the current position of a target.
func (p *HorizonsProvider) GetPosition(target TargetID, t time.Time, obs astro.Observer) (EphemerisPoint, error) {
	// Query for a single point
	path, err := p.queryHorizons(target, t, t.Add(time.Minute), time.Minute, obs)
	if err != nil {
		return EphemerisPoint{Valid: false}, err
	}

	if len(path.Points) == 0 {
		return EphemerisPoint{Valid: false}, fmt.Errorf("no data returned for target %d", target)
	}

	return path.Points[0], nil
}

// GetPath implements Provider.
// Returns a cached path if available, otherwise queries Horizons.
func (p *HorizonsProvider) GetPath(target TargetID, start, end time.Time, step time.Duration, obs astro.Observer) (EphemerisPath, error) {
	// Check cache
	p.mu.RLock()
	cached, ok := p.pathCache[target]
	p.mu.RUnlock()

	if ok && time.Since(cached.fetchedAt) < PathCacheTTL && observerMatch(cached.observer, obs) {
		return cached.path, nil
	}

	// Query fresh data
	path, err := p.queryHorizons(target, start, end, step, obs)
	if err != nil {
		return EphemerisPath{}, err
	}

	// Cache result
	p.mu.Lock()
	p.pathCache[target] = &cachedPath{
		path:      path,
		observer:  obs,
		fetchedAt: time.Now(),
	}
	p.mu.Unlock()

	return path, nil
}

// InvalidateCache clears the path cache for a target.
// Called when focus changes to force fresh data.
func (p *HorizonsProvider) InvalidateCache(target TargetID) {
	p.mu.Lock()
	delete(p.pathCache, target)
	p.mu.Unlock()
}

// Available implements Provider.
func (p *HorizonsProvider) Available(target TargetID) bool {
	_, ok := TargetsByNAIF[target]
	return ok
}

// queryHorizons makes a request to the Horizons API.
func (p *HorizonsProvider) queryHorizons(target TargetID, start, end time.Time, step time.Duration, obs astro.Observer) (EphemerisPath, error) {
	// Build request parameters - values must be quoted with single quotes
	params := url.Values{}
	params.Set("format", "json")
	params.Set("COMMAND", fmt.Sprintf("'%d'", target))
	params.Set("OBJ_DATA", "NO")
	params.Set("MAKE_EPHEM", "YES")
	params.Set("EPHEM_TYPE", "OBSERVER")
	params.Set("CENTER", "'coord@399'")
	params.Set("COORD_TYPE", "GEODETIC")
	params.Set("SITE_COORD", fmt.Sprintf("'%.4f,%.4f,0.1'", obs.LonDeg, obs.LatDeg))
	params.Set("START_TIME", fmt.Sprintf("'%s'", formatHorizonsTime(start)))
	params.Set("STOP_TIME", fmt.Sprintf("'%s'", formatHorizonsTime(end)))
	params.Set("STEP_SIZE", fmt.Sprintf("'%s'", formatStepSize(step)))
	params.Set("QUANTITIES", "'4,20,21'") // 4=Apparent Az/El, 20=Range/RangeRate, 21=One-way light time

	reqURL := HorizonsAPIURL + "?" + params.Encode()

	body, err := horizonsGate.get(p.client, reqURL)
	if err != nil {
		return EphemerisPath{}, err
	}

	return parseHorizonsResponse(target, body, obs)
}

// horizonsResponse represents the JSON API response.
type horizonsResponse struct {
	Signature struct {
		Version string `json:"version"`
		Source  string `json:"source"`
	} `json:"signature"`
	Result string `json:"result"`
}

// parseHorizonsResponse parses the Horizons JSON response.
func parseHorizonsResponse(target TargetID, body []byte, obs astro.Observer) (EphemerisPath, error) {
	// Check for HTML error page (Horizons returns HTML on some errors)
	bodyStr := string(body)
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "<!DOCTYPE") ||
		strings.HasPrefix(strings.TrimSpace(bodyStr), "<html") ||
		strings.HasPrefix(strings.TrimSpace(bodyStr), "<HTML") {
		return EphemerisPath{}, fmt.Errorf("horizons API returned HTML error page (service may be unavailable)")
	}

	var resp horizonsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return EphemerisPath{}, fmt.Errorf("failed to parse Horizons response as JSON")
	}

	// The actual ephemeris data is in resp.Result as a text blob
	points, err := parseEphemerisTable(resp.Result, obs)
	if err != nil {
		return EphemerisPath{}, err
	}

	path := EphemerisPath{
		TargetID: target,
		Points:   points,
	}

	if len(points) > 0 {
		path.Start = points[0].Time
		path.End = points[len(points)-1].Time
	}

	return path, nil
}

// parseEphemerisTable extracts ephemeris points from the Horizons text output.
func parseEphemerisTable(result string, obs astro.Observer) ([]EphemerisPoint, error) {
	var points []EphemerisPoint

	// Find the data section between $$SOE and $$EOE markers
	soeIdx := strings.Index(result, "$$SOE")
	eoeIdx := strings.Index(result, "$$EOE")
	if soeIdx == -1 || eoeIdx == -1 || soeIdx >= eoeIdx {
		return nil, fmt.Errorf("could not find ephemeris data markers")
	}

	dataSection := result[soeIdx+5 : eoeIdx]
	lines := strings.Split(dataSection, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		point, err := parseEphemerisLine(line, obs)
		if err != nil {
			continue // Skip unparseable lines
		}
		points = append(points, point)
	}

	return points, nil
}

// parseEphemerisLine parses a single ephemeris data line.
// Format for QUANTITIES='4,20,21' (Az/El, Range/RangeRate, LightTime):
// 2025-Dec-05 00:00 *   261.032124  32.878027  1.234567  -0.123  10.234
// Fields: date, time, flags, azimuth, elevation, delta(AU), deldot(km/s), LT(min)
func parseEphemerisLine(line string, obs astro.Observer) (EphemerisPoint, error) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return EphemerisPoint{}, fmt.Errorf("insufficient fields: %d", len(fields))
	}

	// Parse date/time (first two fields)
	dateStr := fields[0] + " " + fields[1]
	t, err := parseHorizonsDateTime(dateStr)
	if err != nil {
		return EphemerisPoint{}, err
	}

	// Collect all numeric values after date/time
	// Skip any flag fields (like *, *m, Cm, Nm, Am, etc.)
	var nums []float64
	for i := 2; i < len(fields); i++ {
		val, err := strconv.ParseFloat(fields[i], 64)
		if err == nil {
			nums = append(nums, val)
		}
	}

	if len(nums) < 2 {
		return EphemerisPoint{}, fmt.Errorf("could not find Az/El values")
	}

	// nums[0]=az, nums[1]=el
	point := EphemerisPoint{
		Time: t,
		Coord: astro.SkyCoord{
			AzDeg: nums[0],
			ElDeg: nums[1],
		},
		Valid: true,
	}

	// If we have range data: nums[2]=deltaAU, nums[3]=deldotKmS, nums[4]=oneWayMin
	if len(nums) >= 5 {
		point.RangeAU = nums[2]
		point.RangeKm = astro.AUToKm(nums[2])
		point.RangeRateKmS = nums[3]
		point.OneWayLTMin = nums[4]
		point.HasRangeData = true
	}

	return point, nil
}

// parseHorizonsDateTime parses Horizons date format like "2025-Dec-05 00:00".
func parseHorizonsDateTime(s string) (time.Time, error) {
	// Horizons uses format like "2025-Dec-05 00:00"
	t, err := time.Parse("2006-Jan-02 15:04", s)
	if err == nil {
		return t.UTC(), nil
	}

	// Try with seconds
	t, err = time.Parse("2006-Jan-02 15:04:05", s)
	if err == nil {
		return t.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

// formatHorizonsTime formats a time for Horizons API.
func formatHorizonsTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04")
}

// formatStepSize formats a duration as a Horizons step size.
func formatStepSize(d time.Duration) string {
	minutes := int(d.Minutes())
	if minutes >= 60 {
		hours := minutes / 60
		return fmt.Sprintf("%d h", hours)
	}
	return fmt.Sprintf("%d m", minutes)
}

// observerMatch checks if two observers are close enough to share cache.
func observerMatch(a, b astro.Observer) bool {
	const tolerance = 0.1 // degrees
	if abs(a.LatDeg-b.LatDeg) > tolerance {
		return false
	}
	if abs(a.LonDeg-b.LonDeg) > tolerance {
		return false
	}
	return true
}

// RADecCacheTTL is how long to cache RA/Dec path data.
const RADecCacheTTL = 5 * time.Minute

// cachedRADec stores cached RA/Dec samples.
type cachedRADec struct {
	samples   []astro.RADecAtTime
	fetchedAt time.Time
}

// raDecCache stores RA/Dec paths by target ID.
var raDecCache = struct {
	sync.RWMutex
	data map[TargetID]*cachedRADec
}{data: make(map[TargetID]*cachedRADec)}

// GetRADecPath returns RA/Dec samples for a target over a time range.
// This is used for pass planning where we need geocentric RA/Dec, not observer-centric Az/El.
func (p *HorizonsProvider) GetRADecPath(target TargetID, start, end time.Time, step time.Duration) ([]astro.RADecAtTime, error) {
	// Check cache
	raDecCache.RLock()
	cached, ok := raDecCache.data[target]
	raDecCache.RUnlock()

	if ok && time.Since(cached.fetchedAt) < RADecCacheTTL {
		return cached.samples, nil
	}

	// Query fresh data
	samples, err := p.queryRADec(target, start, end, step)
	if err != nil {
		return nil, err
	}

	// Cache result
	raDecCache.Lock()
	raDecCache.data[target] = &cachedRADec{
		samples:   samples,
		fetchedAt: time.Now(),
	}
	raDecCache.Unlock()

	return samples, nil
}

// queryRADec queries Horizons for RA/Dec over a time range.
func (p *HorizonsProvider) queryRADec(target TargetID, start, end time.Time, step time.Duration) ([]astro.RADecAtTime, error) {
	// Build request parameters for geocentric RA/Dec
	params := url.Values{}
	params.Set("format", "json")
	params.Set("COMMAND", fmt.Sprintf("'%d'", target))
	params.Set("OBJ_DATA", "NO")
	params.Set("MAKE_EPHEM", "YES")
	params.Set("EPHEM_TYPE", "OBSERVER")
	params.Set("CENTER", "'500@399'") // Geocentric (Earth center)
	params.Set("START_TIME", fmt.Sprintf("'%s'", formatHorizonsTime(start)))
	params.Set("STOP_TIME", fmt.Sprintf("'%s'", formatHorizonsTime(end)))
	params.Set("STEP_SIZE", fmt.Sprintf("'%s'", formatStepSize(step)))
	params.Set("QUANTITIES", "'1'") // 1 = Astrometric RA/Dec

	reqURL := HorizonsAPIURL + "?" + params.Encode()

	body, err := horizonsGate.get(p.client, reqURL)
	if err != nil {
		return nil, err
	}

	return parseRADecResponse(body)
}

// parseRADecResponse parses the Horizons JSON response for RA/Dec data.
func parseRADecResponse(body []byte) ([]astro.RADecAtTime, error) {
	// Check for HTML error page (Horizons returns HTML on some errors)
	bodyStr := string(body)
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "<!DOCTYPE") ||
		strings.HasPrefix(strings.TrimSpace(bodyStr), "<html") ||
		strings.HasPrefix(strings.TrimSpace(bodyStr), "<HTML") {
		return nil, fmt.Errorf("horizons API returned HTML error page (service may be unavailable)")
	}

	var resp horizonsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		// Don't include the body in error to avoid dumping HTML/garbage
		return nil, fmt.Errorf("failed to parse Horizons response as JSON")
	}

	// Find the data section between $$SOE and $$EOE markers
	soeIdx := strings.Index(resp.Result, "$$SOE")
	eoeIdx := strings.Index(resp.Result, "$$EOE")
	if soeIdx == -1 || eoeIdx == -1 || soeIdx >= eoeIdx {
		return nil, fmt.Errorf("could not find RA/Dec data markers")
	}

	dataSection := resp.Result[soeIdx+5 : eoeIdx]
	lines := strings.Split(dataSection, "\n")

	var samples []astro.RADecAtTime

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		sample, err := parseRADecLine(line)
		if err != nil {
			continue // Skip unparseable lines
		}
		samples = append(samples, sample)
	}

	return samples, nil
}

// parseRADecLine parses a single RA/Dec data line.
// Format for QUANTITIES='1' (Astrometric RA/Dec):
// 2025-Dec-05 00:00 *   261.032124  32.878027
// Fields: date, time, flags, RA (deg), Dec (deg)
func parseRADecLine(line string) (astro.RADecAtTime, error) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return astro.RADecAtTime{}, fmt.Errorf("insufficient fields: %d", len(fields))
	}

	// Parse date/time (first two fields)
	dateStr := fields[0] + " " + fields[1]
	t, err := parseHorizonsDateTime(dateStr)
	if err != nil {
		return astro.RADecAtTime{}, err
	}

	// Find RA/Dec values - they're the last two numeric fields
	// Skip any flag fields (like *, *m, Cm, Nm, Am, etc.)
	var ra, dec float64
	numericCount := 0

	for i := 2; i < len(fields); i++ {
		val, err := strconv.ParseFloat(fields[i], 64)
		if err == nil {
			numericCount++
			if numericCount == 1 {
				ra = val
			} else if numericCount == 2 {
				dec = val
				break
			}
		}
	}

	if numericCount < 2 {
		return astro.RADecAtTime{}, fmt.Errorf("could not find RA/Dec values")
	}

	return astro.RADecAtTime{
		Time:   t,
		RAdeg:  ra,
		DecDeg: dec,
	}, nil
}

// InvalidateRADecCache clears the RA/Dec cache for a target.
func (p *HorizonsProvider) InvalidateRADecCache(target TargetID) {
	raDecCache.Lock()
	delete(raDecCache.data, target)
	raDecCache.Unlock()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
