import { test, expect } from "@playwright/test";
import { readFile, mkdir } from "node:fs/promises";

test.describe("Custom Scripts persisted workflow", () => {
  test.skip(
    process.env.KRIEMHILD_LANGUAGES_TEST !== "1",
    "Explicit opt-in with an isolated test server required.",
  );
  let world,
    age,
    headers,
    extra = [];
  test.beforeEach(async ({ request }) => {
    const me = await (await request.get("/api/me")).json();
    headers = { "X-CSRF-Token": me.csrfToken };
    const response = await request.post("/api/worlds", {
      headers,
      data: { name: "Disposable Script browser audit" },
    });
    expect(response.ok()).toBeTruthy();
    const doc = await response.json();
    world = doc.world.id;
    age = doc.world.currentAge;
    extra = [];
  });
  test.afterEach(async ({ request }) => {
    for (const id of [...extra, world].filter(Boolean))
      expect(
        (await request.delete(`/api/worlds/${id}`, { headers })).ok(),
      ).toBeTruthy();
  });
  const output = (page) =>
    page.getByRole("region", { name: "Script render result" });
  async function paint(page, button = "Preview draft sequence") {
    const response = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        r
          .url()
          .includes(
            button === "Render saved sequence" ? "/run" : "/render-script",
          ),
    );
    await page.getByRole("button", { name: button, exact: true }).click();
    const data = await (await response).json();
    await expect(
      output(page).getByRole("heading", {
        name: data.scriptResult.status,
        exact: true,
      }),
    ).toBeVisible();
    return data.scriptResult;
  }
  async function saveCompile(page) {
    await page.getByRole("button", { name: "Save draft", exact: true }).click();
    await expect(
      page.getByText("Draft saved. Compilation and activation are separate."),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Compile saved revision", exact: true })
      .click();
    await expect(
      page.getByText("Compile report saved for this revision."),
    ).toBeVisible();
  }
  test("draw artwork, map a reading, compose, publish, reload and retain an older revision", async ({
    page,
    request,
  }) => {
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(`/worlds/${world}/languages`);
    await page
      .getByRole("button", { name: "Create language or script", exact: true })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog
      .getByRole("combobox", { name: "Record type", exact: true })
      .selectOption("script");
    await dialog
      .getByRole("textbox", { name: "Name", exact: true })
      .fill("My drawn alphabet");
    await dialog.getByRole("button", { name: "Continue", exact: true }).click();
    await dialog
      .getByRole("combobox", { name: "Script model", exact: true })
      .selectOption("2");
    for (let i = 0; i < 3; i++)
      await dialog
        .getByRole("button", { name: "Continue", exact: true })
        .click();
    await dialog
      .getByRole("button", { name: "Save draft", exact: true })
      .click();
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    const work = page.getByRole("region", {
      name: "Custom writing system workbench",
    });
    await work.getByRole("button", { name: "New glyph", exact: true }).click();
    await work
      .getByRole("textbox", { name: "Glyph name", exact: true })
      .fill("Triangle");
    const glyphID = await work
      .getByRole("textbox", { name: "Stable glyph ID", exact: true })
      .inputValue();
    const canvas = work.getByRole("img", {
      name: "Glyph drawing area",
      exact: true,
    });
    for (const point of [
      [100, 800],
      [500, 100],
      [900, 800],
    ]) {
      const position = await canvas.evaluate((svg, p) => {
        const q = svg.createSVGPoint();
        q.x = p[0];
        q.y = p[1];
        const pixel = q.matrixTransform(svg.getScreenCTM()),
          r = svg.getBoundingClientRect();
        return { x: pixel.x - r.left, y: pixel.y - r.top };
      }, point);
      await canvas.click({ position });
    }
    await work
      .getByRole("button", { name: "Add Z command", exact: true })
      .click();
    const coordinate = work.getByRole("spinbutton", {
      name: "Command 2 coordinate 2",
      exact: true,
    });
    const originalCoordinate = await coordinate.inputValue();
    await coordinate.fill("120");
    await coordinate.press("Tab");
    await work
      .getByRole("button", { name: "Undo glyph edit", exact: true })
      .click();
    await expect(coordinate).toHaveValue(originalCoordinate);
    await work
      .getByRole("button", { name: "Redo glyph edit", exact: true })
      .click();
    await expect(coordinate).toHaveValue("120");
    await work
      .getByRole("textbox", { name: "Glyph name", exact: true })
      .fill("Renamed triangle");
    await expect(
      work.getByRole("textbox", { name: "Stable glyph ID", exact: true }),
    ).toHaveValue(glyphID);
    for (const size of [24, 64, 192])
      await expect(
        work.getByRole("img", {
          name: `Renamed triangle glyph at ${size} pixels`,
          exact: true,
        }),
      ).toBeVisible();
    await work.getByRole("button", { name: "New sign", exact: true }).click();
    await work
      .getByRole("textbox", { name: "Sign label", exact: true })
      .fill("ka");
    await work.getByRole("checkbox", { name: "syllabic", exact: true }).check();
    await work
      .getByRole("textbox", {
        name: "Authored Unicode/grapheme association (optional)",
        exact: true,
      })
      .fill("e\u0301");
    const signID = await work
      .getByRole("textbox", { name: "Stable sign ID", exact: true })
      .inputValue();
    const readings = work
      .locator("fieldset.language-array")
      .filter({ hasText: "Readings" });
    await readings
      .getByRole("button", { name: "Add readings", exact: true })
      .click();
    await readings
      .getByRole("textbox", { name: "Text", exact: true })
      .fill("ka");
    await work
      .getByRole("button", { name: "New sequence", exact: true })
      .click();
    await work
      .getByRole("textbox", { name: "Sequence name", exact: true })
      .fill("Authored phrase");
    await work.getByRole("button", { name: "Add sign", exact: true }).click();
    const node = work.getByRole("group", {
      name: "sequence node 1",
      exact: true,
    });
    await node
      .getByRole("combobox", { name: "Sign", exact: true })
      .selectOption(signID);
    await node
      .getByRole("combobox", { name: "Reading", exact: true })
      .selectOption({ index: 1 });
    await node.getByRole("checkbox", { name: "syllabic", exact: true }).check();
    const preview = await paint(page);
    expect(preview.status).toBe("success");
    expect(preview.signs[0].glyph).toBe(glyphID);
    expect(preview.signs[0].grapheme).toBe("e\u0301");
    expect(preview.svg).toContain("data-glyph");
    await saveCompile(page);
    await page
      .getByRole("button", { name: "Activate revision", exact: true })
      .click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Activate revision", exact: true })
      .click();
    await expect(
      page.getByText("Active definition updated for this Age."),
    ).toBeVisible();
    const first = await paint(page, "Render saved sequence");
    expect(first).toEqual(preview);
    const firstHash = first.revision;
    await page.reload();
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    expect(await paint(page, "Render saved sequence")).toEqual(first);
    await page
      .getByRole("spinbutton", { name: "Command 2 coordinate 2", exact: true })
      .fill("180");
    await page
      .getByRole("spinbutton", { name: "Command 2 coordinate 2", exact: true })
      .press("Tab");
    await page
      .getByRole("combobox", { name: "Writing direction", exact: true })
      .selectOption("rtl");
    await saveCompile(page);
    const next = await paint(page, "Render saved sequence");
    expect(next.revision).not.toBe(firstHash);
    expect(next.svg).not.toBe(first.svg);
    const doc = await (await request.get(`/api/worlds/${world}`)).json(),
      script = doc.entities.find((e) => e.kind === "script");
    expect(script.script.activeRevision).toBe(firstHash);
    expect(script.script.revisions).toHaveLength(3);
    await page
      .getByRole("combobox", { name: "Definition revision", exact: true })
      .selectOption(firstHash);
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    expect(await paint(page, "Render saved sequence")).toEqual(first);
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(
      page.getByRole("button", { name: "Render saved sequence", exact: true }),
    ).toBeVisible();
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => document.activeElement.tagName)).not.toBe(
      "BODY",
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
    ).toBe(true);
    await mkdir(".cache/languages/brief3/browser", { recursive: true });
    await page.screenshot({
      path: ".cache/languages/brief3/browser/drawn-mobile.png",
      fullPage: true,
    });
    expect(errors).toEqual([]);
  });
  test("safe SVG review, mixed roles, explicit readings, RTL and World ZIP replay", async ({
    page,
    request,
  }) => {
    const definition = JSON.parse(
      await readFile("internal/languages/testdata/vector-script.json", "utf8"),
    );
    definition.writing.signs[3].readings.forEach((r) => (r.when = {}));
    let doc = await (await request.get(`/api/worlds/${world}`)).json();
    const saved = await request.post(`/api/worlds/${world}/languages`, {
      headers,
      data: {
        ageId: age,
        revision: doc.world.revision,
        kind: "script",
        definition,
        metadata: {},
      },
    });
    expect(saved.ok()).toBeTruthy();
    await page.goto(
      `/worlds/${world}/languages?language=${definition.scriptId}`,
    );
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    const work = page.getByRole("region", {
      name: "Custom writing system workbench",
    });
    const stable = await work
      .getByRole("textbox", { name: "Stable glyph ID", exact: true })
      .inputValue();
    let external = 0;
    page.on("request", (r) => {
      if (r.url().includes("example.com")) external++;
    });
    await work.getByText("Import safe SVG artwork", { exact: true }).click();
    await work
      .getByRole("textbox", { name: "SVG source", exact: true })
      .fill(
        '<svg><script>window.unsafeGlyph=true;fetch("https://example.com/unsafe")</script></svg>',
      );
    await work
      .getByRole("button", { name: "Normalize SVG", exact: true })
      .click();
    await expect(
      work.getByRole("alert").filter({ hasText: "svg_import_rejected" }),
    ).toBeVisible();
    expect(await page.evaluate(() => window.unsafeGlyph)).toBeUndefined();
    expect(external).toBe(0);
    await work
      .getByRole("textbox", { name: "SVG source", exact: true })
      .fill(
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><path fill="none" stroke="#182126" stroke-width="2" d="m10 80 l40 -60 l40 60 z"/></svg>',
      );
    await work
      .getByRole("button", { name: "Normalize SVG", exact: true })
      .click();
    await expect(
      work.getByRole("region", { name: "Normalized SVG review" }),
    ).toBeVisible();
    await work
      .getByRole("button", { name: "Use imported artwork", exact: true })
      .click();
    await expect(
      work.getByRole("textbox", { name: "Stable glyph ID", exact: true }),
    ).toHaveValue(stable);
    const mixed = await paint(page);
    expect(mixed.status).toBe("success");
    expect(mixed.signs).toHaveLength(3);
    expect(mixed.signs[1].roles).toEqual(["logographic", "semantic"]);
    expect(mixed.signs[2].text).toBe("");
    expect(mixed.signs[2].roles).toEqual(["classifier"]);
    expect(mixed.signs[0].advance).toBe(1080);
    await page
      .getByRole("combobox", { name: "Writing direction", exact: true })
      .selectOption("rtl");
    await saveCompile(page);
    const rendered = await paint(page, "Render saved sequence");
    expect(rendered.signs[0].x).toBe(-1080);
    const zip = await request.get(`/api/worlds/${world}/export`);
    expect(zip.ok()).toBeTruthy();
    const importedResponse = await request.post("/api/worlds/import", {
      headers: { ...headers, "Content-Type": "application/zip" },
      data: await zip.body(),
    });
    expect(importedResponse.ok(), await importedResponse.text()).toBeTruthy();
    const imported = await importedResponse.json();
    extra.push(imported.world.id);
    const copied = await request.post(
      `/api/worlds/${imported.world.id}/languages/${definition.scriptId}/run`,
      {
        headers,
        data: {
          ageId: age,
          definitionRevision: rendered.revision,
          scriptRequest: { ...rendered.input },
        },
      },
    );
    expect(copied.ok()).toBeTruthy();
    expect((await copied.json()).scriptResult).toEqual(rendered);
    const body = await (
      await request.get(
        `/api/worlds/${world}/languages/${definition.scriptId}?age=${age}`,
      )
    ).json();
    expect(JSON.stringify(body)).not.toContain("<svg");
    expect(body.writing.glyphs[0].id).toBe(stable);
    await work
      .getByRole("button", { name: "New sequence", exact: true })
      .click();
    await work
      .getByRole("textbox", { name: "Sequence name", exact: true })
      .fill("Explicit ambiguity");
    await work.getByRole("button", { name: "Add sign", exact: true }).click();
    const node = work.getByRole("group", {
      name: "sequence node 1",
      exact: true,
    });
    await node
      .getByRole("combobox", { name: "Sign", exact: true })
      .selectOption("polyphonic");
    await node.getByRole("checkbox", { name: "phonetic", exact: true }).check();
    await node.getByRole("checkbox", { name: "semantic", exact: true }).check();
    const ambiguous = await paint(page);
    expect(ambiguous.status).toBe("choice_required");
    expect(ambiguous.choices[0].readings).toEqual(["front", "back"]);
    expect(ambiguous.svg).toBe("");
    await node
      .getByRole("combobox", { name: "Reading", exact: true })
      .selectOption("front");
    const chosen = await paint(page);
    expect(chosen.status).toBe("success");
    expect(chosen.signs[0].roles).toEqual(["phonetic", "semantic"]);
    expect(external).toBe(0);
  });
  test("large glyph libraries and paths remain paginated and keyboard accessible", async ({
    page,
    request,
  }) => {
    const definition = JSON.parse(
      await readFile("internal/languages/testdata/vector-script.json", "utf8"),
    );
    const original = structuredClone(definition.writing.glyphs[0]);
    definition.writing.glyphs = [
      original,
      ...Array.from({ length: 63 }, (_, i) => ({
        ...structuredClone(original),
        id: `library-${i}`,
        name: `Library glyph ${i}`,
      })),
      ...definition.writing.glyphs.slice(1),
    ];
    original.paths[0].commands = [
      { op: "M", values: [100, 100] },
      ...Array.from({ length: 100 }, (_, i) => ({
        op: "L",
        values: [100 + i, 100 + i],
      })),
      { op: "Z", values: [] },
    ];
    const doc = await (await request.get(`/api/worlds/${world}`)).json();
    expect(
      (
        await request.post(`/api/worlds/${world}/languages`, {
          headers,
          data: {
            ageId: age,
            revision: doc.world.revision,
            kind: "script",
            definition,
            metadata: {},
          },
        })
      ).ok(),
    ).toBeTruthy();
    await page.goto(
      `/worlds/${world}/languages?language=${definition.scriptId}`,
    );
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    await expect(
      page.getByText("Glyph page 1 of 4", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Command page 1 of 6", { exact: true }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Next commands", exact: true })
      .click();
    await expect(
      page.getByRole("spinbutton", {
        name: "Command 21 coordinate 1",
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("spinbutton", {
        name: "Command 1 coordinate 1",
        exact: true,
      }),
    ).toHaveCount(0);
    await page
      .getByRole("button", { name: "Next glyphs", exact: true })
      .click();
    await expect(
      page.getByText("Glyph page 2 of 4", { exact: true }),
    ).toBeVisible();
    const search = page.getByRole("textbox", {
      name: "Search glyphs",
      exact: true,
    });
    await search.fill("Library glyph 62");
    await page
      .getByRole("button", { name: "Library glyph 62", exact: true })
      .click();
    await expect(
      page.getByRole("textbox", { name: "Stable glyph ID", exact: true }),
    ).toHaveValue("library-62");
    await page.setViewportSize({ width: 390, height: 844 });
    await search.focus();
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => document.activeElement.tagName)).not.toBe(
      "BODY",
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
    ).toBeTruthy();
  });
  test("changing sequence supersedes a late preview", async ({
    page,
    request,
  }) => {
    const definition = JSON.parse(
      await readFile("internal/languages/testdata/vector-script.json", "utf8"),
    );
    definition.writing.sequences.push({
      id: "silent",
      name: "Silent classifier",
      items: [{ kind: "sign", sign: "living-marker", roles: ["classifier"] }],
    });
    const doc = await (await request.get(`/api/worlds/${world}`)).json();
    expect(
      (
        await request.post(`/api/worlds/${world}/languages`, {
          headers,
          data: {
            ageId: age,
            revision: doc.world.revision,
            kind: "script",
            definition,
            metadata: {},
          },
        })
      ).ok(),
    ).toBeTruthy();
    await page.goto(
      `/worlds/${world}/languages?language=${definition.scriptId}`,
    );
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    let release, captured;
    const held = new Promise((resolve) => {
      release = resolve;
    });
    const received = new Promise((resolve) => {
      captured = resolve;
    });
    await page.route(
      "**/render-script",
      async (route) => {
        const response = await route.fetch();
        captured();
        await held;
        await route.fulfill({ response });
      },
      { times: 1 },
    );
    await page
      .getByRole("button", { name: "Preview draft sequence", exact: true })
      .click();
    await received;
    await page
      .getByRole("combobox", { name: "Selected sequence", exact: true })
      .selectOption("silent");
    release();
    const next = await paint(page);
    expect(next.input.sequenceId).toBe("silent");
    expect(next.signs).toHaveLength(1);
    expect(next.signs[0].sign).toBe("living-marker");
    await expect(
      output(page).getByRole("img", {
        name: "Rendered authored sign sequence",
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      output(page).getByRole("heading", { name: "success", exact: true }),
    ).toBeVisible();
  });
});
