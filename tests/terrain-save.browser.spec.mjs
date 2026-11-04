import { test, expect } from "@playwright/test";

// This test saves and closes a session. Supply an expendable database/world.
const project = process.env.KRIEMHILD_SAVE_TEST_PROJECT;
const world = process.env.KRIEMHILD_SAVE_TEST_WORLD;

test("State and Settlement drafts publish only on named Apply; saved settlement art fills Atlas and responsive banners", async ({
  page,
  request,
}) => {
  test.skip(!project || !world, "Requires an expendable world.");
  test.setTimeout(180000);
  const suffix = Date.now().toString(36),
    stateName = `Regression March ${suffix}`,
    settlementName = `Regression Harbor ${suffix}`,
    houseLabel = `The Candle House ${suffix}`;
  const state = await (
    await request.post(`/api/projects/${project}/open`, { data: {} })
  ).json();
  const baseline = await (await request.get(`/api/worlds/${world}`)).json();
  let site;
  for (const [lon, lat] of [
    [32, -15],
    [12, -4],
    [0, 0],
    [-60, 30],
    [120, -30],
  ]) {
    const sample = await (
      await request.get(
        `/api/sessions/${state.id}/terrain/sample?lon=${lon}&lat=${lat}`,
      )
    ).json();
    if (sample.water === "dry") {
      site = [(lon + 180) / 180, (90 - lat) / 180];
      break;
    }
  }
  test.skip(!site, "No dry test site found in this fixture.");
  await page.goto(
    `/world/${project}/build?world=${world}&age=${baseline.world.currentAge}&tileDebug=1`,
  );
  await page.getByLabel("Autosave accepted terrain after 30 seconds").uncheck();
  const svg = page.getByRole("img", {
    name: "Interactive continuous world map",
    exact: true,
  });
  await page.evaluate(
    (p) =>
      window.dispatchEvent(
        new CustomEvent("terrain-diagnostic-camera", {
          detail: {
            x: p[0] - 0.00025,
            y: p[1] - 0.000125,
            w: 0.0005,
            h: 0.00025,
          },
        }),
      ),
    site,
  );
  const click = async (p) => {
    const q = await svg.evaluate((s, p) => {
      const r = s.getBoundingClientRect(),
        c = JSON.parse(s.dataset.camera),
        scale = Math.min(r.width / c.w, r.height / c.h);
      return {
        x: (p[0] - c.x) * scale + (r.width - c.w * scale) / 2,
        y: (p[1] - c.y) * scale + (r.height - c.h * scale) / 2,
      };
    }, p);
    await svg.click({ position: q });
  };
  const draw = async (tool, offset) => {
    await page.getByRole("button", { name: tool, exact: true }).click();
    for (const [x, y] of [
      [-1, -1],
      [1, -1],
      [1, 1],
      [-1, 1],
    ])
      await click([site[0] + x * offset, site[1] + y * offset]);
    await page.getByRole("button", { name: new RegExp("^Finish ") }).click();
  };
  const inspector = page.locator(".studio-inspector");
  await page.getByRole("button", { name: "Map Builder", exact: true }).click();
  await draw("State", 0.000065);
  const placeholder = await inspector
    .getByLabel("Name", { exact: true })
    .inputValue();
  let doc = await (await request.get(`/api/worlds/${world}`)).json();
  expect(doc.entities.some((e) => e.name === placeholder && !e.deleted)).toBe(
    false,
  );
  await inspector.getByLabel("Selected place type").selectOption("empire");
  await inspector.getByLabel("Name", { exact: true }).fill(stateName);
  await inspector
    .getByRole("button", { name: "Apply changes", exact: true })
    .click();
  await expect(
    inspector.getByRole("button", {
      name: `Open ${stateName} record`,
      exact: true,
    }),
  ).toBeVisible();
  doc = await (await request.get(`/api/worlds/${world}`)).json();
  expect(
    doc.entities.some(
      (e) => e.name === stateName && e.kind === "empire" && !e.deleted,
    ),
  ).toBe(true);
  await page
    .getByRole("button", { name: "Settlement Builder", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Settlement", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await draw("Settlement", 0.000025);
  await inspector.getByLabel("Selected place type").selectOption("hamlet");
  await inspector.getByLabel("Name", { exact: true }).fill(settlementName);
  doc = await (await request.get(`/api/worlds/${world}`)).json();
  expect(
    doc.entities.some((e) => e.name === settlementName && !e.deleted),
  ).toBe(false);
  await inspector
    .getByRole("button", { name: "Apply changes", exact: true })
    .click();
  await expect(
    inspector.getByRole("button", {
      name: `Open ${settlementName} record`,
      exact: true,
    }),
  ).toBeVisible();
  doc = await (await request.get(`/api/worlds/${world}`)).json();
  const settlement = doc.entities.find(
    (e) => e.name === settlementName && !e.deleted,
  );
  expect(settlement.kind).toBe("hamlet");
  const shapes = await (
    await request.get(`/api/sessions/${state.id}/world/entities?all=1&detail=8`)
  ).json();
  expect(
    shapes.entities.find((e) => e.id === settlement.id).points,
  ).toHaveLength(4);
  await page.getByRole("button", { name: "house", exact: true }).click();
  await click(site);
  await expect(inspector.getByLabel("Name", { exact: true })).toHaveValue(
    /^house /,
  );
  const houseName = await inspector
    .getByLabel("Name", { exact: true })
    .inputValue();
  doc = await (await request.get(`/api/worlds/${world}`)).json();
  expect(doc.entities.some((e) => e.name === houseName && !e.deleted)).toBe(
    false,
  );
  await inspector.getByLabel("Name", { exact: true }).fill(houseLabel);
  await inspector
    .getByRole("button", { name: "Apply changes", exact: true })
    .click();
  await expect(
    inspector.getByRole("button", {
      name: `Open ${houseLabel} record`,
      exact: true,
    }),
  ).toBeVisible();
  await page.getByText("Layers & objects", { exact: true }).click();
  await page.getByLabel("Find an object", { exact: true }).fill(settlementName);
  await expect(inspector.getByLabel("Name", { exact: true })).toHaveValue(
    settlementName,
  );
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  await page.getByLabel("Illustrated map name").fill("Harbor map");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  const response = page.waitForResponse(
    (r) =>
      r.url().endsWith("/illustrated-maps") && r.request().method() === "POST",
  );
  await page
    .getByRole("button", { name: "Save illustration", exact: true })
    .click();
  const saved = await response;
  expect(saved.status(), await saved.text()).toBe(201);
  await page.goto(`/worlds/${world}/atlas?age=${baseline.world.currentAge}`);
  await page.getByLabel("Atlas collection").selectOption("villages");
  const tile = page
    .locator(".entity-tile")
    .filter({
      has: page.getByRole("heading", {
        name: settlementName,
        exact: true,
      }),
    })
    .last();
  await expect(
    tile.getByRole("img", { name: `${settlementName} illustrated map` }),
  ).toBeVisible();
  await expect
    .poll(() => tile.locator("img").evaluate((e) => e.naturalWidth))
    .toBeGreaterThan(0);
  await tile.locator(".entity-tile-open").click();
  const banner = page.locator(".settlement-banner"),
    image = banner.getByRole("img", {
      name: `${settlementName} illustrated map`,
    });
  await expect(image).toBeVisible();
  await expect
    .poll(() => image.evaluate((e) => e.naturalWidth))
    .toBeGreaterThan(0);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const b = await banner.boundingBox(),
      main = await page.locator(".entity-profile").boundingBox();
    expect(Math.abs(b.width - main.width)).toBeLessThan(2);
    expect(Math.abs(b.x - main.x)).toBeLessThan(2);
    expect(await banner.evaluate((e) => getComputedStyle(e).borderRadius)).toBe(
      "0px",
    );
  }
});
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
    expect(
      response.status(),
      response.status() === 200 ? "" : await response.text(),
    ).toBe(200);
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
  await page
    .getByRole("dialog", { name: "Name illustrated map" })
    .getByLabel("Illustrated map name")
    .fill("Regression atlas");
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/terrain/land-selection")),
    page.getByRole("button", { name: "Continue", exact: true }).click(),
  ]);
  await expect(
    page.getByText("Finding connected land…", { exact: true }),
  ).toHaveCount(0);
  const selectionMap = await page
    .getByRole("img", { name: "Interactive continuous world map", exact: true })
    .boundingBox();
  await page.mouse.move(selectionMap.x + 1, selectionMap.y + 1);
  await page.mouse.down();
  await page.mouse.move(
    selectionMap.x + selectionMap.width - 1,
    selectionMap.y + selectionMap.height - 1,
    { steps: 3 },
  );
  await page.mouse.up();
  await page
    .getByRole("button", { name: "Create illustrated map", exact: true })
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
  const bannerPNG = await request.get(
    `/api/worlds/${world}/library/${focused.fileId}?age=${doc.world.currentAge}&inline=1&variant=banner`,
  );
  const square = await png.body(),
    wide = await bannerPNG.body();
  expect(square.readUInt32BE(16)).toBe(square.readUInt32BE(20));
  expect(wide.readUInt32BE(16) / wide.readUInt32BE(20)).toBeCloseTo(3, 2);
  expect(png.headers()["content-type"]).toBe("image/png");
  expect((await png.body()).subarray(0, 8).toString("hex")).toBe(
    "89504e470d0a1a0a",
  );
  await saveIllustration();
  const timings = await page.evaluate(() => window.__illustrationSaveLog);
  expect(timings.at(-1).cached).toBe(true);
  console.log("Illustration save timings", JSON.stringify(timings));
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
  await page.getByRole("button", { name: "Continue", exact: true }).click();
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
  await page.getByRole("button", { name: "Continue", exact: true }).click();
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
