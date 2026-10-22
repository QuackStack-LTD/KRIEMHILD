import { chromium } from "@playwright/test";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import assert from "node:assert/strict";
const old = process.env.AUDIT_OLD === "1",
  prefix = old ? "old" : "new",
  base = old ? "http://127.0.0.1:8126" : "http://127.0.0.1:8125";
const baseline = JSON.parse(
  await readFile(`docs/viewport-investigation/baseline/${prefix}.json`),
);
const out = "docs/viewport-investigation/prototype";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } }),
  errors = [],
  states = [];
page.on("pageerror", (e) => errors.push(e.message));
page.on("dialog", (d) => d.accept());
const ready = async () => {
  await page.waitForFunction(
    () =>
      document.querySelector("footer")?.innerText.endsWith("Ready") &&
      document.querySelector('[data-phase="final"]'),
    null,
    { timeout: 180000 },
  );
  await page.waitForLoadState("networkidle", { timeout: 180000 });
};
async function capture(name, wait = true) {
  if (wait) await ready();
  const s = await page.evaluate(() => {
    const svg = document.querySelector(".studio-stage svg"),
      r = svg.getBoundingClientRect(),
      g = svg.querySelector('[aria-label="Committed terrain viewport"]');
    return {
      camera: svg.getAttribute("viewBox"),
      rect: r.toJSON(),
      footer: document.querySelector("footer").innerText,
      phase: g?.dataset.phase,
      generation: g?.dataset.generation,
      revision: g?.dataset.revision,
      images: [...g.querySelectorAll("image")].map((i) => ({
        bounds: ["x", "y", "width", "height"].map((a) => +i.getAttribute(a)),
        generation: i.dataset.generation,
        revision: i.dataset.revision,
        z: i.dataset.lod,
      })),
      heap: performance.memory?.usedJSHeapSize,
      log: window.__terrainViewportLog,
    };
  });
  assert.equal(s.images.length, 2);
  assert.ok(
    s.images.every(
      (i) => i.generation === s.generation && i.revision === s.revision,
    ),
  );
  assert.deepEqual(s.images[0].bounds, s.images[1].bounds);
  states.push({ name, ...s });
  await page.screenshot({ path: `${out}/${prefix}-${name}.png` });
  console.log(prefix, name, s.camera, s.phase);
  return s;
}
async function zoom(n, delay = 300, delta = -266) {
  const r = await page.locator(".studio-stage svg").boundingBox();
  await page.mouse.move(r.x + r.width * 0.42, r.y + r.height * 0.4);
  for (let i = 0; i < n; i++) {
    await page.mouse.wheel(0, delta);
    await page.waitForTimeout(delay);
  }
}
try {
  await page.goto(
    `${base}/world/${baseline.world}/build?tileDebug=1&terrainRenderer=viewport`,
  );
  await page.waitForSelector("[data-phase]", { timeout: 180000 });
  await ready();
  await page.getByRole("button", { name: "Fit world", exact: true }).click();
  await capture("00-global");
  for (let i = 1; i <= 3; i++) {
    await zoom(2);
    const s = await capture(`0${i}-zoom`);
    assert.equal(s.camera, baseline.states[i].camera);
  }
  let release, heldURL;
  const held = new Promise((r) => (release = r));
  await page.route("**/terrain/tiles/**", async (route) => {
    const u = route.request().url();
    if (!heldURL && u.includes("style=hillshade") && u.includes("/tiles/6/")) {
      heldURL = u;
      await held;
    }
    await route.continue();
  });
  await zoom(2, 30);
  for (let i = 0; i < 360 && !heldURL; i++) await page.waitForTimeout(250);
  assert.ok(heldURL);
  const pending = await capture("04-delayed-shading", false);
  assert.ok(!pending.footer.endsWith("Ready"));
  assert.notEqual(pending.phase, "final");
  release();
  await page.unrouteAll({ behavior: "wait" });
  await capture("05-complete");
  await zoom(3, 30);
  await capture("06-close");
  const r = await page.locator(".studio-stage svg").boundingBox();
  await page.getByRole("button", { name: /^inspect$/i }).click();
  await page.mouse.move(r.x + 40, r.y + 40);
  await page.mouse.down();
  await page.mouse.move(r.x + 160, r.y + 150, { steps: 5 });
  await page.mouse.up();
  await capture("07-pan");
  await zoom(3, 30, 266);
  await capture("08-out");
  await zoom(3, 30);
  await capture("09-return");
  await page.reload();
  await page.waitForSelector("[data-phase]", { timeout: 180000 });
  await capture("10-reload");
  assert.deepEqual(errors, []);
} finally {
  await writeFile(
    `${out}/${prefix}.json`,
    JSON.stringify({ baseline: baseline.world, states, errors }, null, 2),
  );
  await browser.close();
}
