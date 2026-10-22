import { chromium } from "@playwright/test";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
const exec = promisify(execFile),
  out =
    process.env.MEMORY_QUALITY_OUT ||
    "docs/preview-climate-investigation/preview-memory",
  runs = [];
await mkdir(out, { recursive: true });
for (let repeat = 0; repeat < +(process.env.MEMORY_REPEATS || 2); repeat++)
  for (const dpr of process.env.MEMORY_ONLY_DPR
    ? [+process.env.MEMORY_ONLY_DPR]
    : [1, 2, 3])
    for (const preset of process.env.MEMORY_ONLY_PRESET
      ? [process.env.MEMORY_ONLY_PRESET]
      : ["reference", "balanced"]) {
      const browser = await chromium.launch({
        headless: true,
        executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
      });
      try {
        const page = await browser.newPage({
          viewport: { width: 1920, height: 1080 },
          deviceScaleFactor: dpr,
        });
        const cdp = await browser.newBrowserCDPSession();
        const systemInfo=await cdp.send('SystemInfo.getInfo');
        // Both frontends read the SAME pristine backend. Override only the
        // initial UI camera/style, before any frame starts; do not compare
        // retained resources from different saved views or authored revisions.
        await page.route("**/api/**", async (route) => {
          const url = new URL(route.request().url());
          url.port = "8128";
          const headers = {
            ...route.request().headers(),
            origin: "http://127.0.0.1:8128",
            referer: "http://127.0.0.1:8128/",
          };
          delete headers.host;
          const response = await route.fetch({ url: url.href, headers });
          if (url.pathname.endsWith("/open")) {
            const state = await response.json();
            state.projectUI ||= {};
            state.projectUI.continuousView = {
              camera: { x: 1.29996, y: 0.21998, w: 0.00008, h: 0.00004 },
              style: "natural",
              hillshade: +(process.env.MEMORY_HILLSHADE || 0.55),
              mode: "map",
              showRivers: process.env.MEMORY_HIDE_RIVERS !== "1",
            };
            await route.fulfill({ response, json: state });
          } else await route.fulfill({ response });
        });
        const sample = async () => {
          const processInfo = (await cdp.send("SystemInfo.getProcessInfo"))
            .processInfo;
          const ids = processInfo
            .map((p) => +p.id)
            .filter((n) => Number.isSafeInteger(n) && n > 0);
          const records = JSON.parse(
            (
              await exec(
                "powershell.exe",
                [
                  "-NoProfile",
                  "-Command",
                  `Get-Process | Where-Object { $_.Id -in @(${ids.join(",")}) } | Select-Object Id,WorkingSet64,PrivateMemorySize64 | ConvertTo-Json -Compress`,
                ],
                { windowsHide: true },
              )
            ).stdout,
          );
          const all = Array.isArray(records) ? records : [records];
          return {
            time: Date.now(),
            workingSet: all.reduce((s, p) => s + p.WorkingSet64, 0),
            privateBytes: all.reduce((s, p) => s + p.PrivateMemorySize64, 0),
            processes: all,
            processTypes: Object.fromEntries(
              processInfo.map((p) => [p.id, p.type]),
            ),
          };
        };
        await page.addInitScript(() =>
          localStorage.setItem(
            "kriemhild.preview-quality.v1",
            JSON.stringify({ preset: "balanced", customScale: 1 }),
          ),
        );
        const base =
          preset === "reference"
            ? "http://127.0.0.1:8128"
            : "http://127.0.0.1:8127";
        await page.goto(
          base + "/world/977d134ee5aadc480e1410ade3c84587/build?tileDebug=1",
        );
        await page
          .getByLabel("Autosave accepted terrain after 30 seconds")
          .uncheck();
        await page
          .locator("select")
          .filter({ hasText: "watersheds" })
          .selectOption("natural");
        await page
          .getByLabel("Hillshade strength")
          .fill(process.env.MEMORY_HILLSHADE || "0.55");
        await page.evaluate(() =>
          window.dispatchEvent(
            new CustomEvent("terrain-diagnostic-camera", {
              detail: { x: 1.29996, y: 0.21998, w: 0.00008, h: 0.00004 },
            }),
          ),
        );
        const samples = [],
          start = Date.now();
        let complete = false;
        while (Date.now() - start < 180000) {
          samples.push(await sample());
          complete = await page.evaluate(() => {
            const r = window.__terrainViewportLog?.findLast(
                (e) => e.event === "request",
              ),
              g = document.querySelector(
                '[aria-label="Committed terrain viewport"]',
              );
            return (
              !!r &&
              g?.dataset.phase === "final" &&
              +g.dataset.generation === r.generation &&
              document.querySelector("footer")?.textContent.endsWith("Ready")
            );
          });
          if (complete) break;
          await page.waitForTimeout(500);
        }
        const frame = await page.evaluate(() =>
          window.__terrainViewportLog.filter((e) => e.event === "frame").at(-1),
        );
        const pageCDP = await page.context().newCDPSession(page);
        await pageCDP.send("HeapProfiler.collectGarbage");
        await page.waitForTimeout(1000);
        const afterGC = await sample();
        const heap = await pageCDP.send("Runtime.getHeapUsage"),
          dom = await pageCDP.send("Memory.getDOMCounters");
        const resources = await page.evaluate(() =>
          performance
            .getEntriesByType("resource")
            .filter((e) => !e.name.includes("/terrain/tiles/"))
            .map((e) => ({ url: e.name, bytes: e.decodedBodySize })),
        );
        runs.push({
          repeat,
          dpr,
          preset,
          complete,
          frame,
          request: await page.evaluate(()=>window.__terrainViewportLog.findLast(e=>e.event==='request')),
          sharedBackend: 'http://127.0.0.1:8128',
          graphics:{devices:systemInfo.gpu.devices,features:systemInfo.gpu.featureStatus,renderer:systemInfo.gpu.auxAttributes?.glRenderer},
          samples,
          afterGC,
          heap,
          dom,
          resources,
          sampledPeakWorkingSet: Math.max(afterGC.workingSet,...samples.map((s) => s.workingSet)),
          note: "New browser per case; real DPR; 1920x1080 window; same camera/styles; periodic simultaneous OS working-set samples, not an exact instantaneous peak. Timings from this instrumented run are not used for latency comparisons.",
        });
        await writeFile(out + "/results.json", JSON.stringify(runs, null, 2));
        if(repeat===0 && (dpr===1 || dpr===3))
          await page.screenshot({path:`${out}/dpr-${dpr}-${preset}-extreme.png`});
        console.log(
          repeat,
          dpr,
          preset,
          complete,
          Math.round(afterGC.workingSet / 1048576),
          "MiB",
        );
        assert.equal(complete, true);
      } finally {
        await browser.close();
      }
    }
