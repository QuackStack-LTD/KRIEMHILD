import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { CartographicGeometry } from "../src/continuous/cartography.js";
import { pathText } from "../src/continuous/geographic-path.js";
import { prepareGeographicExport } from "../src/continuous/export-geography.js";

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
