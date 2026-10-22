import { chromium } from "@playwright/test";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import assert from "node:assert/strict";
const out = "docs/deepzoom-investigation/faults";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
const results = [];
try {
  for (const fault of [503, 429, 408, "disconnect", "png", "revision"]) {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 800 },
    });
    await page.goto(
      "http://127.0.0.1:8125/world/977d134ee5aadc480e1410ade3c84587/build?tileDebug=1",
    );
    await page.waitForSelector('[data-phase="final"]', { timeout: 180000 });
    const initialStyle = await page
      .locator("select")
      .filter({ hasText: "watersheds" })
      .inputValue();
    const target = initialStyle === "biome" ? "elevation" : "biome";
    await page
      .locator("select")
      .filter({ hasText: "watersheds" })
      .selectOption("natural");
    await page.evaluate(await readFile("tests/deepzoom-browser.js", "utf8"));
    await page.evaluate(() => window.deepzoomStep("close", 0.003));
    const wanted = await page.evaluate(
      () =>
        window.__terrainViewportLog.filter((e) => e.event === "request").at(-1)
          .z,
    );
    let failing = true,
      count = 0;
    await page.route("**/terrain/tiles/**", async (route) => {
      const url = new URL(route.request().url());
      if (
        !failing ||
        url.searchParams.get("style") !== target ||
        +url.pathname.split("/").at(-3) !== wanted
      )
        return route.continue();
      count++;
      if (fault === "disconnect") return route.abort("failed");
      if (fault === "png")
        return route.fulfill({
          status: 200,
          contentType: "image/png",
          headers: { "X-Terrain-Revision": url.searchParams.get("revision") },
          body: "truncated PNG",
        });
      if (fault === "revision")
        return route.fulfill({
          status: 409,
          contentType: "application/json",
          body: '{"error":"revision mismatch"}',
        });
      return route.fulfill({ status: fault, body: "temporary failure" });
    });
    await page
      .locator("select")
      .filter({ hasText: "watersheds" })
      .selectOption(target);
    await page
      .getByRole("button", { name: "Retry terrain", exact: true })
      .waitFor({ timeout: 90000 });
    const attempts = count;
    await page.waitForTimeout(1100);
    assert.equal(
      count,
      attempts,
      "failure must terminate without layout-triggered retries",
    );
    if (fault === "revision")
      assert.ok(count <= 3, "permanent 409 must not retry");
    assert.ok(
      (await page.locator("[data-terrain-style] image").count()) > 0,
      "keep valid frame",
    );
    failing = false;
    await page
      .getByRole("button", { name: "Retry terrain", exact: true })
      .click();
    await page.waitForFunction(
      () =>
        document.querySelector("footer")?.innerText.endsWith("Ready") &&
        document.querySelector('[data-phase="final"]'),
      null,
      { timeout: 90000 },
    );
    const record = await page.evaluate(() => ({
      images: [...document.querySelectorAll("[data-terrain-style] image")].map(
        (i) => ({ ...i.dataset, style: i.parentNode.dataset.terrainStyle }),
      ),
      log: window.__terrainViewportLog,
    }));
    assert.equal(record.images[0].style, target);
    assert.equal(record.images[0].generation, record.images[1].generation);
    results.push({ fault, attempts, ...record });
    await writeFile(`${out}/results.json`, JSON.stringify(results, null, 2));
    await page.screenshot({ path: `${out}/${fault}-recovered.png` });
    console.log(fault, "recovered", attempts);
    await page.close();
  }
} finally {
  await browser.close();
}
