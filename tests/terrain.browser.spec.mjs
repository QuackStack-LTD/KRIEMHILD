import { test, expect } from "@playwright/test";

// Read-only smoke test against an expendable saved continuous terrain project.
// Its source need not finish a 2D raster before the 3D view can open.
const project = process.env.KRIEMHILD_TERRAIN_PROJECT;
test("3D loads a coherent terrain cover, renders geographic overlays and supports an overhead view", async ({
  page,
}) => {
  test.skip(
    !project,
    "Set KRIEMHILD_TERRAIN_PROJECT to a continuous terrain project.",
  );
  test.setTimeout(180000);
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(`/world/${project}/build?tileDebug=1`);
  await page
    .getByRole("button", { name: "3D Terrain", exact: true })
    .first()
    .click();
  const failure = page.waitForEvent("pageerror").then((e) => {
    throw e;
  });
  const covered = () =>
    page.waitForFunction(
      () =>
        document.querySelector(".studio-3d-canvas")?.dataset.coverComplete ===
        "true",
    );
  await Promise.race([covered(), failure]);
  const diagnostics = await page
    .locator(".studio-3d-canvas")
    .evaluate((e) => ({ ...e.dataset }));
  expect(+diagnostics.visibleTiles).toBeGreaterThan(0);
  expect(+diagnostics.visibleTiles).toBeLessThanOrEqual(64);
  expect(+diagnostics.labelBytes).toBeLessThanOrEqual(16 * 1024 * 1024);
  expect(+diagnostics.overlayBytes).toBeLessThanOrEqual(32 * 1024 * 1024);
  await expect(page.locator(".studio-3d-canvas")).toHaveAttribute(
    "data-rendered-triangles",
    /[1-9]\d*/,
  );
  const frames = await page
    .locator(".studio-3d-canvas")
    .getAttribute("data-rendered-frames");
  const exaggeration = page.getByLabel("Vertical exaggeration", {
    exact: true,
  });
  await expect(exaggeration).toHaveAttribute("max", "60");
  await exaggeration.focus();
  await exaggeration.press("End");
  await expect(exaggeration).toHaveValue("60");
  await page
    .getByRole("button", { name: "Look from above", exact: true })
    .click();
  await Promise.race([covered(), failure]);
  await page.waitForFunction(
    (old) =>
      +document.querySelector(".studio-3d-canvas").dataset.renderedFrames >
      +old,
    frames,
  );
  expect(errors).toEqual([]);
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Terrain in 3D" })).toHaveCount(
    0,
  );
});
