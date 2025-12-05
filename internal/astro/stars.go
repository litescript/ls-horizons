package astro

// Star represents a cataloged star with position and brightness.
type Star struct {
	Name   string  // Common name (e.g., "Sirius", "Vega")
	RAdeg  float64 // Right Ascension in degrees (J2000)
	DecDeg float64 // Declination in degrees (J2000)
	Mag    float64 // Apparent visual magnitude (lower = brighter)
}

// StarCatalog holds a collection of stars for rendering.
type StarCatalog struct {
	Stars []Star
}

// DefaultStarCatalog returns a catalog of ~100 bright stars (mag < 4.0).
// Coordinates are J2000 epoch.
// Data sourced from Yale Bright Star Catalog and IAU star names.
func DefaultStarCatalog() StarCatalog {
	return StarCatalog{
		Stars: defaultStars,
	}
}

// defaultStars contains bright stars visible from various latitudes.
// Ordered roughly by magnitude (brightest first).
var defaultStars = []Star{
	// Magnitude < 0 (exceptionally bright)
	{"Sirius", 101.287, -16.716, -1.46},
	{"Canopus", 95.988, -52.696, -0.74},
	{"Arcturus", 213.915, 19.182, -0.05},
	{"Vega", 279.235, 38.784, 0.03},
	{"Capella", 79.172, 45.998, 0.08},
	{"Rigel", 78.634, -8.202, 0.13},
	{"Procyon", 114.826, 5.225, 0.34},
	{"Achernar", 24.429, -57.237, 0.46},
	{"Betelgeuse", 88.793, 7.407, 0.50},
	{"Hadar", 210.956, -60.373, 0.61},

	// Magnitude 0.5-1.0
	{"Altair", 297.696, 8.868, 0.76},
	{"Acrux", 186.650, -63.099, 0.76},
	{"Aldebaran", 68.980, 16.509, 0.85},
	{"Antares", 247.352, -26.432, 0.96},
	{"Spica", 201.298, -11.161, 0.97},
	{"Pollux", 116.329, 28.026, 1.14},

	// Magnitude 1.0-1.5
	{"Fomalhaut", 344.413, -29.622, 1.16},
	{"Deneb", 310.358, 45.280, 1.25},
	{"Mimosa", 191.930, -59.689, 1.25},
	{"Regulus", 152.093, 11.967, 1.35},
	{"Adhara", 104.656, -28.972, 1.50},
	{"Castor", 113.650, 31.889, 1.58},

	// Magnitude 1.5-2.0
	{"Gacrux", 187.791, -57.113, 1.63},
	{"Shaula", 263.402, -37.104, 1.63},
	{"Bellatrix", 81.283, 6.350, 1.64},
	{"Elnath", 81.573, 28.608, 1.65},
	{"Miaplacidus", 138.300, -69.717, 1.68},
	{"Alnilam", 84.053, -1.202, 1.69},
	{"Alnair", 332.058, -46.961, 1.74},
	{"Alnitak", 85.190, -1.943, 1.77},
	{"Alioth", 193.507, 55.960, 1.77},
	{"Dubhe", 165.932, 61.751, 1.79},
	{"Mirfak", 51.081, 49.861, 1.79},
	{"Wezen", 107.098, -26.393, 1.84},
	{"Sargas", 264.330, -42.998, 1.87},
	{"Kaus Australis", 276.043, -34.384, 1.85},
	{"Avior", 125.629, -59.509, 1.86},
	{"Alkaid", 206.885, 49.313, 1.86},
	{"Menkalinan", 89.882, 44.948, 1.90},
	{"Atria", 252.166, -69.028, 1.92},
	{"Alhena", 99.428, 16.399, 1.93},
	{"Peacock", 306.412, -56.735, 1.94},
	{"Alsephina", 131.176, -54.709, 1.96},
	{"Mirzam", 95.675, -17.956, 1.98},
	{"Polaris", 37.954, 89.264, 2.02},
	{"Alphard", 141.897, -8.659, 2.00},

	// Magnitude 2.0-2.5
	{"Hamal", 31.793, 23.463, 2.00},
	{"Algieba", 146.463, 19.842, 2.08},
	{"Diphda", 10.897, -17.987, 2.02},
	{"Nunki", 283.816, -26.297, 2.02},
	{"Mizar", 200.981, 54.925, 2.04},
	{"Alpheratz", 2.097, 29.091, 2.06},
	{"Saiph", 86.939, -9.670, 2.09},
	{"Mirach", 17.433, 35.621, 2.05},
	{"Kochab", 222.676, 74.156, 2.08},
	{"Rasalhague", 263.734, 12.560, 2.08},
	{"Algol", 47.042, 40.957, 2.12},
	{"Denebola", 177.265, 14.572, 2.13},
	{"Muhlifain", 190.379, -48.960, 2.17},
	{"Naos", 120.896, -40.003, 2.25},
	{"Aspidiske", 139.273, -59.275, 2.25},
	{"Suhail", 136.999, -43.433, 2.21},
	{"Alphecca", 233.672, 26.715, 2.23},
	{"Mintaka", 83.002, -0.299, 2.23},
	{"Sadr", 305.557, 40.257, 2.23},
	{"Eltanin", 269.152, 51.489, 2.23},
	{"Schedar", 10.127, 56.537, 2.23},
	{"Caph", 2.295, 59.150, 2.27},
	{"Dschubba", 240.083, -22.622, 2.32},
	{"Larawag", 254.655, -34.293, 2.29},
	{"Merak", 165.460, 56.382, 2.37},
	{"Izar", 221.247, 27.074, 2.37},

	// Magnitude 2.5-3.0
	{"Enif", 326.046, 9.875, 2.39},
	{"Ankaa", 6.571, -42.306, 2.38},
	{"Phecda", 178.458, 53.695, 2.44},
	{"Sabik", 257.595, -15.725, 2.43},
	{"Scheat", 345.944, 28.083, 2.42},
	{"Alderamin", 319.645, 62.586, 2.51},
	{"Aludra", 111.024, -29.303, 2.45},
	{"Markeb", 140.528, -55.011, 2.47},
	{"Girtab", 265.622, -39.030, 2.41},
	{"Navi", 14.177, 60.717, 2.47},
	{"Markab", 346.190, 15.205, 2.49},
	{"Aljanah", 311.553, 33.970, 2.48},
	{"Acrab", 241.359, -19.805, 2.62},

	// Magnitude 3.0-3.5
	{"Aldhanab", 319.966, -16.127, 3.00},
	{"Gienah", 183.952, -17.542, 2.59},
	{"Zubeneschamali", 229.252, -9.383, 2.61},
	{"Unukalhai", 236.067, 6.426, 2.65},
	{"Sheratan", 28.660, 20.808, 2.64},
	{"Phact", 84.912, -34.074, 2.64},
	{"Menkent", 211.671, -36.370, 2.06},
	{"Zosma", 168.527, 20.524, 2.56},
	{"Arneb", 83.183, -17.822, 2.58},
	{"Gomeisa", 111.788, 8.289, 2.90},
	{"Deneb Kaitos", 10.897, -17.987, 2.04},
	{"Thuban", 211.097, 64.376, 3.65},
	{"Rastaban", 262.608, 52.301, 2.79},
	{"Cor Caroli", 194.007, 38.318, 2.81},
	{"Vindemiatrix", 195.544, 10.959, 2.83},
	{"Algorab", 187.466, -16.515, 2.95},
	{"Zubenelgenubi", 222.720, -16.042, 2.75},
	{"Porrima", 190.415, -1.449, 2.74},

	// Magnitude 3.5-4.0 (subtle stars)
	{"Albireo", 292.680, 27.960, 3.18},
	{"Sadalmelik", 331.446, -0.320, 2.96},
	{"Sadalsuud", 322.890, -5.571, 2.91},
	{"Yed Prior", 243.586, -3.694, 2.75},
	{"Alcyone", 56.871, 24.105, 2.87},
	{"Tarazed", 296.565, 10.613, 2.72},
	{"Alshain", 298.828, 6.407, 3.71},
	{"Nihal", 82.061, -20.759, 2.84},
	{"Wazn", 90.399, -35.768, 3.85},
	{"Muscida", 127.566, 60.718, 3.35},
	{"Talitha", 134.802, 48.042, 3.14},
	{"Tania Australis", 155.582, 41.499, 3.05},
	{"Alula Australis", 169.545, 31.529, 3.78},
	{"Megrez", 183.857, 57.033, 3.31},
	{"Alcor", 201.306, 54.988, 3.99},
	{"Syrma", 214.004, -6.001, 4.08},
	{"Khambalia", 218.877, -13.371, 4.66},
	{"Kraz", 188.597, -23.397, 2.65},
	{"Alkes", 164.944, -18.299, 4.08},
	{"Minkar", 182.531, -22.620, 3.02},
	{"Sceptrum", 62.966, -8.898, 4.45},
	{"Cursa", 76.963, -5.086, 2.79},
	{"Hassaleh", 75.492, 33.166, 2.69},
	{"Hoedus I", 75.620, 41.234, 3.04},
	{"Hoedus II", 75.248, 41.076, 3.17},
	{"Saclateni", 79.402, 40.010, 3.69},

	// Magnitude 4.0-4.5 (dim background stars)
	{"Furud", 95.078, -30.063, 3.96},
	{"Muliphein", 105.940, -15.633, 4.11},
	{"Tejat", 95.740, 22.513, 2.88},
	{"Mebsuta", 100.983, 25.131, 3.06},
	{"Propus", 93.719, 22.506, 3.28},
	{"Wasat", 110.031, 21.982, 3.53},
	{"Kappa Gem", 116.112, 24.398, 3.57},
	{"Asellus Australis", 131.171, 18.154, 3.94},
	{"Asellus Borealis", 130.821, 21.469, 4.66},
	{"Acubens", 134.622, 11.858, 4.25},
	{"Alterf", 139.711, 22.968, 4.31},
	{"Rasalas", 146.463, 26.007, 3.88},
	{"Adhafera", 154.173, 23.417, 3.43},
	{"Subra", 148.191, 9.893, 3.52},
	{"Chertan", 168.560, 15.430, 3.33},
	{"Zavijava", 177.674, 1.765, 3.61},

	// Magnitude 4.5-5.0 (very dim, adds density)
	{"Tyl", 288.439, 67.661, 4.01},
	{"Edasich", 231.232, 58.966, 3.29},
	{"Giausar", 175.942, 69.331, 3.85},
	{"Grumium", 268.382, 56.873, 3.75},
	{"Alsafi", 282.520, 52.301, 4.67},
	{"Alrakis", 245.998, 61.514, 4.67},
	{"Dziban", 270.162, 72.149, 4.54},
	{"Pherkad", 230.182, 71.834, 3.00},
	{"Yildun", 263.054, 86.586, 4.36},
	{"Epsilon Dra", 297.043, 70.268, 3.83},
	{"Chi Dra", 274.966, 72.733, 3.57},
	{"Gianfar", 284.073, 75.388, 4.13},
	{"Aldhibah", 256.343, 65.715, 3.17},
	{"Nodus Secundus", 246.998, 61.514, 3.07},
	{"Tania Borealis", 154.274, 42.914, 3.45},
	{"Alula Borealis", 169.620, 33.094, 3.49},
	{"Chara", 188.436, 41.357, 4.26},
	{"Asterion", 194.289, 38.318, 4.25},
	{"Diadem", 197.497, 17.529, 4.32},
	{"Zaniah", 184.976, -0.667, 3.89},
	{"Auva", 192.855, 3.397, 3.38},
	{"Heze", 203.673, -0.596, 3.37},
}
