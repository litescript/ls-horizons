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

	"github.com/peter/ls-horizons/internal/dsn"
	"github.com/peter/ls-horizons/internal/logging"
	"github.com/peter/ls-horizons/internal/state"
	"github.com/peter/ls-horizons/internal/ui"
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

	// Create TUI model
	model := ui.New(stateMgr)

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
	// Do initial fetch immediately
	doFetch(ctx, fetcher, stateMgr, p, logger)

	ticker := time.NewTicker(stateMgr.RefreshInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("Fetch loop shutting down")
			return
		case <-ticker.C:
			doFetch(ctx, fetcher, stateMgr, p, logger)
		}
	}
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
