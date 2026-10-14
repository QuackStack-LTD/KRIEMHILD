import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import { archivePixels } from "../src/continuous/archive-preview.js";

test("only the continuous editor implementation remains, with shared world pages preserved", () => {
  for (const file of [
    "src/TerrainApp.jsx",
    "src/views/GeneratorView.jsx",
    "src/views/BuilderView.jsx",
    "src/terrain/controller.js",
    "src/builder/map.js",
  ])
    assert.equal(fs.existsSync(file), false, file);
  for (const file of [
    "src/continuous/Studio.jsx",
    "src/worldbuilding/Library.jsx",
    "src/worldbuilding/Workspace.jsx",
    "src/worldbuilding/Profile.jsx",
    "src/worldbuilding/Writing.jsx",
    "src/worldbuilding/Biosphere.jsx",
  ])
    assert.ok(fs.existsSync(file), file);
  assert.ok(
    !fs
      .readFileSync("src/continuous/TerrainRouter.jsx", "utf8")
      .includes("LegacyTerrain"),
  );
});
test("archive preview reads the original saved cell colors including the unsigned high bit", () => {
  const types = Array.from({ length: 32 }, () => ({ color: "#000000" }));
  types[0].color = "#123456";
  types[31].color = "#abcdef";
  const original = { W: 2, H: 1, dom: [1, 0x80000000], config: { types } };
  const before = JSON.stringify(original);
  assert.deepEqual(
    [...archivePixels(original).pixels],
    [18, 52, 86, 255, 171, 205, 239, 255],
  );
  assert.equal(JSON.stringify(original), before);
  assert.throws(() => archivePixels({ W: Infinity, H: 1 }));
});
