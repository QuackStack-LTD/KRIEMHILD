import test from "node:test";
import assert from "node:assert/strict";
import { parseRoute } from "../src/project.js";

test("project routes retain stable identity across both editors", () => {
  const id = "0123456789abcdef0123456789abcdef";
  assert.deepEqual(parseRoute("/"), { view: "home" });
  assert.deepEqual(parseRoute("/world/new"), { view: "generate", fresh: true });
  for (const view of ["generate", "build"])
    assert.deepEqual(parseRoute(`/world/${id}/${view}`), { id, view });
  for (const url of [
    "/world/unknown/build",
    `/world/${id}/delete`,
    "/assets/missing.js",
  ])
    assert.equal(parseRoute(url).view, "missing");
});
