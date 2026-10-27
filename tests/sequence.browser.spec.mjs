import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";

test.describe("Sequence spelling persisted workflow", () => {
  test.skip(
    process.env.KRIEMHILD_LANGUAGES_TEST !== "1",
    "Requires explicit opt-in and an isolated test server.",
  );
  let world, age, headers, extra;
  const work = (p) =>
    p.getByRole("region", { name: "Sequence orthography workbench" });
  const result = (p) =>
    work(p).getByRole("region", { name: "Sequence operation result" });
  test.beforeEach(async ({ request }) => {
    const me = await (await request.get("/api/me")).json();
    headers = { "X-CSRF-Token": me.csrfToken };
    const r = await request.post("/api/worlds", {
      headers,
      data: { name: "Disposable Brief 4 browser World" },
    });
    expect(r.ok()).toBeTruthy();
    const d = await r.json();
    world = d.world.id;
    age = d.world.currentAge;
    extra = [];
  });
  test.afterEach(async ({ request }) => {
    for (const id of [...extra, world])
      expect(
        (await request.delete(`/api/worlds/${id}`, { headers })).ok(),
      ).toBeTruthy();
  });
  async function seed(request) {
    let doc = await (await request.get(`/api/worlds/${world}`)).json();
    let language;
    for (const [kind, name] of [
      ["script", "sequence-script.json"],
      ["language", "sequence.json"],
    ]) {
      const definition = JSON.parse(
        await readFile(`internal/languages/testdata/${name}`, "utf8"),
      );
      if (kind === "language") language = definition;
      const r = await request.post(`/api/worlds/${world}/languages`, {
        headers,
        data: {
          ageId: age,
          revision: doc.world.revision,
          kind,
          definition,
          metadata: {},
        },
      });
      expect(r.ok(), await r.text()).toBeTruthy();
      doc = await r.json();
    }
    return language;
  }
  async function open(page, language) {
    await page.goto(
      `/worlds/${world}/languages?language=${language.languageId}`,
    );
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
  }
  async function compile(page, save = false) {
    if (save) {
      await page
        .getByRole("button", { name: "Save draft", exact: true })
        .click();
      await expect(
        page.getByText("Draft saved. Compilation and activation are separate."),
      ).toBeVisible();
    }
    await page
      .getByRole("button", { name: "Compile saved revision", exact: true })
      .click();
    await expect(
      page.getByText("Compile report saved for this revision."),
    ).toBeVisible();
  }
  async function raw(page, value = "p a d a") {
    await work(page)
      .getByRole("combobox", { name: "Spelling input mode", exact: true })
      .selectOption("units");
    await work(page)
      .getByRole("textbox", {
        name: "Source segment IDs and boundaries",
        exact: true,
      })
      .fill(value);
  }
  async function run(page) {
    const waiting = page.waitForResponse(
      (r) => r.url().endsWith("/run") && r.request().method() === "POST",
    );
    await work(page)
      .getByRole("button", { name: "Run sequence operation", exact: true })
      .click();
    const data = await (await waiting).json();
    await expect(
      result(page).getByRole("heading", {
        name: data.result.status,
        exact: true,
      }),
    ).toBeVisible();
    return data.result;
  }
  test("spell, inspect clusters, inverse ambiguity, publish normalization and replay old revision and ZIP", async ({
    page,
    request,
  }) => {
    const language = await seed(request);
    await open(page, language);
    await compile(page);
    await raw(page);
    const first = await run(page);
    expect(first.spelling.spellings[0].text).toBe("pe\u0301de\u0301");
    expect(first.spelling.spellings[0].render.signs).toHaveLength(4);
    await expect(
      result(page).getByRole("img", {
        name: "Spelling rendered by pinned Script",
        exact: true,
      }),
    ).toBeVisible();
    await result(page)
      .getByRole("button", { name: "Select grapheme cluster 2", exact: true })
      .click();
    const selection = await result(page)
      .getByRole("textbox", { name: "Exact written output", exact: true })
      .evaluate((el) => [el.selectionStart, el.selectionEnd]);
    expect(selection).toEqual([1, 3]);
    await result(page)
      .getByText("Stages, rule spans and source provenance", { exact: true })
      .click();
    await expect(result(page).getByText(/"map-a"/)).toBeVisible();
    await result(page)
      .getByRole("button", { name: "Use output for inverse", exact: true })
      .click();
    await work(page)
      .getByRole("checkbox", {
        name: "Run A2 after inverse spelling",
        exact: true,
      })
      .check();
    const inverse = await run(page);
    expect(inverse.status).toBe("ambiguous");
    expect(inverse.knownValid).toBe(2);
    expect(inverse.spelling.preimages[0].analyses.map((c) => c.lexeme)).toEqual(
      ["rock", "stone"],
    );
    const response = page.waitForResponse((r) => r.url().endsWith("/run"));
    await result(page)
      .getByRole("button", { name: /Choose preimage/ })
      .click();
    expect(
      (await (await response).json()).result.spelling.preimages,
    ).toHaveLength(1);
    await work(page)
      .getByRole("combobox", { name: "Sequence operation", exact: true })
      .selectOption("spell");
    // A real authored normalization stage runs between Unicode spelling and signs.
    await work(page)
      .getByRole("button", { name: "Add spelling stage", exact: true })
      .click();
    await work(page)
      .getByRole("combobox", { name: "Stage kind", exact: true })
      .selectOption("normalize");
    await work(page)
      .getByRole("button", { name: "Move stage earlier", exact: true })
      .click();
    await work(page)
      .getByRole("combobox", { name: "Selected spelling stage", exact: true })
      .selectOption("2");
    const input = work(page)
      .getByRole("group", { name: "Spelling rule 1", exact: true })
      .getByRole("group", { name: "Input pattern", exact: true });
    await input
      .getByRole("textbox", { name: "Exact unit value", exact: true })
      .fill("\u00e9");
    await compile(page, true);
    await raw(page);
    const next = await run(page);
    expect(next.revision).not.toBe(first.revision);
    expect(next.spelling.spellings[0].text).toBe("p\u00e9d\u00e9");
    expect(next.spelling.spellings[0].stages[1].original).toBe(
      "pe\u0301de\u0301",
    );
    await work(page)
      .getByRole("combobox", { name: "Sequence operation", exact: true })
      .selectOption("unspell");
    await expect(
      work(page).getByRole("button", {
        name: "Run sequence operation",
        exact: true,
      }),
    ).toBeDisabled();
    await page
      .getByRole("combobox", { name: "Definition revision", exact: true })
      .selectOption(first.revision);
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    await raw(page);
    expect(await run(page)).toEqual(first);
    await page.reload();
    await page
      .getByRole("combobox", { name: "Definition revision", exact: true })
      .selectOption(first.revision);
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    await raw(page);
    expect(await run(page)).toEqual(first);
    const zip = await request.get(`/api/worlds/${world}/export`);
    expect(zip.ok()).toBeTruthy();
    const imported = await request.post("/api/worlds/import", {
      headers: { ...headers, "Content-Type": "application/zip" },
      data: await zip.body(),
    });
    expect(imported.ok(), await imported.text()).toBeTruthy();
    const doc = await imported.json();
    extra.push(doc.world.id);
    const replay = await request.post(
      `/api/worlds/${doc.world.id}/languages/${language.languageId}/run`,
      {
        headers,
        data: {
          ageId: age,
          definitionRevision: first.revision,
          request: first.input,
        },
      },
    );
    expect(replay.ok()).toBeTruthy();
    expect((await replay.json()).result).toEqual(first);
    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
    ).toBeTruthy();
    await work(page)
      .getByRole("button", { name: "Run sequence operation", exact: true })
      .focus();
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => document.activeElement.tagName)).not.toBe(
      "BODY",
    );
  });
  test("a changed example supersedes a late spelling preview", async ({
    page,
    request,
  }) => {
    const language = await seed(request);
    await open(page, language);
    await compile(page);
    await raw(page, "p");
    let release, captured;
    const hold = new Promise((r) => (release = r)),
      received = new Promise((r) => (captured = r));
    await page.route(
      "**/run",
      async (route) => {
        const response = await route.fetch();
        captured();
        await hold;
        await route.fulfill({ response });
      },
      { times: 1 },
    );
    await work(page)
      .getByRole("button", { name: "Run sequence operation", exact: true })
      .click();
    await received;
    await work(page)
      .getByRole("textbox", {
        name: "Source segment IDs and boundaries",
        exact: true,
      })
      .fill("a");
    release();
    const r = await run(page);
    expect(r.spelling.spellings[0].text).toBe("e\u0301");
  });
  test("rule validation is executable and large rule lists paginate", async ({
    page,
    request,
  }) => {
    const language = await seed(request);
    let doc = await (await request.get(`/api/worlds/${world}`)).json();
    for (let i = 0; i < 24; i++)
      language.sequenceWriting.stages[0].rules.push({
        id: `extra-${i}`,
        priority: 100 + i,
        input: [
          { kind: "segment", value: "p" },
          { kind: "segment", value: "p" },
        ],
        left: [],
        right: [],
        output: [{ kind: "grapheme", value: "p" }],
      });
    const saved = await request.post(`/api/worlds/${world}/languages`, {
      headers,
      data: {
        kind: "language",
        ageId: age,
        revision: doc.world.revision,
        definition: language,
        metadata: {},
      },
    });
    expect(saved.ok()).toBeTruthy();
    await open(page, language);
    await expect(
      work(page).getByText("Rules page 1 of 2", { exact: true }),
    ).toBeVisible();
    await expect(
      work(page).getByRole("group", { name: /^Spelling rule \d+$/ }),
    ).toHaveCount(20);
    await work(page)
      .getByRole("button", { name: "Next spelling rules", exact: true })
      .click();
    await expect(
      work(page).getByRole("group", { name: /^Spelling rule \d+$/ }),
    ).toHaveCount(10);
    const rule = work(page).getByRole("group", {
      name: "Spelling rule 21",
      exact: true,
    });
    await rule
      .getByRole("spinbutton", { name: "Priority", exact: true })
      .fill("100");
    await compile(page, true);
    await raw(page, "p p");
    await expect(
      work(page).getByRole("button", {
        name: "Run sequence operation",
        exact: true,
      }),
    ).toBeDisabled();
    await expect(
      page.getByText("rule_conflict", { exact: true }),
    ).toBeVisible();
  });
});
