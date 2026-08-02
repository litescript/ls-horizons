package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAtomicWriteIsNeverObservedPartially is the reason atomicWriteFile exists:
// a web server reading the file while it is being rewritten must never receive
// a truncated body.
func TestAtomicWriteIsNeverObservedPartially(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")

	// A payload large enough that a non-atomic write would be observably torn.
	payload := map[string]any{"filler": strings.Repeat("x", 512*1024), "n": 0}

	write := func(n int) func(io.Writer) error {
		return func(w io.Writer) error {
			payload["n"] = n
			return json.NewEncoder(w).Encode(payload)
		}
	}

	// Seed the file so readers always have something to read.
	if err := atomicWriteFile(path, write(0)); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	var stop atomic.Bool
	var reads, tornReads int64
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				data, err := os.ReadFile(path)
				if err != nil {
					// A missing file would itself be a torn observation.
					atomic.AddInt64(&tornReads, 1)
					continue
				}
				var out map[string]any
				if err := json.Unmarshal(data, &out); err != nil {
					atomic.AddInt64(&tornReads, 1)
					continue
				}
				atomic.AddInt64(&reads, 1)
			}
		}()
	}

	for n := 1; n <= 40; n++ {
		if err := atomicWriteFile(path, write(n)); err != nil {
			t.Fatalf("write %d: %v", n, err)
		}
	}
	stop.Store(true)
	wg.Wait()

	if got := atomic.LoadInt64(&tornReads); got != 0 {
		t.Errorf("%d readers observed a torn or missing file; writes are not atomic", got)
	}
	if atomic.LoadInt64(&reads) == 0 {
		t.Fatal("no successful reads; the test did not exercise the race")
	}
}

// TestAtomicWriteLeavesNoTempFiles guards against littering the serve directory,
// which a static file server would happily expose.
func TestAtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")

	for i := 0; i < 5; i++ {
		err := atomicWriteFile(path, func(w io.Writer) error {
			_, err := w.Write([]byte(`{"ok":true}`))
			return err
		})
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "payload.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only payload.json", names)
	}
}

// TestAtomicWriteCleansUpAfterFailure ensures a failed serializer doesn't leave
// a stray temp file behind, and doesn't clobber the previous good payload.
func TestAtomicWriteCleansUpAfterFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")

	good := []byte(`{"generation":1}`)
	if err := atomicWriteFile(path, func(w io.Writer) error {
		_, err := w.Write(good)
		return err
	}); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	wantErr := fmt.Errorf("serializer exploded")
	if err := atomicWriteFile(path, func(w io.Writer) error {
		_, _ = w.Write([]byte(`{"partial":`))
		return wantErr
	}); err == nil {
		t.Fatal("expected the write to fail")
	}

	// The previous payload must survive untouched.
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(current, good) {
		t.Errorf("failed write damaged the existing file: got %q, want %q", current, good)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("failed write left %d files behind, want 1", len(entries))
	}
}

func TestAtomicWriteIsWorldReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")

	if err := atomicWriteFile(path, func(w io.Writer) error {
		_, err := w.Write([]byte(`{}`))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// os.CreateTemp makes 0600 files; a web server running as another user has
	// to be able to read the result.
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("mode = %o, want 644 so the web server can read it", perm)
	}
}

// TestJitterOnlyLengthensInterval matters because jitter that could shorten the
// wait would raise the request rate against NASA rather than smooth it.
func TestJitterOnlyLengthensInterval(t *testing.T) {
	const base = 60 * time.Second
	maxSeen := time.Duration(0)

	for i := 0; i < 1000; i++ {
		got := jitteredInterval(base)
		if got < base {
			t.Fatalf("jitter produced %s, shorter than the %s interval", got, base)
		}
		if got > maxSeen {
			maxSeen = got
		}
	}

	upper := base + time.Duration(float64(base)*pollJitterFraction)
	if maxSeen > upper {
		t.Errorf("jitter reached %s, above the %s ceiling", maxSeen, upper)
	}
	// Sanity: it should actually vary.
	if maxSeen == base {
		t.Error("jitter never varied the interval")
	}
}

func TestJitterHandlesZeroInterval(t *testing.T) {
	if got := jitteredInterval(0); got != 0 {
		t.Errorf("jitteredInterval(0) = %s, want 0", got)
	}
}

// TestWriteServeFilesPublishesBothEndpoints covers the directory-publishing path
// end to end, including creating the directory.
func TestWriteServeFilesPublishesBothEndpoints(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "web")

	if err := writeServeFiles(dir, nil, time.Now()); err != nil {
		t.Fatalf("writeServeFiles: %v", err)
	}

	for _, name := range []string{dsnEndpointFile, solarSystemEndpointFile} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Errorf("%s is not valid JSON: %v", name, err)
		}
		if parsed["schema_version"] == nil {
			t.Errorf("%s is missing schema_version", name)
		}
	}
}
