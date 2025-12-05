// Command ls-horizons is a terminal UI for visualizing NASA Deep Space Network activity.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/litescript/ls-horizons/internal/dsn"
	"github.com/litescript/ls-horizons/internal/ephem"
	"github.com/litescript/ls-horizons/internal/logging"
	"github.com/litescript/ls-horizons/internal/state"
	"github.com/litescript/ls-horizons/internal/ui"
)

// CLI flags for headless mode
var (
	summaryMode   bool
	watchInterval time.Duration
	snapshotPath  string
	miniSkyMode   bool
	nowMode       bool
	scName        string
	diffMode      bool
	beepMode      bool
	eventsMode    bool
	ephemMode     string
)

const (
	defaultRefresh = 5 * time.Second
	minRefresh     = 1 * time.Second
	maxRefresh     = 5 * time.Minute
)

func main() {
	// Parse flags
	refresh := flag.Duration("refresh", defaultRefresh, "Data refresh interval (e.g., 5s, 1m)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.BoolVar(&summaryMode, "summary", false, "Print text summary instead of TUI")
	flag.DurationVar(&watchInterval, "watch", 0, "Repeat fetch at interval (e.g., 30s)")
	flag.StringVar(&snapshotPath, "snapshot-path", "", "Export JSON snapshot to file (use - for stdout)")
	flag.BoolVar(&miniSkyMode, "mini-sky", false, "Show ASCII mini sky view")
	flag.BoolVar(&nowMode, "now", false, "Single-line now-playing mode")
	flag.StringVar(&scName, "sc", "", "Show card for specific spacecraft")
	flag.BoolVar(&diffMode, "diff", false, "Show only changes between fetches")
	flag.BoolVar(&beepMode, "beep", false, "Beep on important events (TTY only)")
	flag.BoolVar(&eventsMode, "events", false, "Show event log")
	flag.StringVar(&ephemMode, "ephem", "auto", "Ephemeris source: horizons, dsn, or auto")
	flag.Parse()

	// Validate refresh interval
	if *refresh < minRefresh {
		*refresh = minRefresh
	} else if *refresh > maxRefresh {
		*refresh = maxRefresh
	}

	// Set up logging
	logger := logging.New(logging.ParseLevel(*logLevel))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Initialize components
	stateCfg := state.DefaultConfig()
	stateCfg.RefreshInterval = *refresh
	stateMgr := state.NewManager(stateCfg)

	fetcher := dsn.NewFetcher()

	// Headless mode: no TUI
	headless := summaryMode || snapshotPath != "" || miniSkyMode || nowMode || scName != "" || diffMode || eventsMode
	if headless {
		runHeadless(ctx, fetcher, stateMgr, logger)
		return
	}

	// Create ephemeris provider based on mode
	var ephemProvider ephem.Provider
	mode := ephem.ParseMode(ephemMode)
	switch mode {
	case ephem.ModeHorizons:
		ephemProvider = ephem.NewHorizonsProvider()
		logger.Info("Using JPL Horizons ephemeris")
	case ephem.ModeDSN:
		ephemProvider = ephem.NewDSNProvider()
		logger.Info("Using DSN-derived ephemeris")
	case ephem.ModeAuto:
		// Try Horizons, will fall back gracefully if unavailable
		ephemProvider = ephem.NewHorizonsProvider()
		logger.Info("Using auto ephemeris mode (Horizons with fallback)")
	}

	// Create TUI model with ephemeris provider
	model := ui.New(stateMgr, ephemProvider)

	// Create Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Start fetch loop in background
	go runFetchLoop(ctx, fetcher, stateMgr, p, logger)

	// Run TUI (blocks until quit)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runFetchLoop(ctx context.Context, fetcher *dsn.Fetcher, stateMgr *state.Manager, p *tea.Program, logger *logging.Logger) {
	interval := stateMgr.RefreshInterval()

	// Calculate next aligned refresh time and set it before initial fetch
	next := nextAlignedTime(time.Now(), interval)
	stateMgr.SetNextRefresh(next)

	// Do initial fetch immediately
	doFetch(ctx, fetcher, stateMgr, p, logger)

	for {
		// Calculate time until next aligned refresh
		now := time.Now()
		next = nextAlignedTime(now, interval)
		stateMgr.SetNextRefresh(next)

		// Create timer for the wait duration
		waitDuration := next.Sub(now)
		timer := time.NewTimer(waitDuration)

		select {
		case <-ctx.Done():
			timer.Stop()
			logger.Debug("Fetch loop shutting down")
			return
		case <-timer.C:
			doFetch(ctx, fetcher, stateMgr, p, logger)
		}
	}
}

// nextAlignedTime calculates the next refresh time aligned to wall clock.
// For example, with 5s interval, refreshes happen at :00, :05, :10, etc.
func nextAlignedTime(now time.Time, interval time.Duration) time.Time {
	// Get the interval in seconds
	intervalSec := int64(interval.Seconds())
	if intervalSec < 1 {
		intervalSec = 1
	}

	// Calculate seconds since start of minute
	sec := int64(now.Second())
	nsec := now.Nanosecond()

	// Find next aligned second
	nextSec := ((sec / intervalSec) + 1) * intervalSec

	// Build the next time
	next := time.Date(now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), 0, 0, now.Location())

	// Add the aligned seconds (may roll over to next minute)
	next = next.Add(time.Duration(nextSec) * time.Second)

	// If we're very close to the boundary (within 100ms), skip to the next one
	if next.Sub(now) < 100*time.Millisecond {
		next = next.Add(interval)
	}

	_ = nsec // unused but kept for clarity
	return next
}

func doFetch(ctx context.Context, fetcher *dsn.Fetcher, stateMgr *state.Manager, p *tea.Program, logger *logging.Logger) {
	logger.Debug("Fetching DSN data...")

	result := fetcher.Fetch(ctx)

	if result.Error != nil {
		logger.Error("Fetch failed: %v", result.Error)
		stateMgr.Update(nil, result.Duration, result.Error)
		p.Send(ui.ErrorMsg{Error: result.Error})
		return
	}

	logger.Debug("Fetch complete: %d stations, %d links in %v",
		len(result.Data.Stations), len(result.Data.Links), result.Duration)

	stateMgr.Update(result.Data, result.Duration, nil)
	p.Send(ui.DataUpdateMsg{Snapshot: stateMgr.Snapshot()})
}

// runHeadless handles all headless modes without starting TUI.
func runHeadless(ctx context.Context, fetcher *dsn.Fetcher, stateMgr *state.Manager, logger *logging.Logger) {
	var prevData *dsn.DSNData
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	outputOnce := func() error {
		result := fetcher.Fetch(ctx)
		if result.Error != nil {
			return result.Error
		}

		stateMgr.Update(result.Data, result.Duration, nil)
		snap := stateMgr.Snapshot()

		// Diff mode
		if diffMode {
			diff := dsn.ComputeDiff(prevData, snap.Data)
			dsn.WriteDiff(os.Stdout, diff, snap.LastFetch)
			// Beep on changes
			if beepMode && isTTY && diff.HasChanges() {
				fmt.Print("\a")
			}
			prevData = snap.Data
			return nil
		}

		// Now-playing mode
		if nowMode {
			dsn.WriteNowPlaying(os.Stdout, snap.Data)
			return nil
		}

		// Spacecraft card mode
		if scName != "" {
			events := convertEvents(snap.Events)
			dsn.WriteSpacecraftCard(os.Stdout, snap.Data, scName, events)
			return nil
		}

		// Export JSON if requested
		if snapshotPath != "" {
			export := dsn.ExportSnapshot(snap.Data, snap.LastFetch)
			if snapshotPath == "-" {
				if err := export.WriteJSON(os.Stdout); err != nil {
					return fmt.Errorf("write JSON to stdout: %w", err)
				}
			} else {
				f, err := os.Create(snapshotPath)
				if err != nil {
					return fmt.Errorf("create snapshot file: %w", err)
				}
				defer f.Close()
				if err := export.WriteJSON(f); err != nil {
					return fmt.Errorf("write JSON to file: %w", err)
				}
			}
		}

		// Print summary table if requested
		if summaryMode {
			dsn.WriteSummaryTable(os.Stdout, snap.Data, snap.LastFetch)
		}

		// Mini sky view
		if miniSkyMode {
			fmt.Println()
			dsn.WriteMiniSky(os.Stdout, snap.Data, dsn.DefaultMiniSkyConfig())
		}

		// Events log
		if eventsMode {
			fmt.Println()
			events := convertEvents(snap.Events)
			dsn.WriteEvents(os.Stdout, events, 10)
		}

		// Beep check for events (only in non-diff mode)
		if beepMode && isTTY && len(snap.Events) > 0 {
			// Beep if there are recent events (within last interval)
			for _, e := range snap.Events {
				if time.Since(e.Timestamp) < watchInterval+time.Second {
					fmt.Print("\a")
					break
				}
			}
		}

		prevData = snap.Data
		return nil
	}

	// Single run
	if watchInterval == 0 {
		if err := outputOnce(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Watch mode: repeat at interval
	if err := outputOnce(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !diffMode && !nowMode {
				fmt.Println() // Blank line between outputs (except diff/now mode)
			}
			if err := outputOnce(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}
}

// convertEvents converts state.Event to dsn.Event (avoiding import cycle).
func convertEvents(stateEvents []state.Event) []dsn.Event {
	events := make([]dsn.Event, len(stateEvents))
	for i, e := range stateEvents {
		events[i] = dsn.Event{
			Type:       dsn.EventType(e.Type),
			Timestamp:  e.Timestamp,
			Spacecraft: e.Spacecraft,
			OldStation: e.OldStation,
			NewStation: e.NewStation,
			AntennaID:  e.AntennaID,
			Complex:    e.Complex,
		}
	}
	return events
}
