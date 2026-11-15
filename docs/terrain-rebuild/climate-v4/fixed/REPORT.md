# Climate version 4 paired validation

Result: PASS. Seeds: 24.

Rainfall remains 1.65. All shares are percentages of spherical-area-weighted dry land; aggregate rows give equal weight to each world. Validation never feeds generation.

| Biome | Version 1 | Version 4 | Change (percentage points) |
| --- | ---: | ---: | ---: |
| Temperate broadleaf and mixed forest | 11.30 | 23.38 | +12.09 |
| Temperate woodland | 3.30 | 7.04 | +3.75 |
| Temperate grassland | 5.27 | 7.06 | +1.79 |
| Tropical seasonal forest / woodland | 6.47 | 3.59 | -2.88 |
| Tropical rainforest | 10.88 | 6.63 | -4.25 |
| Temperate rainforest | 7.06 | 3.21 | -3.85 |
| Boreal forest (taiga) | 0.49 | 4.79 | +4.30 |
| Tundra | 8.45 | 7.87 | -0.58 |
| Desert | 2.50 | 0.69 | -1.81 |
| Savanna | 4.34 | 0.55 | -3.80 |
| Total steppe (all types) | 6.16 | 6.00 | -0.15 |
| Temperate steppe | 0.00 | 4.14 | +4.14 |
| Subtropical steppe | 0.00 | 1.70 | +1.70 |
| Tropical steppe | 0.00 | 0.16 | +0.16 |

## Individual dryland changes

| Seed | Held out | Desert | Savanna | Steppe |
| --- | --- | ---: | ---: | ---: |
| Kriemhild | False | -1.27 | -3.70 | -1.06 |
| d1feae98-6510-4a72-951b-44e93a01ae86 | False | -4.03 | -2.85 | +0.31 |
| gallery-001 | False | -2.92 | -3.73 | +0.80 |
| gallery-002 | False | -0.93 | -1.69 | +0.04 |
| gallery-003 | False | -1.26 | -3.47 | -0.46 |
| gallery-004 | False | -0.56 | -3.26 | +0.72 |
| gallery-005 | False | -2.96 | -3.86 | -2.42 |
| gallery-006 | False | -4.72 | -3.78 | +0.42 |
| gallery-007 | False | -3.34 | -3.59 | +2.09 |
| gallery-008 | False | -3.33 | -3.15 | +2.37 |
| gallery-009 | False | -0.59 | -4.61 | -1.03 |
| gallery-010 | False | -0.23 | -3.19 | -0.19 |
| gallery-011 | False | +0.04 | -4.55 | -1.56 |
| gallery-012 | False | -1.47 | -3.03 | -0.29 |
| gallery-013 | False | -0.85 | -4.69 | -0.65 |
| gallery-014 | False | -1.93 | -6.44 | +0.59 |
| gallery-015 | False | -5.35 | -2.60 | +2.47 |
| gallery-016 | False | -1.24 | -3.62 | +0.01 |
| gallery-017 | False | -1.18 | -4.74 | -3.42 |
| gallery-018 | False | +0.16 | -3.86 | +0.95 |
| gallery-019 | True | -2.22 | -3.42 | -1.91 |
| gallery-020 | True | +0.23 | -4.12 | +1.29 |
| gallery-021 | True | -1.17 | -5.17 | -1.85 |
| gallery-022 | True | -2.31 | -3.96 | -0.94 |

## Continental moisture contrast

Coastal/interior mean-rain ratios use the same latitude band and at least 100,000 square kilometres in each group. Coast distance separates coastal land below 250 km from the remaining interior. These generated-world comparisons do not control elevation; idealized terrain tests separately check mountain causality.

| Absolute latitude | Worlds | Coastal / interior rainfall | Worlds with drier interiors |
| --- | ---: | ---: | ---: |
| 00-15 | 24 | 1.54 | 24 |
| 15-30 | 24 | 1.56 | 24 |
| 30-45 | 24 | 1.88 | 24 |
| 45-60 | 24 | 1.82 | 24 |

## Failures

None.

## Evidence and limits

See `results.json` for latitude/coast/elevation/exposure measurements, monthly budgets, raw field hashes, and biome transition matrices. `validation.json` separates tuning and held-out results. `index.html` displays paired maps at fixed scales. These tests establish broad plausibility and regression behavior, not an Earth reconstruction.
