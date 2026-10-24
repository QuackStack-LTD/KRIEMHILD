import { test, expect } from "@playwright/test";
import { readFile, mkdir } from "node:fs/promises";

// Explicit opt-in: this suite writes only to a new, disposable World on the
// isolated test server. It never selects or edits an author's existing World.
test.describe("Languages persisted authoring lifecycle", () => {
  test.skip(
    process.env.KRIEMHILD_LANGUAGES_TEST !== "1",
    "Set KRIEMHILD_LANGUAGES_TEST=1 and KRIEMHILD_UI_URL to an isolated server.",
  );
  let world, headers, age;
  async function seedFixture(request, withDescendant = false) {
    let doc = await (await request.get(`/api/worlds/${world}`)).json();
    const script = JSON.parse(
        await readFile("internal/languages/testdata/script.json", "utf8"),
      ),
      definition = JSON.parse(
        await readFile("internal/languages/testdata/reference.json", "utf8"),
      );
    for (const [kind, body] of [
      ["script", script],
      ["language", definition],
    ]) {
      const response = await request.post(`/api/worlds/${world}/languages`, {
        headers,
        data: {
          ageId: age,
          revision: doc.world.revision,
          kind,
          definition: body,
          metadata: {},
        },
      });
      expect(response.ok(), await response.text()).toBeTruthy();
      doc = await response.json();
    }
    let target;
    if (withDescendant) {
      target = JSON.parse(
        await readFile("internal/languages/testdata/descendant.json", "utf8"),
      );
      target.history.sourceAge = age;
      const response = await request.post(`/api/worlds/${world}/languages`, {
        headers,
        data: {
          ageId: age,
          revision: doc.world.revision,
          kind: "language",
          definition: target,
          metadata: {},
        },
      });
      expect(response.ok(), await response.text()).toBeTruthy();
      doc = await response.json();
    }
    return { definition, script, target, doc };
  }
  test.beforeEach(async ({ request }) => {
    const me = await (await request.get("/api/me")).json();
    headers = { "X-CSRF-Token": me.csrfToken };
    const response = await request.post("/api/worlds", {
      headers,
      data: { name: "Disposable Languages browser audit" },
    });
    expect(response.ok()).toBeTruthy();
    const doc = await response.json();
    world = doc.world.id;
    age = doc.world.currentAge;
  });
  test.afterEach(async ({ request }) => {
    if (world)
      expect(
        (await request.delete(`/api/worlds/${world}`, { headers })).ok(),
      ).toBeTruthy();
  });
  test("saving in a locally selected Age preserves the view and other Age states", async ({
    page,
    request,
  }) => {
    const { definition, doc } = await seedFixture(request);
    const branch = await request.post(`/api/worlds/${world}`, {
      headers,
      data: {
        kind: "age",
        ageId: age,
        mode: "after",
        name: "Successor Age",
        entityNames: { [definition.languageId]: "Successor language" },
        revision: doc.world.revision,
      },
    });
    expect(branch.ok(), await branch.text()).toBeTruthy();
    const branched = await branch.json(),
      nextAge = branched.world.currentAge;
    await page.goto(
      `/worlds/${world}/languages?language=${definition.languageId}`,
    );
    await expect(page.locator(".language-page-header .eyebrow")).toHaveText(
      "Successor Age",
    );
    await expect(
      page.getByRole("textbox", { name: "Name", exact: true }),
    ).toHaveValue("Successor language");
    await page.getByRole("button", { name: "First Age", exact: true }).click();
    await expect(page.locator(".language-page-header .eyebrow")).toHaveText(
      "First Age",
    );
    await page
      .getByRole("textbox", { name: "Author notes", exact: true })
      .fill("Only First Age changes");
    await page.getByRole("button", { name: "Save draft", exact: true }).click();
    await expect(
      page.getByText("Draft saved. Compilation and activation are separate."),
    ).toBeVisible();
    await expect(page.locator(".language-page-header .eyebrow")).toHaveText(
      "First Age",
    );
    await expect(
      page.getByRole("textbox", { name: "Author notes", exact: true }),
    ).toHaveValue("Only First Age changes");
    const saved = await (await request.get(`/api/worlds/${world}`)).json();
    expect(
      saved.entities.find(
        (e) => e.id === definition.languageId && e.ageId === age,
      ).language.revisions,
    ).toHaveLength(2);
    expect(
      saved.entities.find(
        (e) => e.id === definition.languageId && e.ageId === nextAge,
      ).language.revisions,
    ).toHaveLength(1);
  });
  test("historical preview requires explicit approval and appends only the target", async ({
    page,
    request,
  }) => {
    const { definition, target } = await seedFixture(request, true);
    await page.goto(`/worlds/${world}/languages?language=${target.languageId}`);
    await page
      .getByRole("navigation", { name: "Language sections" })
      .getByRole("button", { name: "History", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Preview historical changes", exact: true })
      .click();
    await expect(
      page.getByRole("button", {
        name: "Apply reviewed historical changes",
        exact: true,
      }),
    ).toBeEnabled();
    let doc = await (await request.get(`/api/worlds/${world}`)).json();
    expect(
      doc.entities.find((e) => e.id === target.languageId).language.revisions,
    ).toHaveLength(1);
    await page
      .getByRole("button", {
        name: "Apply reviewed historical changes",
        exact: true,
      })
      .click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Apply historical changes", exact: true })
      .click();
    await expect(
      page.getByText("Historical changes saved as a new target revision.", {
        exact: false,
      }),
    ).toBeVisible();
    doc = await (await request.get(`/api/worlds/${world}`)).json();
    expect(
      doc.entities.find((e) => e.id === target.languageId).language.revisions,
    ).toHaveLength(2);
    expect(
      doc.entities.find((e) => e.id === definition.languageId).language
        .revisions,
    ).toHaveLength(1);
    const oldHash = doc.entities.find((e) => e.id === target.languageId)
      .language.revisions[0].hash;
    await page
      .getByRole("combobox", { name: "Definition revision", exact: true })
      .selectOption(oldHash);
    await page
      .getByRole("button", {
        name: "Restore saved revision as draft",
        exact: true,
      })
      .click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Restore draft", exact: true })
      .click();
    await expect(
      page.getByText("Saved definition restored as the draft;", {
        exact: false,
      }),
    ).toBeVisible();
    doc = await (await request.get(`/api/worlds/${world}`)).json();
    expect(
      doc.entities.find((e) => e.id === target.languageId).language
        .draftRevision,
    ).toBe(oldHash);
    expect(
      doc.entities.find((e) => e.id === target.languageId).language.revisions,
    ).toHaveLength(2);
  });
  test("a late run response cannot overwrite another selected definition", async ({
    page,
    request,
  }) => {
    const { definition, script } = await seedFixture(request);
    let release, started;
    const held = new Promise((r) => (release = r)),
      began = new Promise((r) => (started = r));
    await page.goto(
      `/worlds/${world}/languages?language=${definition.languageId}`,
    );
    await page
      .getByRole("button", { name: "Compile saved revision", exact: true })
      .click();
    await expect(
      page.getByText("Compile report saved for this revision."),
    ).toBeVisible();
    await page.getByRole("button", { name: "Laboratory", exact: true }).click();
    await page
      .getByRole("combobox", { name: "Lexeme", exact: true })
      .selectOption("stone");
    await page
      .getByRole("combobox", { name: "number", exact: true })
      .selectOption("many");
    await page.route("**/api/worlds/*/languages/*/run", async (route) => {
      const response = await route.fetch();
      started();
      await held;
      try {
        await route.fulfill({ response });
      } catch {}
    });
    await page
      .getByRole("button", { name: "Run operation", exact: true })
      .click();
    await began;
    await page
      .locator(".language-index")
      .getByRole("button", { name: /Reference transcription/ })
      .click();
    await expect(
      page.getByRole("heading", { name: script.name, exact: true }),
    ).toBeVisible();
    release();
    await expect(
      page.getByText("Operation finished; author data is unchanged.", {
        exact: true,
      }),
    ).toHaveCount(0);
    await expect(page.locator(".language-results")).toHaveCount(0);
    expect(new URL(page.url()).searchParams.get("language")).toBe(
      script.scriptId,
    );
  });
  test("incomplete wizard, save/reload, dirty navigation, search and narrow viewport", async ({
    page,
    request,
  }) => {
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(`/worlds/${world}/languages`);
    await expect(
      page.getByText("No languages or scripts in this Age.", { exact: false }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Import definition", exact: true })
      .click();
    await page
      .getByRole("dialog")
      .getByLabel("JSON file", { exact: true })
      .setInputFiles({
        name: "invalid-utf8.json",
        mimeType: "application/json",
        buffer: Buffer.from([0xc3, 0x28]),
      });
    await expect(page.getByRole("dialog").getByRole("alert")).toBeVisible();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Create language or script", exact: true })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog
      .getByRole("textbox", { name: "Name", exact: true })
      .fill("An incomplete language");
    for (let i = 0; i < 4; i++)
      await dialog
        .getByRole("button", { name: "Continue", exact: true })
        .click();
    await dialog
      .getByRole("button", { name: "Save draft", exact: true })
      .click();
    await expect(dialog).toHaveCount(0);
    await expect(
      page.getByRole("heading", {
        name: "An incomplete language",
        exact: true,
      }),
    ).toBeVisible();
    await page
      .getByRole("textbox", { name: "Name", exact: true })
      .fill("Revised incomplete language");
    await page.getByRole("button", { name: "Save draft", exact: true }).click();
    await expect(
      page.getByText("Draft saved. Compilation and activation are separate."),
    ).toBeVisible();
    await page.reload();
    await expect(
      page.getByRole("textbox", { name: "Name", exact: true }),
    ).toHaveValue("Revised incomplete language");
    await page
      .getByRole("textbox", { name: "Author notes", exact: true })
      .fill("Keep my unsaved draft");
    await page.getByRole("button", { name: "Atlas", exact: true }).click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await expect(
      page.getByRole("textbox", { name: "Author notes", exact: true }),
    ).toHaveValue("Keep my unsaved draft");
    await page.getByRole("button", { name: "Save draft", exact: true }).click();
    await page
      .getByRole("button", { name: "Compile saved revision", exact: true })
      .click();
    await expect(
      page.getByText("Compile report saved for this revision."),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Activate revision", exact: true }),
    ).toBeDisabled();
    const doc = await (await request.get(`/api/worlds/${world}`)).json(),
      record = doc.entities.find((e) => e.kind === "language");
    expect(record.language.revisions).toHaveLength(2);
    expect(record.language.activeRevision).toBeFalsy();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(
      page.getByRole("heading", { name: "Languages", exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
    ).toBeTruthy();
    await mkdir(".cache/languages/browser", { recursive: true });
    await page.screenshot({
      path: ".cache/languages/browser/incomplete-mobile.png",
      fullPage: true,
    });
    expect(errors).toEqual([]);
  });
  test("authored scripts, compile, inflect, parse, traces, corpus and revision independence", async ({
    page,
    request,
  }) => {
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    let doc = await (await request.get(`/api/worlds/${world}`)).json();
    const script = JSON.parse(
        await readFile("internal/languages/testdata/script.json", "utf8"),
      ),
      definition = JSON.parse(
        await readFile("internal/languages/testdata/reference.json", "utf8"),
      );
    for (const [kind, body] of [
      ["script", script],
      ["language", definition],
    ]) {
      const saved = await request.post(`/api/worlds/${world}/languages`, {
        headers,
        data: {
          ageId: age,
          revision: doc.world.revision,
          kind,
          definition: body,
          metadata: {},
          note: "Browser fixture",
        },
      });
      expect(saved.ok(), await saved.text()).toBeTruthy();
      doc = await saved.json();
    }
    await page.goto(
      `/worlds/${world}/languages?language=${definition.languageId}`,
    );
    await page
      .getByRole("button", { name: "Compile saved revision", exact: true })
      .click();
    await expect(
      page.getByText("Compile report saved for this revision."),
    ).toBeVisible();
    await page.getByRole("button", { name: "Laboratory", exact: true }).click();
    await page
      .getByRole("combobox", { name: "Lexeme", exact: true })
      .selectOption("stone");
    await page
      .getByRole("combobox", { name: "number", exact: true })
      .selectOption("many");
    await page
      .getByRole("button", { name: "Run operation", exact: true })
      .click();
    await expect(
      page.locator(".language-results article>strong").first(),
    ).toHaveText("pad̪a");
    await page.getByText("Derivation trace", { exact: true }).first().click();
    await expect(
      page.getByText("phonology · intervocalic-voicing:", { exact: false }),
    ).toBeVisible();
    await page
      .getByRole("combobox", { name: "Operation", exact: true })
      .selectOption("write");
    await page
      .getByRole("textbox", { name: "Segment IDs", exact: true })
      .fill("p a d a");
    await page
      .getByRole("button", { name: "Run operation", exact: true })
      .click();
    await expect(
      page.locator(".language-results article>strong").first(),
    ).toHaveText("pad̪a");
    await page
      .getByRole("combobox", { name: "Operation", exact: true })
      .selectOption("parse");
    await page.getByLabel("Written form", { exact: true }).fill("pad̪a / nin");
    await page
      .getByRole("button", { name: "Run operation", exact: true })
      .click();
    await expect(page.locator(".language-results>h3")).toHaveText("ambiguous");
    await expect(page.locator(".language-results article")).toHaveCount(2);
    await page
      .getByRole("combobox", { name: "Operation", exact: true })
      .selectOption("regression");
    await page
      .getByRole("button", { name: "Run operation", exact: true })
      .click();
    await expect(
      page.getByText("stone-many: passed", { exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Overview", exact: true }).click();
    await page
      .getByRole("textbox", { name: "Author notes", exact: true })
      .fill("A distinct immutable revision");
    await expect(
      page.getByRole("button", { name: "Compile saved revision", exact: true }),
    ).toBeDisabled();
    await page.getByRole("button", { name: "Save draft", exact: true }).click();
    await expect(
      page.getByText("Draft saved. Compilation and activation are separate."),
    ).toBeVisible();
    await page.getByRole("button", { name: "Laboratory", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Run operation", exact: true }),
    ).toBeDisabled();
    const current = await (await request.get(`/api/worlds/${world}`)).json(),
      state = current.entities.find(
        (e) => e.id === definition.languageId,
      ).language;
    expect(state.revisions).toHaveLength(2);
    expect(state.compiled[0].revision).not.toBe(state.draftRevision);
    await page.setViewportSize({ width: 1440, height: 960 });
    await mkdir(".cache/languages/browser", { recursive: true });
    await page.screenshot({
      path: ".cache/languages/browser/revision-laboratory.png",
      fullPage: true,
    });
    expect(errors).toEqual([]);
  });
});
