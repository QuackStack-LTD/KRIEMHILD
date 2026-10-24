import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { CartographicGeometry } from "../src/continuous/cartography.js";
import { pathText } from "../src/continuous/geographic-path.js";
import { prepareGeographicExport } from "../src/continuous/export-geography.js";
import { exportTerrain } from "../src/continuous/illustration-export.js";
import { mapProjection } from "../src/continuous/projection.js";

test("illustration exports fetch canonical tiles and paint both sides of the fitted seam", async (t) => {
  const dom = new JSDOM(
    '<svg xmlns="http://www.w3.org/2000/svg"><g aria-label="Continuous terrain tiles" data-terrain-style="atlas" /></svg>',
  );
  const prior = {
    document: globalThis.document,
    FileReader: globalThis.FileReader,
  };
  Object.assign(globalThis, {
    document: dom.window.document,
    FileReader: dom.window.FileReader,
  });
  t.after(() => {
    Object.assign(globalThis, prior);
    dom.window.close();
  });
  const camera = { x: 1.9, y: 0.3, w: 0.2, h: 0.2 },
    projection = mapProjection(camera, 256, 256),
    urls = [];
  const svg = document.querySelector("svg");
  await exportTerrain(svg, {
    camera,
    width: 256,
    projection,
    session: "fixture",
    revision: 0,
    identity: "seam-export",
    wrapLongitude: true,
    fetchTile: async (url) => {
      urls.push(url);
      return {
        ok: true,
        blob: async () => new dom.window.Blob(["synthetic-png"]),
      };
    },
  });
  const images = [...svg.querySelectorAll("image")];
  assert.ok(images.length > 1);
  assert.ok(
    images.some((e) => projection.inverse([+e.getAttribute("x"), 0])[0] >= 2),
  );
  assert.ok(
    images.some((e) => projection.inverse([+e.getAttribute("x"), 0])[0] < 2),
  );
  assert.ok(images.every((e) => e.getAttribute("href").startsWith("data:")));
  for (const url of urls) {
    const [, z, x] = url.match(/tiles\/(\d+)\/(\d+)\//);
    assert.ok(+x >= 0 && +x < 2 ** (+z + 1));
  }
});

test("map exports replace Fast simplification and omit editor feedback independently of preview quality", () => {
  const cache = new CartographicGeometry();
  const rivers = [
    {
      id: "r",
      class: "major",
      destination: "ocean",
      path: [
        [0, 0],
        [0.1, 0.0003],
        [0.2, 0],
      ],
    },
  ];
  const source = {
    identity: "world",
    revision: 1,
    rivers,
    coasts: [],
    level: 0,
  };
  const output = [];
  for (const fast of [true, false]) {
    const preview = cache.derive({ ...source, fast });
    const dom = new JSDOM('<svg xmlns="http://www.w3.org/2000/svg"></svg>');
    const svg = dom.window.document.querySelector("svg");
    svg.innerHTML =
      preview.rivers
        .map(
          (f) => `<path data-river-id="${f.id}" d="${pathText(f.points)}" />`,
        )
        .join("") + '<circle data-editor-handle="selection" />';
    prepareGeographicExport(svg, cache.derive({ ...source, fast: false }));
    assert.equal(svg.querySelector("circle"), null);
    output.push(svg.outerHTML);
    dom.window.close();
  }
  assert.equal(output[0], output[1]);
});
