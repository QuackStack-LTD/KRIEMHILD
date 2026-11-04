// Repeatable A/B camera sequence. Browser-cold then browser-warm; server cache
// status is recorded rather than incorrectly labelling every run server-cold.
import { chromium } from "@playwright/test";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { visibleTiles } from "../src/continuous/camera.js";
const out = "docs/viewport-investigation/benchmark";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
const runs = [];
try {
  for (const old of [false, true])
    for (const renderer of ["tiles", "viewport"])
      for (let repeat = 0; repeat < 3; repeat++) {
        const fixture = JSON.parse(
          await readFile(
            `docs/viewport-investigation/baseline/${old ? "old" : "new"}.json`,
          ),
        );
        const page = await browser.newPage({
            viewport: { width: 1280, height: 800 },
          }),
          network = [],
          errors = [];
        page.on("pageerror", (e) => errors.push(e.message));
        page.on("response", (r) => {
          if (r.url().includes("/terrain/tiles/"))
            network.push({ url: r.url(), status: r.status(), ...r.headers() });
        });
        const run = {
          old,
          renderer,
          repeat,
          world: fixture.world,
          stages: [],
          network,
          errors,
        };
        runs.push(run);
        await page.goto(
          `http://127.0.0.1:${old ? 8126 : 8125}/world/${fixture.world}/build?terrainRenderer=${renderer}&tileDebug=1`,
        );
        await page.waitForSelector("g[data-terrain-style] image", {
          timeout: 180000,
        });
        await page.addStyleTag({
          content:
            '[aria-label="Tile diagnostics"] { display: none !important; }',
        });
        const style = page.locator("select").filter({ hasText: "watersheds" });
        await style.selectOption("natural");
        await page.getByLabel("Hillshade strength").fill("0.55");
        async function settled() {
          const s = await page.evaluate(() => {
            const svg = document.querySelector(".studio-stage svg"),
              r = svg.getBoundingClientRect(),
              a = svg.getAttribute("viewBox").split(" ").map(Number);
            return {
              camera: { x: a[0], y: a[1], w: a[2], h: a[3] },
              width: r.width,
              height: r.height,
            };
          });
          const wanted = visibleTiles(s.camera, s.width, 1, s.height).map(
            (t) => `${t.z}/${t.x}/${t.y}`,
          );
          await page.waitForFunction(
            ({ wanted, renderer }) => {
              if (renderer === "viewport")
                return (
                  document
                    .querySelector("footer")
                    ?.innerText.endsWith("Ready") &&
                  document.querySelector('[data-phase="final"]')
                );
              return ["natural", "hillshade"].every((style) => {
                const g = document.querySelector(
                  `[data-terrain-style="${style}"]`,
                );
                return wanted.every((k) =>
                  g?.querySelector(`image[data-tile="${k}"]`),
                );
              });
            },
            { wanted, renderer },
            { timeout: 180000 },
          );
          return s;
        }
        async function measure(name, action) {
          const start = performance.now();
          await action();
          await page.waitForTimeout(100);
          const s = await settled(),
            ms = performance.now() - start;
          const detail = await page.evaluate(() => ({
            heap: performance.memory?.usedJSHeapSize,
            images: document.querySelectorAll(".studio-stage image").length,
            frame: window.__terrainViewportLog
              ?.filter((e) => e.event === "frame")
              .at(-1),
            log: window.__terrainViewportLog || window.__terrainTileLog,
          }));
          run.stages.push({ name, ms, ...s, ...detail });
          console.log(
            old ? "old" : "new",
            renderer,
            repeat,
            name,
            Math.round(ms),
          );
          if (repeat === 0 && ["global", "close", "return"].includes(name)) {
            await page.screenshot({
              path: `${out}/${old ? "old" : "new"}-${renderer}-${name}.png`,
            });
          }
        }
        async function zoom(n, delta = -266) {
          const r = await page.locator(".studio-stage svg").boundingBox();
          await page.mouse.move(r.x + r.width * 0.42, r.y + r.height * 0.4);
          for (let i = 0; i < n; i++) {
            await page.mouse.wheel(0, delta);
            await page.waitForTimeout(100);
          }
        }
        await measure("global", () =>
          page.getByRole("button", { name: "Fit world", exact: true }).click(),
        );
        await measure("regional", () => zoom(6));
        await measure("close", () => zoom(5));
        await measure("out", () => zoom(5, 266));
        await measure("return", () => zoom(5));
        await page.close();
        await writeFile(`${out}/results.json`, JSON.stringify(runs, null, 2));
      }
} finally {
  await writeFile(`${out}/results.json`, JSON.stringify(runs, null, 2));
  await browser.close();
}
