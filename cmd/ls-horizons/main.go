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

const (
	defaultRefresh = 5 * time.Second
	minRefresh     = 1 * time.Second
	maxRefresh     = 5 * time.Minute
)

func main() {
	// Parse flags
	refresh := flag.Duration("refresh", defaultRefresh, "Data refresh interval (e.g., 5s, 1m)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
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
