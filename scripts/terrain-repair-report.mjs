import fs from "node:fs/promises";
import path from "node:path";
const root = path.resolve("docs/terrain-repair");
const read = async (p) =>
  JSON.parse(await fs.readFile(path.join(root, p), "utf8"));
const distribution = (a) => {
  a = [...a].sort((x, y) => x - y);
  return {
    min: a[0],
    median:
      (a[Math.floor((a.length - 1) / 2)] + a[Math.ceil((a.length - 1) / 2)]) /
      2,
    max: a.at(-1),
  };
};
const summary = {};
for (const name of ["baseline", "after"]) {
  const data = await read(`${name}/metrics.json`);
  {
    await fs.writeFile(
      path.join(root, `${name}/metrics.json`),
      JSON.stringify(data),
    );
    for (let i = 0; i < data.length; i++)
      await fs.writeFile(
        path.join(root, `${name}/seed-${String(i).padStart(2, "0")}.json`),
        JSON.stringify(data[i]),
      );
  }
  const biomeNames = [
    ...new Set(data.flatMap((s) => Object.keys(s.groups.land.biomes))),
  ].sort();
  summary[name] = {
    seeds: data.length,
    biomes: Object.fromEntries(
      biomeNames.map((b) => [
        b,
        distribution(data.map((s) => s.groups.land.biomes[b] || 0)),
      ]),
    ),
    steppeAlarm: data
      .filter((s) => s.groups.land.biomes.steppe > 0.45)
      .map((s) => s.spec.seed),
    drylandAlarm: data
      .filter(
        (s) =>
          (s.groups.land.biomes.steppe || 0) +
            (s.groups.land.biomes.desert || 0) >
          0.6,
      )
      .map((s) => s.spec.seed),
    rivers: distribution(data.map((s) => s.riverCount)),
    generationMs: distribution(
      data.map((s) =>
        Object.values(s.diagnostics.timingsMs).reduce((a, b) => a + b, 0),
      ),
    ),
    majorContinents: distribution(
      data.map((s) => s.diagnostics.majorContinents),
    ),
    moistureBudgetRelativeError: distribution(
      data.map((s) => s.diagnostics.moistureBudgetRelativeError),
    ),
    missingFields: [...new Set(data.flatMap((s) => s.missingFields))],
  };
  const shapes = data.flatMap((s) =>
    Object.values(s.componentShapes || {}).filter((c) => c.cells > 100),
  );
  if (shapes.length)
    summary[name].majorShapes = {
      count: shapes.length,
      compactness: distribution(shapes.map((s) => s.compactness)),
      aspectRatio: distribution(shapes.map((s) => s.aspectRatio)),
      orientationDegrees: distribution(shapes.map((s) => s.orientationDegrees)),
    };
}
summary.notes = [
  "Area weights use cos(latitude) on the 192×96 macro grid. Land includes inland lakes but excludes ocean.",
  "Shape measurements are macro-grid estimates; they are not fine coastline/gulf/peninsula measurements.",
  "Generation timings exclude tile rendering and run amid other test jobs; these are observed timings, not isolated benchmarks.",
  "Baseline binary was built before repair; after images and metrics share seed/spec but landforms intentionally differ.",
];
await fs.writeFile(
  path.join(root, "summary.json"),
  JSON.stringify(summary, null, 2),
);
const head =
  '<!doctype html><meta charset="utf-8"><title>KRIEMHILD terrain repair evidence</title><style>body{margin:24px;background:#102236;color:#eee;font:16px system-ui}a{color:#d3b974}section{display:grid;grid-template-columns:repeat(3,1fr);gap:12px}figure{margin:0;padding:8px;background:#1b3044}img{width:50%;image-rendering:auto}figcaption{margin-top:6px}h2{grid-column:1/-1}</style>';
let html =
  head +
  '<h1>Terrain repair — 30 seeds before and after</h1><p><a href="REPORT.md">Implementation and limitations</a> · <a href="summary.json">Summary metrics</a> · <a href="baseline/index.html">Full baseline gallery</a> · <a href="after/index.html">Full repaired gallery</a></p><p>Each pair joins the western and eastern hemispheres. The coastline, elevation, biome and two regional views are in the full galleries. No screenshot filtering or hand-picked seed replacement.</p><section>';
for (let i = 0; i < 30; i++) {
  const s = String(i).padStart(2, "0");
  html += `<figure><img src="baseline/${s}-natural-0.png"><img src="baseline/${s}-natural-1.png"><figcaption>${s} baseline</figcaption></figure><figure><img src="after/${s}-natural-0.png"><img src="after/${s}-natural-1.png"><figcaption>${s} repaired material</figcaption></figure><figure><img src="after/${s}-biome-0.png"><img src="after/${s}-biome-1.png"><figcaption>${s} actual biomes</figcaption></figure>`;
}
html +=
  "</section><h2>Special presets</h2><ul>" +
  ["archipelago", "rugged", "supercontinent", "arid", "ocean"]
    .map(
      (p) => `<li><a href="presets/${p}/index.html">${p}: two seeds</a></li>`,
    )
    .join("") +
  "</ul>";
await fs.writeFile(path.join(root, "index.html"), html);
console.log(JSON.stringify(summary, null, 2));
