import test from "node:test";
import assert from "node:assert/strict";
import {
  applyWorldNameChange,
  rebaseProfileName,
} from "../src/worldbuilding/name-changes.js";
import { patchTerrainNames } from "../src/continuous/terrain-updates.js";

test("renames replace labels while preserving catalogue data, geometry and the selected Age", () => {
  const biology = [{ id: "bio" }],
    points = [[1, 2]],
    city = {
      id: "city",
      ageId: "first",
      name: "Old",
      properties: { notes: "Keep" },
    },
    otherAge = { ...city, ageId: "later", name: "Historic" },
    species = {
      id: "species",
      ageId: "first",
      biology: { traits: ["original"] },
    },
    doc = {
      world: { id: "world", revision: 1, currentAge: "later" },
      biospheres: biology,
      entities: [city, otherAge, species],
      representations: [
        { id: "rep", ageId: "first", shape: { name: "Old", points } },
      ],
      maps: [
        { id: "map", ageId: "first", name: "Old map", snapshot: "original" },
      ],
      terrains: [],
    },
    change = {
      base: 1,
      world: { id: "world", revision: 2, currentAge: "first" },
      entities: [
        {
          ...city,
          name: "New",
          properties: { notes: "Unrelated server field" },
        },
      ],
      representations: [
        {
          id: "rep",
          ageId: "first",
          shape: { name: "New", points: [[99, 99]] },
        },
      ],
      maps: [
        { id: "map", ageId: "first", name: "New map", snapshot: "changed" },
      ],
    };
  const next = applyWorldNameChange(doc, change);
  assert.equal(applyWorldNameChange(doc, undefined), null);
  assert.equal(next.world.currentAge, "later");
  assert.equal(next.entities[0].name, "New");
  assert.equal(next.entities[0].properties, city.properties);
  assert.equal(next.entities[1], otherAge);
  assert.equal(next.entities[2], species);
  assert.equal(next.biospheres, biology);
  assert.equal(next.representations[0].shape.points, points);
  assert.equal(next.maps[0].snapshot, "original");
  assert.equal(doc.entities[0].name, "Old");
  assert.equal(applyWorldNameChange(doc, { ...change, base: 3 }), null);
  assert.equal(
    applyWorldNameChange(next, {
      ...change,
      world: { ...change.world, revision: 1 },
    }),
    null,
  );
  assert.equal(
    applyWorldNameChange(doc, {
      ...change,
      world: { ...change.world, id: "other" },
    }),
    null,
  );
});

test("terrain name patches retain pending geometry, drafts and unchanged object identity", () => {
  const points = [[10, 10]],
    city = { id: "city", name: "Old", points },
    other = { id: "other", name: "Other" },
    draft = { id: "draft", name: "Pending", recordStatus: "draft" };
  const current = [city, other, draft],
    names = [
      { id: "city", name: "New", labelName: "Local label" },
      { id: "draft", name: "Published" },
    ];
  const next = patchTerrainNames(current, names);
  assert.equal(next[0].name, "New");
  assert.equal(next[0].points, points);
  assert.equal(next[1], other);
  assert.equal(next[2], draft);
  assert.equal(patchTerrainNames(next, names), next);
});

test("saved name metadata stays clean while genuine profile drafts survive renaming", () => {
  const old = {
      id: "city",
      name: "Old",
      updatedAt: 10,
      names: [{ type: "canonical", name: "Old" }],
      properties: { notes: "Old notes" },
    },
    next = {
      ...old,
      name: "New",
      updatedAt: 20,
      names: [{ type: "canonical", name: "New" }],
    };
  assert.equal(rebaseProfileName(old, old, next), next);
  const draft = {
    ...old,
    properties: { notes: "Pending notes" },
    names: [...old.names, { type: "local", name: "Local" }],
  };
  const changed = rebaseProfileName(draft, old, next);
  assert.equal(changed.name, "New");
  assert.equal(changed.updatedAt, 20);
  assert.equal(changed.properties, draft.properties);
  assert.deepEqual(changed.names, [
    ...next.names,
    { type: "local", name: "Local" },
  ]);
  assert.equal(
    rebaseProfileName({ ...draft, name: "Pending name" }, old, next).name,
    "Pending name",
  );
});
