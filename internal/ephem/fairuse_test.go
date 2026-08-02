// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package ephem

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/version"
)

// TestGateNeverIssuesSimultaneousRequests is the direct check on JPL's fair use
// requirement: "submit only one API request at a time (no simultaneous requests)".
func TestGateNeverIssuesSimultaneousRequests(t *testing.T) {
	var inFlight int32
	var maxObserved int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			observed := atomic.LoadInt32(&maxObserved)
			if current <= observed || atomic.CompareAndSwapInt32(&maxObserved, observed, current) {
				break
			}
		}
		// Hold the request open long enough that any concurrency would overlap.
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	gate := &requestGate{spacing: time.Millisecond}
	client := srv.Client()

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := gate.get(client, srv.URL); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&maxObserved); got != 1 {
		t.Fatalf("observed %d simultaneous requests, fair use policy allows 1", got)
	}
}

func TestGateEnforcesSpacing(t *testing.T) {
	var timestamps []time.Time
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	const spacing = 40 * time.Millisecond
	gate := &requestGate{spacing: spacing}

	for i := 0; i < 4; i++ {
		if _, err := gate.get(srv.Client(), srv.URL); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(timestamps); i++ {
		gap := timestamps[i].Sub(timestamps[i-1])
		// Allow a small scheduling tolerance below the nominal spacing.
		if gap < spacing-10*time.Millisecond {
			t.Errorf("requests %d and %d only %s apart, want >= %s", i-1, i, gap, spacing)
		}
	}
}

func TestGateSendsIdentifyingUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	gate := &requestGate{spacing: time.Millisecond}
	if _, err := gate.get(srv.Client(), srv.URL); err != nil {
		t.Fatal(err)
	}

	if got != version.UserAgent() {
		t.Errorf("User-Agent = %q, want %q", got, version.UserAgent())
	}
	if !strings.Contains(got, version.Version) {
		t.Errorf("User-Agent %q should carry the real version %q", got, version.Version)
	}
	if !strings.Contains(got, "https://") {
		t.Errorf("User-Agent %q should carry a contactable project URL", got)
	}
}

// TestGateBacksOffOnRateLimit verifies that a 429 pauses process-wide traffic
// rather than only delaying the request that received it.
func TestGateBacksOffOnRateLimit(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	gate := &requestGate{spacing: time.Millisecond}
	_, err := gate.get(srv.Client(), srv.URL)

	if err == nil {
		t.Fatal("expected an error when the server rate limits")
	}
	if !strings.Contains(err.Error(), "back off") {
		t.Errorf("error should describe the backoff, got: %v", err)
	}
	if gate.cooldownTill.IsZero() || time.Until(gate.cooldownTill) <= 0 {
		t.Error("a 429 should arm a process-wide cooldown")
	}
	// It should have retried, but a bounded number of times.
	if n := atomic.LoadInt32(&calls); n < 2 || n > maxAttempts {
		t.Errorf("made %d attempts, want between 2 and %d", n, maxAttempts)
	}
}

// TestGateDoesNotRetryClientErrors keeps a malformed query from being replayed
// at JPL repeatedly when the fault is ours.
func TestGateDoesNotRetryClientErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	gate := &requestGate{spacing: time.Millisecond}
	if _, err := gate.get(srv.Client(), srv.URL); err == nil {
		t.Fatal("expected an error on 400")
	}

	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("made %d attempts on a 400, want exactly 1", n)
	}
}

func TestRetryAfterDelay(t *testing.T) {
	fallback := 30 * time.Second

	if got := retryAfterDelay("", fallback); got != fallback {
		t.Errorf("empty header: got %s, want fallback %s", got, fallback)
	}
	if got := retryAfterDelay("5", fallback); got != 5*time.Second {
		t.Errorf("seconds header: got %s, want 5s", got)
	}
	if got := retryAfterDelay("garbage", fallback); got != fallback {
		t.Errorf("unparseable header: got %s, want fallback %s", got, fallback)
	}
	if got := retryAfterDelay("-3", fallback); got != fallback {
		t.Errorf("negative header: got %s, want fallback %s", got, fallback)
	}
	// An absurd Retry-After must not park the client indefinitely.
	if got := retryAfterDelay("999999", fallback); got != maxBackoff {
		t.Errorf("oversized header: got %s, want cap %s", got, maxBackoff)
	}
	// HTTP-date form.
	future := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	if got := retryAfterDelay(future, fallback); got <= 0 || got > 11*time.Second {
		t.Errorf("http-date header: got %s, want ~10s", got)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if got := retryAfterDelay(past, fallback); got != fallback {
		t.Errorf("past http-date: got %s, want fallback %s", got, fallback)
	}
}

func TestBackoffDelayIsBoundedAndJittered(t *testing.T) {
	seen := make(map[time.Duration]bool)
	for i := 0; i < 200; i++ {
		d := backoffDelay(0)
		if d < baseBackoff/2 || d > baseBackoff {
			t.Fatalf("delay %s outside [%s, %s]", d, baseBackoff/2, baseBackoff)
		}
		seen[d] = true
	}
	if len(seen) < 10 {
		t.Errorf("backoff produced only %d distinct values; jitter looks absent", len(seen))
	}

	// High attempt counts must stay capped.
	for attempt := 0; attempt < 8; attempt++ {
		if d := backoffDelay(attempt); d > maxBackoff {
			t.Errorf("attempt %d produced %s, exceeding cap %s", attempt, d, maxBackoff)
		}
	}
}
