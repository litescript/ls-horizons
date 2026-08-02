package astro

import (
	"math"
	"testing"
	"time"
)

// Reference positions from JPL Horizons at JD 2461041.5 (2026-01-01 00:00),
// heliocentric ecliptic J2000, in AU, for each planet's system barycenter.
// These are fixed fixtures, not live lookups -- they pin the math, not the date.
var horizonsReference = []struct {
	name    string
	x, y, z float64
	// tolAU is the accepted deviation, scaled to the element table's published
	// accuracy at that body's distance (roughly arcminutes of angle).
	tolAU float64
}{
	{"Mercury", -2.152013043776677e-01, -4.092076157002721e-01, -1.370326959192366e-02, 0.001},
	{"Venus", 8.887724961635655e-02, -7.217623823170807e-01, -1.504405657506193e-02, 0.001},
	{"Earth", -1.742697585483051e-01, 9.677856743241913e-01, -5.686608136245510e-05, 0.001},
	{"Mars", 3.405796768620874e-01, -1.387002015945923e+00, -3.741722678768196e-02, 0.002},
	{"Jupiter", -1.694003030185846e+00, 4.928882714636233e+00, 1.742623990332420e-02, 0.010},
	{"Saturn", 9.507343365221871e+00, 2.577384578021522e-01, -3.829355285077531e-01, 0.020},
	{"Uranus", 9.880317975194275e+00, 1.680001540981795e+01, -6.571979913731432e-02, 0.020},
	{"Neptune", 2.987212004088898e+01, 5.189394632411259e-01, -6.990671752036290e-01, 0.020},
}

func TestHeliocentricPositionMatchesHorizons(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, ref := range horizonsReference {
		t.Run(ref.name, func(t *testing.T) {
			el, ok := PlanetElementsByName(ref.name)
			if !ok {
				t.Fatalf("no elements for %s", ref.name)
			}

			got := el.HeliocentricPosition(epoch)
			want := Vec3{X: ref.x, Y: ref.y, Z: ref.z}
			deviation := got.Sub(want).Norm()

			// Report the deviation as a fraction of heliocentric distance, which
			// is the meaningful accuracy measure for a rendered scene.
			angularArcmin := (deviation / want.Norm()) * (180 / math.Pi) * 60

			if deviation > ref.tolAU {
				t.Errorf("%s off by %.6f AU (tolerance %.6f AU, ~%.2f arcmin)",
					ref.name, deviation, ref.tolAU, angularArcmin)
			} else {
				t.Logf("%s within %.6f AU (~%.2f arcmin)", ref.name, deviation, angularArcmin)
			}
		})
	}
}

// TestHeliocentricDistancesAreSane guards against a rotation or unit error that
// happens to land near the reference epoch but breaks elsewhere in the orbit.
func TestHeliocentricDistancesAreSane(t *testing.T) {
	expectedAU := map[string][2]float64{
		"Mercury": {0.30, 0.47},
		"Venus":   {0.71, 0.73},
		"Earth":   {0.98, 1.02},
		"Mars":    {1.38, 1.67},
		"Jupiter": {4.95, 5.46},
		"Saturn":  {9.02, 10.07},
		"Uranus":  {18.28, 20.10},
		"Neptune": {29.80, 30.33},
	}

	// Sample across a full decade so every planet is seen at many orbital phases.
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, el := range PlanetElementTable {
		bounds := expectedAU[el.Name]
		for day := 0; day < 3650; day += 7 {
			at := start.AddDate(0, 0, day)
			r := el.HeliocentricPosition(at).Norm()
			if r < bounds[0] || r > bounds[1] {
				t.Fatalf("%s at %s: distance %.4f AU outside [%.2f, %.2f]",
					el.Name, at.Format("2006-01-02"), r, bounds[0], bounds[1])
			}
		}
	}
}

func TestSolveKeplerConverges(t *testing.T) {
	// Verify the solver inverts the equation across the eccentricity range that
	// matters here, including well past Mercury's 0.21.
	for _, e := range []float64{0, 0.01, 0.1, 0.21, 0.5, 0.9} {
		for deg := -180; deg <= 180; deg += 5 {
			meanAnom := degToRad(float64(deg))
			eccAnom := solveKepler(meanAnom, e)
			residual := eccAnom - e*math.Sin(eccAnom) - meanAnom
			if math.Abs(residual) > 1e-9 {
				t.Fatalf("e=%.2f M=%d deg: residual %.3e", e, deg, residual)
			}
		}
	}
}

func TestWrapDegrees(t *testing.T) {
	cases := map[float64]float64{
		0: 0, 180: -180, -180: -180, 190: -170, 360: 0, 540: -180, -370: -10,
	}
	for in, want := range cases {
		if got := wrapDegrees(in); math.Abs(got-want) > 1e-9 {
			t.Errorf("wrapDegrees(%.0f) = %.6f, want %.0f", in, got, want)
		}
	}
}
