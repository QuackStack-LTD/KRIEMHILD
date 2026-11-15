import { test, expect } from "@playwright/test";

test("resource circles copy, archive by proximity, and reopen saved maps and Age timelines", async ({
  page,
}, testInfo) => {
  const world = process.env.KRIEMHILD_RESOURCE_ARCHIVE_WORLD,
    project = process.env.KRIEMHILD_RESOURCE_ARCHIVE_PROJECT,
    archive = process.env.KRIEMHILD_RESOURCE_ARCHIVE_ID;
  test.skip(
    !world || !project || !archive,
    "Requires an expendable saved terrain and an archive linked to a settlement.",
  );
  test.setTimeout(180000);
  page.setDefaultTimeout(20000);
  const errors = [];
  // Keep reruns newer than retained observations in the expendable fixture.
  const firstYear = Math.floor(Date.now() / 1000);
  let featureRequests = 0;
  page.on("request", (r) => {
    if (new URL(r.url()).pathname.endsWith("/terrain/features"))
      featureRequests++;
  });
  page.on("pageerror", (e) => errors.push(e.message));
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/");
  await expect(
    page.getByRole("textbox", { name: "New world name" }),
  ).toBeVisible();
  const lon = Number(process.env.KRIEMHILD_RESOURCE_ARCHIVE_LON || 0),
    lat = Number(process.env.KRIEMHILD_RESOURCE_ARCHIVE_LAT || 0);
  const context = await page.evaluate(
    async ({ world, project, lon, lat, archive }) => {
      const open = await fetch(`/api/projects/${project}/open?view=terrain`, {
        method: "POST",
        body: "{}",
        headers: { "Content-Type": "application/json" },
      });
      if (!open.ok) throw Error(await open.text());
      const state = await open.json();
      const w = 0.006,
        view = {
          ...(state.projectUI?.continuousView || {}),
          camera: {
            x: lon / 180 + 1 - w / 2,
            y: 0.5 - lat / 180 - w / 4,
            w,
            h: w / 2,
          },
        };
      const save = await fetch(`/api/sessions/${state.id}/view`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ui: { continuousView: view }, merge: true }),
      });
      if (!save.ok) throw Error(await save.text());
      const meta = await (
        await fetch(`/api/worlds/${world}?view=metadata`)
      ).json();
      const linked = await (
        await fetch(`/api/worlds/${world}/writer-archive/inworld`)
      ).json();
      return {
        session: state.id,
        age: meta.world.currentAge,
        ageName: meta.ages.find((a) => a.id === meta.world.currentAge).name,
        archiveName: linked.entities.find(
          (e) => e.ageId === meta.world.currentAge && e.id === archive,
        ).name,
      };
    },
    { world, project, lon, lat, archive },
  );
  await page.goto(
    `/world/${project}/build?world=${world}&age=${context.age}&tileDebug=1`,
  );
  await page
    .getByRole("button", { name: "Terrain Resources", exact: true })
    .click();
  const map = page.locator(
    'svg[aria-label="Interactive continuous world map"]',
  );
  await expect(map).toBeVisible();
  const box = await map.boundingBox();
  await map.click({ position: { x: box.width * 0.5, y: box.height * 0.5 } });
  const areaInput = page.getByRole("spinbutton", {
    name: "Terrain resource area in square kilometres",
  });
  const captures = [];
  for (const [area, year] of [
    [314, firstYear],
    [400, firstYear + 1],
    [150, firstYear + 2],
  ]) {
    const response = page.waitForResponse(async (r) => {
      if (
        !r.url().includes("/terrain/resources?") ||
        r.request().method() !== "GET" ||
        r.status() !== 200
      )
        return false;
      try {
        return Math.abs((await r.json()).areaKm2 - area) < 0.0001;
      } catch {
        return false;
      }
    });
    await areaInput.fill(String(area));
    const report = await (await response).json();
    expect(report.areaKm2).toBeCloseTo(area, 4);
    await expect(
      page.getByRole("button", { name: "Copy information", exact: true }),
    ).toBeVisible();
    if (year === firstYear) {
      await page
        .getByRole("button", { name: "Copy information", exact: true })
        .click();
      await expect(
        page.getByRole("status").filter({ hasText: "copied as one paragraph" }),
      ).toBeVisible();
      const copied = await page.evaluate(() => navigator.clipboard.readText());
      expect(copied).toContain("Terrain resource report.");
      expect(copied).toContain("fresh Water");
      expect(copied).not.toMatch(/[\r\n\t]/);
    }
    await page
      .getByRole("button", { name: "Archive information", exact: true })
      .click();
    const dialog = page.getByRole("dialog", {
      name: "Archive resource information",
    });
    await expect(
      dialog.getByRole("combobox", { name: "Resource report archive" }),
    ).toBeEnabled();
    const options = await dialog
      .getByRole("combobox", { name: "Resource report archive" })
      .locator("option")
      .allTextContents();
    expect(options.at(-1)).toContain("location not recorded");
    await dialog
      .getByRole("combobox", { name: "Resource report archive" })
      .selectOption(archive);
    await dialog
      .getByRole("textbox", { name: "Resource report year" })
      .fill(String(year));
    const archived = page.waitForResponse(
      (r) =>
        r.url().includes("/terrain/resource-reports?") &&
        r.request().method() === "POST",
    );
    await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
    const result = await archived;
    expect(result.status()).toBe(201);
    captures.push(await result.json());
    await expect(dialog).toHaveCount(0);
  }
  expect(captures[1].unchangedCells).toBeGreaterThan(0);
  expect(captures[2].newCells).toBe(0);
  expect(captures[2].unchangedCells).toBeGreaterThan(0);
  await page.waitForTimeout(4500);
  expect(featureRequests).toBe(1);
  const timings = await page.evaluate(
    () =>
      window.__terrainViewportLog
        ?.filter((e) => e.event === "frame")
        .slice(-3)
        .map((e) => ({
          phase: e.phase,
          totalMs: e.duration,
          encodeMs: e.timings.encode,
          fetchMs: e.timings.fetch,
        })) || [],
  );
  console.log(
    JSON.stringify({
      previewTimings: timings,
      delta: captures.map((c) => ({
        new: c.newCells,
        changed: c.changedCells,
        unchanged: c.unchangedCells,
      })),
    }),
  );
  await page.getByRole("link", { name: "Open archive resources" }).click();
  await page
    .getByRole("button", { name: "Review Terrain Resources", exact: true })
    .click();
  const recorded = page.getByRole("group", {
    name: "Map of archived terrain resource circles",
  });
  await expect(recorded).toBeVisible();
  await recorded.locator('path[role=button][tabindex="0"]').last().focus();
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("heading", { name: "Terrain resource record", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Historical resources", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("combobox", { name: "Historical resource measure" }),
  ).toBeVisible();
  expect(
    await page.locator(".resource-chart-grid > section").count(),
  ).toBeGreaterThanOrEqual(2);
  await expect(page.locator(".resource-line")).toHaveCount(1);
  const original = page.waitForResponse(
    (r) => r.url().includes("original=1") && r.status() === 200,
  );
  await page
    .getByText("Complete information in the original document", { exact: true })
    .click();
  expect((await (await original).json()).report.areaKm2).toBeGreaterThan(0);
  await expect(page.locator(".terrain-resource-report")).toBeVisible();
  const biology = page.getByText("Recorded habitats and lifeforms", {
    exact: true,
  });
  if (await biology.count()) {
    const savedBiology = page.waitForResponse(
      (r) => r.url().includes("lifeforms=1") && r.status() === 200,
    );
    await biology.click();
    expect((await (await savedBiology).json()).biosphere.available).toBe(true);
    await expect(page.getByText(/Biology is recorded for/)).toBeVisible();
  }
  await page
    .getByRole("combobox", { name: "Resource observation year" })
    .selectOption(String(firstYear));
  await expect(page.locator(".resource-stat-cards")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("archived-resource-statistics.png"),
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Resource documents", exact: true })
    .click();
  await page
    .locator(".archive-list")
    .getByRole("button", { name: context.ageName })
    .first()
    .click();
  await page
    .locator(".archive-list")
    .getByRole("button", { name: `Year ${firstYear}` })
    .click();
  await page.locator(".library-file-list button").first().click();
  await expect(
    page.getByRole("link", { name: "Download file", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Review this terrain area", exact: true })
    .click();
  await expect(page.locator(".resource-stat-cards")).toBeVisible();
  await page.goto(`/worlds/${world}/writing`);
  await page
    .getByRole("button", { name: "Inworld Archive", exact: true })
    .first()
    .click();
  await page
    .getByRole("button", { name: context.ageName, exact: true })
    .click();
  await page
    .getByRole("button", { name: context.archiveName, exact: true })
    .click();
  await page.getByRole("button", { name: "Resources", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Review Terrain Resources", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Resource documents", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Review Terrain Resources", exact: true })
    .click();
  await expect(
    page.getByRole("group", {
      name: "Map of archived terrain resource circles",
    }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("writer-archive-resources.png"),
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
