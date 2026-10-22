import test from "node:test";
import assert from "node:assert/strict";
import { requestTile } from "../src/continuous/tile-request.js";
const ok = () =>
  new Response(new Blob(["tile"]), { headers: { "X-Terrain-Revision": "3" } });
const config = { delays: [1, 2], timeout: 20 };
test("transient errors stop after three attempts; permanent errors never retry", async () => {
  for (const status of [503, 429, 408, 400, 404, 409]) {
    let calls = 0;
    globalThis.fetch = async () => {
      calls++;
      return new Response("", { status });
    };
    await assert.rejects(
      requestTile("/tile", 3, new AbortController().signal, config),
    );
    assert.equal(calls, [503, 429, 408].includes(status) ? 3 : 1);
  }
});
test("deadline aborts a stalled request, then recovers", async () => {
  let calls = 0;
  globalThis.fetch = (url, { signal }) =>
    ++calls === 1
      ? new Promise((resolve, reject) =>
          signal.addEventListener("abort", () => reject(signal.reason)),
        )
      : Promise.resolve(ok());
  assert.equal(
    (await requestTile("/tile", 3, new AbortController().signal, config)).size,
    4,
  );
  assert.equal(calls, 2);
});
test("obsolete work cancels backoff and never retries", async () => {
  const abort = new AbortController();
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return new Response("", { status: 503 });
  };
  const promise = requestTile("/tile", 3, abort.signal, {
    ...config,
    delays: [500, 500],
  });
  setTimeout(() => abort.abort(), 10);
  await assert.rejects(promise, { name: "AbortError" });
  assert.equal(calls, 1);
});
test("a response from another revision is never accepted", async () => {
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return ok();
  };
  await assert.rejects(
    requestTile("/tile", 4, new AbortController().signal, config),
    /revision mismatch/,
  );
  assert.equal(calls, 1);
});
