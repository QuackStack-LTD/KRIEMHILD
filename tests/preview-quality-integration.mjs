import { chromium } from "@playwright/test";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import assert from "node:assert/strict";
const out = "docs/preview-climate-investigation/preview-integration",
  base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8127";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
const page = await browser.newPage({
  viewport: { width: 1280, height: 800 },
  deviceScaleFactor: 3,
});
const result = { stages: [], exports: [] },
  errors = [];
let generationJobs=0;
page.on('request',r=>{if(r.method()==='POST'&&r.url().endsWith('/terrain/generation-jobs'))generationJobs++;});
page.on("pageerror", (e) => errors.push(e.message));
const quality = () => page.getByLabel("Preview Quality", { exact: true });
const ready = async () => {
  await page.waitForTimeout(150);
  await page.waitForFunction(
    () => {
      const l = window.__terrainViewportLog || [],
        r = l.findLast((e) => e.event === "request"),
        g = document.querySelector('[aria-label="Committed terrain viewport"]');
      return (
        r &&
        g?.dataset.phase === "final" &&
        +g.dataset.generation === r.generation &&
        document.querySelector("footer")?.textContent.endsWith("Ready")
      );
    },
    null,
    { timeout: 180000 },
  );
};
const snapshot = async (name) => {
  const s = await page.evaluate(() => ({
    camera: document.querySelector(".studio-stage svg").getAttribute("viewBox"),
    request: window.__terrainViewportLog.findLast((e) => e.event === "request"),
    images: [
      ...document.querySelectorAll(
        '[aria-label="Committed terrain viewport"] image',
      ),
    ].map((i) => ({
      ...i.dataset,
      x: i.getAttribute("x"),
      y: i.getAttribute("y"),
      w: i.getAttribute("width"),
      h: i.getAttribute("height"),
    })),
  }));
  assert.equal(new Set(s.images.map((i) => i.generation)).size, 1);
  assert.ok(s.images.every((i) => +i.revision === s.request.revision));
  result.stages.push({ name, ...s });
  return s;
};
try {
  await page.goto(
    base + "/world/977d134ee5aadc480e1410ade3c84587/build?tileDebug=1",
  );
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  await page.getByLabel("Autosave accepted terrain after 30 seconds").uncheck();
  await page.getByRole("button", { name: "Map Builder", exact: true }).click();
  await page
    .locator("select")
    .filter({ hasText: "watersheds" })
    .selectOption("natural");
  await page.evaluate(() =>
    window.dispatchEvent(
      new CustomEvent("terrain-diagnostic-camera", {
        detail: { x: 1.285, y: 0.2125, w: 0.03, h: 0.015 },
      }),
    ),
  );
  await ready();
  const initial = await snapshot("baseline");
  const sampleURL=`${base}/api/sessions/${initial.request.session}/terrain/sample?lon=54&lat=50.4`;
  const initialSample=await(await page.request.get(sampleURL)).json();
  await page.getByLabel("Export width").selectOption("1024");
  for (const preset of ["performance", "high", "balanced"]) {
    await quality().selectOption(preset);
    await ready();
    const s = await snapshot(preset);
    assert.equal(s.camera, initial.camera);
    assert.deepEqual(await(await page.request.get(sampleURL)).json(),initialSample,'preview quality changed authoritative terrain or climate');
    await page.screenshot({ path: `${out}/${preset}-preview.png` });
    for (const format of ["SVG", "PNG"]) {
      const promise = page.waitForEvent("download", { timeout: 120000 });
      await page.getByRole("button", { name: format, exact: true }).click();
      const download = await promise;
      const path = `${out}/${preset}.${format.toLowerCase()}`;
      await download.saveAs(path);
      const bytes = await readFile(path);
      if (format === "PNG") {
        assert.equal(bytes.readUInt32BE(16), 1024);
        assert.equal(bytes.readUInt32BE(20), 512);
      }
      result.exports.push({
        preset,
        format,
        path,
        bytes: bytes.length,
        sha256: createHash("sha256").update(bytes).digest("hex"),
      });
    }
  }
  const pngs = result.exports.filter((x) => x.format === "PNG");
  assert.equal(
    new Set(pngs.map((x) => x.sha256)).size,
    1,
    "PNG export changed with preview quality",
  );
  // SVG image insertion order can vary with network completion. Compare decoded
  // output exactly via PNG above; transient preview attributes must be absent.
  for (const e of result.exports.filter((x) => x.format === "SVG"))
    assert.doesNotMatch(
      await readFile(e.path, "utf8"),
      /data-pixel-width|data-pixel-height|data-generation/,
    );
  await quality().selectOption("performance");
  await ready();
  await page.reload();
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  assert.equal(await quality().inputValue(), "performance");
  await snapshot("preference-reopened");
  await page.getByLabel("Autosave accepted terrain after 30 seconds").uncheck();
  let release, heldResolve;
  const usedStyles=await page.evaluate(()=>[...new Set(window.__terrainViewportLog.filter(e=>e.event==='request').flatMap(e=>e.styles))]);
  const delayedStyle=['exposure','rainShadow','soil','pet','slope','moisture'].find(s=>!usedStyles.includes(s));
  assert.ok(delayedStyle,'pending-render fixture needs an uncached style');
  const held = new Promise((r) => (heldResolve = r)),
    gate = new Promise((r) => (release = r));
  let heldOnce = false;
  await page.route("**/terrain/tiles/**", async (route) => {
    if (!heldOnce && route.request().url().includes(`style=${delayedStyle}`)) {
      heldOnce = true;
      const response = await route.fetch();
      heldResolve();
      await gate;
      try {
        await route.fulfill({ response });
      } catch {}
      return;
    }
    await route.continue();
  });
  await page
    .locator("select")
    .filter({ hasText: "watersheds" })
    .selectOption(delayedStyle);
  await Promise.race([held,new Promise((_,reject)=>setTimeout(()=>reject(Error('delay fixture did not intercept an uncached request')),30000))]);
  await quality().selectOption("high");
  await page
    .getByRole("button", { name: "Map Builder", exact: true })
    .click();
  await page.getByRole("button", { name: /^raise$/i }).click();
  const box = await page.locator(".studio-stage svg").boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  release();
  await ready();
  const edited = await snapshot("edit-and-quality-while-pending");
  assert.ok(
    edited.request.revision > initial.request.revision,
    "terrain edit did not advance revision",
  );
  assert.equal(edited.request.preview.preset, "high");
  await page.getByRole("button", { name: "Undo", exact: true }).click();
  await ready();
  await snapshot("undo");
  await page.getByRole("button", { name: "Redo", exact: true }).click();
  await ready();
  await snapshot("redo");
  assert.deepEqual(errors, []);
  assert.equal(generationJobs,0,'preview quality regenerated terrain');
  result.generationJobs=generationJobs;
  result.status = "complete";
} catch (e) {
  result.status = "failed";
  result.error = e.stack;
  throw e;
} finally {
  result.errors = errors;
  await writeFile(out + "/results.json", JSON.stringify(result, null, 2));
  await browser.close();
}
