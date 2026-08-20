// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package astro

// Star represents a cataloged star with position, brightness, and color.
type Star struct {
	Name   string  // Common name (e.g., "Sirius", "Vega"); empty if unnamed
	RAdeg  float64 // Right Ascension in degrees (J2000)
	DecDeg float64 // Declination in degrees (J2000)
	Mag    float64 // Apparent visual magnitude (lower = brighter)

	// CatalogID is a stable identifier in the source catalog. Empty for
	// hand-maintained entries with no upstream record to point at.
	CatalogID string

	// BV is the B-V color index, nil where the catalog has no photometry.
	// A pointer rather than a sentinel: B-V is legitimately near zero for
	// A0 stars, so no numeric value can stand in for "unknown".
	BV *float64

	// SpectralType is the MK classification where the catalog gives one.
	SpectralType string
}

// StarCatalog holds a collection of stars for rendering, along with the
// provenance and selection criteria a consumer needs to interpret it.
type StarCatalog struct {
	Stars []Star

	// MagnitudeLimit is the faintest apparent visual magnitude the catalog
	// was selected down to, describing the selection rather than whichever
	// star happens to be faintest.
	MagnitudeLimit float64

	// Source records where the data came from, so provenance travels with
	// the catalog into every export rather than living only in a comment.
	Source StarSource
}

// handMaintainedMagnitudeLimit is the selection limit of the hand-maintained
// table below. The original header claimed mag < 4.0; entries down to 4.67
// were added later without the comment following, so this states what the
// table actually contains.
const handMaintainedMagnitudeLimit = 4.7

// DefaultStarCatalog returns the built-in catalog of naked-eye stars.
// Coordinates are J2000 epoch.
func DefaultStarCatalog() StarCatalog {
	return StarCatalog{
		Stars:          defaultStars,
		MagnitudeLimit: handMaintainedMagnitudeLimit,
		Source: StarSource{
			Catalog:   "ls-horizons hand-maintained bright star table",
			Reference: "Compiled from Yale Bright Star Catalogue and IAU star names",
		},
	}
}

// defaultStars contains bright stars visible from various latitudes.
// Ordered roughly by magnitude (brightest first).
var defaultStars = []Star{
	// Magnitude < 0 (exceptionally bright)
	{Name: "Sirius", RAdeg: 101.287, DecDeg: -16.716, Mag: -1.46},
	{Name: "Canopus", RAdeg: 95.988, DecDeg: -52.696, Mag: -0.74},
	{Name: "Arcturus", RAdeg: 213.915, DecDeg: 19.182, Mag: -0.05},
	{Name: "Vega", RAdeg: 279.235, DecDeg: 38.784, Mag: 0.03},
	{Name: "Capella", RAdeg: 79.172, DecDeg: 45.998, Mag: 0.08},
	{Name: "Rigel", RAdeg: 78.634, DecDeg: -8.202, Mag: 0.13},
	{Name: "Procyon", RAdeg: 114.826, DecDeg: 5.225, Mag: 0.34},
	{Name: "Achernar", RAdeg: 24.429, DecDeg: -57.237, Mag: 0.46},
	{Name: "Betelgeuse", RAdeg: 88.793, DecDeg: 7.407, Mag: 0.50},
	{Name: "Hadar", RAdeg: 210.956, DecDeg: -60.373, Mag: 0.61},

	// Magnitude 0.5-1.0
	{Name: "Altair", RAdeg: 297.696, DecDeg: 8.868, Mag: 0.76},
	{Name: "Acrux", RAdeg: 186.650, DecDeg: -63.099, Mag: 0.76},
	{Name: "Aldebaran", RAdeg: 68.980, DecDeg: 16.509, Mag: 0.85},
	{Name: "Antares", RAdeg: 247.352, DecDeg: -26.432, Mag: 0.96},
	{Name: "Spica", RAdeg: 201.298, DecDeg: -11.161, Mag: 0.97},
	{Name: "Pollux", RAdeg: 116.329, DecDeg: 28.026, Mag: 1.14},

	// Magnitude 1.0-1.5
	{Name: "Fomalhaut", RAdeg: 344.413, DecDeg: -29.622, Mag: 1.16},
	{Name: "Deneb", RAdeg: 310.358, DecDeg: 45.280, Mag: 1.25},
	{Name: "Mimosa", RAdeg: 191.930, DecDeg: -59.689, Mag: 1.25},
	{Name: "Regulus", RAdeg: 152.093, DecDeg: 11.967, Mag: 1.35},
	{Name: "Adhara", RAdeg: 104.656, DecDeg: -28.972, Mag: 1.50},
	{Name: "Castor", RAdeg: 113.650, DecDeg: 31.889, Mag: 1.58},

	// Magnitude 1.5-2.0
	{Name: "Gacrux", RAdeg: 187.791, DecDeg: -57.113, Mag: 1.63},
	{Name: "Shaula", RAdeg: 263.402, DecDeg: -37.104, Mag: 1.63},
	{Name: "Bellatrix", RAdeg: 81.283, DecDeg: 6.350, Mag: 1.64},
	{Name: "Elnath", RAdeg: 81.573, DecDeg: 28.608, Mag: 1.65},
	{Name: "Miaplacidus", RAdeg: 138.300, DecDeg: -69.717, Mag: 1.68},
	{Name: "Alnilam", RAdeg: 84.053, DecDeg: -1.202, Mag: 1.69},
	{Name: "Alnair", RAdeg: 332.058, DecDeg: -46.961, Mag: 1.74},
	{Name: "Alnitak", RAdeg: 85.190, DecDeg: -1.943, Mag: 1.77},
	{Name: "Alioth", RAdeg: 193.507, DecDeg: 55.960, Mag: 1.77},
	{Name: "Dubhe", RAdeg: 165.932, DecDeg: 61.751, Mag: 1.79},
	{Name: "Mirfak", RAdeg: 51.081, DecDeg: 49.861, Mag: 1.79},
	{Name: "Wezen", RAdeg: 107.098, DecDeg: -26.393, Mag: 1.84},
	{Name: "Sargas", RAdeg: 264.330, DecDeg: -42.998, Mag: 1.87},
	{Name: "Kaus Australis", RAdeg: 276.043, DecDeg: -34.384, Mag: 1.85},
	{Name: "Avior", RAdeg: 125.629, DecDeg: -59.509, Mag: 1.86},
	{Name: "Alkaid", RAdeg: 206.885, DecDeg: 49.313, Mag: 1.86},
	{Name: "Menkalinan", RAdeg: 89.882, DecDeg: 44.948, Mag: 1.90},
	{Name: "Atria", RAdeg: 252.166, DecDeg: -69.028, Mag: 1.92},
	{Name: "Alhena", RAdeg: 99.428, DecDeg: 16.399, Mag: 1.93},
	{Name: "Peacock", RAdeg: 306.412, DecDeg: -56.735, Mag: 1.94},
	{Name: "Alsephina", RAdeg: 131.176, DecDeg: -54.709, Mag: 1.96},
	{Name: "Mirzam", RAdeg: 95.675, DecDeg: -17.956, Mag: 1.98},
	{Name: "Polaris", RAdeg: 37.954, DecDeg: 89.264, Mag: 2.02},
	{Name: "Alphard", RAdeg: 141.897, DecDeg: -8.659, Mag: 2.00},

	// Magnitude 2.0-2.5
	{Name: "Hamal", RAdeg: 31.793, DecDeg: 23.463, Mag: 2.00},
	{Name: "Algieba", RAdeg: 146.463, DecDeg: 19.842, Mag: 2.08},
	{Name: "Diphda", RAdeg: 10.897, DecDeg: -17.987, Mag: 2.02},
	{Name: "Nunki", RAdeg: 283.816, DecDeg: -26.297, Mag: 2.02},
	{Name: "Mizar", RAdeg: 200.981, DecDeg: 54.925, Mag: 2.04},
	{Name: "Alpheratz", RAdeg: 2.097, DecDeg: 29.091, Mag: 2.06},
	{Name: "Saiph", RAdeg: 86.939, DecDeg: -9.670, Mag: 2.09},
	{Name: "Mirach", RAdeg: 17.433, DecDeg: 35.621, Mag: 2.05},
	{Name: "Kochab", RAdeg: 222.676, DecDeg: 74.156, Mag: 2.08},
	{Name: "Rasalhague", RAdeg: 263.734, DecDeg: 12.560, Mag: 2.08},
	{Name: "Algol", RAdeg: 47.042, DecDeg: 40.957, Mag: 2.12},
	{Name: "Denebola", RAdeg: 177.265, DecDeg: 14.572, Mag: 2.13},
	{Name: "Muhlifain", RAdeg: 190.379, DecDeg: -48.960, Mag: 2.17},
	{Name: "Naos", RAdeg: 120.896, DecDeg: -40.003, Mag: 2.25},
	{Name: "Aspidiske", RAdeg: 139.273, DecDeg: -59.275, Mag: 2.25},
	{Name: "Suhail", RAdeg: 136.999, DecDeg: -43.433, Mag: 2.21},
	{Name: "Alphecca", RAdeg: 233.672, DecDeg: 26.715, Mag: 2.23},
	{Name: "Mintaka", RAdeg: 83.002, DecDeg: -0.299, Mag: 2.23},
	{Name: "Sadr", RAdeg: 305.557, DecDeg: 40.257, Mag: 2.23},
	{Name: "Eltanin", RAdeg: 269.152, DecDeg: 51.489, Mag: 2.23},
	{Name: "Schedar", RAdeg: 10.127, DecDeg: 56.537, Mag: 2.23},
	{Name: "Caph", RAdeg: 2.295, DecDeg: 59.150, Mag: 2.27},
	{Name: "Dschubba", RAdeg: 240.083, DecDeg: -22.622, Mag: 2.32},
	{Name: "Larawag", RAdeg: 254.655, DecDeg: -34.293, Mag: 2.29},
	{Name: "Merak", RAdeg: 165.460, DecDeg: 56.382, Mag: 2.37},
	{Name: "Izar", RAdeg: 221.247, DecDeg: 27.074, Mag: 2.37},

	// Magnitude 2.5-3.0
	{Name: "Enif", RAdeg: 326.046, DecDeg: 9.875, Mag: 2.39},
	{Name: "Ankaa", RAdeg: 6.571, DecDeg: -42.306, Mag: 2.38},
	{Name: "Phecda", RAdeg: 178.458, DecDeg: 53.695, Mag: 2.44},
	{Name: "Sabik", RAdeg: 257.595, DecDeg: -15.725, Mag: 2.43},
	{Name: "Scheat", RAdeg: 345.944, DecDeg: 28.083, Mag: 2.42},
	{Name: "Alderamin", RAdeg: 319.645, DecDeg: 62.586, Mag: 2.51},
	{Name: "Aludra", RAdeg: 111.024, DecDeg: -29.303, Mag: 2.45},
	{Name: "Markeb", RAdeg: 140.528, DecDeg: -55.011, Mag: 2.47},
	{Name: "Girtab", RAdeg: 265.622, DecDeg: -39.030, Mag: 2.41},
	{Name: "Navi", RAdeg: 14.177, DecDeg: 60.717, Mag: 2.47},
	{Name: "Markab", RAdeg: 346.190, DecDeg: 15.205, Mag: 2.49},
	{Name: "Aljanah", RAdeg: 311.553, DecDeg: 33.970, Mag: 2.48},
	{Name: "Acrab", RAdeg: 241.359, DecDeg: -19.805, Mag: 2.62},

	// Magnitude 3.0-3.5
	{Name: "Aldhanab", RAdeg: 319.966, DecDeg: -16.127, Mag: 3.00},
	{Name: "Gienah", RAdeg: 183.952, DecDeg: -17.542, Mag: 2.59},
	{Name: "Zubeneschamali", RAdeg: 229.252, DecDeg: -9.383, Mag: 2.61},
	{Name: "Unukalhai", RAdeg: 236.067, DecDeg: 6.426, Mag: 2.65},
	{Name: "Sheratan", RAdeg: 28.660, DecDeg: 20.808, Mag: 2.64},
	{Name: "Phact", RAdeg: 84.912, DecDeg: -34.074, Mag: 2.64},
	{Name: "Menkent", RAdeg: 211.671, DecDeg: -36.370, Mag: 2.06},
	{Name: "Zosma", RAdeg: 168.527, DecDeg: 20.524, Mag: 2.56},
	{Name: "Arneb", RAdeg: 83.183, DecDeg: -17.822, Mag: 2.58},
	{Name: "Gomeisa", RAdeg: 111.788, DecDeg: 8.289, Mag: 2.90},
	{Name: "Deneb Kaitos", RAdeg: 10.897, DecDeg: -17.987, Mag: 2.04},
	{Name: "Thuban", RAdeg: 211.097, DecDeg: 64.376, Mag: 3.65},
	{Name: "Rastaban", RAdeg: 262.608, DecDeg: 52.301, Mag: 2.79},
	{Name: "Cor Caroli", RAdeg: 194.007, DecDeg: 38.318, Mag: 2.81},
	{Name: "Vindemiatrix", RAdeg: 195.544, DecDeg: 10.959, Mag: 2.83},
	{Name: "Algorab", RAdeg: 187.466, DecDeg: -16.515, Mag: 2.95},
	{Name: "Zubenelgenubi", RAdeg: 222.720, DecDeg: -16.042, Mag: 2.75},
	{Name: "Porrima", RAdeg: 190.415, DecDeg: -1.449, Mag: 2.74},

	// Magnitude 3.5-4.0 (subtle stars)
	{Name: "Albireo", RAdeg: 292.680, DecDeg: 27.960, Mag: 3.18},
	{Name: "Sadalmelik", RAdeg: 331.446, DecDeg: -0.320, Mag: 2.96},
	{Name: "Sadalsuud", RAdeg: 322.890, DecDeg: -5.571, Mag: 2.91},
	{Name: "Yed Prior", RAdeg: 243.586, DecDeg: -3.694, Mag: 2.75},
	{Name: "Alcyone", RAdeg: 56.871, DecDeg: 24.105, Mag: 2.87},
	{Name: "Tarazed", RAdeg: 296.565, DecDeg: 10.613, Mag: 2.72},
	{Name: "Alshain", RAdeg: 298.828, DecDeg: 6.407, Mag: 3.71},
	{Name: "Nihal", RAdeg: 82.061, DecDeg: -20.759, Mag: 2.84},
	{Name: "Wazn", RAdeg: 90.399, DecDeg: -35.768, Mag: 3.85},
	{Name: "Muscida", RAdeg: 127.566, DecDeg: 60.718, Mag: 3.35},
	{Name: "Talitha", RAdeg: 134.802, DecDeg: 48.042, Mag: 3.14},
	{Name: "Tania Australis", RAdeg: 155.582, DecDeg: 41.499, Mag: 3.05},
	{Name: "Alula Australis", RAdeg: 169.545, DecDeg: 31.529, Mag: 3.78},
	{Name: "Megrez", RAdeg: 183.857, DecDeg: 57.033, Mag: 3.31},
	{Name: "Alcor", RAdeg: 201.306, DecDeg: 54.988, Mag: 3.99},
	{Name: "Syrma", RAdeg: 214.004, DecDeg: -6.001, Mag: 4.08},
	{Name: "Khambalia", RAdeg: 218.877, DecDeg: -13.371, Mag: 4.66},
	{Name: "Kraz", RAdeg: 188.597, DecDeg: -23.397, Mag: 2.65},
	{Name: "Alkes", RAdeg: 164.944, DecDeg: -18.299, Mag: 4.08},
	{Name: "Minkar", RAdeg: 182.531, DecDeg: -22.620, Mag: 3.02},
	{Name: "Sceptrum", RAdeg: 62.966, DecDeg: -8.898, Mag: 4.45},
	{Name: "Cursa", RAdeg: 76.963, DecDeg: -5.086, Mag: 2.79},
	{Name: "Hassaleh", RAdeg: 75.492, DecDeg: 33.166, Mag: 2.69},
	{Name: "Hoedus I", RAdeg: 75.620, DecDeg: 41.234, Mag: 3.04},
	{Name: "Hoedus II", RAdeg: 75.248, DecDeg: 41.076, Mag: 3.17},
	{Name: "Saclateni", RAdeg: 79.402, DecDeg: 40.010, Mag: 3.69},

	// Magnitude 4.0-4.5 (dim background stars)
	{Name: "Furud", RAdeg: 95.078, DecDeg: -30.063, Mag: 3.96},
	{Name: "Muliphein", RAdeg: 105.940, DecDeg: -15.633, Mag: 4.11},
	{Name: "Tejat", RAdeg: 95.740, DecDeg: 22.513, Mag: 2.88},
	{Name: "Mebsuta", RAdeg: 100.983, DecDeg: 25.131, Mag: 3.06},
	{Name: "Propus", RAdeg: 93.719, DecDeg: 22.506, Mag: 3.28},
	{Name: "Wasat", RAdeg: 110.031, DecDeg: 21.982, Mag: 3.53},
	{Name: "Kappa Gem", RAdeg: 116.112, DecDeg: 24.398, Mag: 3.57},
	{Name: "Asellus Australis", RAdeg: 131.171, DecDeg: 18.154, Mag: 3.94},
	{Name: "Asellus Borealis", RAdeg: 130.821, DecDeg: 21.469, Mag: 4.66},
	{Name: "Acubens", RAdeg: 134.622, DecDeg: 11.858, Mag: 4.25},
	{Name: "Alterf", RAdeg: 139.711, DecDeg: 22.968, Mag: 4.31},
	{Name: "Rasalas", RAdeg: 146.463, DecDeg: 26.007, Mag: 3.88},
	{Name: "Adhafera", RAdeg: 154.173, DecDeg: 23.417, Mag: 3.43},
	{Name: "Subra", RAdeg: 148.191, DecDeg: 9.893, Mag: 3.52},
	{Name: "Chertan", RAdeg: 168.560, DecDeg: 15.430, Mag: 3.33},
	{Name: "Zavijava", RAdeg: 177.674, DecDeg: 1.765, Mag: 3.61},

	// Magnitude 4.5-5.0 (very dim, adds density)
	{Name: "Tyl", RAdeg: 288.439, DecDeg: 67.661, Mag: 4.01},
	{Name: "Edasich", RAdeg: 231.232, DecDeg: 58.966, Mag: 3.29},
	{Name: "Giausar", RAdeg: 175.942, DecDeg: 69.331, Mag: 3.85},
	{Name: "Grumium", RAdeg: 268.382, DecDeg: 56.873, Mag: 3.75},
	{Name: "Alsafi", RAdeg: 282.520, DecDeg: 52.301, Mag: 4.67},
	{Name: "Alrakis", RAdeg: 245.998, DecDeg: 61.514, Mag: 4.67},
	{Name: "Dziban", RAdeg: 270.162, DecDeg: 72.149, Mag: 4.54},
	{Name: "Pherkad", RAdeg: 230.182, DecDeg: 71.834, Mag: 3.00},
	{Name: "Yildun", RAdeg: 263.054, DecDeg: 86.586, Mag: 4.36},
	{Name: "Epsilon Dra", RAdeg: 297.043, DecDeg: 70.268, Mag: 3.83},
	{Name: "Chi Dra", RAdeg: 274.966, DecDeg: 72.733, Mag: 3.57},
	{Name: "Gianfar", RAdeg: 284.073, DecDeg: 75.388, Mag: 4.13},
	{Name: "Aldhibah", RAdeg: 256.343, DecDeg: 65.715, Mag: 3.17},
	{Name: "Nodus Secundus", RAdeg: 246.998, DecDeg: 61.514, Mag: 3.07},
	{Name: "Tania Borealis", RAdeg: 154.274, DecDeg: 42.914, Mag: 3.45},
	{Name: "Alula Borealis", RAdeg: 169.620, DecDeg: 33.094, Mag: 3.49},
	{Name: "Chara", RAdeg: 188.436, DecDeg: 41.357, Mag: 4.26},
	{Name: "Asterion", RAdeg: 194.289, DecDeg: 38.318, Mag: 4.25},
	{Name: "Diadem", RAdeg: 197.497, DecDeg: 17.529, Mag: 4.32},
	{Name: "Zaniah", RAdeg: 184.976, DecDeg: -0.667, Mag: 3.89},
	{Name: "Auva", RAdeg: 192.855, DecDeg: 3.397, Mag: 3.38},
	{Name: "Heze", RAdeg: 203.673, DecDeg: -0.596, Mag: 3.37},
}
