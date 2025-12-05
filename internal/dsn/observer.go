package dsn

import (
	"github.com/litescript/ls-horizons/internal/astro"
)

// ObserverForComplex returns an astro.Observer for the given DSN complex.
// Defaults to Goldstone for unknown or zero values.
func ObserverForComplex(c Complex) astro.Observer {
	info, ok := KnownComplexes[c]
	if !ok {
		// Default to Goldstone
		info = KnownComplexes[ComplexGoldstone]
	}

	return astro.Observer{
		LatDeg: info.Latitude,
		LonDeg: info.Longitude,
		Name:   info.Name,
	}
}
