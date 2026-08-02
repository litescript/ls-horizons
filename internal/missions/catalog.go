// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package missions

import "time"

var catalog []*MissionProfile

func init() {
	catalog = append(catalog, voyager1())
}

// Catalog returns all registered mission profiles.
func Catalog() []*MissionProfile {
	return catalog
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
