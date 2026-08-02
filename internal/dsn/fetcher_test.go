package dsn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/version"
)

// TestFetcherRevalidatesWithConditionalRequest checks that a second fetch sends
// the stored validators and treats a 304 as "unchanged" rather than an error.
func TestFetcherRevalidatesWithConditionalRequest(t *testing.T) {
	const etag = `"abc123"`
	const lastMod = "Sun, 02 Aug 2026 00:00:00 GMT"

	var sawIfNoneMatch, sawIfModifiedSince string
	var requests int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n == 1 {
			w.Header().Set("ETag", etag)
			w.Header().Set("Last-Modified", lastMod)
			_, _ = w.Write([]byte(realisticXML))
			return
		}
		sawIfNoneMatch = r.Header.Get("If-None-Match")
		sawIfModifiedSince = r.Header.Get("If-Modified-Since")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	f := NewFetcher(WithURL(srv.URL), WithHTTPClient(srv.Client()))
	ctx := context.Background()

	first := f.Fetch(ctx)
	if first.Error != nil {
		t.Fatalf("first fetch: %v", first.Error)
	}
	if first.NotModified {
		t.Error("first fetch should not report NotModified")
	}
	if first.Data == nil || len(first.Data.Stations) == 0 {
		t.Fatal("first fetch returned no parsed data")
	}

	second := f.Fetch(ctx)
	if second.Error != nil {
		t.Fatalf("second fetch: %v", second.Error)
	}
	if !second.NotModified {
		t.Error("second fetch should report NotModified on a 304")
	}
	if second.Data == nil {
		t.Fatal("a 304 must still surface the cached snapshot")
	}
	if len(second.Data.Stations) != len(first.Data.Stations) {
		t.Errorf("cached snapshot has %d stations, want %d",
			len(second.Data.Stations), len(first.Data.Stations))
	}

	if sawIfNoneMatch != etag {
		t.Errorf("If-None-Match = %q, want %q", sawIfNoneMatch, etag)
	}
	if sawIfModifiedSince != lastMod {
		t.Errorf("If-Modified-Since = %q, want %q", sawIfModifiedSince, lastMod)
	}
}

// TestFetcherFirstRequestIsUnconditional guards against sending validators we
// don't have, which would make a cold start silently return an empty snapshot.
func TestFetcherFirstRequestIsUnconditional(t *testing.T) {
	var hadValidators bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" || r.Header.Get("If-Modified-Since") != "" {
			hadValidators = true
		}
		_, _ = w.Write([]byte(realisticXML))
	}))
	defer srv.Close()

	f := NewFetcher(WithURL(srv.URL), WithHTTPClient(srv.Client()))
	if res := f.Fetch(context.Background()); res.Error != nil {
		t.Fatalf("fetch: %v", res.Error)
	}
	if hadValidators {
		t.Error("first request must not carry conditional headers")
	}
}

func TestFetcherSendsIdentifyingUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(realisticXML))
	}))
	defer srv.Close()

	f := NewFetcher(WithURL(srv.URL), WithHTTPClient(srv.Client()))
	if res := f.Fetch(context.Background()); res.Error != nil {
		t.Fatalf("fetch: %v", res.Error)
	}

	if got != version.UserAgent() {
		t.Errorf("User-Agent = %q, want %q", got, version.UserAgent())
	}
	if strings.Contains(got, "1.0 (DSN Visualization Tool)") {
		t.Error("User-Agent still carries the old hardcoded version")
	}
}

// TestFetcherBacksOffOnRateLimit verifies that NASA asking us to slow down arms
// a cooldown rather than being treated as an ordinary failure.
func TestFetcherBacksOffOnRateLimit(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	f := NewFetcher(WithURL(srv.URL), WithHTTPClient(srv.Client()))
	res := f.Fetch(context.Background())

	if res.Error == nil {
		t.Fatal("expected an error when rate limited")
	}
	if !strings.Contains(res.Error.Error(), "back off") {
		t.Errorf("error should describe the backoff, got: %v", res.Error)
	}
	f.mu.Lock()
	cooldown := f.cooldownTill
	f.mu.Unlock()
	if cooldown.IsZero() || time.Until(cooldown) <= 0 {
		t.Error("a 429 should arm a cooldown")
	}
	if n := atomic.LoadInt32(&calls); n < 2 || n > maxFetchAttempts {
		t.Errorf("made %d attempts, want between 2 and %d", n, maxFetchAttempts)
	}
}

// TestFetcherDoesNotRetryClientErrors keeps a bad request from being replayed.
func TestFetcherDoesNotRetryClientErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := NewFetcher(WithURL(srv.URL), WithHTTPClient(srv.Client()))
	if res := f.Fetch(context.Background()); res.Error == nil {
		t.Fatal("expected an error on 404")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("made %d attempts on a 404, want exactly 1", n)
	}
}

// TestFetcherRespectsContextCancellation ensures a shutdown signal interrupts a
// pending backoff instead of holding the process open.
func TestFetcherRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := NewFetcher(WithURL(srv.URL), WithHTTPClient(srv.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	res := f.Fetch(ctx)
	elapsed := time.Since(start)

	if res.Error == nil {
		t.Fatal("expected an error")
	}
	// Without cancellation this would sit through multiple seconds of backoff.
	if elapsed > 2*time.Second {
		t.Errorf("fetch took %s; context cancellation should have cut the backoff short", elapsed)
	}
}
