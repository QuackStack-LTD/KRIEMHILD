package generate

import (
	"kriemhild/internal/terrain/model"
	"math"
)

// Daily mean top-of-atmosphere flux on a circular orbit, including polar night
// and midnight sun. Solar longitude retains the existing month/tilt convention.
func dailyInsolation(lat, decl float64) float64 {
	h := math.Acos(model.Clamp(-math.Tan(lat)*math.Tan(decl), -1, 1))
	return math.Max(0, 1361/math.Pi*(h*math.Sin(lat)*math.Sin(decl)+math.Cos(lat)*math.Cos(decl)*math.Sin(h)))
}

// Experimental seasonal anomaly only: the annual mean and lapse rate are kept
// from v2, so this cannot hide an annual-mean bias with a blanket warming offset.
// Solve C dT/dt + B T = absorbed seasonal forcing in Fourier space. Parameters
// C_land=2 m water, C_ocean=76 m, B=2.48 W/m2/K and land-response scale .52
// follow McKinnon & Huybers (2014), doi:10.1002/2014GL060404, Table 1.
// This is a reduced approximation, NOT that paper's calibrated circulation model:
// coast distance substitutes for back trajectories, coalbedo is fixed .7, and
// meridional seasonal heat convergence/cloud/ice feedbacks are not simulated.
func seasonalThermalAnomaly(lat, coast, _ /* elevation is applied separately */ float64, tilt float64) (out [12]float64) {
	var solar [12]float64
	for m := range solar {
		decl := tilt * math.Pi / 180 * math.Sin(model.Tau*(float64(m)-2)/12)
		solar[m] = dailyInsolation(lat, decl) * .7
	}
	continental := 1 - math.Exp(-math.Max(0, coast)/700)
	// Coastal air responds to both land and ocean. This surface contribution is
	// explicit and uncalibrated; coast distance cannot distinguish western and
	// eastern coasts. Deep ocean has only the mixed-layer response.
	// A 50-km coastal transition avoids an air-temperature jump at the land mask.
	// Its width, and the .3 near-coast land response, still need calibration.
	land := .52 * (.3 + .7*continental) * (.5 + .5*math.Tanh(coast/50))
	const yearSeconds = 365.2425 * 24 * 60 * 60
	for harmonic := 1; harmonic <= 5; harmonic++ {
		a, b := 0., 0.
		for m, q := range solar {
			phase := model.Tau * float64(harmonic*m) / 12
			a += q * math.Cos(phase) / 6
			b += q * math.Sin(phase) / 6
		}
		omega := model.Tau * float64(harmonic) / yearSeconds
		for _, surface := range []struct{ weight, capacity float64 }{{land, 2 * 4.18e6}, {1 - land, 76 * 4.18e6}} {
			inertia := omega * surface.capacity
			denominator := 2.48*2.48 + inertia*inertia
			// Positive heat capacity delays temperature behind incoming solar forcing.
			ta := (2.48*a - inertia*b) / denominator * surface.weight
			tb := (inertia*a + 2.48*b) / denominator * surface.weight
			for m := range out {
				phase := model.Tau * float64(harmonic*m) / 12
				out[m] += ta*math.Cos(phase) + tb*math.Sin(phase)
			}
		}
	}
	return
}
