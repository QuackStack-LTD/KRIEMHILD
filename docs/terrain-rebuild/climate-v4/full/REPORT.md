# Climate version 4 paired validation

Result: PASS. Seeds: 24.

Rainfall remains 1.65. All shares are percentages of spherical-area-weighted dry land; aggregate rows give equal weight to each world. Validation never feeds generation.

| Biome | Version 1 | Version 4 | Change (percentage points) |
| --- | ---: | ---: | ---: |
| Temperate broadleaf and mixed forest | 11.43 | 23.63 | +12.20 |
| Temperate woodland | 3.28 | 6.91 | +3.63 |
| Temperate grassland | 5.14 | 6.51 | +1.37 |
| Tropical seasonal forest / woodland | 6.52 | 3.67 | -2.84 |
| Tropical rainforest | 10.95 | 6.77 | -4.18 |
| Temperate rainforest | 7.21 | 3.34 | -3.87 |
| Boreal forest (taiga) | 0.49 | 4.51 | +4.02 |
| Tundra | 8.45 | 7.91 | -0.54 |
| Desert | 2.44 | 0.69 | -1.76 |
| Savanna | 4.31 | 0.46 | -3.85 |
| Total steppe (all types) | 6.08 | 6.30 | +0.22 |
| Temperate steppe | 0.00 | 4.34 | +4.34 |
| Subtropical steppe | 0.00 | 1.79 | +1.79 |
| Tropical steppe | 0.00 | 0.16 | +0.16 |

## Individual dryland changes

| Seed | Held out | Desert | Savanna | Steppe |
| --- | --- | ---: | ---: | ---: |
| Kriemhild | False | -1.23 | -3.70 | -0.55 |
| d1feae98-6510-4a72-951b-44e93a01ae86 | False | -3.82 | -2.84 | +0.85 |
| gallery-001 | False | -2.87 | -3.66 | +0.95 |
| gallery-002 | False | -0.90 | -1.70 | +0.44 |
| gallery-003 | False | -1.17 | -3.52 | -0.20 |
| gallery-004 | False | -0.65 | -3.36 | +1.46 |
| gallery-005 | False | -2.93 | -3.91 | -2.41 |
| gallery-006 | False | -4.63 | -3.83 | +0.97 |
| gallery-007 | False | -3.10 | -3.49 | +2.24 |
| gallery-008 | False | -3.33 | -3.16 | +2.67 |
| gallery-009 | False | -0.58 | -4.53 | -0.80 |
| gallery-010 | False | -0.25 | -3.28 | +0.02 |
| gallery-011 | False | -0.02 | -4.49 | -1.39 |
| gallery-012 | False | -1.45 | -3.06 | -0.23 |
| gallery-013 | False | -0.81 | -5.00 | -0.22 |
| gallery-014 | False | -1.80 | -6.21 | +0.99 |
| gallery-015 | False | -5.25 | -2.67 | +2.62 |
| gallery-016 | False | -1.13 | -3.61 | +0.42 |
| gallery-017 | False | -1.10 | -5.00 | -3.21 |
| gallery-018 | False | +0.17 | -4.10 | +2.07 |
| gallery-019 | True | -2.16 | -3.59 | -1.23 |
| gallery-020 | True | +0.37 | -4.38 | +2.10 |
| gallery-021 | True | -1.21 | -5.28 | -1.58 |
| gallery-022 | True | -2.27 | -3.92 | -0.77 |

## Continental moisture contrast

Coastal/interior mean-rain ratios use the same latitude band and at least 100,000 square kilometres in each group. Coast distance separates coastal land below 250 km from the remaining interior. These generated-world comparisons do not control elevation; idealized terrain tests separately check mountain causality.

| Absolute latitude | Worlds | Coastal / interior rainfall | Worlds with drier interiors |
| --- | ---: | ---: | ---: |
| 00-15 | 24 | 1.55 | 24 |
| 15-30 | 24 | 1.58 | 24 |
| 30-45 | 24 | 1.90 | 24 |
| 45-60 | 24 | 1.84 | 24 |

## Failures

None.

## Evidence and limits

See `results.json` for latitude/coast/elevation/exposure measurements, monthly budgets, raw field hashes, and biome transition matrices. `validation.json` separates tuning and held-out results. `index.html` displays paired maps at fixed scales. These tests establish broad plausibility and regression behavior, not an Earth reconstruction.
