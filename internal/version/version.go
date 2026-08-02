// Package version provides build and version information.
package version

// Version is the current application version.
const Version = "0.10.0"

// ProjectURL is the canonical home of this project, included in the User-Agent
// so upstream data providers can identify and contact the operator of a client.
const ProjectURL = "https://github.com/litescript/ls-horizons"

// UserAgent returns the HTTP User-Agent this client presents to upstream data
// providers (NASA DSN, JPL Horizons). Identifying the client honestly, with a
// contactable project URL, is a condition of being a good citizen on public
// science APIs: it lets an operator reach out rather than silently block.
func UserAgent() string {
	return "ls-horizons/" + Version + " (+" + ProjectURL + ")"
}

// Milestones:
// 0.10.0 - Solar system JSON endpoint, --serve-dir publishing, locally computed planet positions
// 0.9.1 - Retire completed Artemis II mission profile from spotlight catalog
// 0.9.0 - Mission Spotlight: curated profiles (Artemis II, Voyager 1), data provenance, graceful gating
// 0.8.0 - Signal propagation delay visualizer, ephemeris range/light-time fallback via Horizons
// 0.7.3 - Fix orbit trace mismatch when rapidly switching focused spacecraft
// 0.7.2 - Fix Mission tab spacecraft selection, fix "pass in now" grammar
// 0.7.1 - Only shimmer update result, not "checking" state
// 0.7.0 - Seamless in-app restart after update (Unix), Windows graceful fallback
// 0.6.0 - Update check UX with shimmer reveal animation, in-app update install
// 0.5.0 - Elevation sparkline in Mission view, per-spacecraft caching
// 0.4.0 - Visibility engine, sun separation angle, Doppler modeling
// 0.3.0 - JPL Horizons ephemeris integration, trajectory path arcs (WIP), --ephem flag
// 0.2.0 - Real star catalog, astronomical projection, SpacecraftView abstraction
// 0.1.0 - Initial release: TUI dashboard, sky view, headless modes, event tracking
