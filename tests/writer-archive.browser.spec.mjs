import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { unzipSync, strFromU8 } from "fflate";
import { PDFDocument } from "pdf-lib";

test("Writer's Archive creates, autosaves, exports, imports images, moves, deletes and restores", async ({
  page,
}, testInfo) => {
  test.skip(
    process.env.KRIEMHILD_ARCHIVE_AUDIT !== "1",
    "Run against an isolated archive audit server.",
  );
  page.setDefaultTimeout(15000);
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await expect(
    page.getByRole("textbox", { name: "New world name" }),
  ).toBeVisible();
  const world = await page.evaluate(async () => {
    const r = await fetch("/api/worlds", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "Writer Archive audit" }),
    });
    if (!r.ok) throw Error(await r.text());
    return (await r.json()).world.id;
  });
  const url = `/worlds/${world}/writing`;
  const create = async (type, name) => {
    await page.locator(".archive-new-menu > summary").click();
    await page.getByRole("button", { name: type, exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name", { exact: true }).fill(name);
    await dialog.getByRole("button", { name: "Create", exact: true }).click();
    await expect(dialog).toHaveCount(0);
  };
  const back = () =>
    page.getByRole("button", { name: "Back to archive", exact: true }).click();
  try {
    await page.goto(url);
    await expect(
      page.getByRole("heading", { name: "Writer's Archive", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("navigation", { name: "World spaces" })
      .getByRole("button", { name: "Ages", exact: true })
      .click();
    await expect(
      page.getByRole("heading", { name: "Time Graph", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("navigation", { name: "World spaces" })
      .getByRole("button", { name: "Writer's Archive", exact: true })
      .click();
    await expect(
      page.getByRole("heading", { name: "Writer's Archive", exact: true }),
    ).toBeVisible();
    await create("New folder", "World notes");
    await page
      .getByRole("button", { name: "World notes", exact: true })
      .click();
    await create("Word document (.docx)", "Opening");
    const body = page.getByRole("textbox", {
      name: "Document body",
      exact: true,
    });
    await expect(body).toBeVisible();
    await body.fill("Світи and stories. This paragraph saves automatically.");
    await expect(
      page.locator(".archive-document-heading [role=status]"),
    ).toHaveText("Saved");
    await page.getByLabel("Paragraph style", { exact: true }).selectOption("2");
    await expect(body.locator("h2")).toContainText("Світи");
    const png = Buffer.from(
      await page.evaluate(() => {
        const canvas = document.createElement("canvas");
        canvas.width = 160;
        canvas.height = 90;
        const ctx = canvas.getContext("2d");
        ctx.fillStyle = "#cbbc89";
        ctx.fillRect(0, 0, 160, 90);
        ctx.fillStyle = "#253b2b";
        ctx.font = "18px serif";
        ctx.fillText("World sketch", 25, 50);
        return canvas.toDataURL("image/png").split(",")[1];
      }),
      "base64",
    );
    await page
      .locator(".archive-document input[type=file][accept='image/*']")
      .setInputFiles({
        name: "reference.png",
        mimeType: "image/png",
        buffer: png,
      });
    await expect(body.locator("img")).toHaveCount(1);
    await expect(
      page.locator(".archive-document-heading [role=status]"),
    ).toHaveText("Saved");
    const files = {};
    for (const [f, label] of [
      ["docx", "Word document (.docx)"],
      ["pdf", "PDF document (.pdf)"],
      ["txt", "Plain text (.txt)"],
      ["md", "Markdown (.md)"],
    ]) {
      await page.locator(".archive-export-menu > summary").click();
      const pending = page.waitForEvent("download", { timeout: 15000 });
      await page.getByRole("button", { name: label, exact: true }).click();
      const d = await pending;
      const path = testInfo.outputPath("Opening." + f);
      await d.saveAs(path);
      files[f] = await readFile(path);
      await page
        .locator(".archive-export-menu")
        .evaluate((el) => el.removeAttribute("open"));
    }
    expect(strFromU8(unzipSync(files.docx)["word/document.xml"])).toContain(
      "Світи",
    );
    expect(
      Object.keys(unzipSync(files.docx)).some((n) =>
        n.startsWith("word/media/"),
      ),
    ).toBeTruthy();
    expect((await PDFDocument.load(files.pdf)).getPageCount()).toBeGreaterThan(
      0,
    );
    expect(files.txt.toString()).toContain("saves automatically");
    expect(files.md.toString()).toContain("## Світи");
    await page.screenshot({
      path: testInfo.outputPath("editor.png"),
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: testInfo.outputPath("editor-mobile.png"),
      fullPage: true,
    });
    await page.setViewportSize({ width: 1280, height: 720 });
    await back();
    await expect(
      page.getByRole("button", { name: "Opening.docx", exact: true }),
    ).toBeVisible();
    // A normal reload must reopen the acknowledged text and embedded image.
    await page.reload();
    await page
      .getByRole("button", { name: "World notes", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Opening.docx", exact: true })
      .click();
    await expect(body).toContainText("saves automatically");
    await expect(body.locator("img")).toHaveCount(1);
    await back();
    await page.locator("input[type=file][multiple]").setInputFiles([
      { name: "drawing.png", mimeType: "image/png", buffer: png },
      {
        name: "notes.md",
        mimeType: "text/markdown",
        buffer: Buffer.from("# Imported notes\n\n**Bold research**"),
      },
      { name: "original.pdf", mimeType: "application/pdf", buffer: files.pdf },
    ]);
    await expect(
      page.getByRole("button", { name: "notes.md", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "original.pdf", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "drawing.png", exact: true })
      .click();
    await expect(page.locator(".archive-image-stage img")).toBeVisible();
    await back();
    await page.getByRole("button", { name: "notes.md", exact: true }).click();
    await expect(body).toContainText("Imported notes");
    await expect(body.locator("strong")).toContainText("Bold research");
    await back();
    await page
      .getByRole("button", { name: "original.pdf", exact: true })
      .click();
    await expect(body).toContainText("Світи");
    await back();
    // Move the document to the World root, then delete and restore its former folder.
    await page
      .getByRole("row")
      .filter({
        has: page.getByRole("button", { name: "Opening.docx", exact: true }),
      })
      .click();
    await page
      .locator(".archive-selection-bar")
      .getByRole("button", { name: "Move", exact: true })
      .click();
    await page
      .getByRole("dialog")
      .getByLabel("Destination folder")
      .selectOption("root");
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Move here", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await page
      .locator(".archive-sidebar nav")
      .getByRole("button", { name: "My archive", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Opening.docx", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("row")
      .filter({
        has: page.getByRole("button", { name: "World notes", exact: true }),
      })
      .click();
    await page
      .locator(".archive-selection-bar")
      .getByRole("button", { name: "Delete", exact: true })
      .click();
    await page
      .locator(".archive-sidebar nav")
      .getByRole("button", { name: "Trash", exact: true })
      .click();
    await page
      .getByRole("row")
      .filter({
        has: page.getByRole("button", { name: "World notes", exact: true }),
      })
      .click();
    await page
      .locator(".archive-selection-bar")
      .getByRole("button", { name: "Restore", exact: true })
      .click();
    await page
      .locator(".archive-sidebar nav")
      .getByRole("button", { name: "My archive", exact: true })
      .click();
    await page
      .getByRole("button", { name: "World notes", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "drawing.png", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Grid view", exact: true }).click();
    await page.screenshot({
      path: testInfo.outputPath("archive.png"),
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    await page.screenshot({
      path: testInfo.outputPath("archive-mobile.png"),
      fullPage: true,
    });
    await page.setViewportSize({ width: 1280, height: 720 });
    await page
      .locator(".archive-sidebar nav")
      .getByRole("button", { name: "Inworld Archive", exact: true })
      .click();
    const ages = await page.evaluate(
      async (world) =>
        (await (await fetch(`/api/worlds/${world}?view=metadata`)).json()).ages,
      world,
    );
    for (const age of ages)
      await expect(
        page.getByRole("button", { name: age.name, exact: true }),
      ).toBeVisible();
    expect(errors).toEqual([]);
    await expect(page.locator("[role=alert]")).toHaveCount(0);
  } finally {
    if (!page.isClosed())
      await page.evaluate(async (world) => {
        const r = await fetch(`/api/worlds/${world}`, { method: "DELETE" });
        if (!r.ok) throw Error(await r.text());
      }, world);
  }
});

test("Inworld Archive opens the same Age folders and autosaves back to the Library", async ({
  page,
}) => {
  test.skip(
    process.env.KRIEMHILD_ARCHIVE_AUDIT !== "1",
    "Run against an isolated archive audit server.",
  );
  page.setDefaultTimeout(15000);
  await page.goto("/");
  await page.getByRole("textbox", { name: "New world name" }).waitFor();
  const fixture = await page.evaluate(async () => {
    const post = async (path, body) => {
      const r = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!r.ok) throw Error(await r.text());
      return r.json();
    };
    let d = await post("/api/worlds", { name: "Linked Archive audit" });
    const world = d.world.id,
      age = d.world.currentAge,
      ageName = d.ages.find((a) => a.id === age).name;
    for (const [id, name, parentArchive] of [
      ["royal", "Royal records", ""],
      ["letters", "Letters", "royal"],
    ])
      d = await post(`/api/worlds/${world}`, {
        kind: "entity",
        ageId: age,
        revision: d.world.revision,
        entity: {
          id,
          ageId: age,
          name,
          kind: "archive",
          properties: { parentArchive },
        },
      });
    const form = new FormData();
    form.append(
      "metadata",
      JSON.stringify({
        revision: d.world.revision,
        archiveId: "letters",
        entity: {
          id: "letter",
          ageId: age,
          name: "A letter",
          kind: "document",
          properties: {},
        },
      }),
    );
    form.append(
      "file",
      new Blob(["# From the Library\n\nOriginal text"]),
      "letter.md",
    );
    const r = await fetch(`/api/worlds/${world}/library`, {
      method: "POST",
      body: form,
    });
    if (!r.ok) throw Error(await r.text());
    return { world, age, ageName };
  });
  try {
    await page.goto(`/worlds/${fixture.world}/writing`);
    await page
      .locator(".archive-sidebar nav")
      .getByRole("button", { name: "Inworld Archive", exact: true })
      .click();
    for (const name of [
      fixture.ageName,
      "Royal records",
      "Letters",
      "A letter",
    ])
      await page.getByRole("button", { name, exact: true }).click();
    const body = page.getByRole("textbox", {
      name: "Document body",
      exact: true,
    });
    await expect(body).toContainText("From the Library");
    await body.fill("A letter edited in the Writer's Archive.");
    await expect(
      page.locator(".archive-document-heading [role=status]"),
    ).toHaveText("Saved to Library");
    const saved = await page.evaluate(async ({ world, age }) => {
      const r = await fetch(`/api/worlds/${world}/library/letter?age=${age}`);
      return r.text();
    }, fixture);
    expect(saved).toContain("edited in the Writer's Archive");
    await page
      .getByRole("button", { name: "Open in Library", exact: true })
      .click();
    await expect(page).toHaveURL(
      new RegExp(`/library\\?age=${fixture.age}&record=letter`),
    );
    await expect(
      page.getByRole("heading", { name: "Library", exact: true }),
    ).toBeVisible();
    await expect(page.locator("[role=alert]")).toHaveCount(0);
  } finally {
    if (!page.isClosed())
      await page.evaluate(async (world) => {
        const r = await fetch(`/api/worlds/${world}`, { method: "DELETE" });
        if (!r.ok) throw Error(await r.text());
      }, fixture.world);
  }
});
