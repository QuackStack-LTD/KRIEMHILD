"""Validate paired climate diagnostics; never supplies inputs to generation."""
import argparse
import json
from pathlib import Path
from statistics import mean


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", type=Path)
    parser.add_argument("--limits", type=Path, default=Path("docs/terrain-rebuild/climate-v4/regression-limits.json"))
    parser.add_argument("--manifest", type=Path, default=Path("docs/terrain-rebuild/climate-v4/seeds.json"))
    parser.add_argument("--require-all-seeds", action="store_true")
    args = parser.parse_args()
    results = json.loads((args.directory / "results.json").read_text())
    limits = json.loads(args.limits.read_text())
    manifest = json.loads(args.manifest.read_text())
    failures = []
    rows = []
    metrics = ["forest", "woodland", "grassland", "tropical-woodland", "rainforest", "temperate-rainforest", "taiga", "tundra", "desert", "savanna", "steppe", "temperate-steppe", "subtropical-steppe", "tropical-steppe"]
    if args.require_all_seeds:
        missing = set(manifest["tuning"] + manifest["heldOut"]) - results.keys()
        if missing:
            failures.append(f"Missing frozen seeds: {sorted(missing)}")

    def share(band, key):
        if key == "steppe":
            return sum(band["biomeShares"].get(k, 0) for k in ["steppe", "temperate-steppe", "subtropical-steppe", "tropical-steppe"])
        return sum(band["biomeShares"].get(k, 0) for k in key.split("+"))

    for seed, versions in results.items():
        old, new = [versions[v]["bands"]["all"] for v in ["1", "4"]]
        delta = {key: share(new, key) - share(old, key) for key in metrics}
        delta["desert+savanna"] = delta["desert"] + delta["savanna"]
        for key, tolerance in limits["perSeedMaximumIncrease"].items():
            if delta[key] > tolerance:
                failures.append(f"{seed}: {key} expands {delta[key] * 100:.3f} pp (limit {tolerance * 100:.3f})")
        if versions["4"]["waterBudgetError"] > 1e-9:
            failures.append(f"{seed}: atmospheric conservation failure")
        if versions["4"]["spec"]["rainfall"] != versions["1"]["spec"]["rainfall"]:
            failures.append(f"{seed}: unequal rainfall controls")
        # Geographic means are not a complete thermal calibration; absent bands
        # are excluded and every value retains its actual land denominator.
        bands = versions["4"]["bands"]
        if all(k in bands for k in ["latitude/00-15", "latitude/30-45", "latitude/75-90"]):
            temperatures = [bands[k]["meanAnnualTemperature"] for k in ["latitude/00-15", "latitude/30-45", "latitude/75-90"]]
            if not temperatures[0] > temperatures[1] > temperatures[2]:
                failures.append(f"{seed}: geographic temperature ordering {temperatures}")
        rows.append({"seed": seed, "heldOut": seed in manifest["heldOut"], "before": old, "after": new, "changes": delta})

    aggregate = {}
    for name, subset in [("all", rows), ("tuning", [r for r in rows if not r["heldOut"]]), ("heldOut", [r for r in rows if r["heldOut"]])]:
        if not subset:
            continue
        changes = {key: mean(r["changes"][key] for r in subset) for key in metrics + ["desert+savanna"]}
        aggregate[name] = {
            "seedCount": len(subset),
            "meanBefore": {key: mean(share(r["before"], key) for r in subset) for key in metrics},
            "meanAfter": {key: mean(share(r["after"], key) for r in subset) for key in metrics},
            "meanChanges": changes,
            "meanPrecipitationBefore": mean(r["before"]["meanAnnualPrecipitation"] for r in subset),
            "meanPrecipitationAfter": mean(r["after"]["meanAnnualPrecipitation"] for r in subset),
            "meanPETBefore": mean(r["before"]["meanAnnualPET"] for r in subset),
            "meanPETAfter": mean(r["after"]["meanAnnualPET"] for r in subset),
        }
        for key, tolerance in limits["ensembleMaximumIncrease"].items():
            if changes[key] > tolerance:
                failures.append(f"{name} ensemble: {key} expands {changes[key] * 100:.3f} pp (limit {tolerance * 100:.3f})")
        if changes["forest"] <= 0:
            failures.append(f"{name} ensemble: temperate forest did not increase")
        if changes["rainforest"] >= 0:
            failures.append(f"{name} ensemble: tropical rainforest did not decrease")
    continental = {}
    for band in ["00-15", "15-30", "30-45", "45-60"]:
        ratios = []
        for versions in results.values():
            bands = versions["4"]["bands"]
            coastal = bands.get(f"latitudeCoast/{band}/coastal")
            interior = bands.get(f"latitudeCoast/{band}/interior")
            if coastal and interior and min(coastal["landAreaKm2"], interior["landAreaKm2"]) > 100000:
                ratios.append(coastal["meanAnnualPrecipitation"] / max(1, interior["meanAnnualPrecipitation"]))
        if ratios:
            continental[band] = {"worldCount": len(ratios), "meanCoastalToInteriorRain": mean(ratios), "worldsWithDrierInteriors": sum(r > 1 for r in ratios)}
            if mean(ratios) <= 1:
                failures.append(f"{band}: continental interiors are not statistically drier than coasts")
    report = {"passed": not failures, "failures": failures, "aggregate": aggregate, "seeds": rows, "continentalDrying": continental,
              "aggregation": "equal weight per seed; underlying world diagnostics use spherical land area; area shares exclude lakes"}
    (args.directory / "validation.json").write_text(json.dumps(report, indent=2) + "\n")
    lines = ["# Climate version 4 paired validation", "", f"Result: {'PASS' if not failures else 'FAIL'}. Seeds: {len(rows)}.", "",
             "Rainfall remains 1.65. All shares are percentages of spherical-area-weighted dry land; aggregate rows give equal weight to each world. Validation never feeds generation.", "",
             "| Biome | Version 1 | Version 4 | Change (percentage points) |", "| --- | ---: | ---: | ---: |"]
    overall = aggregate["all"]
    names = {"forest": "Temperate broadleaf and mixed forest", "woodland": "Temperate woodland", "grassland": "Temperate grassland", "tropical-woodland": "Tropical seasonal forest / woodland", "rainforest": "Tropical rainforest", "temperate-rainforest": "Temperate rainforest", "taiga": "Boreal forest (taiga)", "tundra": "Tundra", "desert": "Desert", "savanna": "Savanna", "steppe": "Total steppe (all types)", "temperate-steppe": "Temperate steppe", "subtropical-steppe": "Subtropical steppe", "tropical-steppe": "Tropical steppe"}
    for key in metrics:
        lines.append(f"| {names[key]} | {overall['meanBefore'][key]*100:.2f} | {overall['meanAfter'][key]*100:.2f} | {overall['meanChanges'][key]*100:+.2f} |")
    lines += ["", "## Individual dryland changes", "", "| Seed | Held out | Desert | Savanna | Steppe |", "| --- | --- | ---: | ---: | ---: |"]
    for row in rows:
        lines.append(f"| {row['seed']} | {row['heldOut']} | {row['changes']['desert']*100:+.2f} | {row['changes']['savanna']*100:+.2f} | {row['changes']['steppe']*100:+.2f} |")
    lines += ["", "## Continental moisture contrast", "", "Coastal/interior mean-rain ratios use the same latitude band and at least 100,000 square kilometres in each group. Coast distance separates coastal land below 250 km from the remaining interior. These generated-world comparisons do not control elevation; idealized terrain tests separately check mountain causality.", "", "| Absolute latitude | Worlds | Coastal / interior rainfall | Worlds with drier interiors |", "| --- | ---: | ---: | ---: |"]
    for band, stats in continental.items():
        lines.append(f"| {band} | {stats['worldCount']} | {stats['meanCoastalToInteriorRain']:.2f} | {stats['worldsWithDrierInteriors']} |")
    lines += ["", "## Failures", ""] + ([f"- {f}" for f in failures] or ["None."])
    lines += ["", "## Evidence and limits", "", "See `results.json` for latitude/coast/elevation/exposure measurements, monthly budgets, raw field hashes, and biome transition matrices. `validation.json` separates tuning and held-out results. `index.html` displays paired maps at fixed scales. These tests establish broad plausibility and regression behavior, not an Earth reconstruction.", ""]
    (args.directory / "REPORT.md").write_text("\n".join(lines))
    print("PASS" if not failures else "FAIL", f"({len(rows)} seeds)")
    for failure in failures:
        print(failure)
    raise SystemExit(bool(failures))


if __name__ == "__main__":
    main()
