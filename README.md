# ls-horizons

A terminal UI for visualizing NASA's Deep Space Network in real-time.

> **Note:** This project is under active development. Features may change and bugs are expected.

![ls-horizons demo](demo.gif)

## Features

- **Real-time DSN monitoring** — Live data from NASA's Deep Space Network XML feed
- **Pass planning** — Computed visibility windows for all three DSN complexes using JPL Horizons ephemeris
- **Elevation sparkline** — Real-time ±2h elevation trace with truecolor gradient in Mission view
- **Real star catalog** — 150+ bright stars with accurate J2000 coordinates rendered in the sky view
- **Astronomical projection** — Proper RA/Dec to Az/El conversion using GMST/LST calculations
- **Local planetary ephemeris** — Planet positions propagated in-process from Keplerian orbital elements, accurate to a few arcminutes with no network dependency
- **JPL Horizons integration** — Trajectory path arcs and geocentric RA/Dec for pass planning
- **Signal propagation visualizer** — Animated light-time display showing one-way/round-trip delay with pulse animation
- **Mission Spotlight** — Curated mission profiles with live phase tracking, MET countdown, crew info, and timeline rail (Voyager 1)
- **Four view modes**:
  - **Dashboard** — Complex status and active spacecraft table with multi-antenna tracking and mission spotlight badges
  - **Mission Detail** — Per-spacecraft deep dive with pass schedules, link details, propagation delay, and mission spotlight panel
  - **Sky View** — Animated star field with spacecraft positions and smooth camera transitions
  - **Orbit View** — Solar system visualization with real planet positions, spacecraft trajectories, and mission-aware HUD
- **Derived metrics**:
  - Distance calculated from round-trip light time (RTLT), with JPL Horizons fallback
  - Velocity estimation from RTLT delta
  - "Struggle index" — composite difficulty metric based on distance, data rate, and elevation
- **Event detection** — Tracks link handoffs between complexes, new acquisitions, and signal losses
- **Headless mode** — JSON export and text summaries for scripting and monitoring
- **Data endpoints** — Publish DSN state and heliocentric solar system positions as JSON for a web server to serve ([deployment guide](deploy/README.md))
- **Deliberately light on upstreams** — Conditional requests, honest User-Agent, `Retry-After` handling, jittered backoff, and strict request serialization against JPL's fair use policy ([details](#being-a-good-citizen))

## Screenshots

### Dashboard View
Real-time status of all three DSN complexes with active spacecraft table showing antennas, bands, data rates, distances, and struggle indicators.

![Dashboard](docs/screenshots/dashboard.png?v=0.9.0)

### Mission Detail View
Deep dive into individual spacecraft with link details, pass schedules, elevation sparkline, and signal propagation visualizer showing light-time delay with animated pulse. Press `Enter` from Dashboard to jump directly here. Curated missions (like Voyager 1) show a spotlight panel with phase, MET, and timeline rail — clearly labeled as schedule-derived data.

![Mission Detail](docs/screenshots/mission.png?v=0.9.0)

### Sky View
Animated celestial view with real star positions, spacecraft locations, and trajectory path arcs. Smooth camera transitions when cycling between spacecraft.

![Sky View](docs/screenshots/sky-view.png?v=0.9.0)

### Orbit View
Solar system visualization showing planets at real positions and active spacecraft with their trajectories. Planet positions are propagated locally from orbital elements, so this view works with no network and never waits on an API. Toggle star background with `t`.

![Orbit View](docs/screenshots/orbit-view.png?v=0.9.0)

## Installation

### Requirements

- **Terminal with truecolor support** — The UI uses 24-bit color for gradients and styling. Most modern terminals work fine (iTerm2, Alacritty, Kitty, Windows Terminal, GNOME Terminal, etc.). Basic terminals like older xterm or screen may have limited color support.
- **Go 1.21+** — For building from source

### From source

```bash
go install github.com/litescript/ls-horizons/cmd/ls-horizons@latest
```

Make sure your Go bin directory is on your `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

### Build locally

```bash
git clone https://github.com/litescript/ls-horizons.git
cd ls-horizons
go build -o ls-horizons ./cmd/ls-horizons
```

### Pre-built binaries

Pre-built binaries are available from [Releases](https://github.com/litescript/ls-horizons/releases) and in `os-builds/`. These have no runtime dependencies. Note that the in-tree copies may lag behind the latest source.

**Linux (x64):**
```bash
./os-builds/linux-amd64/ls-horizons
```

**macOS ARM (Apple Silicon):**
```bash
./os-builds/mac-arm/ls-horizons
```

**Windows (x64):**
```powershell
.\os-builds\windows-amd64\ls-horizons.exe
```

> **Windows users:** Use [Windows Terminal](https://aka.ms/terminal) for best results. It's included by default on Windows 11, or install free from the Microsoft Store on Windows 10. The legacy cmd.exe and PowerShell windows have limited color support and may not render correctly. Windows Terminal defaults to a dark background; the legacy blue PowerShell background will look odd with this app.

## Usage

### Interactive TUI

```bash
# Launch with default 5-second refresh
ls-horizons

# Custom refresh interval
ls-horizons --refresh 30s

# Use specific ephemeris source
ls-horizons --ephem horizons   # JPL Horizons (default)
ls-horizons --ephem dsn        # DSN-derived only
ls-horizons --ephem auto       # Horizons with fallback
```

**Keybindings:**

| Key | Action |
|-----|--------|
| `1` or `d` | Dashboard view |
| `2` or `m` | Mission detail view |
| `3` or `s` | Sky view |
| `4` or `o` | Orbit view |
| `Tab` | Cycle through views |
| `Enter` | Open Mission view for selected spacecraft (Dashboard) |
| `j/k` or `↑/↓` | Navigate lists |
| `[/]` or `←/→` | Cycle spacecraft (Mission/Sky/Orbit) |
| `h` | Toggle pass panel (Mission view) |
| `l` | Toggle labels (Sky view) |
| `c` | Cycle complex filter (Sky view) |
| `p` | Toggle trajectory path (Sky view) |
| `t` | Toggle star background (Orbit view) |
| `q` | Quit |

To upgrade, re-run the `go install` command above or download a newer build from
[Releases](https://github.com/litescript/ls-horizons/releases).

### Headless Mode

```bash
# Print summary table once
ls-horizons --summary

# Summary with ASCII mini sky view
ls-horizons --summary --mini-sky

# Watch mode: refresh every 30 seconds
ls-horizons --summary --watch 30s

# Single-line "now playing" mode
ls-horizons --now

# Show card for specific spacecraft
ls-horizons --sc VGR1

# Show only changes between fetches
ls-horizons --diff --watch 30s

# Beep on important events (TTY only)
ls-horizons --summary --watch 30s --beep

# Show event log
ls-horizons --events

# Export DSN JSON snapshot to file
ls-horizons --snapshot-path snapshot.json

# Export JSON to stdout (for piping)
ls-horizons --snapshot-path -

# Export solar system positions (heliocentric, for 3D consumers)
ls-horizons --solar-snapshot-path solarsystem.json
```

### Data endpoints

`--serve-dir` writes both JSON payloads into a directory for a web server to
serve as static files. Writes are atomic, so a reader never sees a partial file.

```bash
# Write once and exit (pair with a systemd timer or cron)
ls-horizons --serve-dir /var/lib/ls-horizons/web

# Run as a daemon, refreshing every 60 seconds
ls-horizons --serve-dir /var/lib/ls-horizons/web --watch 60s
```

This produces two files:

| File | Contents |
|------|----------|
| `dsn.json` | Stations, antennas, active links, per-complex load |
| `solarsystem.json` | Heliocentric positions for the Sun, planets, and range-resolved spacecraft |

`solarsystem.json` uses **J2000 heliocentric ecliptic coordinates in AU** with the
Sun at the origin and the ecliptic as the XY plane, so the coordinates drop
straight into a 3D scene without any frame conversion. Each body reports whether
its position came from local orbital propagation or live DSN tracking.

Note that a spacecraft only appears in `solarsystem.json` when the DSN feed
publishes a ranging solution for it. The feed often reports `-1` across every
target at once, in which case the payload contains the Sun and planets only.
Consumers should tolerate an empty spacecraft list.

See [deploy/README.md](deploy/README.md) for a systemd unit, a Caddy config, and
the full setup.

### All Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--refresh` | `5s` | TUI data refresh interval (5s - 5m). Headless cadence is set by `--watch` |
| `--ephem` | `auto` | Ephemeris source: `horizons`, `dsn`, or `auto` |
| `--summary` | `false` | Print text summary instead of TUI |
| `--mini-sky` | `false` | Show ASCII mini sky view |
| `--now` | `false` | Single-line now-playing mode |
| `--sc` | `""` | Show card for specific spacecraft |
| `--diff` | `false` | Show only changes between fetches |
| `--beep` | `false` | Beep on important events (TTY only) |
| `--events` | `false` | Show event log |
| `--watch` | `0` | Repeat output at interval (floored at 10s with `--serve-dir`) |
| `--snapshot-path` | `""` | Export DSN JSON to file (`-` for stdout) |
| `--solar-snapshot-path` | `""` | Export solar system JSON to file (`-` for stdout) |
| `--serve-dir` | `""` | Write `dsn.json` and `solarsystem.json` into a directory |
| `--log-level` | `info` | Log level (debug, info, warn, error) |
| `-l`, `--log-file` | `""` | Write logs to file (e.g., `~/ls-horizons.log`) |

## Data Sources

### NASA Deep Space Network

Live telemetry data from NASA's publicly available DSN feed:

```
https://eyes.nasa.gov/dsn/data/dsn.xml
```

The DSN consists of three antenna complexes positioned roughly 120° apart around Earth:
- **Goldstone** (GDSCC) — Mojave Desert, California, USA
- **Canberra** (CDSCC) — Tidbinbilla, Australia
- **Madrid** (MDSCC) — Robledo de Chavela, Spain

This positioning ensures continuous coverage for deep space missions as Earth rotates.

### JPL Horizons

Spacecraft trajectory data from NASA/JPL's Horizons system:

```
https://ssd.jpl.nasa.gov/api/horizons.api
```

Used for computing accurate sky positions, trajectory path arcs, and range/light-time estimates when DSN data is unavailable. Supports 45+ spacecraft with NAIF SPICE ID mappings including Voyager 1/2, JWST, Mars rovers, Juno, New Horizons, and more.

Horizons is **not** used for planet positions — those are propagated locally (see below).

### Local planetary ephemeris

Planet positions come from JPL's published "Approximate Positions of the Major
Planets" Keplerian element set, propagated in-process. Validated against Horizons
at a fixed epoch, the inner planets agree to under a tenth of an arcminute and
Saturn — the worst case — to 4.5 arcminutes, far below the resolution of any view
here.

This is a deliberate trade. Querying Horizons for eight planets on a timer was
the single heaviest demand this app placed on a live NASA computation service,
for bodies that move imperceptibly between refreshes. Computing them locally
costs microseconds, needs no network, and makes the Orbit view work offline.

### Yale Bright Star Catalog

Star positions sourced from the Yale Bright Star Catalog and IAU star names. The sky view renders 150+ stars down to magnitude ~4.5, with brightness-based rendering (brighter stars get larger glyphs).

## Being a good citizen

Both upstreams are public science services, run on public money, free to use and
requiring no API key. That's a privilege worth not abusing. This client tries to
be a guest worth having:

**NASA DSN feed** — a static XML file behind CloudFront, regenerated roughly every
five seconds:

- Revalidates with `ETag` / `If-Modified-Since`, so an unchanged feed transfers
  zero bytes instead of re-downloading.
- Identifies itself with a real version and a project URL, so someone can make
  contact rather than silently block an anonymous agent.
- Honors `Retry-After` on 429 and 503 and pauses process-wide when asked to.
- Retries with jittered exponential backoff, and never retries a 4xx.
- Jitters every poll interval so separate instances don't synchronize into a
  thundering herd.
- Floors the interactive interval at the feed's own 5s regeneration period —
  polling faster cannot surface newer data.

**JPL Horizons** — a live computation service under a
[fair use policy](https://ssd-api.jpl.nasa.gov/) that this client honors:

> *"You agree to submit only one API request at a time (no simultaneous requests)."*

Every Horizons request in the process passes through a single gate that holds its
lock across the network call, with at least a second between requests. Results are
cached for five minutes.

> *"You may not embed these APIs in your website (per NASA CORS policy)."*

Horizons is never reachable from a browser and is never proxied per-visitor. The
`--serve-dir` deployment publishes cached, derived snapshots on a timer, so
upstream request rate depends only on time and never on how many people are
looking. **Do not** reverse-proxy Horizons to work around its missing CORS
headers — the absent header is that policy being enforced, not an oversight.

If you deploy this at meaningful scale, contact JPL and NASA rather than assuming
these defaults still apply.

## Architecture

```
cmd/ls-horizons/
├── main.go             Entry point and CLI flags
└── serve.go            Atomic snapshot publishing and poll jitter
deploy/                 systemd unit, Caddy config, deployment guide
internal/
├── astro/              Astronomical calculations
│   ├── coords.go       RA/Dec ↔ Az/El transforms, GMST/LST
│   ├── frames.go       Coordinate frame conversions (ecliptic, etc.)
│   ├── planets.go      Keplerian planet propagation (local ephemeris)
│   ├── visibility.go   Ground station visibility calculations
│   ├── sun.go          Sun position calculations
│   └── stars.go        Star catalog with 150+ bright stars
├── dsn/
│   ├── models.go       Data structures (Station, Antenna, Link, etc.)
│   ├── parser.go       XML feed parsing
│   ├── fetcher.go      HTTP client with conditional requests and backoff
│   ├── derive.go       Distance, velocity, struggle index
│   ├── passplan.go     Pass planning with elevation thresholds
│   ├── elevtrace.go    Elevation trace computation for sparklines
│   ├── spacecraft.go   Spacecraft catalog with mission metadata
│   ├── spacecraft_view.go  Multi-antenna tracking abstraction
│   ├── solarsystem.go  Solar system cache with planet positions
│   ├── observer.go     DSN complex observer locations
│   ├── export.go       DSN JSON and text export
│   └── export_solarsystem.go  Heliocentric body export for 3D consumers
├── ephem/              Ephemeris providers
│   ├── provider.go     EphemerisProvider interface
│   ├── horizons.go     JPL Horizons API client (ephemeris + RA/Dec)
│   ├── fairuse.go      Process-wide Horizons request gate and backoff
│   ├── dsn_provider.go DSN-derived fallback
│   └── targets.go      NAIF SPICE ID mappings (45+ spacecraft)
├── state/
│   └── state.go        Thread-safe state with pass plan and elevation trace caching
├── missions/           Mission spotlight layer
│   ├── models.go       MissionProfile, SpotlightState, DataProvenance
│   ├── catalog.go      Curated profiles (Voyager 1)
│   ├── aliases.go      Spacecraft name/code resolution
│   ├── runtime.go      Live phase/MET/countdown computation
│   └── viewmodel.go    Display formatting helpers
├── ui/
│   ├── ui.go           Bubble Tea main model with request queue
│   ├── dashboard.go    Dashboard view with Enter→Mission flow and spotlight badges
│   ├── mission_detail.go  Mission view with pass panel, elevation sparkline, and spotlight
│   ├── sky_view.go     Sky projection with braille arc rendering
│   └── solarsystem_view.go  Orbit view with ecliptic projection and mission HUD
├── logging/
│   └── logging.go      Structured logging
└── version/
    └── version.go      Version constant and upstream User-Agent
```

## Why "ls-horizons"?

A play on the Unix `ls` command — this tool lets you "list" what's happening at the horizons of our solar system. Also a nod to NASA's New Horizons mission to Pluto and beyond.

## Changelog

- **0.11.1** — Fixed the Linux release binary, which was dynamically linked against a recent glibc and would not start on distributions shipping anything older than glibc 2.34 (Debian 11, Ubuntu 20.04, and similar). It is statically linked again, as documented. Only the Linux download was affected
- **0.11.0** — Relicensed from MIT to Apache-2.0 (adds an explicit patent grant and a `NOTICE` attribution mechanism; earlier releases remain MIT). Added `THIRD-PARTY-NOTICES` reproducing the license texts of every dependency statically linked into the release binaries, and release archives now ship `LICENSE` and `NOTICE` alongside the binary
- **0.10.0** — Solar system JSON endpoint with heliocentric positions for external consumers, `--serve-dir` to publish data endpoints for a web server, planet positions computed locally so the Orbit view no longer depends on network availability, and far lighter traffic against NASA and JPL. **Removed:** the in-app update check and installer, which relied on `go install` and so never worked for anyone running a pre-built binary. **Breaking:** JSON exports now carry `schema_version`, `complex_loads` keys are snake_case, and unknown range/light-time is `null` rather than `-1`
- **0.9.1** — Retire completed Artemis II mission profile from spotlight catalog
- **0.9.0** — Mission Spotlight: curated Artemis II & Voyager 1 profiles with live phase/MET/countdown, crew display, timeline rail, data provenance labels, and graceful handling of unsupported ephemeris lookups
- **0.8.0** — Signal propagation delay visualizer, ephemeris range/light-time fallback via Horizons
- **0.7.3** — Fix orbit trace mismatch when rapidly switching focused spacecraft
- **0.7.2** — Fix Mission tab spacecraft selection, fix "pass in now" grammar
- **0.7.1** — Only shimmer update result, not "checking" state
- **0.7.0** — Seamless in-app restart after update (Unix), Windows graceful fallback
- **0.6.0** — Update check UX with shimmer reveal animation, in-app update install
- **0.5.0** — Elevation sparkline in Mission view, per-spacecraft caching
- **0.4.0** — Visibility engine, sun separation angle, Doppler modeling
- **0.3.0** — JPL Horizons ephemeris integration, trajectory path arcs, `--ephem` flag
- **0.2.0** — Real star catalog, astronomical projection, SpacecraftView abstraction
- **0.1.0** — Initial release: TUI dashboard, sky view, headless modes, event tracking

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Run tests: `go test ./...` and `go vet ./...`
4. Submit a pull request

## License

Apache License 2.0 — see [LICENSE](LICENSE) for the full text and [NOTICE](NOTICE)
for attribution.

If you redistribute ls-horizons, in source or binary form, the license asks you to
keep the copyright notice, include a copy of the license, carry the `NOTICE` file
forward, and state that you changed any files you modified.

Releases up to and including **v0.10.0** were published under the MIT License.
That grant is perpetual and irrevocable, so those versions remain available under
MIT; **v0.11.0 onward is Apache-2.0**. Both are permissive, so this changes very
little in practice — it mainly adds an explicit patent grant and a clearer
attribution mechanism.

### Third-party components

ls-horizons ships as a statically linked binary that incorporates open source
components under the MIT and BSD-3-Clause licenses. Their copyright notices and
full license texts are reproduced in [THIRD-PARTY-NOTICES](THIRD-PARTY-NOTICES),
which is included with every release. Regenerate it with `scripts/gen-notices.sh`
after changing dependencies.

## Acknowledgments

- NASA/JPL for the public DSN data feed and Horizons ephemeris system
- Yale Bright Star Catalog for star position data
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the excellent TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) for terminal styling
