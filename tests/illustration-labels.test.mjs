import test from "node:test";
import assert from "node:assert/strict";
import {
  buildIllustration,
  newIllustration,
} from "../src/continuous/illustration.js";

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
