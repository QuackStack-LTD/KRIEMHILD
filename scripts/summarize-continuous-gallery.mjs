import fs from "node:fs";
import path from "node:path";

const root = path.resolve("docs/terrain-rebuild");
const gallery = path.join(root, "gallery");
const metrics = JSON.parse(
  fs.readFileSync(path.join(gallery, "metrics.json"), "utf8"),
);
if (metrics.length !== 50) throw Error("Expected 50 completed seeds");
const defaults = metrics.filter((x) => x.preset === "earthlike");
const median = (values) => {
  const a = [...values].sort((a, b) => a - b),
    m = Math.floor(a.length / 2);
  return a.length % 2 ? a[m] : (a[m - 1] + a[m]) / 2;
};
let images = 0;
for (let i = 0; i < 50; i++)
  for (const style of [
    "natural",
    "mask",
    "elevation",
    "bathymetry",
    "precipitation",
    "biome",
    "coastline",
    "archipelago",
    "local",
  ]) {
    const bytes = fs.readFileSync(
      path.join(gallery, `seed-${String(i).padStart(3, "0")}-${style}.png`),
    );
    if (
      bytes.toString("hex", 0, 8) !== "89504e470d0a1a0a" ||
      bytes.readUInt32BE(16) !== 512 ||
      ![256, 512].includes(bytes.readUInt32BE(20))
    )
      throw Error("Invalid gallery image");
    images++;
  }
const summary = {
  seeds: metrics.length,
  images,
  defaultSeeds: defaults.length,
  threeOrMore: defaults.filter((x) => x.diagnostics.majorContinents >= 3)
    .length,
  medianLand: median(defaults.map((x) => x.diagnostics.landCoverage)),
  medianDesert: median(
    defaults.map((x) => x.diagnostics.biomeShares.desert || 0),
  ),
  medianSteppe: median(
    defaults.map((x) => x.diagnostics.biomeShares.steppe || 0),
  ),
  medianGenerationMs: median(
    metrics.map((x) =>
      Object.values(x.diagnostics.timingsMs).reduce((a, b) => a + b, 0),
    ),
  ),
  minimumContinents: Math.min(
    ...defaults.map((x) => x.diagnostics.majorContinents),
  ),
  maximumContinents: Math.max(
    ...defaults.map((x) => x.diagnostics.majorContinents),
  ),
  maximumMoistureBudgetRelativeError: Math.max(
    ...metrics.map((x) => x.diagnostics.moistureBudgetRelativeError),
  ),
  medianLongestRiverKm: median(
    defaults.map((x) => x.diagnostics.longestRiverKm),
  ),
  medianMacroCoastLengthKm: median(
    defaults.map((x) => x.diagnostics.macroCoastLengthKm),
  ),
  medianLakeCoverage: median(defaults.map((x) => x.diagnostics.lakeCoverage)),
};
fs.writeFileSync(
  path.join(root, "metrics-summary.json"),
  JSON.stringify(summary, null, 2) + "\n",
);
console.log(summary);
