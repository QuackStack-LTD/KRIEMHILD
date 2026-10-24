import test from "node:test";
import assert from "node:assert/strict";
import {
  buildIllustration,
  newIllustration,
} from "../src/continuous/illustration.js";
import { landPaths, selectedLand } from "../src/continuous/land-selection.js";

const state = { W: 192, H: 96 };
const country = {
  id: "country",
  name: "Zenon",
  kind: "country",
  geometry: "polygon",
  points: [
    [101.80218001325028, 49.466027819891885],
    [101.93440322092371, 49.80581578089669],
    [102.3420905411973, 49.751010923190236],
    [102.29801613863948, 49.433145384551395],
    [101.93440322092371, 49.345458091504455],
    [101.81319891501845, 49.45506720781314],
  ],
};

test("countries, names and rivers follow the fitted longitude branch without moving source data", () => {
  const fit = selectedLand(
    [
      {
        id: "seam",
        paths: landPaths([
          [
            [1.85, 0.3],
            [0.15, 0.3],
            [0.15, 0.6],
            [1.85, 0.6],
            [1.85, 0.3],
          ],
        ]),
      },
    ],
    ["seam"],
  );
  const config = {
    ...newIllustration("continent", fit.bounds),
    regions: fit.regions,
  };
  const local = { W: 201, H: 101 };
  const realm = {
    id: "seam-state",
    name: "Seam Realm",
    kind: "country",
    geometry: "polygon",
    points: [
      [198, 40],
      [2, 40],
      [2, 50],
      [198, 50],
    ],
  };
  const rivers = [
    {
      id: "crossing",
      class: "major",
      path: [
        [0.98 * Math.PI, 0],
        [-0.98 * Math.PI, 0.01],
      ],
      name: "Seam River",
    },
  ];
  const before = structuredClone({ realm, rivers });
  const scene = buildIllustration(config, [realm], rivers, null, local);
  const polygon = scene.find((o) => o.id === realm.id);
  assert.ok(polygon);
  assert.ok(polygon.points.every((p) => p[0] >= 1.98 && p[0] <= 2.02));
  assert.ok(scene.find((o) => o.id === "label-seam-state" && !o.hidden));
  const reaches = scene.filter((o) => o.river);
  assert.equal(reaches.length, 2);
  assert.equal(reaches[0].points.at(-1)[0], reaches[1].points[0][0]);
  assert.ok(
    reaches.every((o) => o.points.every((p) => p[0] >= 1.98 && p[0] <= 2.02)),
  );
  assert.deepEqual({ realm, rivers }, before);
});

test("relief from both mesh branches meets without duplicate symbols at their shared seam", () => {
  const config = {
    ...newIllustration("continent", { x: 1.85, y: 0.3, w: 0.3, h: 0.4 }),
    regions: [
      [
        [1.9, 0.3],
        [2.1, 0.3],
        [2.1, 0.7],
        [1.9, 0.7],
        [1.9, 0.3],
      ],
    ],
  };
  const mesh = (west, east) => ({
    bounds: [west, -0.5, east, 0.5],
    columns: 8,
    rows: 64,
    bed: Array(512).fill(2000),
    wet: Array(512).fill(false),
  });
  const meshes = [mesh(0.9 * Math.PI, Math.PI), mesh(-Math.PI, -0.9 * Math.PI)];
  const scene = buildIllustration(config, [], [], meshes, state, []);
  assert.ok(scene.some((o) => o.symbol === "mountain" && o.position[0] < 2));
  assert.ok(scene.some((o) => o.symbol === "mountain" && o.position[0] > 2));
  assert.equal(new Set(scene.map((o) => o.id)).size, scene.length);
});
test("a small country's name stays legible with a leader at world scale", () => {
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  const before = structuredClone(country);
  const scene = buildIllustration(config, [country], [], null, state);
  const label = scene.find((o) => o.id === "label-country");
  assert.equal(label.text, "Zenon");
  assert.ok(!label.hidden);
  assert.ok(label.size >= (2 / 1200) * 9);
  assert.ok(label.leaderFrom);
  assert.deepEqual(country, before);
  config.objects[label.id] = { hidden: true };
  assert.equal(
    buildIllustration(config, [country], [], null, state).find(
      (o) => o.id === label.id,
    ).hidden,
    true,
  );
});
test("major cities and countries retain visible names at edges and in crowded areas", () => {
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  const cities = Array.from({ length: 12 }, (_, i) => ({
    id: `city-${i}`,
    name: `City ${i}`,
    kind: "city",
    geometry: "point",
    points: [[i < 6 ? 0.001 : 190.999, 48 + i * 0.001]],
  }));
  const scene = buildIllustration(
    config,
    [country, ...cities],
    [],
    null,
    state,
  );
  for (const city of cities) {
    const label = scene.find((o) => o.id === `label-${city.id}`);
    assert.ok(label && !label.hidden);
    assert.ok(label.position[0] >= 0 && label.position[0] <= 2);
  }
  assert.deepEqual(
    scene,
    buildIllustration(config, [country, ...cities], [], null, state),
  );
});
test("manual country label placement and text survive automatic fallback", () => {
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  config.objects["label-country"] = {
    position: [1.1, 0.4],
    text: "The Realm",
    size: 0.02,
  };
  const label = buildIllustration(config, [country], [], null, state).find(
    (o) => o.id === "label-country",
  );
  assert.deepEqual(label.position, [1.1, 0.4]);
  assert.equal(label.text, "The Realm");
  assert.equal(label.size, 0.02);
});
