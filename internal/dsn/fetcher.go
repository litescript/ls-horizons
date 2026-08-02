// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package dsn

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/litescript/ls-horizons/internal/version"
)

const (
	// DefaultDSNURL is the official NASA DSN Now XML feed.
	DefaultDSNURL = "https://eyes.nasa.gov/dsn/data/dsn.xml"

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second

	// maxFetchAttempts bounds retries within a single Fetch call.
	maxFetchAttempts = 3

	// baseFetchBackoff is the first retry delay; it doubles per attempt.
	baseFetchBackoff = 2 * time.Second

	// maxFetchBackoff caps the retry delay.
	maxFetchBackoff = 60 * time.Second

	// defaultOverloadCooldown is how long to pause after NASA signals rate
	// limiting or unavailability without supplying a Retry-After.
	defaultOverloadCooldown = 120 * time.Second
)

// Fetcher handles HTTP fetching of DSN data.
//
// The feed is a static XML file served from CloudFront and regenerated roughly
// every five seconds. It supports conditional requests, so this fetcher stores
// the last ETag and Last-Modified and revalidates rather than blindly
// re-downloading: when nothing has changed, NASA serves a 304 with no body.
type Fetcher struct {
	client  *http.Client
	url     string
	timeout time.Duration

	mu           sync.Mutex
	etag         string
	lastModified string
	cached       *DSNData
	cooldownTill time.Time
}

// FetcherOption configures a Fetcher.
type FetcherOption func(*Fetcher)

// WithURL sets a custom URL for the DSN feed.
func WithURL(url string) FetcherOption {
	return func(f *Fetcher) {
		f.url = url
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) FetcherOption {
	return func(f *Fetcher) {
		f.timeout = d
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) FetcherOption {
	return func(f *Fetcher) {
		f.client = client
	}
}

// NewFetcher creates a new DSN data fetcher.
func NewFetcher(opts ...FetcherOption) *Fetcher {
	f := &Fetcher{
		url:     DefaultDSNURL,
		timeout: DefaultTimeout,
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.client == nil {
		f.client = &http.Client{
			Timeout: f.timeout,
		}
	}

	return f
}

// FetchResult contains the result of a fetch operation.
type FetchResult struct {
	Data      *DSNData
	RawBytes  []byte
	FetchedAt time.Time
	Duration  time.Duration
	Error     error

	// NotModified is true when the feed was revalidated and found unchanged.
	// Data still carries the previously parsed snapshot, so callers can skip
	// redundant downstream work rather than re-deriving identical state.
	NotModified bool
}

// Fetch retrieves and parses the DSN XML feed.
func (f *Fetcher) Fetch(ctx context.Context) FetchResult {
	start := time.Now()
	result := FetchResult{
		FetchedAt: start,
	}

	var lastErr error

	for attempt := 0; attempt < maxFetchAttempts; attempt++ {
		if err := f.waitForCooldown(ctx); err != nil {
			result.Duration = time.Since(start)
			result.Error = err
			return result
		}

		rawData, notModified, retryAfter, err := f.fetchRaw(ctx)
		if err == nil {
			result.Duration = time.Since(start)

			if notModified {
				f.mu.Lock()
				result.Data = f.cached
				f.mu.Unlock()
				result.NotModified = true
				return result
			}

			data, parseErr := Parse(rawData)
			if parseErr != nil {
				result.Error = fmt.Errorf("parse DSN data: %w", parseErr)
				return result
			}

			f.mu.Lock()
			f.cached = data
			f.mu.Unlock()

			result.RawBytes = rawData
			result.Data = data
			return result
		}

		lastErr = err
		if retryAfter == nil || attempt == maxFetchAttempts-1 {
			break
		}
		if err := sleepCtx(ctx, *retryAfter); err != nil {
			lastErr = err
			break
		}
	}

	result.Duration = time.Since(start)
	result.Error = lastErr
	return result
}

// FetchRaw retrieves the raw XML bytes without parsing.
func (f *Fetcher) FetchRaw(ctx context.Context) ([]byte, error) {
	body, _, _, err := f.fetchRaw(ctx)
	return body, err
}

// waitForCooldown blocks while a server-requested backoff is in effect.
func (f *Fetcher) waitForCooldown(ctx context.Context) error {
	f.mu.Lock()
	wait := time.Until(f.cooldownTill)
	f.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	return sleepCtx(ctx, wait)
}

// fetchRaw performs one conditional GET. It returns the body, whether the feed
// was unchanged, and a retry delay when the failure is worth retrying.
func (f *Fetcher) fetchRaw(ctx context.Context) (body []byte, notModified bool, retryAfter *time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, false, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/xml, text/xml")

	// Revalidate rather than re-download when we've seen this feed before.
	f.mu.Lock()
	etag, lastMod, haveCached := f.etag, f.lastModified, f.cached != nil
	f.mu.Unlock()

	if haveCached {
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		if lastMod != "" {
			req.Header.Set("If-Modified-Since", lastMod)
		}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, nil, ctx.Err()
		}
		delay := fetchBackoff(0)
		return nil, false, &delay, fmt.Errorf("fetch DSN XML: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return nil, true, nil, nil

	case resp.StatusCode == http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, false, nil, fmt.Errorf("read response body: %w", err)
		}
		f.mu.Lock()
		f.etag = resp.Header.Get("ETag")
		f.lastModified = resp.Header.Get("Last-Modified")
		f.mu.Unlock()
		return body, false, nil, nil

	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusServiceUnavailable:
		delay := parseRetryAfter(resp.Header.Get("Retry-After"), defaultOverloadCooldown)
		f.mu.Lock()
		f.cooldownTill = time.Now().Add(delay)
		f.mu.Unlock()
		return nil, false, &delay, fmt.Errorf("DSN feed asked us to back off (status %d), pausing %s",
			resp.StatusCode, delay.Round(time.Second))

	case resp.StatusCode >= 500:
		delay := fetchBackoff(0)
		return nil, false, &delay, fmt.Errorf("unexpected status code: %d", resp.StatusCode)

	default:
		return nil, false, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// URL returns the configured feed URL.
func (f *Fetcher) URL() string {
	return f.url
}

// fetchBackoff returns an exponentially increasing delay with jitter.
func fetchBackoff(attempt int) time.Duration {
	delay := baseFetchBackoff << attempt
	if delay > maxFetchBackoff {
		delay = maxFetchBackoff
	}
	half := delay / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

// parseRetryAfter reads a Retry-After header in either seconds or HTTP-date form.
func parseRetryAfter(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(header); err == nil {
		if secs < 0 {
			return fallback
		}
		delay := time.Duration(secs) * time.Second
		if delay > maxFetchBackoff {
			return maxFetchBackoff
		}
		return delay
	}
	if when, err := http.ParseTime(header); err == nil {
		if delay := time.Until(when); delay > 0 {
			if delay > maxFetchBackoff {
				return maxFetchBackoff
			}
			return delay
		}
	}
	return fallback
}

// sleepCtx sleeps for d unless the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
