import test from "node:test";
import assert from "node:assert/strict";
import {
  blankGlyph,
  blankPath,
  safeGlyph,
  boundedHistory,
  VECTOR_VERSIONS,
} from "../src/worldbuilding/vector-script.js";
import {
  definitionVersions,
  matchingCompile,
} from "../src/worldbuilding/languages.js";
test("custom vector profiles stay distinct from A1 and preserve stable artwork identity", () => {
  const glyph = blankGlyph("authored-id"),
    before = structuredClone(glyph);
  glyph.name = "Renamed";
  assert.equal(glyph.id, before.id);
  assert.equal(glyph.paths.length, 0);
  assert.deepEqual(
    definitionVersions({ schema: 2, scriptId: "script" }),
    VECTOR_VERSIONS,
  );
  assert.equal(
    definitionVersions({ schema: 2, languageId: "language" }).engine,
    "languages/2",
  );
  assert.equal(
    definitionVersions({ schema: 3, languageId: "language" }).engine,
    "languages/3",
  );
  assert.equal(
    matchingCompile({ revision: "hash", ...VECTOR_VERSIONS }, "hash", {
      schema: 2,
      scriptId: "script",
    }),
    true,
  );
  assert.equal(
    matchingCompile({ revision: "hash", ...VECTOR_VERSIONS }, "hash", {
      schema: 2,
      languageId: "language",
    }),
    false,
  );
});
test("local geometry preview cannot introduce SVG resources or executable markup", () => {
  const glyph = blankGlyph("g");
  glyph.paths = [
    {
      ...blankPath("p"),
      commands: [
        { op: "M", values: [0, 0] },
        { op: "L", values: [20, 20] },
      ],
    },
  ];
  assert.equal(safeGlyph(glyph), true);
  for (const patch of [
    { fill: "url(https://example.com/a)" },
    { stroke: "javascript:alert(1)" },
    { strokeWidth: Infinity },
    { commands: [{ op: "image", values: [] }] },
    { commands: [{ op: "M", values: [0, NaN] }] },
  ])
    assert.equal(
      safeGlyph({ ...glyph, paths: [{ ...glyph.paths[0], ...patch }] }),
      false,
    );
  const history = boundedHistory(
    Array.from({ length: 80 }, (_, i) => ({
      revision: i,
      geometry: "x".repeat(100),
    })),
    1000,
  );
  assert.ok(history.length < 32);
  assert.equal(history.at(-1).revision, 79);
  assert.ok(
    history.reduce((sum, s) => sum + JSON.stringify(s).length * 2, 0) <= 1000,
  );
});
