import { test, expect } from "@playwright/test";

test("profile and terrain renames update labels without reloading World data or terrain", async ({
  page,
  context,
}) => {
  const world = process.env.KRIEMHILD_RENAME_WORLD,
    entity = process.env.KRIEMHILD_RENAME_ENTITY,
    project = process.env.KRIEMHILD_RENAME_PROJECT;
  test.skip(
    !world || !entity || !project,
    "Requires an expendable saved World and mapped entity.",
  );
  test.setTimeout(180000);
  page.setDefaultTimeout(30000);
  const meta = await (
      await page.request.get(`/api/worlds/${world}?view=metadata`)
    ).json(),
    age = meta.world.currentAge;
  const errors = [],
    traffic = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("request", (r) => traffic.push({ url: r.url(), method: r.method() }));
  await page.goto(`/worlds/${world}/profile?entity=${entity}&age=${age}`);
  const rename = page.getByRole("button", { name: "Rename", exact: true });
  await expect(rename).toBeVisible({ timeout: 45000 });
  const heading = page.locator("main h1").first();
  const original = await heading.innerText(),
    name = `Rename audit ${Date.now()}`;
  traffic.length = 0;
  await rename.click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("textbox", { name: "Entity name" }).fill(name);
  const start = Date.now(),
    response = page.waitForResponse(
      (r) =>
        r.url().endsWith(`worlds/${world}?view=changes`) &&
        r.request().method() === "POST",
    );
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(heading).toHaveText(name);
  const profileMs = Date.now() - start;
  console.log(JSON.stringify({ stage: "profile renamed", ms: profileMs }));
  expect(
    traffic.filter(
      (r) =>
        r.method === "GET" &&
        /\/api\/worlds\/[^/?]+\?view=(workspace|atlas)/.test(r.url),
    ),
  ).toHaveLength(0);

  const studio = await context.newPage(),
    studioTraffic = [];
  studio.on("pageerror", (e) => errors.push(e.message));
  studio.on("request", (r) => studioTraffic.push(r.url()));
  const features = studio.waitForResponse(
    (r) => r.url().includes("/terrain/features") && r.status() === 200,
  );
  await studio.goto(
    `/world/${project}/build?world=${world}&age=${age}&tileDebug=1`,
  );
  await features;
  console.log("Terrain opened");
  const find = studio.getByRole("searchbox", { name: "Find an object" });
  await studio.getByText("Layers & objects", { exact: true }).click();
  await find.fill(name);
  const entityName = studio.getByRole("textbox", { name: "Name", exact: true });
  await expect(entityName).toHaveValue(name);
  await expect
    .poll(async () =>
      studio.evaluate(
        () =>
          window.__terrainViewportLog?.filter(
            (e) => e.event === "frame" && e.phase === "final",
          ).length || 0,
      ),
    )
    .toBeGreaterThan(0);
  await studio.waitForTimeout(500);
  studioTraffic.length = 0;
  await page.bringToFront();
  await rename.click();
  await page
    .getByRole("dialog")
    .getByRole("textbox", { name: "Entity name" })
    .fill(original);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Save", exact: true })
    .click();
  await expect(heading).toHaveText(original);
  await studio.bringToFront();
  await studio.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect(entityName).toHaveValue(original);
  expect(
    studioTraffic.filter(
      (url) =>
        url.includes("/terrain/features") ||
        url.includes("/terrain/tiles/") ||
        url.includes("/world/entities?"),
    ),
  ).toHaveLength(0);

  const terrainStart = Date.now();
  await entityName.fill(name);
  await studio
    .getByRole("button", { name: "Rename entity", exact: true })
    .click();
  await expect(
    studio.getByRole("button", { name: "Rename entity", exact: true }),
  ).toBeEnabled();
  await page.bringToFront();
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect(heading).toHaveText(name);
  expect(
    studioTraffic.filter(
      (url) =>
        url.includes("/terrain/features") ||
        url.includes("/terrain/tiles/") ||
        url.includes("/world/entities?"),
    ),
  ).toHaveLength(0);
  console.log(
    JSON.stringify({
      profileRenameMs: profileMs,
      terrainRenameAndSyncMs: Date.now() - terrainStart,
    }),
  );
  // Restore the expendable fixture's human-readable name.
  await rename.click();
  await page
    .getByRole("dialog")
    .getByRole("textbox", { name: "Entity name" })
    .fill(original);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Save", exact: true })
    .click();
  await expect(heading).toHaveText(original);
  expect(errors).toEqual([]);
  await studio.close();
});
