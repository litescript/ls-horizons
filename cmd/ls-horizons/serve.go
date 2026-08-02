package main

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/litescript/ls-horizons/internal/dsn"
)

// Filenames written into the serve directory. These are the two endpoints a web
// consumer fetches; they refresh independently because they answer different
// questions and age at very different rates.
const (
	dsnEndpointFile         = "dsn.json"
	solarSystemEndpointFile = "solarsystem.json"
)

// pollJitterFraction is how much each wait is randomly stretched, as a fraction
// of the interval.
//
// Without this, every instance polling on a wall-clock-aligned schedule would
// hit NASA on the same second. One client doesn't matter; a popular client with
// synchronized instances is a self-inflicted thundering herd. Spreading arrivals
// costs nothing and keeps load flat rather than spiky.
const pollJitterFraction = 0.2

// jitteredInterval returns d stretched by a random amount up to
// pollJitterFraction. The result is never shorter than d, so jitter can only
// reduce request rate, never increase it.
func jitteredInterval(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	spread := float64(d) * pollJitterFraction
	return d + time.Duration(rand.Float64()*spread)
}

// atomicWriteFile writes via a temporary file and renames it into place.
//
// A plain create-and-write leaves a window where the file on disk is empty or
// truncated. Anything serving that file -- Caddy, nginx, a static host -- can
// hand a client a partial JSON body during that window. Rename is atomic within
// a filesystem, so readers see either the previous complete file or the new
// complete file and never a torn one.
func atomicWriteFile(path string, write func(io.Writer) error) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Clean up the temp file on any failure path.
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if err = write(tmp); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	// Flush to disk before renaming so a crash can't publish a truncated file.
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", tmpName, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	// Readable by the web server, writable only by us.
	if err = os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename into %s: %w", path, err)
	}
	return nil
}

// writeServeFiles publishes both endpoint payloads into dir.
func writeServeFiles(dir string, data *dsn.DSNData, fetchedAt time.Time) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create serve directory %s: %w", dir, err)
	}

	snapshot := dsn.ExportSnapshot(data, fetchedAt)
	if err := atomicWriteFile(filepath.Join(dir, dsnEndpointFile), snapshot.WriteJSON); err != nil {
		return err
	}

	solar := dsn.ExportSolarSystem(dsn.BuildSolarSystemSnapshot(data, fetchedAt))
	if err := atomicWriteFile(filepath.Join(dir, solarSystemEndpointFile), solar.WriteJSON); err != nil {
		return err
	}

	return nil
}

// writeJSONTarget writes an exporter's JSON to a path, or to stdout for "-".
func writeJSONTarget(path string, write func(io.Writer) error) error {
	if path == "-" {
		return write(os.Stdout)
	}
	return atomicWriteFile(path, write)
}
