import test from "node:test";
import assert from "node:assert/strict";
import { generationSpec } from "../src/continuous/seed.js";
import { biomeName } from "../src/continuous/biomes.js";
import { defaults, presets } from "../src/continuous/api.js";

test("new worlds use climate four without reducing the Earth-like rainfall baseline", () => {
  assert.equal(defaults.climateVersion, 4);
  assert.equal(defaults.rainfall, 1.65);
  assert.equal(presets.earthlike.rainfall, 1.65);
});

test("regenerating saved settings preserves omitted historical version zero", () => {
  assert.equal(
    generationSpec({ seed: "saved", version: "continuous-3" }).climateVersion,
    0,
  );
  for (const climateVersion of [0, 1, 2, 3, 4]) {
    assert.equal(
      generationSpec({ seed: "saved", climateVersion }).climateVersion,
      climateVersion,
    );
  }
});

test("version four vegetation names preserve persisted identifiers", () => {
  assert.equal(biomeName("forest", 4), "Temperate broadleaf and mixed forest");
  assert.equal(
    biomeName("tropical-woodland", 4),
    "Tropical seasonal forest and woodland",
  );
  assert.equal(biomeName("forest", 1), "forest");
});
