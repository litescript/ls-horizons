// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package ephem

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/litescript/ls-horizons/internal/version"
)

// JPL's SSD/CNEOS API fair use policy states:
//
//	"You agree to submit only one API request at a time (no simultaneous requests)."
//
// Every Horizons request in this package goes through horizonsGate, which is the
// single choke point that makes that true. Without it the four independent call
// paths -- pass plans, elevation traces, ephemeris range, and formerly planets --
// could each issue a request from their own goroutine at the same moment.
//
// The gate also paces requests, and backs the whole process off when JPL signals
// overload, so a degraded upstream is met with less traffic rather than more.

const (
	// MinRequestSpacing is the minimum gap between consecutive Horizons requests.
	MinRequestSpacing = 1 * time.Second

	// maxAttempts bounds retries for a single logical request.
	maxAttempts = 3

	// baseBackoff is the first retry delay; it doubles per attempt.
	baseBackoff = 2 * time.Second

	// maxBackoff caps the retry delay.
	maxBackoff = 60 * time.Second

	// overloadCooldown is how long the entire process pauses Horizons traffic
	// after JPL signals rate limiting or unavailability without a Retry-After.
	overloadCooldown = 60 * time.Second
)

// horizonsGate serializes and paces all Horizons traffic from this process.
var horizonsGate = &requestGate{spacing: MinRequestSpacing}

// requestGate enforces one-request-at-a-time plus minimum spacing and a global
// cooldown. The mutex is deliberately held across the network call: that is what
// makes concurrent requests impossible rather than merely unlikely.
type requestGate struct {
	mu           sync.Mutex
	spacing      time.Duration
	lastRequest  time.Time
	cooldownTill time.Time
}

// get performs a rate-limited, serialized GET and returns the response body.
func (g *requestGate) get(client *http.Client, url string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		g.waitForTurn()

		body, retryAfter, err := g.attempt(client, url)
		g.lastRequest = time.Now()

		if err == nil {
			return body, nil
		}
		lastErr = err

		// A nil retryAfter means the failure is not worth retrying.
		if retryAfter == nil {
			return nil, err
		}

		// Don't sleep after the final attempt.
		if attempt == maxAttempts-1 {
			break
		}
		time.Sleep(*retryAfter)
	}

	return nil, lastErr
}

// waitForTurn blocks until both the spacing interval and any active cooldown
// have elapsed. The caller must hold g.mu.
func (g *requestGate) waitForTurn() {
	now := time.Now()

	wait := time.Duration(0)
	if since := now.Sub(g.lastRequest); since < g.spacing {
		wait = g.spacing - since
	}
	if until := g.cooldownTill.Sub(now); until > wait {
		wait = until
	}

	if wait > 0 {
		time.Sleep(wait)
	}
}

// attempt performs one HTTP GET. It returns the body on success. On failure it
// returns a non-nil retry delay when the request is worth retrying, or nil when
// it is not. The caller must hold g.mu.
func (g *requestGate) attempt(client *http.Client, url string) ([]byte, *time.Duration, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build horizons request: %w", err)
	}
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Transport-level failure: worth one more try after a backoff.
		delay := backoffDelay(0)
		return nil, &delay, fmt.Errorf("horizons request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, nil, fmt.Errorf("read horizons response: %w", readErr)
		}
		return body, nil, nil

	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusServiceUnavailable:
		// JPL is telling us to slow down. Honor Retry-After when present and
		// pause all Horizons traffic process-wide, not just this request.
		delay := retryAfterDelay(resp.Header.Get("Retry-After"), overloadCooldown)
		g.cooldownTill = time.Now().Add(delay)
		return nil, &delay, fmt.Errorf("horizons asked us to back off (status %d), pausing %s",
			resp.StatusCode, delay.Round(time.Second))

	case resp.StatusCode >= 500:
		delay := backoffDelay(0)
		return nil, &delay, fmt.Errorf("horizons returned status %d (service may be unavailable)", resp.StatusCode)

	default:
		// 4xx other than 429 means our request is wrong; retrying won't fix it.
		return nil, nil, fmt.Errorf("horizons returned status %d", resp.StatusCode)
	}
}

// backoffDelay returns an exponentially increasing delay with full jitter, so
// that independent instances do not retry in lockstep.
func backoffDelay(attempt int) time.Duration {
	delay := baseBackoff << attempt
	if delay > maxBackoff {
		delay = maxBackoff
	}
	// Full jitter across [delay/2, delay].
	half := delay / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

// retryAfterDelay parses a Retry-After header, which may be either a count of
// seconds or an HTTP date. Falls back to the supplied default when absent or
// unparseable.
func retryAfterDelay(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(header); err == nil {
		if secs < 0 {
			return fallback
		}
		delay := time.Duration(secs) * time.Second
		if delay > maxBackoff {
			return maxBackoff
		}
		return delay
	}
	if when, err := http.ParseTime(header); err == nil {
		if delay := time.Until(when); delay > 0 {
			if delay > maxBackoff {
				return maxBackoff
			}
			return delay
		}
	}
	return fallback
}
