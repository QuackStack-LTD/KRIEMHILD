import test from "node:test";
import assert from "node:assert/strict";
import {
  blankDefinition,
  blankScript,
  definitionStats,
  diffDefinitions,
  metadataFor,
  retainLanguageAge,
  matchingCompile,
  emptyProductive,
} from "../src/worldbuilding/languages.js";
import { mergeConflicts } from "../src/history.js";
import { readFile } from "node:fs/promises";
const source = await readFile(
    new URL("../src/worldbuilding/model.js", import.meta.url),
    "utf8",
  ),
  registry = await readFile(
    new URL("../internal/history/registry.json", import.meta.url),
    "utf8",
  );
const { compareAge } = await import(
  "data:text/javascript;base64," +
    Buffer.from(
      source.replace(
        "import registry from '../../internal/history/registry.json';",
        `const registry=${registry};`,
      ),
    ).toString("base64")
);

test("language drafts do not fabricate inventories or defaults", () => {
  const d = blankDefinition("Authored");
  assert.match(d.languageId, /^[a-f0-9]{32}$/);
  assert.equal(d.phonology, null);
  assert.deepEqual(d.lexicon, []);
  assert.equal(d.orthography, null);
  assert.deepEqual(definitionStats(d), {
    segments: 0,
    lexemes: 0,
    paradigms: 0,
    constructions: 0,
  });
  assert.equal(blankScript("Writing").direction, "undecided");
});

test("productive drafts stay empty and compiler pins gate compiled readiness", () => {
  const model = emptyProductive();
  assert.deepEqual(model.features, []);
  assert.deepEqual(model.paradigms, []);
  assert.deepEqual(model.stratum.rules, []);
  const compiled = {
    revision: "retained-body",
    engine: "languages/2",
    adapter: "finite-compose/1",
    compiler: "languages-compiler/2",
  };
  assert.equal(matchingCompile(compiled, "retained-body", { schema: 2 }), true);
  assert.equal(
    matchingCompile(
      { ...compiled, compiler: "languages-compiler/future" },
      "retained-body",
      { schema: 2 },
    ),
    false,
  );
  assert.equal(matchingCompile(compiled, "changed-body", { schema: 2 }), false);
  assert.equal(
    matchingCompile(compiled, "retained-body", { schema: 1 }),
    false,
  );
  assert.equal(
    matchingCompile(
      {
        revision: "legacy",
        engine: "languages/1",
        adapter: "token-relation/1",
      },
      "legacy",
      { schema: 1 },
    ),
    true,
  );
});
test("a save response retains client Age selection without changing authoritative response data", () => {
  const response = {
    world: { currentAge: "server-age", revision: 8 },
    ages: [{ id: "server-age" }, { id: "selected-age" }],
  };
  assert.equal(
    retainLanguageAge(response, "selected-age").world.currentAge,
    "selected-age",
  );
  assert.equal(response.world.currentAge, "server-age");
  assert.equal(retainLanguageAge(response, "deleted-age"), response);
});
test("metadata copies cannot mutate another Age and comparisons expose definition changes", () => {
  const e = {
    language: { status: "living", links: [{ kind: "parent", id: "ancestor" }] },
  };
  const meta = metadataFor(e);
  meta.links[0].id = "changed";
  assert.equal(e.language.links[0].id, "ancestor");
  assert.deepEqual(
    diffDefinitions(
      { phonology: { rules: [] } },
      { phonology: { rules: [{ id: "rule" }] } },
    ),
    [
      {
        path: "definition.phonology.rules",
        before: [],
        after: [{ id: "rule" }],
      },
    ],
  );
});
test("Age comparisons and merge reviews include language and script states", () => {
  const entity = {
    id: "language",
    continuityId: "language",
    kind: "language",
    name: "Authored",
    properties: {},
    language: { draftRevision: "first" },
  };
  const doc = {
    world: {},
    ages: [{ id: "a" }, { id: "b", inheritedFrom: "a" }],
    entities: [
      { ...entity, ageId: "a" },
      { ...entity, ageId: "b", language: { draftRevision: "next" } },
    ],
    maps: [],
    representations: [],
    relations: [],
  };
  assert.equal(mergeConflicts(doc, "a", "b").length, 1);
  assert.equal(compareAge(doc, "b")[0].status, "modified");
});
