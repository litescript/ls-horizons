package dsn

import (
	"testing"
)

func TestBuildSpacecraftViews_Grouping(t *testing.T) {
	data := &DSNData{
		Links: []Link{
			{SpacecraftID: 32, Spacecraft: "VGR2", AntennaID: "DSS34", Complex: ComplexCanberra, Band: "X"},
			{SpacecraftID: 32, Spacecraft: "VGR2", AntennaID: "DSS36", Complex: ComplexCanberra, Band: "X"},
			{SpacecraftID: 31, Spacecraft: "VGR1", AntennaID: "DSS63", Complex: ComplexMadrid, Band: "S"},
		},
		Stations: []Station{
			{
				Complex: ComplexCanberra,
				Antennas: []Antenna{
					{ID: "DSS34", Azimuth: 242, Elevation: 20},
					{ID: "DSS36", Azimuth: 243, Elevation: 18},
				},
			},
			{
				Complex: ComplexMadrid,
				Antennas: []Antenna{
					{ID: "DSS63", Azimuth: 180, Elevation: 45},
				},
			},
		},
	}

	elevMap := BuildElevationMap(data)
	views := BuildSpacecraftViews(data, elevMap)

	// Should have 2 spacecraft (VGR1 and VGR2)
	if len(views) != 2 {
		t.Fatalf("expected 2 spacecraft, got %d", len(views))
	}

	// Find VGR2
	var vgr2 *SpacecraftView
	for i := range views {
		if views[i].Code == "VGR2" {
			vgr2 = &views[i]
			break
		}
	}

	if vgr2 == nil {
		t.Fatal("VGR2 not found in views")
	}

	// VGR2 should have 2 links
	if len(vgr2.Links) != 2 {
		t.Errorf("VGR2 expected 2 links, got %d", len(vgr2.Links))
	}

	// Should be grouped under one SpacecraftView
	if vgr2.Name != "Voyager 2" {
		t.Errorf("VGR2 name = %q, want %q", vgr2.Name, "Voyager 2")
	}
}

func TestBuildSpacecraftViews_PrimaryLinkSelection(t *testing.T) {
	tests := []struct {
		name        string
		links       []Link
		antennas    []Antenna
		wantPrimary string // expected primary station
	}{
		{
			name: "highest elevation wins",
			links: []Link{
				{SpacecraftID: 1, Spacecraft: "TEST", AntennaID: "DSS34", Band: "X"},
				{SpacecraftID: 1, Spacecraft: "TEST", AntennaID: "DSS36", Band: "X"},
			},
			antennas: []Antenna{
				{ID: "DSS34", Elevation: 20},
				{ID: "DSS36", Elevation: 45}, // higher elevation
			},
			wantPrimary: "DSS36",
		},
		{
			name: "equal elevation - lower struggle wins",
			links: []Link{
				{SpacecraftID: 1, Spacecraft: "TEST", AntennaID: "DSS34", Band: "X", DataRate: 1000}, // higher rate = lower struggle
				{SpacecraftID: 1, Spacecraft: "TEST", AntennaID: "DSS36", Band: "X", DataRate: 100},
			},
			antennas: []Antenna{
				{ID: "DSS34", Elevation: 30},
				{ID: "DSS36", Elevation: 30}, // same elevation
			},
			wantPrimary: "DSS34", // higher data rate = lower struggle
		},
		{
			name: "all equal - lowest station ID wins",
			links: []Link{
				{SpacecraftID: 1, Spacecraft: "TEST", AntennaID: "DSS36", Band: "X", DataRate: 1000},
				{SpacecraftID: 1, Spacecraft: "TEST", AntennaID: "DSS34", Band: "X", DataRate: 1000},
			},
			antennas: []Antenna{
				{ID: "DSS34", Elevation: 30},
				{ID: "DSS36", Elevation: 30},
			},
			wantPrimary: "DSS34", // lower station ID
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &DSNData{
				Links: tt.links,
				Stations: []Station{
					{Antennas: tt.antennas},
				},
			}

			elevMap := BuildElevationMap(data)
			views := BuildSpacecraftViews(data, elevMap)

			if len(views) != 1 {
				t.Fatalf("expected 1 spacecraft, got %d", len(views))
			}

			if views[0].PrimaryLink.Station != tt.wantPrimary {
				t.Errorf("primary station = %q, want %q", views[0].PrimaryLink.Station, tt.wantPrimary)
			}
		})
	}
}

func TestBuildSpacecraftViews_FiltersInternal(t *testing.T) {
	data := &DSNData{
		Links: []Link{
			{SpacecraftID: 1, Spacecraft: "DSN", AntennaID: "DSS34"},
			{SpacecraftID: 2, Spacecraft: "DSS", AntennaID: "DSS36"},
			{SpacecraftID: 3, Spacecraft: "JWST", AntennaID: "DSS55"},
		},
		Stations: []Station{
			{Antennas: []Antenna{
				{ID: "DSS34", Elevation: 30},
				{ID: "DSS36", Elevation: 30},
				{ID: "DSS55", Elevation: 45},
			}},
		},
	}

	elevMap := BuildElevationMap(data)
	views := BuildSpacecraftViews(data, elevMap)

	// Should only have JWST (DSN and DSS filtered out)
	if len(views) != 1 {
		t.Fatalf("expected 1 spacecraft, got %d", len(views))
	}

	if views[0].Code != "JWST" {
		t.Errorf("expected JWST, got %q", views[0].Code)
	}
}

func TestBuildSpacecraftViews_FiltersBelowHorizon(t *testing.T) {
	data := &DSNData{
		Links: []Link{
			{SpacecraftID: 1, Spacecraft: "VGR1", AntennaID: "DSS34"},
			{SpacecraftID: 2, Spacecraft: "VGR2", AntennaID: "DSS36"},
		},
		Stations: []Station{
			{Antennas: []Antenna{
				{ID: "DSS34", Elevation: 30},  // above horizon
				{ID: "DSS36", Elevation: -10}, // below horizon
			}},
		},
	}

	elevMap := BuildElevationMap(data)
	views := BuildSpacecraftViews(data, elevMap)

	// Should only have VGR1 (VGR2 below horizon)
	if len(views) != 1 {
		t.Fatalf("expected 1 spacecraft, got %d", len(views))
	}

	if views[0].Code != "VGR1" {
		t.Errorf("expected VGR1, got %q", views[0].Code)
	}
}

func TestSpacecraftView_AntennaList(t *testing.T) {
	tests := []struct {
		name  string
		links []LinkView
		want  string
	}{
		{
			name:  "empty",
			links: nil,
			want:  "",
		},
		{
			name:  "single antenna",
			links: []LinkView{{Station: "DSS63"}},
			want:  "DSS63",
		},
		{
			name: "two antennas sorted",
			links: []LinkView{
				{Station: "DSS36"},
				{Station: "DSS34"},
			},
			want: "DSS34+DSS36",
		},
		{
			name: "three antennas sorted",
			links: []LinkView{
				{Station: "DSS45"},
				{Station: "DSS34"},
				{Station: "DSS36"},
			},
			want: "DSS34+DSS36+DSS45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := SpacecraftView{Links: tt.links}
			got := sv.AntennaList()
			if got != tt.want {
				t.Errorf("AntennaList() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSpacecraftView_IsArrayed(t *testing.T) {
	tests := []struct {
		name  string
		links []LinkView
		want  bool
	}{
		{"no links", nil, false},
		{"one link", []LinkView{{Station: "DSS34"}}, false},
		{"two links", []LinkView{{Station: "DSS34"}, {Station: "DSS36"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := SpacecraftView{Links: tt.links}
			if got := sv.IsArrayed(); got != tt.want {
				t.Errorf("IsArrayed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpacecraftView_Coord(t *testing.T) {
	sv := SpacecraftView{
		PrimaryLink: LinkView{
			AzDeg:      242.5,
			ElDeg:      20.3,
			DistanceKm: 21300000000,
		},
	}

	coord := sv.Coord()

	if coord.AzDeg != 242.5 {
		t.Errorf("AzDeg = %v, want 242.5", coord.AzDeg)
	}
	if coord.ElDeg != 20.3 {
		t.Errorf("ElDeg = %v, want 20.3", coord.ElDeg)
	}
	if coord.RangeKm != 21300000000 {
		t.Errorf("RangeKm = %v, want 21300000000", coord.RangeKm)
	}
}

func TestBuildElevationMap(t *testing.T) {
	data := &DSNData{
		Stations: []Station{
			{
				Antennas: []Antenna{
					{ID: "DSS34", Elevation: 20.5},
					{ID: "DSS36", Elevation: 18.2},
				},
			},
			{
				Antennas: []Antenna{
					{ID: "DSS63", Elevation: 45.0},
				},
			},
		},
	}

	elevMap := BuildElevationMap(data)

	if elevMap["DSS34"] != 20.5 {
		t.Errorf("DSS34 elevation = %v, want 20.5", elevMap["DSS34"])
	}
	if elevMap["DSS36"] != 18.2 {
		t.Errorf("DSS36 elevation = %v, want 18.2", elevMap["DSS36"])
	}
	if elevMap["DSS63"] != 45.0 {
		t.Errorf("DSS63 elevation = %v, want 45.0", elevMap["DSS63"])
	}
}

func TestBuildElevationMap_Nil(t *testing.T) {
	elevMap := BuildElevationMap(nil)
	if elevMap == nil {
		t.Error("expected non-nil map for nil input")
	}
	if len(elevMap) != 0 {
		t.Errorf("expected empty map, got %d entries", len(elevMap))
	}
}
