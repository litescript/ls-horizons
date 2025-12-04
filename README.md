# ls-horizons

A terminal UI for visualizing NASA's Deep Space Network in real-time.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  LS-HORIZONS                                            DSN Status: LIVE    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  GOLDSTONE (GDSCC)        CANBERRA (CDSCC)         MADRID (MDSCC)          │
│  ████████░░ 80%           ██████░░░░ 60%           ████████░░ 80%          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ ANTENNA   SPACECRAFT        BAND   RATE       DISTANCE      HEALTH │   │
│  ├─────────────────────────────────────────────────────────────────────┤   │
│  │ DSS-14    Voyager 1         S/X    160 bps    24.5 B km     POOR   │   │
│  │ DSS-43    Mars Perseverance X      2.1 Mbps   389 M km      GOOD   │   │
│  │ DSS-55    Europa Clipper    X      115 kbps   891 M km      GOOD   │   │
│  │ DSS-34    JWST              Ka     28.6 Mbps  1.5 M km      GOOD   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  [A] Dashboard  [B] Mission Detail  [C] Sky View  [Q] Quit                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Features

- **Real-time DSN monitoring** — Live data from NASA's Deep Space Network XML feed
- **Three view modes**:
  - **Dashboard** — Complex utilization bars and active links table
  - **Mission Detail** — Per-spacecraft deep dive with signal metrics
  - **Sky View** — Animated globe showing antenna pointings with smooth camera transitions
- **Derived metrics**:
  - Distance calculated from round-trip light time (RTLT)
  - Velocity estimation from RTLT delta
  - "Struggle index" — composite difficulty metric based on distance, data rate, and elevation
  - Health classification (GOOD / MARGINAL / POOR)
- **Event detection** — Tracks link handoffs between complexes, new acquisitions, and signal losses
- **Headless mode** — JSON export and text summaries for scripting and monitoring

## Installation

### From source

Requires Go 1.21+

```bash
go install github.com/litescript/ls-horizons/cmd/ls-horizons@latest
```

### Build locally

```bash
git clone https://github.com/litescript/ls-horizons.git
cd ls-horizons
go build -o ls-horizons ./cmd/ls-horizons
```

## Usage

### Interactive TUI

```bash
# Launch with default 5-second refresh
ls-horizons

# Custom refresh interval
ls-horizons --refresh 30s
```

**Keybindings:**
| Key | Action |
|-----|--------|
| `a` | Dashboard view |
| `b` | Mission detail view |
| `c` | Sky view |
| `j/k` or `↑/↓` | Navigate lists |
| `space` | Select spacecraft (sky view focus) |
| `q` | Quit |

### Headless Mode

```bash
# Print summary table once
ls-horizons --summary

# Watch mode: refresh every 30 seconds
ls-horizons --summary --watch 30s

# Export JSON snapshot to file
ls-horizons --snapshot-path snapshot.json

# Export JSON to stdout (for piping)
ls-horizons --snapshot-path -
```

### All Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--refresh` | `5s` | Data refresh interval (1s - 5m) |
| `--summary` | `false` | Print text summary instead of TUI |
| `--watch` | `0` | Repeat output at interval (implies --summary) |
| `--snapshot-path` | `""` | Export JSON to file (`-` for stdout) |
| `--log-level` | `info` | Log level (debug, info, warn, error) |

## Data Source

This tool fetches live data from NASA's publicly available Deep Space Network feed:

```
https://eyes.nasa.gov/dsn/data/dsn.xml
```

The DSN consists of three antenna complexes positioned roughly 120° apart around Earth:
- **Goldstone** (GDSCC) — Mojave Desert, California, USA
- **Canberra** (CDSCC) — Tidbinbilla, Australia
- **Madrid** (MDSCC) — Robledo de Chavela, Spain

This positioning ensures continuous coverage for deep space missions as Earth rotates.

## Architecture

```
cmd/ls-horizons/        Entry point and CLI flags
internal/
├── dsn/
│   ├── models.go       Data structures (Station, Antenna, Link, etc.)
│   ├── parser.go       XML feed parsing
│   ├── fetcher.go      HTTP client with retry logic
│   ├── derive.go       Distance, velocity, struggle index calculations
│   └── export.go       JSON and text export
├── state/
│   └── state.go        Thread-safe state manager with history buffers
├── ui/
│   ├── model.go        Bubble Tea main model
│   ├── dashboard.go    Dashboard view
│   ├── mission.go      Mission detail view
│   └── sky_view.go     Animated sky projection view
└── logging/
    └── logging.go      Structured logging
```

## Why "ls-horizons"?

A play on the Unix `ls` command — this tool lets you "list" what's happening at the horizons of our solar system. Also a nod to NASA's New Horizons mission to Pluto and beyond.

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Run tests: `go test ./...` and `go test -race ./...`
4. Submit a pull request

## License

MIT License — see [LICENSE](LICENSE) for details.

## Acknowledgments

- NASA/JPL for the public DSN data feed
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the excellent TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) for terminal styling
