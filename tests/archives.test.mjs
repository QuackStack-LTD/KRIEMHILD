import test from "node:test";
import assert from "node:assert/strict";
import {
  archiveDefaults,
  matchesArchiveFilters,
} from "../src/worldbuilding/archives.js";
test("archive defaults adapt purpose without granting omniscient knowledge", () => {
  const personal = archiveDefaults({ id: "p", kind: "character" }),
    royal = archiveDefaults({ id: "k", kind: "kingdom" });
  assert.equal(personal.knowledgeScope, "direct");
  assert.equal(royal.knowledgeScope, "institutional");
  assert.equal(royal.ownerId, "k");
  assert.deepEqual(royal.sources, []);
  assert.equal(royal.intensity, "moderate");
  assert.ok(personal.categories.includes("genealogies"));
  personal.categories.push("extra");
  assert.equal(
    archiveDefaults({ kind: "character" }).categories.includes("extra"),
    false,
  );
});
test("archive filters distinguish covered dates from recording date including year zero", () => {
  const f = {
    record: {
      sourceAge: "old",
      category: "chronicles",
      relatedEntities: ["city"],
      fromYear: -2,
      toYear: 0,
      recordingYear: 5,
    },
  };
  const all = { age: "", type: "", entity: "", year: "", recorded: "" };
  assert.equal(
    matchesArchiveFilters(f, { ...all, year: "0", recorded: "5", age: "old" }),
    true,
  );
  assert.equal(matchesArchiveFilters(f, { ...all, recorded: "0" }), false);
  assert.equal(matchesArchiveFilters(f, { ...all, entity: "other" }), false);
  assert.equal(matchesArchiveFilters({}, all), true);
  assert.equal(matchesArchiveFilters({}, { ...all, year: "0" }), false);
});
