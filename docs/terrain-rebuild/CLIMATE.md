# Reduced seasonal climate model

This implementation uses a reduced circulation and moisture model, not a general
circulation model or a calibrated reconstruction of Earth. Biome percentages never
feed back into its precipitation calculation.

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
