package missions

import "time"

var catalog []*MissionProfile

func init() {
	catalog = append(catalog, artemisII(), voyager1())
}

// Catalog returns all registered mission profiles.
func Catalog() []*MissionProfile {
	return catalog
}

func mustParseRFC3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("missions: bad RFC3339 timestamp: " + s)
	}
	return t
}

func artemisII() *MissionProfile {
	// Artemis II launched April 1, 2026 at 22:35 UTC from KSC LC-39B
	// Crew: Wiseman, Glover, Koch, Hansen — ~10-day lunar flyby
	launch := mustParseRFC3339("2026-04-01T22:35:00Z")

	events := []MissionEvent{
		{Name: "Launch", Time: launch, Key: "LAUNCH"},
		{Name: "Upper Stage Sep", Time: launch.Add(8 * time.Minute), Key: "SEP"},
		{Name: "Trans-Lunar Injection", Time: launch.Add(1*time.Hour + 55*time.Minute), Key: "TLI"},
		{Name: "Lunar Flyby", Time: launch.Add(4*24*time.Hour + 8*time.Hour), Key: "FLYBY"},
		{Name: "Entry Interface", Time: launch.Add(10*24*time.Hour + 4*time.Hour), Key: "ENTRY"},
		{Name: "Splashdown", Time: launch.Add(10*24*time.Hour + 4*time.Hour + 30*time.Minute), Key: "SPLASH"},
	}

	phases := []MissionPhase{
		{Name: "Pre-Launch", Start: launch.Add(-8760 * time.Hour), End: launch},
		{Name: "Launch & Ascent", Start: launch, End: launch.Add(1*time.Hour + 55*time.Minute)},
		{Name: "Outbound Transit", Start: launch.Add(1*time.Hour + 55*time.Minute), End: launch.Add(4*24*time.Hour - 12*time.Hour)},
		{Name: "Lunar Flyby", Start: launch.Add(4*24*time.Hour - 12*time.Hour), End: launch.Add(4*24*time.Hour + 20*time.Hour)},
		{Name: "Return Transit", Start: launch.Add(4*24*time.Hour + 20*time.Hour), End: launch.Add(10*24*time.Hour + 4*time.Hour)},
		{Name: "Entry & Recovery", Start: launch.Add(10*24*time.Hour + 4*time.Hour), End: launch.Add(10*24*time.Hour + 6*time.Hour)},
		{Name: "Mission Complete", Start: launch.Add(10*24*time.Hour + 6*time.Hour), End: launch.Add(8760 * time.Hour)},
	}

	return &MissionProfile{
		ID:          "artemis-ii",
		DisplayName: "ARTEMIS II",
		Subtitle:    "First Crewed Lunar Flyby \u00b7 Orion MPCV",
		HeroText:    "Crewed lunar flyby \u2014 validating Orion life support and deep-space navigation",
		Aliases:     []string{"EM2", "ORION", "ARTEMIS II", "ARTEMIS2"},
		Crewed:      true,
		Crew:        []string{"Wiseman", "Glover", "Koch", "Hansen"},
		PrimaryBody: "Moon",
		Accent: MissionAccent{
			Primary:   "#e06020", // NASA orange
			Secondary: "#4488cc", // Deep space blue
			Dim:       "#705030", // Muted orange
		},
		StartTime: launch,
		EndTime:   launch.Add(10*24*time.Hour + 6*time.Hour),
		Events:    events,
		Phases:    phases,
	}
}

func voyager1() *MissionProfile {
	// Voyager 1: launched Sept 5, 1977 — now in interstellar space
	launch := time.Date(1977, 9, 5, 12, 56, 0, 0, time.UTC)

	events := []MissionEvent{
		{Name: "Launch", Time: launch, Key: "LAUNCH"},
		{Name: "Jupiter Flyby", Time: time.Date(1979, 3, 5, 12, 5, 0, 0, time.UTC), Key: "JUPITER"},
		{Name: "Saturn Flyby", Time: time.Date(1980, 11, 12, 23, 46, 0, 0, time.UTC), Key: "SATURN"},
		{Name: "Pale Blue Dot", Time: time.Date(1990, 2, 14, 0, 0, 0, 0, time.UTC), Key: "PBD"},
		{Name: "Termination Shock", Time: time.Date(2004, 12, 16, 0, 0, 0, 0, time.UTC), Key: "TSHOCK"},
		{Name: "Heliopause", Time: time.Date(2012, 8, 25, 0, 0, 0, 0, time.UTC), Key: "HPAUSE"},
	}

	phases := []MissionPhase{
		{Name: "Launch & Cruise", Start: launch, End: time.Date(1979, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "Grand Tour", Start: time.Date(1979, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(1981, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "Extended Mission", Start: time.Date(1981, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2004, 12, 16, 0, 0, 0, 0, time.UTC)},
		{Name: "Heliosheath", Start: time.Date(2004, 12, 16, 0, 0, 0, 0, time.UTC), End: time.Date(2012, 8, 25, 0, 0, 0, 0, time.UTC)},
		{Name: "Interstellar", Start: time.Date(2012, 8, 25, 0, 0, 0, 0, time.UTC), End: time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	return &MissionProfile{
		ID:          "voyager-1",
		DisplayName: "VOYAGER 1",
		Subtitle:    "Interstellar Probe \u00b7 The Farthest Human-Made Object",
		HeroText:    "First spacecraft to reach interstellar space \u2014 still returning data after 48 years",
		Aliases:     []string{"VGR1", "VOYAGER 1", "VOYAGER1"},
		Crewed:      false,
		PrimaryBody: "Interstellar",
		Accent: MissionAccent{
			Primary:   "#c8a832", // Gold
			Secondary: "#6a8cbc", // Pale blue (Pale Blue Dot)
			Dim:       "#7a6a30", // Muted gold
		},
		StartTime: launch,
		EndTime:   time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
		Events:    events,
		Phases:    phases,
	}
}
