package dsn

import (
	"testing"
)

// Realistic DSN XML sample based on actual feed format
const realisticXML = `<?xml version="1.0" encoding="UTF-8"?>
<dsn>
  <station name="mdscc" friendlyName="Madrid" timeUTC="1764860575000" timeZoneOffset="3600000">
    <dish name="DSS55" azimuthAngle="213.5" elevationAngle="18.2" windSpeed="10" isMSPA="false" isArray="false" isDDOR="false" activity="Tracking">
      <downSignal active="true" signalType="data" dataRate="241900" frequency="8420000000" band="X" power="-120" spacecraft="EMM" spacecraftID="-62"/>
      <upSignal active="true" signalType="data" dataRate="2000" frequency="7150000000" band="X" power="18" spacecraft="EMM" spacecraftID="-62"/>
      <target name="EMM" id="62" uplegRange="363000000" downlegRange="363000000" rtlt="2420"/>
    </dish>
    <dish name="DSS65" azimuthAngle="45.0" elevationAngle="30.5" windSpeed="5" isMSPA="true" isArray="false" isDDOR="false" activity="Multiple Spacecraft">
      <downSignal active="true" signalType="data" dataRate="160" frequency="8420000000" band="X" power="-155" spacecraft="VGR1" spacecraftID="-31"/>
      <target name="VGR1" id="31" uplegRange="24000000000" downlegRange="24000000000" rtlt="160200"/>
    </dish>
  </station>
  <station name="gdscc" friendlyName="Goldstone" timeUTC="1764860575000" timeZoneOffset="-28800000">
    <dish name="DSS14" azimuthAngle="180.0" elevationAngle="45.0" windSpeed="8" isMSPA="false" isArray="false" isDDOR="false" activity="Science">
      <downSignal active="true" signalType="data" dataRate="2000000" frequency="8420000000" band="X" power="-100" spacecraft="MARS2020" spacecraftID="-168"/>
      <target name="MARS2020" id="168" uplegRange="225000000" downlegRange="225000000" rtlt="1500"/>
    </dish>
  </station>
  <timestamp>1764860575000</timestamp>
</dsn>`

func TestParse_RealisticXML(t *testing.T) {
	data, err := Parse([]byte(realisticXML))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have 2 stations
	if len(data.Stations) != 2 {
		t.Errorf("expected 2 stations, got %d", len(data.Stations))
	}

	// Should have 3 links total
	if len(data.Links) != 3 {
		t.Errorf("expected 3 links, got %d", len(data.Links))
	}

	// Verify Madrid station
	var madrid *Station
	for i := range data.Stations {
		if data.Stations[i].FriendlyName == "Madrid" {
			madrid = &data.Stations[i]
			break
		}
	}
	if madrid == nil {
		t.Fatal("Madrid station not found")
	}
	if madrid.Complex != ComplexMadrid {
		t.Errorf("Madrid complex = %q, want mdscc", madrid.Complex)
	}
	if len(madrid.Antennas) != 2 {
		t.Errorf("Madrid antennas = %d, want 2", len(madrid.Antennas))
	}

	// Verify DSS55 antenna details
	var dss55 *Antenna
	for i := range madrid.Antennas {
		if madrid.Antennas[i].ID == "DSS55" {
			dss55 = &madrid.Antennas[i]
			break
		}
	}
	if dss55 == nil {
		t.Fatal("DSS55 antenna not found")
	}
	if dss55.Azimuth != 213.5 {
		t.Errorf("DSS55 azimuth = %v, want 213.5", dss55.Azimuth)
	}
	if dss55.Activity != "Tracking" {
		t.Errorf("DSS55 activity = %q, want Tracking", dss55.Activity)
	}
}

func TestParse_LinkExtraction(t *testing.T) {
	data, err := Parse([]byte(realisticXML))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Find EMM link
	var emmLink *Link
	for i := range data.Links {
		if data.Links[i].Spacecraft == "EMM" {
			emmLink = &data.Links[i]
			break
		}
	}
	if emmLink == nil {
		t.Fatal("EMM link not found")
	}

	if emmLink.Band != "X" {
		t.Errorf("EMM band = %q, want X", emmLink.Band)
	}
	if emmLink.DownRate != 241900 {
		t.Errorf("EMM down rate = %v, want 241900", emmLink.DownRate)
	}
	if emmLink.UpRate != 2000 {
		t.Errorf("EMM up rate = %v, want 2000", emmLink.UpRate)
	}
	if emmLink.RTLT != 2420 {
		t.Errorf("EMM RTLT = %v, want 2420", emmLink.RTLT)
	}
	if emmLink.AntennaID != "DSS55" {
		t.Errorf("EMM antenna = %q, want DSS55", emmLink.AntennaID)
	}

	// Distance should be calculated from RTLT
	expectedDist := DistanceFromRTLT(2420)
	if emmLink.Distance != expectedDist {
		t.Errorf("EMM distance = %v, want %v", emmLink.Distance, expectedDist)
	}
}

func TestParse_VoyagerLink(t *testing.T) {
	data, err := Parse([]byte(realisticXML))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var vgrLink *Link
	for i := range data.Links {
		if data.Links[i].Spacecraft == "VGR1" {
			vgrLink = &data.Links[i]
			break
		}
	}
	if vgrLink == nil {
		t.Fatal("VGR1 link not found")
	}

	// Voyager has very high RTLT (~44 hours)
	if vgrLink.RTLT != 160200 {
		t.Errorf("VGR1 RTLT = %v, want 160200", vgrLink.RTLT)
	}
	// Low data rate
	if vgrLink.DownRate != 160 {
		t.Errorf("VGR1 down rate = %v, want 160", vgrLink.DownRate)
	}
}

func TestInferComplex(t *testing.T) {
	tests := []struct {
		name     string
		expected Complex
	}{
		{"DSS-14", ComplexGoldstone},
		{"DSS14", ComplexGoldstone},
		{"DSS-24", ComplexGoldstone},
		{"DSS24", ComplexGoldstone},
		{"DSS-34", ComplexCanberra},
		{"DSS34", ComplexCanberra},
		{"DSS-43", ComplexCanberra},
		{"DSS43", ComplexCanberra},
		{"DSS-54", ComplexMadrid},
		{"DSS54", ComplexMadrid},
		{"DSS-55", ComplexMadrid},
		{"DSS55", ComplexMadrid},
		{"DSS-63", ComplexMadrid},
		{"DSS65", ComplexMadrid},
		{"gdscc", ComplexGoldstone},
		{"cdscc", ComplexCanberra},
		{"mdscc", ComplexMadrid},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferComplex(tt.name)
			if got != tt.expected {
				t.Errorf("inferComplex(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestParse_EmptyData(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?><dsn><timestamp>0</timestamp></dsn>`)

	data, err := Parse(xmlData)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(data.Stations) != 0 {
		t.Errorf("expected 0 stations, got %d", len(data.Stations))
	}
	if len(data.Links) != 0 {
		t.Errorf("expected 0 links, got %d", len(data.Links))
	}
}

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse([]byte(`not valid xml`))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestParse_TimestampMillis(t *testing.T) {
	data, err := Parse([]byte(realisticXML))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Timestamp should be parsed from milliseconds
	if data.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	// 1764860575000 ms = Dec 2025 ish
	if data.Timestamp.Year() < 2025 {
		t.Errorf("Timestamp year = %d, expected >= 2025", data.Timestamp.Year())
	}
}
