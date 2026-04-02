package missions

import "time"

var catalog []*MissionProfile

func init() {
	catalog = append(catalog, artemisII())
}

// Catalog returns all registered mission profiles.
func Catalog() []*MissionProfile {
	return catalog
}

func artemisII() *MissionProfile {
	// Artemis II launched April 1, 2026 from KSC LC-39B
	// Crew: Wiseman, Glover, Koch, Hansen — ~10-day lunar flyby
	launch := time.Date(2026, 4, 1, 12, 33, 0, 0, time.UTC)

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
		Aliases:     []string{"EM2", "ORION", "ARTEMIS", "ARTEMIS II", "ARTEMIS2"},
		Crewed:      true,
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
