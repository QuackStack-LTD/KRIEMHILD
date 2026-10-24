import { test, expect } from "@playwright/test";

// This test saves and closes a session. Supply an expendable database/world.
const project = process.env.KRIEMHILD_SAVE_TEST_PROJECT;
const world = process.env.KRIEMHILD_SAVE_TEST_WORLD;
test("Save publishes records and artwork, and Illustrator names paint at close zoom", async ({
  page,
  request,
}) => {
  test.skip(
    !project || !world,
    "Set KRIEMHILD_SAVE_TEST_PROJECT and KRIEMHILD_SAVE_TEST_WORLD for an expendable saved terrain.",
  );
  test.setTimeout(180000);
  const json = async (response) => {
    expect(response.ok(), await response.text()).toBeTruthy();
    return response.json();
  };
  const doc = await json(await request.get(`/api/worlds/${world}`));
  const state = await json(
    await request.post(`/api/projects/${project}/open`, { data: {} }),
  );
  const before = await json(
    await request.get(
      `/api/sessions/${state.id}/world/entities?detail=8&all=1`,
    ),
  );
  const country = before.entities.find((e) => e.kind === "country" && e.name);
  test.skip(!country, "The saved terrain must contain a named country.");
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(
    `/world/${project}/build?world=${world}&age=${doc.world.currentAge}&tileDebug=1`,
  );
  await page.getByLabel("Autosave accepted terrain after 30 seconds").uncheck();
  const save = async () => {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) => r.url().endsWith("/save") && r.request().method() === "POST",
      ),
      page.getByRole("button", { name: "Save", exact: true }).click(),
    ]);
    expect(response.status(), await response.text()).toBe(200);
  };
  await save();
  const accepted = await json(await request.get(`/api/worlds/${world}`));
  expect(accepted.maps.some((m) => m.projectId === project)).toBeTruthy();
  expect(
    accepted.entities.some(
      (e) => e.id === (country.logicalId || country.id) && !e.deleted,
    ),
  ).toBeTruthy();
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  await page.getByLabel("Map scope", { exact: true }).selectOption("world");
  const label = page.locator(
    `[data-illustration-id="label-${country.id}"] text`,
  );
  await expect(label).toHaveText(country.name);
  await expect(label).toBeInViewport();
  await page.getByLabel("Focus geographical feature").selectOption(country.id);
  await expect(label).toBeInViewport();
  await page.waitForFunction(() => {
    const q = window.__terrainViewportLog?.findLast(
        (e) => e.event === "request",
      ),
      g = document.querySelector('[aria-label="Committed terrain viewport"]');
    return (
      q &&
      g?.dataset.phase === "final" &&
      +g.dataset.generation === q.generation &&
      q.styles.includes("relief-shadow")
    );
  });
  // DOM visibility alone misses SVG blend surfaces covering later text.
  const box = await label.boundingBox();
  const clip = {
    x: Math.floor(box.x) - 2,
    y: Math.floor(box.y) - 2,
    width: Math.ceil(box.width) + 4,
    height: Math.ceil(box.height) + 4,
  };
  const painted = await page.screenshot({ clip });
  await label.evaluate((e) => (e.style.visibility = "hidden"));
  const hidden = await page.screenshot({ clip });
  await label.evaluate((e) => (e.style.visibility = ""));
  expect(
    painted.equals(hidden),
    "The country name must change the painted pixels",
  ).toBe(false);
  const saveIllustration = async () => {
    const [response] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().endsWith("/illustrated-maps") &&
          r.request().method() === "POST",
        { timeout: 120000 },
      ),
      page
        .getByRole("button", { name: "Save illustration", exact: true })
        .click(),
    ]);
    expect(response.status(), await response.text()).toBe(201);
    return response.json();
  };
  const focused = await saveIllustration();
  const record = focused.document.entities.find(
    (e) =>
      e.id === (country.logicalId || country.id) &&
      e.ageId === doc.world.currentAge,
  );
  expect(record.properties.illustratedMap.fileId).toBe(focused.fileId);
  const png = await request.get(
    `/api/worlds/${world}/library/${focused.fileId}?age=${doc.world.currentAge}&inline=1`,
  );
  expect(png.ok()).toBeTruthy();
  expect(png.headers()["content-type"]).toBe("image/png");
  expect((await png.body()).subarray(0, 8).toString("hex")).toBe(
    "89504e470d0a1a0a",
  );
  await page
    .getByRole("button", { name: "Construction mode", exact: true })
    .click();
  const quality = page.getByLabel("Preview Quality", { exact: true });
  const originalQuality = await quality.inputValue();
  await quality.selectOption(
    originalQuality === "performance" ? "high" : "performance",
  );
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  await page.getByLabel("Focus geographical feature").selectOption(country.id);
  await expect(label).toHaveText(country.name);
  const anotherQuality = await saveIllustration();
  expect(anotherQuality.fileId).toBe(focused.fileId);
  expect(
    anotherQuality.document.libraryFiles
      .find((f) => f.id === focused.fileId && f.ageId === doc.world.currentAge)
      .versions.at(-1).hash,
  ).toBe(record.properties.illustratedMap.version);
  await page
    .getByRole("button", { name: "Construction mode", exact: true })
    .click();
  await quality.selectOption(originalQuality);
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  await page.getByLabel("Map scope", { exact: true }).selectOption("world");
  const global = await saveIllustration();
  expect(
    global.document.entities.find(
      (e) => e.id === global.archiveId && e.ageId === doc.world.currentAge,
    ).name,
  ).toBe("Illustrated maps");
  await expect(
    page.getByRole("link", { name: "Open in Library" }),
  ).toBeVisible();
  await save();
  const saved = await json(
    await request.get(`/api/sessions/${state.id}/world/state`),
  );
  await page.goto("/");
  expect((await request.delete(`/api/sessions/${state.id}`)).ok()).toBeTruthy();
  const cold = await json(
    await request.post(`/api/projects/${project}/open`, { data: {} }),
  );
  const after = await json(
    await request.get(`/api/sessions/${cold.id}/world/entities?detail=8&all=1`),
  );
  expect(after.entities.map((e) => [e.id, e.points])).toEqual(
    before.entities.map((e) => [e.id, e.points]),
  );
  expect(cold.project.illustrations).toEqual(saved.project.illustrations);
  expect(cold.project.view).toEqual(saved.project.view);
  expect(cold.projectUI.continuousView).toEqual(saved.project.view);
  await page.goto(
    `/worlds/${world}/profile?entity=${country.logicalId || country.id}&age=${doc.world.currentAge}`,
  );
  const banner = page.getByRole("img", {
    name: `${country.name} illustrated map`,
    exact: true,
  });
  await expect(banner).toBeVisible();
  await expect
    .poll(() => banner.evaluate((img) => img.naturalWidth))
    .toBeGreaterThan(0);
  await page
    .getByRole("link", {
      name: `Open ${country.name} illustrated map`,
      exact: true,
    })
    .click();
  await expect(
    page.getByRole("button", {
      name: `Open ${country.name} record`,
      exact: true,
    }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test("deep zoom keeps geographic features, terrain crops and inspection at the same point while details load", async ({
  page,
  request,
}) => {
  test.skip(!project || !world, "Requires an expendable saved terrain.");
  test.setTimeout(180000);
  const state = await (
    await request.post(`/api/projects/${project}/open`, { data: {} })
  ).json();
  const doc = await (await request.get(`/api/worlds/${world}`)).json();
  const features = await (
    await request.get(`/api/sessions/${state.id}/world/entities?detail=8&all=1`)
  ).json();
  const country = features.entities.find(
    (e) => e.kind === "country" && !e.deleted,
  );
  test.skip(!country, "Requires a country boundary.");
  const p = [
    (country.points[0][0] / (state.W - 1)) * 2,
    country.points[0][1] / (state.H - 1),
  ];
  const original = state.projectUI?.continuousView;
  try {
    for (const w of [0.1, 0.01, 0.001, 0.00008]) {
      await page.goto("/");
      const camera = { x: p[0] - 0.61 * w, y: p[1] - 0.19 * w, w, h: w / 2 };
      const staging = await request.post(`/api/sessions/${state.id}/view`, {
        data: {
          ui: {
            continuousView: {
              camera,
              style: "terrain",
              mode: "map",
              showRivers: true,
            },
          },
          merge: true,
        },
      });
      expect(staging.ok()).toBeTruthy();
      await page.goto(
        `/world/${project}/build?world=${world}&age=${doc.world.currentAge}&tileDebug=1`,
      );
      const svg = page.locator(
        'svg[aria-label="Interactive continuous world map"]',
      );
      await expect(svg).toHaveAttribute("data-camera", JSON.stringify(camera));
      await page
        .getByLabel("Autosave accepted terrain after 30 seconds")
        .uncheck();
      await expect(
        svg.locator(`[data-map-feature="${country.id}"] polygon`),
      ).toHaveCount(1);
      const result = await svg.evaluate(
        (svg, { p, id }) => {
          const c = JSON.parse(svg.dataset.camera),
            r = svg.getBoundingClientRect();
          const scale = Math.min(r.width / c.w, r.height / c.h);
          svg.addEventListener(
            "pointerup",
            (event) => {
              window.__regressionClick = {
                longitude:
                  (c.x +
                    (event.clientX - r.left - (r.width - c.w * scale) / 2) /
                      scale) *
                    180 -
                  180,
                latitude:
                  90 -
                  (c.y +
                    (event.clientY - r.top - (r.height - c.h * scale) / 2) /
                      scale) *
                    180,
              };
            },
            { once: true },
          );
          const x = (p[0] - c.x) * scale + (r.width - c.w * scale) / 2,
            y = (p[1] - c.y) * scale + (r.height - c.h * scale) / 2;
          const matrix = svg.getScreenCTM(),
            screen = new DOMPoint(x, y).matrixTransform(matrix);
          const polygon = svg.querySelector(
            `[data-map-feature="${id}"] polygon`,
          );
          let vertexError = Infinity;
          for (let i = 0; i < polygon.points.numberOfItems; i++) {
            const q = polygon.points.getItem(i);
            vertexError = Math.min(vertexError, Math.hypot(q.x - x, q.y - y));
          }
          return {
            x,
            y,
            vertexError,
            transformError: Math.hypot(
              screen.x - r.left - x,
              screen.y - r.top - y,
            ),
          };
        },
        { p, id: country.id },
      );
      expect(result.vertexError).toBeLessThan(0.001);
      expect(result.transformError).toBeLessThan(0.001);
      const query = page.waitForResponse((r) =>
        r.url().includes("/terrain/sample?"),
      );
      await svg.click({ position: { x: result.x, y: result.y } });
      const response = await query;
      const url = new URL(response.url());
      const click = await page.evaluate(() => window.__regressionClick);
      expect(
        Math.abs(+url.searchParams.get("lon") - click.longitude),
      ).toBeLessThan(1e-8);
      expect(
        Math.abs(+url.searchParams.get("lat") - click.latitude),
      ).toBeLessThan(1e-8);
      await page.waitForFunction(() => {
        const q = window.__terrainViewportLog?.findLast(
            (e) => e.event === "request",
          ),
          f = document.querySelector(
            '[aria-label="Committed terrain viewport"]',
          );
        return (
          q &&
          f?.dataset.phase === "final" &&
          +f.dataset.generation === q.generation
        );
      });
      const geometry = await svg.evaluate((svg) => {
        const b = svg.getBoundingClientRect();
        return [...svg.querySelectorAll("svg[data-world-bounds]")].map((e) => {
          const r = e.getBoundingClientRect();
          return {
            width: r.width,
            height: r.height,
            maxWidth: b.width,
            maxHeight: b.height,
          };
        });
      });
      expect(geometry.length).toBeGreaterThan(0);
      expect(
        geometry.every(
          (b) => b.width <= b.maxWidth + 0.01 && b.height <= b.maxHeight + 0.01,
        ),
      ).toBeTruthy();
      await expect(page.getByLabel("Terrain detail loading")).toHaveCount(0);
    }
    // Hold replacement work so loading feedback and the retained coherent
    // frame are observable even on a machine with a warm terrain cache.
    let release;
    const held = new Promise((r) => (release = r));
    await page.route("**/terrain/tiles/**", async (route) => {
      await held;
      await route.continue();
    });
    const svg = page.locator(
      'svg[aria-label="Interactive continuous world map"]',
    );
    const before = await svg.getAttribute("data-camera");
    const box = await svg.boundingBox();
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.wheel(0, 180);
    await expect(page.getByLabel("Terrain detail loading")).toBeVisible();
    await expect(svg.locator("image")).not.toHaveCount(0);
    expect(await svg.getAttribute("data-camera")).not.toBe(before);
    release();
    await page.unroute("**/terrain/tiles/**");
  } finally {
    await page.goto("/");
    if (original)
      await request.post(`/api/sessions/${state.id}/view`, {
        data: { ui: { continuousView: original }, merge: true },
      });
  }
});
