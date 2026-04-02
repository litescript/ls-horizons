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
		{"EM2", "artemis-ii"},
		{"em2", "artemis-ii"},
		{"  EM2  ", "artemis-ii"},
		{"ORION", "artemis-ii"},
		{"ARTEMIS", "artemis-ii"},
		{"ARTEMIS II", "artemis-ii"},
		{"ARTEMIS2", "artemis-ii"},
		{"JWST", ""},
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
	if !HasSpotlight("EM2") {
		t.Error("HasSpotlight(EM2) should be true")
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

func TestBuildSpotlightState_PreLaunch(t *testing.T) {
	profile := Catalog()[0] // Artemis II
	preLaunch := profile.StartTime.Add(-24 * time.Hour)

	sc := &dsn.Spacecraft{ID: 1, Name: "EM2"}
	st := BuildSpotlightState(preLaunch, sc)

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
	if st.NextEvent == nil {
		t.Fatal("expected NextEvent")
	}
	if st.NextEvent.Key != "LAUNCH" {
		t.Errorf("next event = %q, want LAUNCH", st.NextEvent.Key)
	}
	if st.Countdown <= 0 {
		t.Errorf("countdown should be positive, got %v", st.Countdown)
	}
	if st.MET != 0 {
		t.Errorf("MET should be 0 pre-launch, got %v", st.MET)
	}
}

func TestBuildSpotlightState_MidMission(t *testing.T) {
	profile := Catalog()[0]
	// 50 hours after launch: should be in "Outbound Transit" (2h..84h)
	midFlight := profile.StartTime.Add(50 * time.Hour)

	sc := &dsn.Spacecraft{ID: 1, Name: "EM2"}
	st := BuildSpotlightState(midFlight, sc)

	if st == nil {
		t.Fatal("expected non-nil state")
	}
	if st.IsPreLaunch {
		t.Error("expected IsPreLaunch=false")
	}
	if st.IsComplete {
		t.Error("expected IsComplete=false")
	}
	if st.CurrentPhase != "Outbound Transit" {
		t.Errorf("phase = %q, want Outbound Transit", st.CurrentPhase)
	}
	// Next event should be "Lunar Flyby" (at 96h)
	if st.NextEvent == nil {
		t.Fatal("expected NextEvent")
	}
	if st.NextEvent.Key != "FLYBY" {
		t.Errorf("next event = %q, want FLYBY", st.NextEvent.Key)
	}
	// MET should be ~50h
	if st.MET < 49*time.Hour || st.MET > 51*time.Hour {
		t.Errorf("MET = %v, want ~50h", st.MET)
	}
}

func TestBuildSpotlightState_Complete(t *testing.T) {
	profile := Catalog()[0]
	afterEnd := profile.EndTime.Add(24 * time.Hour)

	sc := &dsn.Spacecraft{ID: 1, Name: "EM2"}
	st := BuildSpotlightState(afterEnd, sc)

	if st == nil {
		t.Fatal("expected non-nil state")
	}
	if st.IsPreLaunch {
		t.Error("expected IsPreLaunch=false")
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
	profile := Catalog()[0]
	// 50h in: LAUNCH, SEP, TLI should be past; TLI is current; FLYBY, ENTRY, SPLASH future
	midFlight := profile.StartTime.Add(50 * time.Hour)

	sc := &dsn.Spacecraft{ID: 1, Name: "EM2"}
	st := BuildSpotlightState(midFlight, sc)

	if len(st.Timeline) != 6 {
		t.Fatalf("timeline items = %d, want 6", len(st.Timeline))
	}

	// LAUNCH (0h) = past, SEP (10m) = past, TLI (2h) = current (most recent past)
	if st.Timeline[0].Status != TimelinePast {
		t.Errorf("LAUNCH status = %d, want Past", st.Timeline[0].Status)
	}
	if st.Timeline[1].Status != TimelinePast {
		t.Errorf("SEP status = %d, want Past", st.Timeline[1].Status)
	}
	if st.Timeline[2].Status != TimelineCurrent {
		t.Errorf("TLI status = %d, want Current", st.Timeline[2].Status)
	}
	// FLYBY (96h), ENTRY (192h), SPLASH (192.5h) = future
	if st.Timeline[3].Status != TimelineFuture {
		t.Errorf("FLYBY status = %d, want Future", st.Timeline[3].Status)
	}
	if st.Timeline[4].Status != TimelineFuture {
		t.Errorf("ENTRY status = %d, want Future", st.Timeline[4].Status)
	}
	if st.Timeline[5].Status != TimelineFuture {
		t.Errorf("SPLASH status = %d, want Future", st.Timeline[5].Status)
	}
}

func TestTimelineAllPast_AfterComplete(t *testing.T) {
	profile := Catalog()[0]
	afterEnd := profile.EndTime.Add(24 * time.Hour)

	sc := &dsn.Spacecraft{ID: 1, Name: "EM2"}
	st := BuildSpotlightState(afterEnd, sc)

	// After mission end, all items should be Past (none Current)
	for _, item := range st.Timeline {
		if item.Status != TimelinePast {
			t.Errorf("item %q status = %d, want Past after mission complete", item.Key, item.Status)
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
	if len(c) == 0 {
		t.Error("catalog should not be empty")
	}
	// Verify Artemis II profile integrity
	p := c[0]
	if p.ID != "artemis-ii" {
		t.Errorf("first profile ID = %q, want artemis-ii", p.ID)
	}
	if len(p.Events) != 6 {
		t.Errorf("events = %d, want 6", len(p.Events))
	}
	if len(p.Phases) != 7 {
		t.Errorf("phases = %d, want 7", len(p.Phases))
	}
	if !p.Crewed {
		t.Error("Artemis II should be crewed")
	}
	if p.PrimaryBody != "Moon" {
		t.Errorf("primary body = %q, want Moon", p.PrimaryBody)
	}
}
