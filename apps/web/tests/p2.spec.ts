import { test, expect, Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import type { State, HistoricalState } from "../lib/types";

async function nav(page: Page, name: string) {
  await page
    .getByRole("navigation", { name: "Workspaces" })
    .getByRole("button", { name, exact: true })
    .click();
}
async function state(page: Page): Promise<State> {
  return page.evaluate(async () => {
    const p = new URLSearchParams(location.search);
    return (
      await fetch(
        `/api/v1/projects/${p.get("world")}/state?age=${p.get("age")}`,
      )
    ).json();
  });
}
async function preview(page: Page) {
  await page
    .getByRole("button", { name: "Preview terrain", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Review terrain proposal" }),
  ).toBeVisible();
}
async function accept(page: Page) {
  await page
    .getByRole("button", { name: "Accept terrain", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Review terrain proposal" }),
  ).toHaveCount(0);
}

test("P2: flood an alternate Age, review impacts, preserve history and date a scene", async ({
  page,
}) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await page
    .getByLabel("World name", { exact: true })
    .fill(`Temporal coast ${randomUUID().slice(0, 8)}`);
  await page.getByLabel("First Age", { exact: true }).fill("Opening Age");
  await page
    .getByRole("button", { name: "Create world →", exact: true })
    .click();
  await nav(page, "Entities");
  await page.getByRole("button", { name: "+ New entity", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Harbor");
  await page.getByLabel("Entity type", { exact: true }).fill("Settlement");
  await page
    .getByRole("button", { name: "Create entity", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Save entity", exact: true }),
  ).toBeVisible();
  await nav(page, "Atlas");
  await page.getByLabel("Map name", { exact: true }).fill("Test coast");
  await page.getByRole("button", { name: "+ Create map", exact: true }).click();
  await expect(page.getByLabel("Entity to place")).toBeVisible();
  const initial = await state(page),
    city = Object.values(initial.records).find((r) => r.kind === "entity")!;
  await page.getByLabel("Entity to place").selectOption(city.id);
  await page.getByRole("button", { name: "Place / move pin" }).click();
  await expect(page.getByRole("cell", { name: "50.0%, 50.0%" })).toBeVisible();
  await preview(page);
  await accept(page);
  await page.getByLabel("Terrain operation").selectOption("sea");
  await page.getByLabel("Target elevation").fill("-1000");
  await preview(page);
  await accept(page);
  await page
    .getByText("Draw rivers, routes, lakes, borders, climate & biome areas", {
      exact: true,
    })
    .click();
  await page.getByLabel("Feature name", { exact: true }).fill("Coastal claim");
  await page.getByLabel("Feature kind").selectOption("border");
  await page.getByLabel("Feature owner (optional)").selectOption(city.id);
  await page.getByRole("button", { name: "Add geographic feature" }).click();
  await expect(
    page.locator(".feature-row").getByText("Coastal claim", { exact: true }),
  ).toBeVisible();
  await nav(page, "Timeline");
  await page.getByLabel("Age start tick", { exact: true }).fill("0");
  await page.getByLabel("Age end tick (exclusive)").fill("100");
  await page.getByRole("button", { name: "Pin opening baseline" }).click();
  await expect(
    page.getByRole("button", { name: "Save chronology" }),
  ).toBeVisible();
  const a = await state(page);
  const map = Object.values(a.records).find((r) => r.kind === "map")!;
  await page.getByRole("button", { name: "+ Begin a new Age" }).click();
  await page.getByLabel("New Age name").fill("Flood Age");
  await page.getByRole("button", { name: "Create independent copy" }).click();
  await expect(page.getByLabel("Current Age")).not.toHaveValue(a.age.id);
  await expect(page.getByLabel("Age start tick", { exact: true })).toHaveValue(
    "",
  );
  await page.getByLabel("Age start tick", { exact: true }).fill("100");
  await page.getByLabel("Age end tick (exclusive)").fill("200");
  await page.getByRole("button", { name: "Save chronology" }).click();
  await expect(
    page.getByRole("button", { name: "Save chronology" }),
  ).toBeEnabled();
  await nav(page, "Atlas");
  await page.getByLabel("Terrain operation").selectOption("flood");
  await preview(page);
  await expect(
    page.getByText("Harbor: newly submerged", { exact: true }),
  ).toBeVisible();
  const before = await state(page);
  expect(before.records[map.id].terrain).toEqual(map.terrain);
  await page.getByLabel("Event title (optional)").fill("The Great Flood");
  await page.getByLabel("Event tick", { exact: true }).fill("120");
  await page
    .getByLabel("Change explanation")
    .fill("The estuary breaks its banks.");
  await page
    .getByRole("button", { name: "Accept terrain & record event", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Review terrain proposal" }),
  ).toHaveCount(0);
  let b = await state(page);
  expect(b.records[map.id].terrain).not.toEqual(map.terrain);
  expect(b.records[city.id]).toEqual(a.records[city.id]);
  await nav(page, "Timeline");
  await expect(
    page.getByRole("heading", { name: "The Great Flood", exact: true }),
  ).toBeVisible();
  await page.getByLabel("Inspect tick", { exact: true }).fill("119");
  await page.getByRole("button", { name: "Inspect date" }).click();
  await expect(
    page.getByRole("img", { name: "Test coast at tick 119" }),
  ).toBeVisible();
  const inspect = async (tick: string): Promise<HistoricalState> =>
    page.evaluate(
      async ({ world, snapshot, tick }) =>
        (
          await fetch(
            `/api/v1/projects/${world}/historical?snapshot=${snapshot}&tick=${tick}`,
          )
        ).json(),
      { world: b.root.world.id, snapshot: b.age.snapshot, tick },
    );
  expect((await inspect("119")).records[map.id].terrain).toEqual(map.terrain);
  expect((await inspect("120")).records[map.id].terrain).toEqual(
    b.records[map.id].terrain,
  );
  await nav(page, "History");
  await expect(
    page.getByRole("img", { name: "Earlier map comparison" }),
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: "Later map comparison" }),
  ).toBeVisible();
  await nav(page, "Writing");
  await page.getByLabel("New scene title").fill("Before the water");
  await page.getByRole("button", { name: "+ Create scene" }).click();
  await page.getByLabel("Scene setting tick (empty uses overview)").fill("119");
  await page.getByRole("button", { name: "Apply scene date" }).click();
  await expect(
    page.getByRole("img", { name: "Test coast scene setting at tick 119" }),
  ).toBeVisible();
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  await page.getByLabel("Scene setting tick (empty uses overview)").fill("120");
  await page.getByRole("button", { name: "Apply scene date" }).click();
  await expect(
    page.getByRole("img", { name: "Test coast scene setting at tick 120" }),
  ).toBeVisible();
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  b = await state(page);
  await page.reload();
  await expect(
    page.getByRole("img", { name: "Test coast scene setting at tick 120" }),
  ).toBeVisible();
  await page.getByLabel("Current Age").selectOption(a.age.id);
  await expect(page).toHaveURL((u) => u.searchParams.get("age") === a.age.id);
  const original = await state(page);
  expect(original.age.snapshot).toBe(a.age.snapshot);
  expect(original.records[map.id]).toEqual(map);
  expect(errors).toEqual([]);
});
