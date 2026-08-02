// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package missions

import (
	"testing"
	"time"

	"github.com/litescript/ls-horizons/internal/dsn"
)

func TestResolveProfile(t *testing.T) {
	tests := []struct {
		input string
		want  string // expected profile ID, or "" for nil
	}{
		{"VGR1", "voyager-1"},
		{"vgr1", "voyager-1"},
		{"  VGR1  ", "voyager-1"},
		{"VOYAGER 1", "voyager-1"},
		{"VOYAGER1", "voyager-1"},
		{"JWST", ""},
		{"VOYAGER", ""}, // bare "VOYAGER" excluded: ambiguous with Voyager 2
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := ResolveProfile(tt.input)
			if tt.want == "" {
				if p != nil {
					t.Errorf("ResolveProfile(%q) = %q, want nil", tt.input, p.ID)
				}
				return
			}
			if p == nil {
				t.Fatalf("ResolveProfile(%q) = nil, want %q", tt.input, tt.want)
			}
			if p.ID != tt.want {
				t.Errorf("ResolveProfile(%q).ID = %q, want %q", tt.input, p.ID, tt.want)
			}
		})
	}
}

func TestHasSpotlight(t *testing.T) {
	if !HasSpotlight("VGR1") {
		t.Error("HasSpotlight(VGR1) should be true")
	}
	if HasSpotlight("JWST") {
		t.Error("HasSpotlight(JWST) should be false")
	}
}

func TestBuildSpotlightState_NilSpacecraft(t *testing.T) {
	st := BuildSpotlightState(time.Now(), nil)
	if st != nil {
		t.Error("expected nil for nil spacecraft")
	}
}

func TestBuildSpotlightState_NoProfile(t *testing.T) {
	sc := &dsn.Spacecraft{ID: 1, Name: "JWST"}
	st := BuildSpotlightState(time.Now(), sc)
	if st != nil {
		t.Error("expected nil for spacecraft with no profile")
	}
}

func syntheticProfile(launch time.Time) *MissionProfile {
	return &MissionProfile{
		ID:          "synthetic",
		DisplayName: "SYNTHETIC",
		Aliases:     []string{"SYN"},
		StartTime:   launch,
		EndTime:     launch.Add(10 * time.Hour),
		Events: []MissionEvent{
			{Name: "Launch", Time: launch, Key: "LAUNCH"},
			{Name: "Stage Sep", Time: launch.Add(1 * time.Hour), Key: "SEP"},
			{Name: "Burn", Time: launch.Add(3 * time.Hour), Key: "BURN"},
			{Name: "Recovery", Time: launch.Add(9 * time.Hour), Key: "END"},
		},
		Phases: []MissionPhase{
			{Name: "Pre-Launch", Start: launch.Add(-24 * time.Hour), End: launch},
			{Name: "Ascent", Start: launch, End: launch.Add(1 * time.Hour)},
			{Name: "Cruise", Start: launch.Add(1 * time.Hour), End: launch.Add(8 * time.Hour)},
			{Name: "Recovery", Start: launch.Add(8 * time.Hour), End: launch.Add(10 * time.Hour)},
		},
	}
}

func buildStateForProfile(now time.Time, profile *MissionProfile) *SpotlightState {
	// Temporarily register the profile in the catalog so BuildSpotlightState
	// can resolve it via the alias lookup.
	original := catalog
	catalog = append([]*MissionProfile{profile}, original...)
	defer func() { catalog = original }()

	sc := &dsn.Spacecraft{ID: 999, Name: profile.Aliases[0]}
	return BuildSpotlightState(now, sc)
}

func TestBuildSpotlightState_PreLaunch(t *testing.T) {
	launch := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	profile := syntheticProfile(launch)
	st := buildStateForProfile(launch.Add(-6*time.Hour), profile)

	if st == nil {
		t.Fatal("expected non-nil state")
	}
	if !st.IsPreLaunch {
		t.Error("expected IsPreLaunch=true")
	}
	if st.IsComplete {
		t.Error("expected IsComplete=false")
	}
	if st.CurrentPhase != "Pre-Launch" {
		t.Errorf("phase = %q, want Pre-Launch", st.CurrentPhase)
	}
	if st.NextEvent == nil || st.NextEvent.Key != "LAUNCH" {
		t.Errorf("next event = %+v, want LAUNCH", st.NextEvent)
	}
	if st.Countdown <= 0 {
		t.Errorf("countdown should be positive, got %v", st.Countdown)
	}
	if st.MET != 0 {
		t.Errorf("MET should be 0 pre-launch, got %v", st.MET)
	}
}

func TestBuildSpotlightState_MidMission(t *testing.T) {
	launch := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	profile := syntheticProfile(launch)
	// 2h after launch: "Cruise" phase (1h..8h); next event is BURN at 3h.
	st := buildStateForProfile(launch.Add(2*time.Hour), profile)

	if st == nil {
		t.Fatal("expected non-nil state")
	}
	if st.IsPreLaunch || st.IsComplete {
		t.Errorf("expected mid-mission flags, got pre=%v complete=%v", st.IsPreLaunch, st.IsComplete)
	}
	if st.CurrentPhase != "Cruise" {
		t.Errorf("phase = %q, want Cruise", st.CurrentPhase)
	}
	if st.NextEvent == nil || st.NextEvent.Key != "BURN" {
		t.Errorf("next event = %+v, want BURN", st.NextEvent)
	}
	if st.MET < 119*time.Minute || st.MET > 121*time.Minute {
		t.Errorf("MET = %v, want ~2h", st.MET)
	}
}

func TestBuildSpotlightState_Complete(t *testing.T) {
	launch := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	profile := syntheticProfile(launch)
	st := buildStateForProfile(profile.EndTime.Add(24*time.Hour), profile)

	if st == nil {
		t.Fatal("expected non-nil state")
	}
	if !st.IsComplete {
		t.Error("expected IsComplete=true")
	}
	if st.CurrentPhase != "Mission Complete" {
		t.Errorf("phase = %q, want Mission Complete", st.CurrentPhase)
	}
	if st.NextEvent != nil {
		t.Error("expected no next event after completion")
	}
}

func TestTimelineStatuses(t *testing.T) {
	launch := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	profile := syntheticProfile(launch)
	// 2h in: LAUNCH (0h) past, SEP (1h) current (most recent past), BURN/END future.
	st := buildStateForProfile(launch.Add(2*time.Hour), profile)

	if len(st.Timeline) != 4 {
		t.Fatalf("timeline items = %d, want 4", len(st.Timeline))
	}
	if st.Timeline[0].Status != TimelinePast {
		t.Errorf("LAUNCH status = %d, want Past", st.Timeline[0].Status)
	}
	if st.Timeline[1].Status != TimelineCurrent {
		t.Errorf("SEP status = %d, want Current", st.Timeline[1].Status)
	}
	if st.Timeline[2].Status != TimelineFuture {
		t.Errorf("BURN status = %d, want Future", st.Timeline[2].Status)
	}
	if st.Timeline[3].Status != TimelineFuture {
		t.Errorf("END status = %d, want Future", st.Timeline[3].Status)
	}
}

func TestTimelineAllPast_AfterComplete(t *testing.T) {
	launch := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	profile := syntheticProfile(launch)
	st := buildStateForProfile(profile.EndTime.Add(24*time.Hour), profile)

	for _, item := range st.Timeline {
		if item.Status != TimelinePast {
			t.Errorf("item %q status = %d, want Past after mission complete", item.Key, item.Status)
		}
	}
}

func TestSpotlightProvenance(t *testing.T) {
	sc := &dsn.Spacecraft{ID: 31, Name: "VGR1"}
	st := BuildSpotlightState(time.Now(), sc)
	if st == nil {
		t.Fatal("expected non-nil state for VGR1")
	}
	if st.Provenance != ProvenanceCurated {
		t.Errorf("VGR1 provenance = %d, want ProvenanceCurated", st.Provenance)
	}
}

func TestProvenanceLabel(t *testing.T) {
	tests := []struct {
		p    DataProvenance
		want string
	}{
		{ProvenanceCurated, "curated mission timeline"},
		{ProvenanceLive, "live telemetry"},
		{ProvenanceUnavailable, ""},
	}
	for _, tt := range tests {
		got := ProvenanceLabel(tt.p)
		if got != tt.want {
			t.Errorf("ProvenanceLabel(%d) = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestFormatCountdown(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{24*time.Hour + 2*time.Hour + 30*time.Minute + 15*time.Second, "T- 1d 02h 30m 15s"},
		{2*time.Hour + 5*time.Minute, "T- 02h 05m 00s"},
		{45 * time.Second, "T- 00m 45s"},
		{-3 * time.Hour, "T+ 03h 00m 00s"},
	}

	for _, tt := range tests {
		got := FormatCountdown(tt.d)
		if got != tt.want {
			t.Errorf("FormatCountdown(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatMET(t *testing.T) {
	got := FormatMET(50 * time.Hour)
	want := "T+ 2d 02h 00m 00s"
	if got != want {
		t.Errorf("FormatMET(50h) = %q, want %q", got, want)
	}
}

func TestCrewBadge(t *testing.T) {
	if CrewBadge(true) == "" {
		t.Error("CrewBadge(true) should return non-empty string")
	}
	if CrewBadge(false) != "" {
		t.Error("CrewBadge(false) should return empty string")
	}
}

func TestCatalogNotEmpty(t *testing.T) {
	c := Catalog()
	if len(c) < 1 {
		t.Fatalf("catalog should have at least 1 profile, got %d", len(c))
	}

	// Verify Voyager 1 profile integrity
	v := c[0]
	if v.ID != "voyager-1" {
		t.Errorf("first profile ID = %q, want voyager-1", v.ID)
	}
	if len(v.Events) != 6 {
		t.Errorf("voyager events = %d, want 6", len(v.Events))
	}
	if v.Crewed {
		t.Error("Voyager 1 should not be crewed")
	}
	if len(v.Crew) != 0 {
		t.Error("Voyager 1 should have empty crew")
	}
}

func TestVoyager1Spotlight(t *testing.T) {
	sc := &dsn.Spacecraft{ID: 31, Name: "VGR1"}
	st := BuildSpotlightState(time.Now(), sc)

	if st == nil {
		t.Fatal("expected non-nil state for VGR1")
	}
	if st.Profile.ID != "voyager-1" {
		t.Errorf("profile = %q, want voyager-1", st.Profile.ID)
	}
	if st.CurrentPhase != "Interstellar" {
		t.Errorf("phase = %q, want Interstellar", st.CurrentPhase)
	}
	if st.IsPreLaunch {
		t.Error("Voyager 1 should not be pre-launch")
	}
	if st.IsComplete {
		t.Error("Voyager 1 should not be complete yet")
	}
	// MET should be ~48 years
	if st.MET < 47*365*24*time.Hour {
		t.Errorf("MET = %v, expected ~48 years", st.MET)
	}
}

func TestNoAliasCrossTalk(t *testing.T) {
	// VGR1 should resolve to voyager-1
	p := ResolveProfile("VGR1")
	if p == nil || p.ID != "voyager-1" {
		t.Error("VGR1 should resolve to voyager-1")
	}

	// VOYAGER (bare) should NOT match — too ambiguous
	p = ResolveProfile("VOYAGER")
	if p != nil {
		t.Errorf("bare VOYAGER should not match, got %q", p.ID)
	}
}
