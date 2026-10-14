import test from "node:test";
import assert from "node:assert/strict";
import { installAuthenticatedFetch } from "../src/auth/api.js";

test("map writes serialize fresh revisions while real conflicts remain visible", async () => {
  const previous = globalThis.window,
    previousLocation = globalThis.location;
  const calls = [];
  let revision = 0,
    active = 0,
    peak = 0;
  globalThis.location = {
    href: "http://localhost/",
    origin: "http://localhost",
  };
  globalThis.window = {
    dispatchEvent() {},
    fetch: async (input, init) => {
      if (input === "/api/sessions")
        return Response.json({ id: "map", mapRevision: 0 });
      active++;
      peak = Math.max(peak, active);
      const expected = Number(init.headers.get("If-Match"));
      calls.push(expected);
      await new Promise((resolve) => setTimeout(resolve, 5));
      active--;
      if (expected !== revision)
        return Response.json(
          { error: "external conflict" },
          { status: 409, headers: { "X-Map-Revision": String(revision) } },
        );
      return Response.json(
        {},
        { headers: { "X-Map-Revision": String(++revision) } },
      );
    },
  };
  try {
    installAuthenticatedFetch();
    await window.fetch("/api/sessions", { method: "POST" });
    const responses = await Promise.all(
      ["view", "step", "save"].map((path) =>
        window.fetch("/api/sessions/map/" + path, { method: "POST" }),
      ),
    );
    assert.deepEqual(calls, [0, 1, 2]);
    assert.equal(peak, 1);
    assert.ok(responses.every((r) => r.ok));
    revision++; // A different client changes the authoritative map.
    const conflict = await window.fetch("/api/sessions/map/paint", {
      method: "POST",
    });
    assert.equal(conflict.status, 409);
    assert.equal(calls.length, 4, "must not silently retry an edit");
  } finally {
    globalThis.window = previous;
    globalThis.location = previousLocation;
  }
});
