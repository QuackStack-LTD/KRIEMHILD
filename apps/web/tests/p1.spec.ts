import { test, expect, Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import type { Entry, State } from "../lib/types";

async function world(page: Page) {
  await page.goto("/");
  await page
    .getByLabel("World name", { exact: true })
    .fill(`Browser test ${randomUUID().slice(0, 8)}`);
  await page.getByLabel("First Age", { exact: true }).fill("Age of Rivers");
  await page
    .getByRole("button", { name: "Create world →", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Age of Rivers", exact: true }),
  ).toBeVisible();
}
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
async function switchAge(page: Page, ageID: string) {
  await page.getByLabel("Current Age").selectOption(ageID);
  // The old dossier remains visible during the request. Wait for installation of
  // the requested snapshot before interacting with a similarly named entity.
  await expect(page).toHaveURL((url) => url.searchParams.get("age") === ageID);
  await expect(page.getByLabel("Current Age")).toBeEnabled();
  await expect(page.getByLabel("Current Age")).toHaveValue(ageID);
}
async function addEntity(page: Page, name: string, type = "Settlement") {
  await nav(page, "Entities");
  await page.getByRole("button", { name: "+ New entity", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByLabel("Entity type", { exact: true }).fill(type);
  await page
    .getByRole("button", { name: "Create entity", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Save entity", exact: true }),
  ).toBeVisible();
}

test("P1 journey: entities, relationships, image pins, scene, independent Age, diff and reopen", async ({
  page,
}) => {
  const errors: string[] = [];
  const externalRequests: string[] = [];
  await page.route("**/*", (route) => {
    const url = new URL(route.request().url());
    if (url.hostname !== "127.0.0.1") {
      externalRequests.push(url.origin);
      return route.abort();
    }
    return route.continue();
  });
  page.on("pageerror", (e) => errors.push(e.message));
  await world(page);
  await addEntity(page, "Valer");
  await addEntity(page, "Mira", "Person");
  let a = await state(page);
  const city = Object.values(a.records).find((r) => r.name === "Valer")!;
  await page.getByLabel("Relationship name").fill("lives in");
  await page.getByLabel("Related entity").selectOption(city.id);
  await page.getByRole("button", { name: "Link", exact: true }).click();
  await expect(page.getByText("lives in", { exact: true })).toBeVisible();
  await nav(page, "Atlas");
  await page.getByLabel("Map name", { exact: true }).fill("Northern coast");
  await page.locator("input[type=file]").setInputFiles({
    name: "coast.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a7XkAAAAASUVORK5CYII=",
      "base64",
    ),
  });
  await page.getByRole("button", { name: "+ Create map", exact: true }).click();
  await expect(page.getByLabel("Entity to place")).toBeVisible();
  await page.getByLabel("Entity to place").selectOption(city.id);
  await page.getByLabel("Pin X percent").fill("30");
  await page.getByLabel("Pin Y percent").fill("40");
  await page.getByRole("button", { name: "Place / move pin" }).click();
  await expect(page.getByRole("cell", { name: "30.0%, 40.0%" })).toBeVisible();
  await nav(page, "Writing");
  await page.getByLabel("New scene title").fill("Arrival");
  await page.getByRole("button", { name: "+ Create scene" }).click();
  await page
    .getByRole("textbox", { name: "Manuscript" })
    .fill("Mira arrived in Valer beneath a silver moon.");
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  a = await state(page);
  const sourceSnapshot = a.age.snapshot,
    sourceRecords = JSON.stringify(a.records);
  const scene = Object.values(a.records).find((r) => r.kind === "scene")!;
  await page.getByRole("button", { name: "+ Begin a new Age" }).click();
  await page.getByLabel("New Age name").fill("Age of Ash");
  await page.getByRole("button", { name: "Create independent copy" }).click();
  await expect(page.getByLabel("Current Age")).not.toHaveValue(a.age.id);
  let b = await state(page);
  expect(b.age.snapshot).toBe(sourceSnapshot);
  expect(b.records[scene.id].settingAge).toBe(a.age.id);
  await nav(page, "Entities");
  await page.getByRole("button", { name: /Valer.*Settlement/ }).click();
  await page.getByLabel("Name", { exact: true }).fill("Velar");
  await page.getByLabel("Lifecycle").selectOption("destroyed");
  await page.getByRole("button", { name: "Save entity", exact: true }).click();
  await expect(
    page.getByRole("button", { name: /Velar.*Settlement/ }),
  ).toBeVisible();
  await nav(page, "Atlas");
  await page.getByLabel("Entity to place").selectOption(city.id);
  await page.getByLabel("Pin X percent").fill("80");
  await page.getByRole("button", { name: "Place / move pin" }).click();
  await expect(page.getByRole("cell", { name: /80.0%/ })).toBeVisible();
  await nav(page, "Writing");
  await page
    .getByRole("textbox", { name: "Manuscript" })
    .fill("Velar lay in ruins.");
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  b = await state(page);
  await nav(page, "History");
  await expect(
    page.getByText("3 changed records", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Undo", exact: true }).click();
  await expect
    .poll(async () =>
      JSON.stringify((await state(page)).records[scene.id].document),
    )
    .toContain("silver moon");
  await page.getByRole("button", { name: "Redo", exact: true }).click();
  await expect
    .poll(async () =>
      JSON.stringify((await state(page)).records[scene.id].document),
    )
    .toContain("ruins");
  await page.reload();
  await expect(page.getByLabel("Current Age")).toHaveValue(b.age.id);
  await switchAge(page, a.age.id);
  await expect
    .poll(async () => (await state(page)).age.snapshot)
    .toBe(sourceSnapshot);
  const unchanged = await state(page);
  expect(JSON.stringify(unchanged.records)).toBe(sourceRecords);
  await page.getByLabel("Search this Age").fill("silver moon");
  await expect(
    page.getByRole("heading", { name: "Arrival", exact: true }),
  ).toBeVisible();
  expect(errors).toEqual([]);
  expect(externalRequests).toEqual([]);
});

test("failed scene save survives reload and requires explicit recovery", async ({
  page,
}) => {
  await world(page);
  await nav(page, "Writing");
  await page.getByLabel("New scene title").fill("Recovery scene");
  await page.getByRole("button", { name: "+ Create scene" }).click();
  await expect(page.getByRole("textbox", { name: "Manuscript" })).toBeVisible();
  await page.route("**/commands", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "Test disk unavailable" }),
    }),
  );
  await page
    .getByRole("textbox", { name: "Manuscript" })
    .fill("These words must survive a failed save.");
  await expect(
    page.getByText("Not saved · recovery draft retained", { exact: true }),
  ).toBeVisible();
  await page.unroute("**/commands");
  page.on("dialog", (dialog) => dialog.accept());
  await page.reload();
  await expect(
    page.getByText("Unsaved scene recovered from this browser.", {
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Restore draft for review" }).click();
  await expect(page.getByRole("textbox", { name: "Manuscript" })).toHaveText(
    "These words must survive a failed save.",
  );
  await page.getByRole("button", { name: "Save scene now" }).click();
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(page.getByRole("textbox", { name: "Manuscript" })).toHaveText(
    "These words must survive a failed save.",
  );
});

test("custom type validation, source conflict review and deliberate correction", async ({
  page,
}) => {
  await world(page);
  await nav(page, "Entity types");
  await page.getByLabel("Type name", { exact: true }).fill("Port");
  await page.getByRole("button", { name: "+ Add field" }).click();
  await page.getByLabel("Field 1 name").fill("Population");
  await page.getByLabel("Field 1 type").selectOption("number");
  await page.getByRole("button", { name: "Save type" }).click();
  await expect(page.getByRole("heading", { name: "Edit type" })).toBeVisible();
  await addEntity(page, "Aster", "Port");
  await page.getByLabel("Population", { exact: true }).fill("100");
  await page.getByRole("button", { name: "Save entity", exact: true }).click();
  await expect
    .poll(
      async () =>
        Object.values((await state(page)).records).find(
          (r) => r.name === "Aster",
        )?.properties?.Population,
    )
    .toBe(100);
  await nav(page, "Entity types");
  await page
    .getByRole("button", { name: "Port 1 fields", exact: true })
    .click();
  await page.getByLabel("Type name", { exact: true }).fill("Seaport");
  await expect(
    page.getByText(
      /Saving this rename updates the type of 1 existing entities/,
    ),
  ).toBeVisible();
  await page.getByRole("button", { name: "Save type", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Seaport 1 fields", exact: true }),
  ).toBeVisible();
  await expect
    .poll(
      async () =>
        Object.values((await state(page)).records).find(
          (r) => r.name === "Aster",
        )?.type,
    )
    .toBe("Seaport");
  await page.getByLabel("Type name", { exact: true }).fill("Port");
  await page.getByRole("button", { name: "Save type", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Port 1 fields", exact: true }),
  ).toBeVisible();
  await nav(page, "Entities");
  const a = await state(page);
  const entity = Object.values(a.records).find((r) => r.name === "Aster")!;
  await page.getByRole("button", { name: "+ Begin a new Age" }).click();
  await page.getByLabel("New Age name").fill("Later");
  await page.getByRole("button", { name: "Create independent copy" }).click();
  await expect(page.getByLabel("Current Age")).not.toHaveValue(a.age.id);
  const b = await state(page);
  await page.getByRole("button", { name: /Aster.*Port/ }).click();
  await page.getByLabel("Name", { exact: true }).fill("Later Aster");
  await page.getByRole("button", { name: "Save entity", exact: true }).click();
  await expect(
    page.getByRole("button", { name: /Later Aster.*Port/ }),
  ).toBeVisible();
  await switchAge(page, a.age.id);
  await page.getByRole("button", { name: /Aster.*Port/ }).click();
  await page.getByLabel("Name", { exact: true }).fill("Corrected Aster");
  await page.getByRole("button", { name: "Save entity", exact: true }).click();
  await expect(
    page.getByRole("button", { name: /Corrected Aster.*Port/ }),
  ).toBeVisible();
  await switchAge(page, b.age.id);
  await nav(page, "History");
  await page.getByRole("button", { name: "Review source corrections" }).click();
  await expect(page.getByText(/Both Ages changed this record/)).toBeVisible();
  await page.getByLabel("Apply this record").check();
  page.once("dialog", (d) => d.accept());
  await page
    .getByRole("button", { name: "Apply 1 selected corrections" })
    .click();
  await expect
    .poll(async () => (await state(page)).records[entity.id].name)
    .toBe("Corrected Aster");
});

test("two tabs reject stale writes without losing either draft", async ({
  page,
  context,
}) => {
  await world(page);
  await addEntity(page, "Port");
  const other = await context.newPage();
  await other.goto(page.url());
  await other.getByRole("button", { name: /Port.*Settlement/ }).click();
  await other.getByLabel("Name", { exact: true }).fill("Stale name");
  await page.getByLabel("Name", { exact: true }).fill("First writer");
  await page.getByRole("button", { name: "Save entity", exact: true }).click();
  await expect(
    page.getByRole("button", { name: /First writer.*Settlement/ }),
  ).toBeVisible();
  await other.getByRole("button", { name: "Save entity", exact: true }).click();
  await expect(
    other.getByRole("alert").filter({ hasText: "Change not saved" }),
  ).toContainText("project changed");
  await expect(other.getByLabel("Name", { exact: true })).toHaveValue(
    "Stale name",
  );
  expect(
    Object.values((await state(page)).records).find(
      (r: Entry) => r.kind === "entity",
    )?.name,
  ).toBe("First writer");
  other.on("dialog", (dialog) => dialog.accept());
  await other.reload();
  await other.getByRole("button", { name: /First writer.*Settlement/ }).click();
  await expect(
    other.getByText("Unsaved entity recovered from this browser.", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    other.getByText(
      "The saved version changed after this draft began. Review the recovered fields before saving.",
      { exact: true },
    ),
  ).toBeVisible();
  await other
    .getByRole("button", {
      name: "Restore entity draft for review",
      exact: true,
    })
    .click();
  await expect(other.getByLabel("Name", { exact: true })).toHaveValue(
    "Stale name",
  );
  // Recovery is still a draft: it does not overwrite the first writer's save.
  expect(
    Object.values((await state(page)).records).find(
      (r: Entry) => r.kind === "entity",
    )?.name,
  ).toBe("First writer");
});
