import test from "node:test";
import assert from "node:assert/strict";
import {
  screenToMap,
  mapToScreen,
  FeatureIndex,
  anchor,
  contains,
} from "../src/continuous/geometry.js";
import {
  newIllustration,
  buildIllustration,
  scopeEntities,
} from "../src/continuous/illustration.js";
import { generationSpec } from "../src/continuous/seed.js";
import { riverFeatures, riverWidth } from "../src/continuous/rivers.js";
import { defaults } from "../src/continuous/api.js";
test("selection transforms survive letterboxing, pan, resize and high DPI at every zoom", () => {
  for (const w of [2, 0.5, 0.01, 0.00008])
    for (const rect of [
      { left: 42, top: 70, width: 892, height: 695 },
      { left: 0, top: 0, width: 1800, height: 750 },
    ])
      for (const dpr of [1, 2, 3]) {
        const camera = { x: 0.7, y: 0.3, w, h: w / 2 },
          p = [camera.x + w * 0.42, camera.y + w * 0.19];
        const q = screenToMap(...mapToScreen(p, rect, camera), rect, camera);
        assert.ok(Math.hypot(q[0] - p[0], q[1] - p[1]) < 1e-12, `DPR ${dpr}`);
      }
});
test("geometry index prioritizes points and thin lines over containing polygons and rebuilds after edits", () => {
  const country = {
      id: "country",
      kind: "country",
      geometry: "polygon",
      points: [
        [0, 0],
        [2, 0],
        [2, 1],
        [0, 1],
      ],
    },
    road = {
      id: "road",
      geometry: "line",
      points: [
        [0.4, 0.5],
        [1.5, 0.5],
      ],
    },
    house = {
      id: "house",
      kind: "house",
      geometry: "point",
      points: [[1, 0.5]],
    };
  let index = new FeatureIndex([country, road, house]);
  assert.deepEqual(
    index.pick([1, 0.501], 0.005).map((f) => f.id),
    ["house", "road", "country"],
  );
  assert.equal(index.pick([1, 0.8], 0.005)[0].id, "country");
  index = new FeatureIndex([country, { ...house, points: [[1.2, 0.6]] }]);
  assert.equal(index.pick([1.2, 0.6], 0.001)[0].id, "house");
  assert.equal(index.pick([1, 0.5], 0.001)[0].id, "country");
});
test("optional seeds are fresh while explicit text and input state remain exact", () => {
  assert.equal(defaults.seed, "");
  const blank = { ...defaults },
    a = generationSpec(blank),
    b = generationSpec(blank);
  assert.notEqual(a.seed, b.seed);
  assert.equal(blank.seed, "");
  assert.equal(generationSpec({ ...blank, seed: "  001 λ " }).seed, "  001 λ ");
});
const state = { W: 201, H: 101 };
const entities = [
  {
    id: "country",
    kind: "country",
    geometry: "polygon",
    points: [
      [20, 20],
      [180, 20],
      [180, 80],
      [20, 80],
    ],
    name: "Northern Realm",
  },
  {
    id: "capital",
    kind: "capital",
    geometry: "point",
    points: [[70, 40]],
    name: "High Hall",
    parentId: "country",
  },
  {
    id: "city",
    kind: "city",
    geometry: "point",
    points: [[100, 50]],
    name: "River Town",
  },
  {
    id: "road",
    kind: "road",
    geometry: "line",
    points: [
      [70, 40],
      [100, 50],
    ],
    name: "Road",
  },
  {
    id: "house",
    kind: "house",
    geometry: "polygon",
    points: [
      [98, 49],
      [99, 49],
      [99, 50],
      [98, 50],
    ],
    name: "House",
  },
];
test("one illustration scene filters world/country/city and keeps manual labels linked after rename", () => {
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  assert.deepEqual(
    scopeEntities(entities, state, config).map((e) => e.id),
    ["country", "capital"],
  );
  const country = { ...config, scope: "country", focus: "country" };
  assert.ok(
    scopeEntities(entities, state, country).some((e) => e.id === "road"),
  );
  assert.ok(
    !scopeEntities(entities, state, country).some((e) => e.id === "house"),
  );
  assert.ok(
    scopeEntities(entities, state, { ...config, scope: "city" }).some(
      (e) => e.id === "house",
    ),
  );
  let scene = buildIllustration(config, entities, [], null, state);
  const label = scene.find((o) => o.id === "label-country");
  assert.ok(
    contains(
      label.position,
      entities[0].points.map((p) => [p[0] / 100, p[1] / 100]),
    ),
  );
  config.objects[label.id] = {
    id: label.id,
    layer: "labels",
    kind: "text",
    source: "country",
    position: [0.9, 0.4],
  };
  scene = buildIllustration(
    config,
    entities.map((e) =>
      e.id === "country" ? { ...e, name: "Renamed Realm" } : e,
    ),
    [],
    null,
    state,
  );
  assert.equal(scene.find((o) => o.id === label.id).text, "Renamed Realm");
  assert.deepEqual(scene.find((o) => o.id === label.id).position, [0.9, 0.4]);
});
test("concave country labels use interior scanline anchors", () => {
  const polygon = [
    [0, 0],
    [1, 0],
    [1, 0.2],
    [0.2, 0.2],
    [0.2, 1],
    [0, 1],
  ];
  assert.ok(contains(anchor(polygon, "polygon"), polygon));
});
test("river seam splits and optical width retain source identity and width hierarchy", () => {
  const river = {
    id: "river-1",
    path: [
      [3.1, 0.2],
      [-3.1, 0.2],
      [-3, 0.1],
    ],
    width: [20, 30, 40],
    discharge: [10, 20, 30],
  };
  const shapes = riverFeatures([river]);
  assert.equal(shapes.length, 1);
  assert.equal(shapes[0].sourceId, river.id);
  assert.ok(
    riverWidth({ ...river, width: [400], discharge: [2000] }, 0.001, 6371) >
      riverWidth(river, 0.001, 6371),
  );
});
