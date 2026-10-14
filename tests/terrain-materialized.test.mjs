import test from "node:test";
import assert from "node:assert/strict";
import {
  buildIllustration,
  newIllustration,
} from "../src/continuous/illustration.js";
import { riverVisible } from "../src/continuous/rivers.js";

test("illustration uses saved shoreline and follows moved country and settlement geometry", () => {
  const config = newIllustration("world", { x: 0, y: 0, w: 2, h: 1 });
  const country = {
    id: "country",
    name: "Country",
    kind: "country",
    geometry: "polygon",
    points: [
      [20, 20],
      [80, 20],
      [80, 60],
      [20, 60],
    ],
  };
  const city = {
    id: "city",
    name: "City",
    kind: "capital",
    geometry: "point",
    points: [[40, 40]],
  };
  config.objects.country = {
    points: [
      [0, 0],
      [1, 0],
      [1, 1],
    ],
  };
  config.objects.city = { position: [1.9, 0.9] };
  const coasts = [
    [
      [-1, 0],
      [-0.5, 0.5],
      [0, 0],
      [-1, 0],
    ],
  ];
  const build = () =>
    buildIllustration(
      config,
      [country, city],
      [],
      null,
      { W: 101, H: 101 },
      coasts,
    );
  let scene = build();
  assert.deepEqual(scene.find((o) => o.id === "country").points, [
    [0.4, 0.2],
    [1.6, 0.2],
    [1.6, 0.6],
    [0.4, 0.6],
  ]);
  assert.deepEqual(scene.find((o) => o.id === "city").position, [0.8, 0.4]);
  assert.deepEqual(
    scene.find((o) => o.id === "shore-0").points,
    coasts[0].map((p) => [p[0] / Math.PI + 1, 0.5 - p[1] / Math.PI]),
  );
  country.points = [
    [30, 20],
    [90, 20],
    [90, 60],
    [30, 60],
  ];
  scene = build();
  assert.equal(scene.find((o) => o.id === "country").points[0][0], 0.6);
});

test("whole-world trunk visibility uses basin classification even with small headwaters", () => {
  assert.equal(
    riverVisible({ class: "major", discharge: [1, 8000] }, "world"),
    true,
  );
  assert.equal(
    riverVisible({ class: "regional", discharge: [1, 800] }, "continent"),
    true,
  );
  assert.equal(
    riverVisible({ class: "stream", discharge: [1, 20] }, "world"),
    false,
  );
  assert.equal(
    riverVisible({ class: "ephemeral", discharge: [1, 20] }, "city"),
    true,
  );
});
