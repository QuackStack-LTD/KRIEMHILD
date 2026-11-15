# Reduced seasonal climate model

This implementation uses a reduced circulation and moisture model, not a general
circulation model or a calibrated reconstruction of Earth. Biome percentages never
feed back into its precipitation calculation.

## Distribution revision (version 4)

The [version-4 brief](CLIMATE_V4_DISTRIBUTION_SPEC.md) supersedes automatic global
drying as a rainforest correction. Rainfall remains **1.65**. Versions 0-3 retain
their original equations and vegetation classification. The historical transport
description below remains applicable, with these version-4 differences:

- Annual sea-level temperature is `27 - 46 sin(latitude)^2`, plus maritime
  redistribution `6 exp(-max(0, coastKm)/700) sin(2 latitude)^2` and spherical
  regional noise of amplitude 1.25 degrees C. Version 3's solar/thermal-inertia
  seasonal anomaly is retained. Elevation uses the existing 6.5 K/km lapse.
- Surface meridional circulation uses `-0.4 sin(6 latitude)` rather than the old
  `sin(3 latitude)` pattern. This creates equatorward trades, poleward
  mid-latitude flow, and equatorward polar flow. The storm-track width is 0.28
  radians instead of 0.23. Moisture capacity and subtropical subsidence strength
  are unchanged; water remains transported and conserved, never painted in.
- Monthly potential evaporation follows the radiation-sensitive Hargreaves form
  in [FAO-56 equation 52](https://www.fao.org/4/X0490E/x0490e07.htm).
  Top-of-atmosphere flux is converted from W/m2 to equivalent mm/day using
  `0.0864 * 0.408`; months have `365.2425/12` days. Daily temperature range is
  approximated as `8 + 4 continentality` degrees C. That approximation is an
  explicit limitation: daily weather and measured humidity are not simulated,
  and this is not a station-calibrated Penman-Monteith calculation.
- Regimes use actual sampled monthly temperatures and rainfall, with PET
  recomputed at local atmospheric elevation. Counts/extrema are computed after
  interpolation. Growing months have mean temperature at least 10 degrees C;
  fewer than two growing months remain tundra, and exposed highland terrain with
  fewer than three remains alpine. Boreal regimes combine growing-season length
  and winter severity, including cold continental sites with warm summers.
- A dry month has precipitation below half of monthly PET. Dry seasons wrap
  December/January. Perhumid climates require minimum monthly rain of 60 mm,
  minimum monthly rain/PET of 0.65, and zero dry months. Tropical rainforest also
  needs coldest month at least 18 degrees C, 1800 mm/year, and annual rain/PET of
  1.15. Temperate rainforest additionally needs 2400 mm/year, rain/PET of 1.8,
  mild winters, summers below 24 degrees C, and continentality below 0.3.
- Wet seasonal tropical climates can retain seasonal forest. Savanna requires
  tropical water limitation; failure of rainforest eligibility alone is
  insufficient. Semi-arid climates receive `temperate-steppe`,
  `subtropical-steppe`, or `tropical-steppe` identities. Tropical winter warmth
  selects tropical steppe; otherwise mild winters (at least 6 degrees C) and a
  warm summer select subtropical steppe; colder cases select temperate steppe.
  These are thermal/water-balance decisions, not fixed latitude/rainfall zones.

The existing `forest`, `woodland`, and `grassland` identifiers describe temperate
broadleaf/mixed forest, temperate woodland, and temperate grassland in version 4.
`tropical-woodland` denotes seasonal tropical forest/woodland; `shrubland` denotes
Mediterranean woodland/scrub. UI labels spell out these meanings. Historical
identifiers and palettes remain readable; the legacy projection stores steppe
subtype identities in climate cells while mapping their compatibility surface
type to steppe.

See the [frozen manifest](climate-v4/seeds.json),
[registered limits](climate-v4/regression-limits.json),
[fixed-terrain report](climate-v4/fixed/REPORT.md),
[full-pipeline report](climate-v4/full/REPORT.md), and
[paired gallery](climate-v4/fixed/index.html). Diagnostics do not control world
generation. Whole-pipeline bed/lake changes can follow the established moisture
conditioning and hydrology; the fixed-terrain comparison isolates climate while
preserving those inputs.

Reproduce from the repository root:

```sh
go run ./cmd/climate-distribution -versions 1,4 -out docs/terrain-rebuild/climate-v4/fixed
go run ./cmd/climate-distribution -versions 1,4 -fixed=false -render=false -out docs/terrain-rebuild/climate-v4/full
python tests/climate-distribution-report.py docs/terrain-rebuild/climate-v4/fixed --require-all-seeds
python tests/climate-distribution-report.py docs/terrain-rebuild/climate-v4/full --require-all-seeds
```

## Historical moisture transport

The conceptual circulation follows the trade-wind, midlatitude-westerly and polar
easterly belts, tropical ascent and subtropical subsidence described in
[NOAA's global atmospheric circulation overview](https://www.noaa.gov/jetstream/global/global-atmospheric-circulations).
The numerical coefficients below are implementation approximations, not equations
or calibration values attributed to NOAA.

For each of twelve seasonal states, latitude, axial tilt, elevation and distance
inland determine temperature. The monthly atmospheric column capacity is
`40 exp(clamp(0.045 T - 0.9, -3, 1)) × rainfallPreference` millimetres.
Ocean evaporation replenishes 18% of the column deficit per six-hour step.
Land transpiration is limited by both PET and a finite soil-water store; 70% of
rain can refill that store, capped at 200 mm. Initial soil water is 80 mm.

Each cell scatters its atmospheric water mass to its downwind neighbors and itself.
Mass, rather than column depth, is transported so different latitude-cell areas
cannot create moisture. Longitude wraps, and polar transport stays inside the cap.
Condensation depends on tropical ascent, storm tracks, subsidence, upwind elevation
gain and saturation. The exact water removed is accumulated as rainfall. There is
no separate rainfall formula or global dryland quota.

Each seasonal state uses 96 spin-up steps followed by 120 recorded steps. These are
equilibrated seasonal snapshots, not a time-accurate integration through a calendar
year. Diagnostics record the largest relative monthly atmospheric budget residual:
`abs(initialAtmosphere + evaporation - rain - finalAtmosphere) / (initialAtmosphere + evaporation)`.
Soil evaporation is included in atmospheric inputs. This is not a complete coupled
ocean/soil/river water-budget test; runoff and drainage still use annual reduced fields.

Tests check the budget to 1e-9 relative tolerance, nonnegative finite rain/PET,
annual/monthly agreement, and an idealized mountain's windward/lee contrast. The
50-seed gallery provides a broader distribution check. The early implementation's
independent rainfall formula gave effectively no deserts; the conservative model
replaces that formula rather than recoloring those cells.

Outstanding scientific work includes circulation calibration, true ocean heat
transport, adaptive seasons, convergence studies, soil periodic equilibrium,
orographic cloud microphysics, snow/glacier budgets and coupled endorheic drainage.
