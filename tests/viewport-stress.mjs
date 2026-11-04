// Disposable fixture only. Faults, every style, illustration, DPI and raster fidelity.
import { chromium } from "@playwright/test";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import assert from "node:assert/strict";
const base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8125";
const world =
  process.env.KRIEMHILD_AUDIT_WORLD ||
  JSON.parse(
    await readFile("docs/viewport-investigation/baseline/new.json", "utf8"),
  ).world;
const out =
  process.env.KRIEMHILD_AUDIT_OUT || "docs/viewport-investigation/stress";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } }),
  results = {},
  errors = [];
const styleSelect = page.locator("select").filter({ hasText: "watersheds" });
page.on("pageerror", (e) => errors.push(e.message));
page.on("dialog", (d) => d.accept());
async function ready(p = page) {
  await p.waitForFunction(
    () =>
      document.querySelector("footer")?.innerText.endsWith("Ready") &&
      document.querySelector('[data-phase="final"]'),
    null,
    { timeout: 180000 },
  );
  await p.waitForLoadState("networkidle", { timeout: 180000 });
}
async function capture(name, p = page) {
  await ready(p);
  const s = await p.evaluate(() => {
    const g = document.querySelector(
      '[aria-label="Committed terrain viewport"]',
    );
    const images = [...g.querySelectorAll("image[data-generation]")].map(
      (i) => ({
        generation: i.dataset.generation,
        revision: i.dataset.revision,
        style: i.parentElement.dataset.terrainStyle,
        bounds: ["x", "y", "width", "height"].map((a) => i.getAttribute(a)),
      }),
    );
    return {
      generation: g.dataset.generation,
      revision: g.dataset.revision,
      phase: g.dataset.phase,
      images,
      log: window.__terrainViewportLog,
    };
  });
  assert.ok(s.images.length);
  assert.ok(
    s.images.every(
      (i) => i.generation === s.generation && i.revision === s.revision,
    ),
  );
  assert.ok(
    s.images.every(
      (i) => JSON.stringify(i.bounds) === JSON.stringify(s.images[0].bounds),
    ),
  );
  await p.screenshot({ path: `${out}/${name}.png` });
  results[name] = s;
  console.log(name, "atomic");
}
try {
  await page.goto(`${base}/world/${world}/build?terrainRenderer=viewport`);
  await ready();
  console.log("loaded");
  const faultStyle =
    (await styleSelect.inputValue()) === "watersheds"
      ? "exposure"
      : "watersheds";
  await styleSelect.selectOption("natural");
  await ready();
  await page.getByRole("button", { name: "Fit world", exact: true }).click();
  await ready();
  // Exhaust bounded retries on every request for a previously unused style.
  let attempts = 0,
    fail = true;
  await page.route("**/terrain/tiles/**", async (route) => {
    if (fail && route.request().url().includes(`style=${faultStyle}`)) {
      attempts++;
      return route.fulfill({ status: 503, body: "injected failure" });
    }
    return route.continue();
  });
  const previous = await page
    .locator('[aria-label="Committed terrain viewport"]')
    .getAttribute("data-generation");
  await styleSelect.selectOption(faultStyle);
  console.log("fault armed", faultStyle);
  await page
    .getByRole("button", { name: "Retry terrain", exact: true })
    .waitFor({ timeout: 20000 });
  // A stable failure must not trigger more camera requests as controls appear.
  const failedAttempts = attempts;
  await page.waitForTimeout(2000);
  await page
    .getByRole("button", { name: "Retry terrain", exact: true })
    .waitFor({ timeout: 20000 });
  assert.equal(
    attempts,
    failedAttempts,
    "failure controls caused automatic render retries",
  );
  assert.equal(
    await page
      .locator('[aria-label="Committed terrain viewport"]')
      .getAttribute("data-generation"),
    previous,
  );
  assert.equal(
    await page.locator('[data-terrain-style="natural"] image').count(),
    1,
    "last valid base image must survive style failure",
  );
  results.failure = { attempts, previous, retained: true };
  await page.screenshot({ path: `${out}/failure-retained.png` });
  fail = false;
  await page
    .getByRole("button", { name: "Retry terrain", exact: true })
    .click();
  await capture("retry");
  await page.unrouteAll({ behavior: "wait" });
  // Rapid style changes exercise cancellation and late-completion rejection.
  for (const style of ["temperature", "biome", "elevation", "natural"]) {
    await styleSelect.selectOption(style);
    await page.waitForTimeout(100);
  }
  await capture("rapid-latest");
  assert.equal(results["rapid-latest"].images[0].style, "natural");
  const styles = await styleSelect
    .locator("option")
    .evaluateAll((o) => o.map((e) => e.value));
  for (const style of styles) {
    await styleSelect.selectOption(style);
    await ready();
    assert.equal(
      await page.locator(`[data-terrain-style="${style}"] image`).count(),
      1,
    );
  }
  results.styles = styles;
  // Saved new-world previews may reopen in Terrain Generator mode. Illustration
  // controls belong to Map Builder; select that workspace explicitly.
  await page.getByRole("button", { name: "Map Builder", exact: true }).click();
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  await capture("illustration");
  await page
    .getByRole("button", { name: "Construction mode", exact: true })
    .click();
  await ready();
  // Import the exact source module in the browser for real Canvas pixel tests.
  await page.route("**/__viewport-test/*.js", async (route) => {
    const file = new URL(route.request().url()).pathname.split("/").pop();
    await route.fulfill({
      contentType: "text/javascript",
      body: await readFile(`src/continuous/${file}`, "utf8"),
    });
  });
  results.pixels = await page.evaluate(async () => {
    const { TerrainPyramid, renderViewport } = await import(
      "/__viewport-test/terrain-pyramid.js"
    );
    const pattern = (x, y) => {
      const line = Math.abs(y - 0.36 * x - 0.18) < 0.008,
        land = (x - 1) ** 2 + (y - 0.5) ** 2 < 0.18;
      return line
        ? [30, 90, 180, 255]
        : land
          ? [Math.round(x * 100), 170, 60, 255]
          : [20, 50, 100, 255];
    };
    async function source(z, x, y, size) {
      const c = new OffscreenCanvas(size, size),
        ctx = c.getContext("2d"),
        data = ctx.createImageData(size, size);
      for (let j = 0; j < size; j++)
        for (let i = 0; i < size; i++)
          data.data.set(
            pattern(
              (x + (i + 0.5) / size) / 2 ** z,
              (y + (j + 0.5) / size) / 2 ** z,
            ),
            (j * size + i) * 4,
          );
      ctx.putImageData(data, 0, 0);
      return c.convertToBlob();
    }
    const p = new TerrainPyramid({
        fetchTile: async (url) => {
          const m = url.match(/tiles\/(\d+)\/(\d+)\/(\d+)/);
          return source(+m[1], +m[2], +m[3], 64);
        },
      }),
      cases = [];
    for (const bounds of [
      { x: 0, y: 0, w: 2, h: 1 },
      { x: 0.33, y: 0.21, w: 0.9, h: 0.6 },
      { x: -0.1, y: -0.15, w: 1.4, h: 1.2 },
    ]) {
      const r = {
        identity: "synthetic",
        session: "synthetic",
        revision: 0,
        invalidations: [],
        generation: 1,
        z: 2,
        size: 64,
        styles: ["natural"],
        bounds,
        pixels: { width: 360, height: 240 },
      };
      let f;
      await renderViewport(p, r, new AbortController().signal, {
        onFrame: (value) => {
          f = value;
        },
      });
      // Generate ONE continuous raster over the same absolute source extent.
      // Skia's fractional interpolation rounds differently when the source
      // origin/extent changes; that independent comparison is archived separately.
      const left = Math.max(0, Math.floor(bounds.x * 4)) * 64,
        top = Math.max(0, Math.floor(bounds.y * 4)) * 64,
        right = Math.min(8, Math.ceil((bounds.x + bounds.w) * 4)) * 64,
        bottom = Math.min(4, Math.ceil((bounds.y + bounds.h) * 4)) * 64,
        rw = right - left,
        rh = bottom - top;
      const reference = new OffscreenCanvas(rw, rh),
        ctx = reference.getContext("2d", { willReadFrequently: true }),
        data = ctx.createImageData(rw, rh);
      for (let j = 0; j < rh; j++)
        for (let i = 0; i < rw; i++)
          data.data.set(
            pattern((left + i + 0.5) / 256, (top + j + 0.5) / 256),
            (j * rw + i) * 4,
          );
      ctx.putImageData(data, 0, 0);
      const expected = new OffscreenCanvas(360, 240),
        e = expected.getContext("2d", { willReadFrequently: true });
      e.drawImage(
        reference,
        ((left / 256 - bounds.x) / bounds.w) * 360,
        ((top / 256 - bounds.y) / bounds.h) * 240,
        (rw / 256 / bounds.w) * 360,
        (rh / 256 / bounds.h) * 240,
      );
      const actual = new OffscreenCanvas(360, 240),
        a = actual.getContext("2d", { willReadFrequently: true }),
        bitmap = await createImageBitmap(f.images[0].blob);
      a.drawImage(bitmap, 0, 0);
      bitmap.close();
      const aa = a.getImageData(0, 0, 360, 240).data,
        ee = e.getImageData(0, 0, 360, 240).data;
      let max = 0,
        different = 0;
      for (let i = 0; i < aa.length; i++) {
        max = Math.max(max, Math.abs(aa[i] - ee[i]));
        if (aa[i] !== ee[i]) different++;
      }
      cases.push({ bounds, max, different });
    }
    return cases;
  });
  assert.ok(
    results.pixels.every((c) => c.max <= 1),
    "tile atlas differs from monolithic authoritative pattern",
  );
  await page.close(); // Each camera fixture is the sole editor of this saved view.
  for (const dpr of [2, 3]) {
    const p = await browser.newPage({
      viewport: { width: 1920, height: 1080 },
      deviceScaleFactor: dpr,
    });
    p.on("pageerror", (e) => errors.push(e.message));
    await p.goto(`${base}/world/${world}/build?terrainRenderer=viewport`);
    try {
      await ready(p);
      await capture(`dpi-${dpr}`, p);
    } catch (e) {
      results[`dpi-${dpr}-failure`] = {
        body: await p.locator("body").innerText(),
        log: await p.evaluate(() => window.__terrainViewportLog),
      };
      throw e;
    }
    await p.close();
  }
  assert.deepEqual(errors, []);
} catch (error) {
  results.failureMessage = error.message;
  if (!page.isClosed()) {
    results.body = await page.locator("body").innerText();
    results.log = await page.evaluate(() => window.__terrainViewportLog);
    await page.screenshot({ path: `${out}/debug-failure.png` });
  }
  throw error;
} finally {
  results.errors = errors;
  await writeFile(`${out}/results.json`, JSON.stringify(results, null, 2));
  await browser.close();
}
