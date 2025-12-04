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

	"github.com/peter/ls-horizons/internal/dsn"
	"github.com/peter/ls-horizons/internal/logging"
	"github.com/peter/ls-horizons/internal/state"
	"github.com/peter/ls-horizons/internal/ui"
)

// CLI flags for headless mode
var (
	summaryMode  bool
	watchInterval time.Duration
	snapshotPath string
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
	flag.DurationVar(&watchInterval, "watch", 0, "Repeat fetch at interval (e.g., 30s). Implies --summary")
	flag.StringVar(&snapshotPath, "snapshot-path", "", "Export JSON snapshot to file (use - for stdout)")
	flag.Parse()

	// Validate refresh interval
	if *refresh < minRefresh {
		*refresh = minRefresh
	} else if *refresh > maxRefresh {
		*refresh = maxRefresh
	}

	// --watch implies --summary
	if watchInterval > 0 {
		summaryMode = true
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
	if summaryMode || snapshotPath != "" {
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

// runHeadless handles --summary and --snapshot-path modes without starting TUI.
func runHeadless(ctx context.Context, fetcher *dsn.Fetcher, stateMgr *state.Manager, logger *logging.Logger) {
	outputOnce := func() error {
		result := fetcher.Fetch(ctx)
		if result.Error != nil {
			return result.Error
		}

		stateMgr.Update(result.Data, result.Duration, nil)
		snap := stateMgr.Snapshot()

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
			fmt.Println() // Blank line between outputs
			if err := outputOnce(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}
}
