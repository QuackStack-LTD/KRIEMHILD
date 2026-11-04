import { chromium } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
const run = promisify(execFile),
  records = [];
const world =
  process.env.KRIEMHILD_AUDIT_WORLD ||
  JSON.parse(
    await readFile("docs/viewport-investigation/baseline/new.json", "utf8"),
  ).world;
const base = process.env.KRIEMHILD_AUDIT_URL || "http://127.0.0.1:8125";
for (const renderer of process.env.MEMORY_RENDERER
  ? [process.env.MEMORY_RENDERER]
  : ["tiles", "viewport"])
  for (const dpr of process.env.MEMORY_DPR
    ? [+process.env.MEMORY_DPR]
    : [1, 3]) {
    const browser = await chromium.launch({
      headless: true,
      executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
    });
    const page = await browser.newPage({
        viewport: { width: 1920, height: 1080 },
        deviceScaleFactor: dpr,
      }),
      cdp = await browser.newBrowserCDPSession();
    try {
      if (process.env.MEMORY_QUALITY)
        await page.addInitScript((preset) => {
          localStorage.setItem(
            "kriemhild.preview-quality.v1",
            JSON.stringify({ preset, customScale: 1 }),
          );
        }, process.env.MEMORY_QUALITY);
      if (process.env.MEMORY_SOFTWARE !== undefined)
        await page.addInitScript((software) => {
          const original = OffscreenCanvas.prototype.getContext;
          OffscreenCanvas.prototype.getContext = function (type, options) {
            return original.call(this, type, {
              ...options,
              willReadFrequently: software,
            });
          };
        }, process.env.MEMORY_SOFTWARE === "1");
      await page.goto(
        `${base}/world/${world}/build?terrainRenderer=${renderer}&tileDebug=1`,
      );
      await page
        .getByLabel("Autosave accepted terrain after 30 seconds")
        .uncheck();
      if (process.env.MEMORY_CAMERA) {
        await page.evaluate(
          (detail) =>
            window.dispatchEvent(
              new CustomEvent("terrain-diagnostic-camera", { detail }),
            ),
          JSON.parse(process.env.MEMORY_CAMERA),
        );
      } else {
        await page
          .getByRole("button", { name: "Fit world", exact: true })
          .click();
      }
      let complete = true;
      try {
        await page.waitForFunction(
          () => document.querySelector("footer")?.innerText.endsWith("Ready"),
          null,
          { timeout: 60000 },
        );
      } catch {
        complete = false;
      }
      try {
        await page.waitForLoadState("networkidle", { timeout: 5000 });
      } catch {
        complete = false;
      }
      // Query only this browser's OS process IDs; no other browser windows included.
      const info = await cdp.send("SystemInfo.getProcessInfo");
      const ids = info.processInfo
        .map((p) => Number(p.id))
        .filter((n) => Number.isSafeInteger(n) && n > 0);
      const measureProcess = async () =>
        JSON.parse(
          (
            await run("powershell.exe", [
              "-NoProfile",
              "-Command",
              `Get-Process -Id ${ids.join(",")} -ErrorAction SilentlyContinue | Select-Object Id,ProcessName,WorkingSet64,PeakWorkingSet64,PrivateMemorySize64 | ConvertTo-Json -Compress`,
            ])
          ).stdout,
        );
      const processBeforeGC = await measureProcess(),
        memory = await page.evaluate(() => ({
          jsHeap: performance.memory?.usedJSHeapSize,
          frame: window.__terrainViewportLog
            ?.filter((e) => e.event === "frame")
            .at(-1),
          images: document.querySelectorAll(".studio-stage image").length,
          status: document.querySelector("footer")?.innerText,
          failures: (
            window.__terrainViewportLog ||
            window.__terrainTileLog ||
            []
          ).filter((e) => e.event === "error" || e.event === "failed"),
        }));
      const pageCDP = await page.context().newCDPSession(page);
      await pageCDP.send("HeapProfiler.collectGarbage");
      await page.waitForTimeout(1000);
      const processAfterGC = await measureProcess();
      records.push({
        renderer,
        dpr,
        complete,
        process: processBeforeGC,
        processAfterGC,
        software: process.env.MEMORY_SOFTWARE !== "0",
        memory,
        note: "Working set after completion; per-process lifetime peaks are upper bounds, not simultaneous aggregate peaks. Warm server caches.",
      });
      console.log(renderer, dpr, memory.jsHeap);
    } finally {
      await browser.close();
      await writeFile(
        process.env.MEMORY_OUT ||
          "docs/viewport-investigation/browser-memory.json",
        JSON.stringify(records, null, 2),
      );
    }
  }
