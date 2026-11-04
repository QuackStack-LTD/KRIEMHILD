import { chromium } from "@playwright/test";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { mkdir, writeFile, rename } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import assert from "node:assert/strict";
import { build } from "esbuild";
const out =
    process.env.PREVIEW_COLD_OUT ||
    "docs/preview-climate-investigation/preview-cold",
  base = "http://127.0.0.1:8128",
  runs = [];
await mkdir(out, { recursive: true });
const auditScript = (
  await build({
    entryPoints: ["tests/preview-quality-benchmark.js"],
    bundle: true,
    format: "iife",
    write: false,
  })
).outputFiles[0].text;
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
try {
  for (
    let repeat = 0;
    repeat < +(process.env.PREVIEW_COLD_REPEATS || 2);
    repeat++
  )
    for (const dpr of [1, 2, 3])
      for (const preset of (
        process.env.PREVIEW_COLD_PRESETS || "reference,balanced"
      ).split(",")) {
        const root = resolve(".cache/preview-cold-data"),
          cache = resolve(root, "cache"),
          archived = resolve(
            root,
            `cache-${Date.now()}-${repeat}-${dpr}-${preset}`,
          );
        if (dirname(cache) !== root || dirname(archived) !== root)
          throw Error("Cache archive escaped disposable workspace");
        try {
          await rename(cache, archived);
        } catch (e) {
          if (e.code !== "ENOENT") throw e;
        }
        const server = spawn(
          ".cache/kriemhild-deepzoom-review.exe",
          [
            "-addr",
            "127.0.0.1:8128",
            "-dist",
            "dist",
            "-data",
            ".cache/preview-cold-data",
          ],
          { windowsHide: true, stdio: "ignore" },
        );
        const exited = once(server, "exit");
        let page;
        try {
          for (let i = 0; i < 100; i++) {
            try {
              if ((await fetch(base)).ok) break;
            } catch {}
            await new Promise((r) => setTimeout(r, 100));
          }
          page = await browser.newPage({
            viewport: { width: 1280, height: 800 },
            deviceScaleFactor: dpr,
          });
          await page.route("**/quality-benchmark-empty", (route) =>
            route.fulfill({
              contentType: "text/html",
              body: "<!doctype html><title>Isolated cold render benchmark</title>",
            }),
          );
          await page.goto(base + "/quality-benchmark-empty");
          await page.addScriptTag({ content: auditScript });
          await page.waitForFunction(() => window.runQualityAudit);
          await page.evaluate(
            async ({ dpr, preset, styles }) => {
              const s = await (
                await fetch(
                  "/api/projects/977d134ee5aadc480e1410ade3c84587/open",
                  {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: "{}",
                  },
                )
              ).json();
              runQualityAudit({
                session: s.id,
                revision: s.project.revision,
                dprs: [dpr],
                presets: [preset],
                styles,
              });
            },
            { dpr, preset,styles:(process.env.PREVIEW_COLD_STYLES||'natural,hillshade').split(',') },
          );
          await page.waitForFunction(
            () => qualityAudit.status !== "running",
            null,
            { timeout: 240000 },
          );
          const result = await page.evaluate(() => qualityAudit);
          assert.equal(result.status, "complete", result.error);
          runs.push({
            repeat,
            dpr,
            preset,
            serverPID: server.pid,
            cache:
              "fresh backend process AND empty persistent tile cache; fresh browser context; paired warm-browser request",
            ...result,
          });
          await writeFile(out + "/results.json", JSON.stringify(runs, null, 2));
          console.log(
            repeat,
            dpr,
            preset,
            "complete",
            result.runs.map((r) => Math.round(r.elapsed)),
          );
        } finally {
          await page?.close();
          server.kill();
          await exited;
        }
      }
} finally {
  await browser.close();
}
