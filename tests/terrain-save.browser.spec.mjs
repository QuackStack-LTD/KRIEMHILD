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
  expect(errors).toEqual([]);
});
