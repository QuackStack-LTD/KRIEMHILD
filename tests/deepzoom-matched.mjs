import { chromium } from "@playwright/test";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import assert from "node:assert/strict";
const out = "docs/deepzoom-investigation/matched";
await mkdir(out, { recursive: true });
const baseline = JSON.parse(
  await readFile("docs/deepzoom-investigation/browser-before.json", "utf8"),
);
const reference = baseline.stages.find((s) => s.name === "extreme");
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
try {
  const page = await browser.newPage({
    viewport: { width: 1280, height: 800 },
  });
  await page.goto(
    "http://127.0.0.1:8126/world/977d134ee5aadc480e1410ade3c84587/build?tileDebug=1",
  );
  await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
  await page.getByLabel("Autosave accepted terrain after 30 seconds").uncheck();
  await page
    .locator("select")
    .filter({ hasText: "watersheds" })
    .selectOption("natural");
  await page.getByLabel("Hillshade strength").fill("0.55");
  await page.evaluate(
    ([x, y, w, h]) =>
      window.dispatchEvent(
        new CustomEvent("terrain-diagnostic-camera", {
          detail: { x, y, w, h },
        }),
      ),
    reference.camera,
  );
  await page.waitForTimeout(200);
  await page.waitForFunction(
    () => {
      const request = window.__terrainViewportLog
        .filter((e) => e.event === "request")
        .at(-1);
      return (
        document.querySelector('[data-phase="final"]')?.dataset.generation ===
        String(request.generation)
      );
    },
    null,
    { timeout: 180000 },
  );
  const actual = await page.evaluate(() => {
    const svg = document.querySelector(".studio-stage svg"),
      r = svg.getBoundingClientRect();
    return {
      camera: svg.getAttribute("viewBox").split(" ").map(Number),
      viewport: { width: r.width, height: r.height },
      images: [...svg.querySelectorAll("[data-terrain-style] image")].map(
        (i) => ({
          ...i.dataset,
          style: i.parentNode.dataset.terrainStyle,
          bounds: ["x", "y", "width", "height"].map((a) => +i.getAttribute(a)),
        }),
      ),
    };
  });
  assert.deepEqual(actual.camera, reference.camera);
  assert.deepEqual(actual.viewport, reference.viewport);
  assert.equal(actual.images[0].revision, reference.images[0].revision);
  assert.deepEqual(actual.images[0].bounds, reference.images[0].bounds);
  const png = await page.evaluate(async () => {
    const url = document
      .querySelector('[data-terrain-style="natural"] image')
      .getAttribute("href");
    const b = await (await fetch(url)).blob();
    return Array.from(new Uint8Array(await b.arrayBuffer()));
  });
  await writeFile(`${out}/extreme-after.png`, Buffer.from(png));
  const pixels = await page.evaluate(
    async ({ before, after }) => {
      const read = async (bytes) => {
        const b = await createImageBitmap(new Blob([new Uint8Array(bytes)]));
        const c = new OffscreenCanvas(b.width, b.height),
          ctx = c.getContext("2d", { willReadFrequently: true });
        ctx.drawImage(b, 0, 0);
        const data = ctx.getImageData(0, 0, b.width, b.height).data;
        const size = [b.width, b.height];
        b.close();
        return { data, size };
      };
      const a = await read(before),
        b = await read(after);
      let maxChannelDifference = 0,
        differentPixels = 0;
      for (let i = 0; i < a.data.length; i += 4) {
        let different = false;
        for (let k = 0; k < 4; k++) {
          const d = Math.abs(a.data[i + k] - b.data[i + k]);
          maxChannelDifference = Math.max(maxChannelDifference, d);
          different ||= d > 0;
        }
        if (different) differentPixels++;
      }
      return {
        beforeSize: a.size,
        afterSize: b.size,
        maxChannelDifference,
        differentPixels,
      };
    },
    {
      before: [
        ...(await readFile("docs/deepzoom-investigation/extreme-before.png")),
      ],
      after: png,
    },
  );
  await writeFile(`${out}/pixels.json`, JSON.stringify(pixels, null, 2));
  await writeFile(
    `${out}/results.json`,
    JSON.stringify({ reference, actual }, null, 2),
  );
  await page.screenshot({ path: `${out}/extreme-ui-after.png` });
} finally {
  await browser.close();
}
