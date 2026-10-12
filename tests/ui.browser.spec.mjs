import { test, expect } from "@playwright/test";
import { mkdir } from "node:fs/promises";

// Use an expendable populated world. The Library test creates and deletes its own
// temporary records. Other checks discard drafts; generation previews are never accepted.
const world = process.env.KRIEMHILD_UI_WORLD;
const base = `/worlds/${world}`;
const screenshots = ".tools/cleanup-screenshots";
test.describe("Editing and specialized surfaces", () => {
  test.skip(!world, "Set KRIEMHILD_UI_WORLD to a populated audit world.");
  test("Library archive and document creation, version saving, dirty switching and deletion", async ({
    page,
    request,
  }) => {
    const doc = await (await request.get(`/api/worlds/${world}`)).json();
    const place = doc.entities.find(
      (e) =>
        e.ageId === doc.world.currentAge &&
        ["character", "organization"].includes(e.kind) &&
        !e.deleted,
    );
    test.skip(!place, "The audit world needs a place or organization.");
    const name = "UI audit archive " + Date.now();
    await page.goto(`${base}/library`);
    await page.getByRole("button", { name: "Create archive folder" }).click();
    let dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill(name);
    await dialog.getByLabel("Archive owner").selectOption(place.id);
    await dialog.locator("select[multiple]").selectOption(place.id);
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await page
      .getByRole("button", { name: "Create document", exact: true })
      .click();
    dialog = page.getByRole("dialog");
    await dialog
      .getByRole("textbox", { name: "Name", exact: true })
      .fill(name + " charter");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const editor = page.locator(".library-text");
    await expect(editor).toBeVisible();
    await editor.fill("A charter saved by the UI audit.");
    await page
      .getByRole("button", { name: "Save file version", exact: true })
      .click();
    await expect(page.getByText("Unsaved edits", { exact: true })).toHaveCount(
      0,
    );
    const version = page.getByLabel("File version");
    await expect(version.locator("option")).toHaveCount(2);
    const first = await version.locator("option").nth(1).getAttribute("value");
    await editor.fill("An intentionally unsaved edit");
    await version.selectOption(first);
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await expect(editor).toHaveValue("An intentionally unsaved edit");
    await version.selectOption(first);
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Discard changes" })
      .click();
    await expect(version).toHaveValue(first);
    await version.selectOption("");
    await expect(editor).toHaveValue("A charter saved by the UI audit.");
    await page
      .getByRole("button", { name: "Delete document", exact: true })
      .click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Confirm", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Create document", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Delete archive", exact: true })
      .click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Confirm", exact: true })
      .click();
    await expect(page.getByText(name, { exact: true })).toHaveCount(0);
  });
  test("System Designer protects drafts across workspace navigation without double confirmation", async ({
    page,
    request,
  }) => {
    const doc = await (await request.get(`/api/worlds/${world}`)).json();
    const root = doc.entities.find(
      (e) =>
        e.ageId === doc.world.currentAge && e.kind === "system" && !e.deleted,
    );
    test.skip(!root, "The audit world needs a saved system.");
    await page.goto(`${base}/magic?system=${root.id}`);
    const name = page
      .locator(".system-record-editor")
      .getByRole("textbox", { name: "Name", exact: true });
    await name.fill(root.name + " unsaved audit");
    await page
      .getByRole("navigation", { name: "World navigation" })
      .getByRole("button", { name: "Atlas", exact: true })
      .click();
    await expect(page.getByRole("alertdialog")).toBeVisible();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await expect(name).toHaveValue(root.name + " unsaved audit");
    await page
      .getByRole("navigation", { name: "World navigation" })
      .getByRole("button", { name: "Atlas", exact: true })
      .click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Discard changes" })
      .click();
    await expect(page.locator("main h1")).toHaveText("Atlas");
    await expect(page.getByRole("alertdialog")).toHaveCount(0);
  });
  test("profile editors and document surfaces remain usable on mobile", async ({
    page,
    request,
  }) => {
    const doc = await (await request.get(`/api/worlds/${world}`)).json();
    const entities = doc.entities.filter(
      (e) => e.ageId === doc.world.currentAge && !e.deleted,
    );
    for (const kind of ["organization", "character", "religion", "lifeform"]) {
      const entity = entities.find((e) => e.kind === kind);
      if (!entity) continue;
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 900 });
        await page.goto(
          `${base}/profile?entity=${entity.id}&age=${entity.ageId}`,
        );
        await expect(page.locator("main h1")).toBeVisible();
        await expect(page.locator(".age-bar")).toHaveCount(0);
        expect(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth + 1,
          ),
          kind,
        ).toBe(true);
        await page.screenshot({
          path: `${screenshots}/${kind}-profile-${width}.png`,
          fullPage: true,
        });
      }
      if (kind === "religion") {
        await page
          .getByRole("button", { name: "Add Content", exact: true })
          .click();
        const dialog = page.getByRole("dialog");
        await dialog.getByRole("button", { name: "Add typed field" }).click();
        await dialog
          .getByRole("button", { name: "Add contextual variation" })
          .click();
        expect(
          await dialog
            .locator(".ui-dialog-body")
            .evaluate((e) => e.scrollWidth <= e.clientWidth + 1),
        ).toBe(true);
        await page.screenshot({
          path: `${screenshots}/culture-content-390.png`,
        });
        await page.keyboard.press("Escape");
        await page
          .getByRole("alertdialog")
          .getByRole("button", { name: "Discard changes" })
          .click();
      }
    }
    for (const extension of ["docx", "xlsx", "pdf"]) {
      const file = doc.libraryFiles?.find(
        (f) =>
          f.ageId === doc.world.currentAge &&
          f.versions.at(-1).name.endsWith("." + extension),
      );
      if (!file) continue;
      await page.goto(`${base}/library?record=${file.id}`);
      await expect(page.locator(".library-editor")).toBeVisible();
      if (extension === "docx")
        await expect(page.locator(".docx-paper textarea").first()).toHaveCSS(
          "background-color",
          "rgb(255, 255, 255)",
        );
      if (extension === "xlsx")
        await expect(
          page.locator(".spreadsheet-scroll input").first(),
        ).toHaveCSS("background-color", "rgb(255, 255, 255)");
      if (extension === "pdf")
        await expect(page.locator(".library-pdf")).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth + 1,
        ),
        extension,
      ).toBe(true);
      await page.screenshot({
        path: `${screenshots}/library-${extension}-390.png`,
        fullPage: true,
      });
    }
  });
  test("legacy terrain builder and generator still load inside their scoped styling", async ({
    page,
    request,
  }) => {
    const doc = await (await request.get(`/api/worlds/${world}`)).json();
    const map = doc.maps.find(
      (m) => m.ageId === doc.world.currentAge && !m.parentId,
    );
    test.skip(!map, "The audit world needs an accepted terrain.");
    for (const view of ["build", "generate"]) {
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(
        `/world/${map.projectId}/${view}?world=${world}&age=${map.ageId}`,
      );
      await expect(page.locator(".terrain-app")).toBeVisible();
      await expect(
        page.locator(view === "build" ? ".builder-layout" : ".generator-host"),
      ).toBeVisible({ timeout: 60000 });
      await page.screenshot({
        path: `${screenshots}/terrain-${view}-1440.png`,
        fullPage: true,
      });
    }
  });
});
test.beforeAll(async () => {
  await mkdir(screenshots, { recursive: true });
});
test.describe("Worldbuilding UI", () => {
  test.skip(!world, "Set KRIEMHILD_UI_WORLD to a populated audit world.");
  for (const width of [1440, 900, 390]) {
    test(`distinct workspaces, routes and navigation at ${width}px`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height: 900 });
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      for (const route of [
        "atlas",
        "biosphere",
        "flora",
        "culture",
        "characters",
        "organizations",
        "magic",
        "library",
        "years",
        "graph",
        "writing",
      ]) {
        await page.goto(`${base}/${route}`);
        await expect(page.locator("main h1")).toBeVisible();
        await expect(page.getByText("Working...", { exact: true })).toHaveCount(
          0,
        );
        await page.evaluate(() => document.fonts.ready);
        expect(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth + 1,
          ),
          route,
        ).toBe(true);
        await expect(page.locator(".age-bar")).toHaveCount(
          ["graph", "writing"].includes(route) ? 0 : 1,
        );
        if (width === 390 && route !== "writing") {
          await page.getByRole("button", { name: "Menu", exact: true }).click();
          await expect(
            page.getByRole("navigation", { name: "World navigation" }),
          ).toBeVisible();
          await page
            .getByRole("button", { name: "Close menu", exact: true })
            .click();
        }
        await page.screenshot({
          path: `${screenshots}/${route}-${width}.png`,
          fullPage: true,
        });
      }
      expect(errors).toEqual([]);
    });
  }
  test("Library dialog, validation, focus, dirty cancellation and responsive sizing", async ({
    page,
  }) => {
    await page.goto(`${base}/library`);
    const trigger = page.getByRole("button", { name: "Create archive folder" });
    await trigger.click();
    const dialog = page.getByRole("dialog", { name: "Create archive folder" });
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByRole("textbox", { name: "Name", exact: true }),
    ).toBeFocused();
    await expect(
      dialog.getByRole("button", { name: "Save", exact: true }),
    ).toBeDisabled();
    for (const width of [1440, 900, 390]) {
      await page.setViewportSize({ width, height: 900 });
      const box = await dialog.boundingBox();
      expect(box.x).toBeGreaterThanOrEqual(15);
      expect(box.x + box.width).toBeLessThanOrEqual(width - 15);
      await page.screenshot({
        path: `${screenshots}/library-dialog-${width}.png`,
      });
    }
    await dialog
      .getByRole("textbox", { name: "Name", exact: true })
      .fill("Unsaved archive");
    await page.keyboard.press("Escape");
    const confirmation = page.getByRole("alertdialog");
    await expect(confirmation).toBeVisible();
    await confirmation
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await expect(
      dialog.getByRole("textbox", { name: "Name", exact: true }),
    ).toHaveValue("Unsaved archive");
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await confirmation.getByRole("button", { name: "Discard changes" }).click();
    await expect(dialog).toHaveCount(0);
    await expect(trigger).toBeFocused();
  });
  test("Biosphere wizard preview keeps habitat groups in its scrolling body", async ({
    page,
  }) => {
    await page.goto(`${base}/biosphere`);
    const trigger = page.getByRole("button", {
      name: "Generate Biosphere",
      exact: true,
    });
    await expect(trigger).toBeEnabled();
    await trigger.click();
    let dialog = page.getByRole("dialog");
    const start = await dialog.boundingBox();
    await dialog.getByRole("button", { name: "Next", exact: true }).click();
    expect((await dialog.boundingBox()).width).toBeCloseTo(start.width, 1);
    expect((await dialog.boundingBox()).height).toBeCloseTo(start.height, 1);
    await dialog.getByRole("button", { name: "Next", exact: true }).click();
    await dialog
      .getByRole("checkbox", { name: /I reviewed the structured rules/ })
      .check();
    await dialog
      .getByRole("button", { name: "Generate preview", exact: true })
      .click();
    dialog = page.getByRole("dialog", { name: "Community preview" });
    await expect(dialog).toBeVisible({ timeout: 60000 });
    await expect(dialog.locator(".bio-preview-group").first()).toBeVisible();
    expect(
      await dialog.locator(".dialog-actions .bio-preview-group").count(),
    ).toBe(0);
    for (const width of [1440, 900, 390]) {
      await page.setViewportSize({ width, height: 900 });
      expect(
        await dialog
          .locator(".ui-dialog-body")
          .evaluate((e) => e.scrollWidth <= e.clientWidth + 1),
      ).toBe(true);
      await expect(
        dialog.getByRole("button", { name: /Accept .* selected ranges/ }),
      ).toBeInViewport();
      await page.screenshot({
        path: `${screenshots}/biosphere-preview-${width}.png`,
      });
    }
    await dialog.getByRole("button", { name: "Discard preview" }).click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Discard changes" })
      .click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });
  test("creation forms and nested Culture editors use shared dialog structure", async ({
    page,
  }) => {
    for (const [route, button] of [
      ["organizations", "Create Organization"],
      ["culture", "Create concept"],
      ["magic", "Create System"],
      ["characters", "Create Character"],
    ]) {
      await page.goto(`${base}/${route}`);
      await page.getByRole("button", { name: button, exact: true }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog.locator(".ui-dialog-body")).toBeVisible();
      await expect(dialog.locator(".dialog-actions")).toBeVisible();
      await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
      await expect(dialog).toHaveCount(0);
    }
  });
});
