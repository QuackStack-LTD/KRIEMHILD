import { test, expect, Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { writeFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";
import type { State, Entry } from "../lib/types";

test("an unreadable browser draft is retained without changing saved world data", async ({page}) => {
  const s=await setup(page);
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=entities`);
  await page.getByRole("button",{name:"+ New entity",exact:true}).click();
  await expect(page.getByLabel("Name",{exact:true})).toBeEnabled();
  const key=await page.evaluate(s=>{
    const key=`kriemhild-entity:${s.root.world.id}:${s.age.id}:new:tab:${sessionStorage.getItem("kriemhild-draft-tab")}`;
    localStorage.setItem(key,"{unreadable-but-preserved");return key;
  },s);
  await page.reload();
  await page.getByRole("button",{name:"+ New entity",exact:true}).click();
  await expect(page.getByText("Browser draft recovery is unavailable. Keep this tab open until your save succeeds.",{exact:true})).toBeVisible();
  expect(await page.evaluate(key=>localStorage.getItem(key),key)).toBe("{unreadable-but-preserved");
  const saved:State=await page.evaluate(async s=>(await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)).json(),s);
  expect(saved.revision).toBe(s.revision);
});

test("search across Ages opens the historical identity and preserves its context", async ({
  page,
}) => {
  let s = await setup(page);
  const city: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Place",
    name: "Harbor before the flood",
  };
  s = await put(page, s, city);
  const original = s;
  s = await page.evaluate(async (s) => {
    const response = await fetch(
      `/api/v1/projects/${s.root.world.id}/commands`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          expected: s.revision,
          age: s.age.id,
          action: "copy-age",
          name: "After the flood",
        }),
      },
    );
    if (!response.ok) throw Error(await response.text());
    return response.json();
  }, s);
  s = await put(page, s, { ...city, name: "Harbor on the hill" });
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=entities`);
  await page.getByLabel("Search this Age", { exact: true }).fill("Harbor");
  await page.getByLabel("Search all Ages", { exact: true }).check();
  await expect(
    page.getByRole("heading", { name: "Search across Ages", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", {
      name: "Open Harbor before the flood in Founding",
      exact: true,
    })
    .click();
  await expect(page.getByLabel("Current Age", { exact: true })).toHaveValue(
    original.age.id,
  );
  await expect(page.getByLabel("Name", { exact: true })).toHaveValue(
    "Harbor before the flood",
  );
});

test("saving a manuscript in one tab preserves another tab's failed-save recovery", async ({
  page,
  context,
}) => {
  let s = await setup(page);
  const scene: Entry = {
    id: randomUUID(),
    kind: "scene",
    name: "Two private drafts",
    settingAge: s.age.id,
    settingSnapshot: s.age.snapshot,
    document: { type: "doc", content: [{ type: "paragraph" }] },
  };
  s = await put(page, s, scene);
  const url = `/?world=${s.root.world.id}&age=${s.age.id}&view=writing`;
  await page.goto(url);
  const other = await context.newPage();
  await other.goto(url);
  await other.route("**/commands", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "Test write failure" }),
    }),
  );
  await other
    .getByRole("textbox", { name: "Manuscript", exact: true })
    .fill("The second writer keeps these unsaved words.");
  await expect(
    other.getByText("Not saved · recovery draft retained", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Manuscript", exact: true })
    .fill("The first writer saved a different passage.");
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  await other.unroute("**/commands");
  other.on("dialog", (dialog) => dialog.accept());
  await other.reload();
  await expect(
    other.getByText("Unsaved scene recovered from this browser.", {
      exact: true,
    }),
  ).toBeVisible();
  await other
    .getByRole("button", { name: "Restore draft for review", exact: true })
    .click();
  await expect(
    other.getByRole("textbox", { name: "Manuscript", exact: true }),
  ).toHaveText("The second writer keeps these unsaved words.");
  const saved: State = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(JSON.stringify(saved.records[scene.id].document)).toContain(
    "The first writer saved a different passage.",
  );
  expect(JSON.stringify(saved.records[scene.id].document)).not.toContain(
    "second writer",
  );
});

test("entity properties relationships notes and custom types recover explicit drafts", async ({
  page,
}) => {
  let s = await setup(page);
  const harbor: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Place",
    name: "Saved harbor",
  };
  const neighbor: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Place",
    name: "Neighbor",
  };
  s = await put(page, s, harbor);
  s = await put(page, s, neighbor);
  const nav = (name: string) =>
    page
      .getByRole("navigation", { name: "Workspaces" })
      .getByRole("button", { name, exact: true });
  page.on("dialog", (dialog) => dialog.accept());
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=entities`);
  await page.getByRole("button", { name: /Saved harbor.*Place/ }).click();
  await page
    .getByLabel("New property name", { exact: true })
    .fill("Unfinished research");
  await page
    .getByLabel("New property value", { exact: true })
    .fill("Keep this source");
  await expect(nav("Overview")).toBeDisabled();
  await page.reload();
  await page.getByRole("button", { name: /Saved harbor.*Place/ }).click();
  await page
    .getByRole("button", {
      name: "Restore property draft for review",
      exact: true,
    })
    .click();
  await expect(
    page.getByLabel("New property value", { exact: true }),
  ).toHaveValue("Keep this source");
  await page.getByRole("button", { name: "Add property", exact: true }).click();
  await page.getByRole("button", { name: "Save entity", exact: true }).click();
  await expect(nav("Overview")).toBeEnabled();
  await page
    .getByLabel("Relationship name", { exact: true })
    .fill("trades with");
  await page
    .getByLabel("Related entity", { exact: true })
    .selectOption(neighbor.id);
  await page.reload();
  await page.getByRole("button", { name: /Saved harbor.*Place/ }).click();
  await page
    .getByRole("button", {
      name: "Restore relationship draft for review",
      exact: true,
    })
    .click();
  await expect(page.getByLabel("Related entity", { exact: true })).toHaveValue(
    neighbor.id,
  );
  await page.getByRole("button", { name: "Link", exact: true }).click();
  await expect(nav("Research & notes")).toBeEnabled();
  await nav("Research & notes").click();
  await page.getByRole("button", { name: "+ New note", exact: true }).click();
  await page
    .getByLabel("Note title", { exact: true })
    .fill("A source fragment");
  await page
    .getByLabel("Notes", { exact: true })
    .fill("Unfinished notes survive a reload.");
  await page.reload();
  await page.getByRole("button", { name: "+ New note", exact: true }).click();
  await page
    .getByRole("button", { name: "Restore note draft for review", exact: true })
    .click();
  await expect(page.getByLabel("Notes", { exact: true })).toHaveValue(
    "Unfinished notes survive a reload.",
  );
  await page.getByRole("button", { name: "Save note", exact: true }).click();
  await expect(nav("Entity types")).toBeEnabled();
  await nav("Entity types").click();
  await page
    .getByLabel("Type name", { exact: true })
    .fill("Harbor custom type");
  await page.getByRole("button", { name: "+ Add field", exact: true }).click();
  await page.getByLabel("Field 1 name", { exact: true }).fill("Founded by");
  await page.getByLabel("Field 1 type", { exact: true }).selectOption("entity");
  await page.reload();
  await page
    .getByRole("button", { name: "Restore type draft for review", exact: true })
    .click();
  await expect(page.getByLabel("Field 1 type", { exact: true })).toHaveValue(
    "entity",
  );
  await page.getByRole("button", { name: "Save type", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Edit type", exact: true }),
  ).toBeVisible();
  // Manually returning to saved content also clears an obsolete recovery copy.
  await page.getByLabel("Type name", { exact: true }).fill("Temporary type");
  await page
    .getByLabel("Type name", { exact: true })
    .fill("Harbor custom type");
  await expect(nav("Overview")).toBeEnabled();
  await page.reload();
  await page
    .getByRole("button", { name: "Harbor custom type 1 fields", exact: true })
    .click();
  await expect(
    page.getByText("Unsaved type recovered from this browser.", {
      exact: true,
    }),
  ).toHaveCount(0);
  const after: State = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(after.records[harbor.id].properties?.["Unfinished research"]).toBe(
    "Keep this source",
  );
  expect(
    Object.values(after.records).filter((r) => r.kind === "relation"),
  ).toHaveLength(1);
  expect(
    Object.values(after.records).filter((r) => r.kind === "note"),
  ).toHaveLength(1);
});

test("authored recipe networks preview exact linked balances and preserve the source Age", async ({
  page,
}) => {
  let s = await setup(page);
  const grain: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Resource or good",
    name: "Grain stock",
    properties: {
      _domain: "resource",
      quantity: { mode: "exact", value: "0.3", unit: "kg" },
    },
  };
  const flour: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Resource or good",
    name: "Flour stock",
    properties: {
      _domain: "resource",
      quantity: { mode: "exact", value: "0", unit: "kg" },
    },
  };
  const recipe: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Production recipe",
    name: "Stone mill",
    properties: { _domain: "recipe" },
  };
  for (const r of [grain, flour, recipe]) s = await put(page, s, r);
  const original = s;
  s = await page.evaluate(async (s) => {
    const r = await fetch(`/api/v1/projects/${s.root.world.id}/commands`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        expected: s.revision,
        age: s.age.id,
        action: "copy-age",
        name: "Milling age",
      }),
    });
    if (!r.ok) throw Error(await r.text());
    return r.json();
  }, s);
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=economy`);
  await page
    .getByLabel("Production recipe", { exact: true })
    .selectOption(recipe.id);
  await page
    .getByLabel("Planned batches per period", { exact: true })
    .fill("3");
  await page.getByRole("button", { name: "Add input", exact: true }).click();
  await page
    .getByLabel("Input 1 resource", { exact: true })
    .selectOption(grain.id);
  await page.getByLabel("Input 1 amount", { exact: true }).fill("0.1");
  await page.getByRole("button", { name: "Add output", exact: true }).click();
  await page
    .getByLabel("Output 1 resource", { exact: true })
    .selectOption(flour.id);
  await page.getByLabel("Output 1 amount", { exact: true }).fill("0.2");
  await expect(page.getByLabel("Output 1 unit", { exact: true })).toHaveValue(
    "kg",
  );
  // An interrupted authoring session offers the same rule without silently saving.
  await page.reload();
  await page
    .getByLabel("Production recipe", { exact: true })
    .selectOption(recipe.id);
  await expect(
    page.getByText("Unsaved recipe recovered from this browser.", {
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByRole("button", {
      name: "Restore recipe draft for review",
      exact: true,
    })
    .click();
  await expect(page.getByLabel("Output 1 amount", { exact: true })).toHaveValue(
    "0.2",
  );
  await page
    .getByRole("button", { name: "Save production recipe", exact: true })
    .click();
  await expect(
    page.getByLabel("Production recipe", { exact: true }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "Experiments", exact: true }).click();
  await page
    .getByLabel("Experiment", { exact: true })
    .selectOption("production-network");
  await page
    .getByLabel("Production periods (1–100)", { exact: true })
    .fill("1");
  await page
    .getByLabel("Stone mill · Production recipe", { exact: true })
    .check();
  const before: State = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  await page
    .getByRole("button", { name: "Preview experiment", exact: true })
    .click();
  await expect(
    page.getByRole("heading", {
      name: "Linked inventory changes",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByLabel("Grain stock", { exact: true })).toBeDisabled();
  const previewed: State = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(previewed.revision).toBe(before.revision);
  await page
    .getByRole("button", { name: "Accept linked balances", exact: true })
    .click();
  await expect(
    page.getByRole("heading", {
      name: "Linked inventory changes",
      exact: true,
    }),
  ).toHaveCount(0);
  const after: State = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect((after.records[grain.id].properties!.quantity as any).value).toBe("0");
  expect((after.records[flour.id].properties!.quantity as any).value).toBe(
    "0.6",
  );
  const old: State = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    original,
  );
  expect(old.records).toEqual(original.records);
});

async function setup(page: Page): Promise<State> {
  await page.goto("/");
  await page.evaluate(async () => {
    const response = await fetch("/api/v1/session");
    if (!response.ok) throw Error("Local session did not initialize");
  });
  return page.evaluate(
    async (name) => {
      const r = await fetch("/api/v1/projects", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, age: "Founding" }),
      });
      if (!r.ok) throw Error(await r.text());
      return r.json();
    },
    "Deep authoring " + randomUUID().slice(0, 8),
  );
}
async function put(page: Page, s: State, record: Entry): Promise<State> {
  return page.evaluate(
    async ({ s, record }) => {
      const r = await fetch(`/api/v1/projects/${s.root.world.id}/commands`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          expected: s.revision,
          age: s.age.id,
          action: "put",
          record,
        }),
      });
      if (!r.ok) throw Error(await r.text());
      return r.json();
    },
    { s, record },
  );
}

test("reviewed restoration retains later work as another restorable revision", async ({
  page,
}) => {
  let s = await setup(page);
  const city: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Place",
    name: "Original harbor",
  };
  s = await put(page, s, city);
  const first = s.revision;
  s = await put(page, s, { ...city, name: "Later harbor" });
  const later = s.revision;
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=project`);
  await page
    .getByLabel("Saved world revision", { exact: true })
    .selectOption(first);
  await page
    .getByRole("button", { name: "Review restoration", exact: true })
    .click();
  await expect(
    page.getByRole("columnheader", {
      name: "Age after restoration",
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByRole("button", {
      name: "Restore reviewed world as a new revision",
      exact: true,
    })
    .click();
  await expect(
    page.getByLabel("Saved world revision", { exact: true }),
  ).toHaveValue("");
  const restored = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(restored.records[city.id].name).toBe("Original harbor");
  expect(restored.root.parent).toBe(later);
  await page
    .getByLabel("Saved world revision", { exact: true })
    .selectOption(later);
  await page
    .getByRole("button", { name: "Review restoration", exact: true })
    .click();
  await page
    .getByRole("button", {
      name: "Restore reviewed world as a new revision",
      exact: true,
    })
    .click();
  await expect(
    page.getByLabel("Saved world revision", { exact: true }),
  ).toHaveValue("");
  const recovered = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(recovered.records[city.id].name).toBe("Later harbor");
});

test("search pages keep results bounded and reset when the query changes", async ({
  page,
}) => {
  let s = await setup(page);
  s = await page.evaluate(
    async ({ s, records }) => {
      const r = await fetch(`/api/v1/projects/${s.root.world.id}/commands`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          expected: s.revision,
          age: s.age.id,
          action: "put-many",
          records,
        }),
      });
      if (!r.ok) throw Error(await r.text());
      return r.json();
    },
    {
      s,
      records: Array.from({ length: 105 }, (_, i) => ({
        id: randomUUID(),
        kind: "entity",
        type: "Place",
        name: `Port ${String(i).padStart(3, "0")}`,
      })),
    },
  );
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}`);
  await page.getByLabel("Search this Age").fill("Port");
  const results = page.locator(".search-results");
  await expect(results.locator("article")).toHaveCount(100);
  await expect(
    results.getByRole("heading", { name: "Port 000", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Next results" }).click();
  await expect(results.locator("article")).toHaveCount(5);
  await expect(
    results.getByRole("heading", { name: "Port 100", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Next results" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Previous results" }).click();
  await expect(results.locator("article")).toHaveCount(100);
  await page.getByRole("button", { name: "Next results" }).click();
  await expect(results.locator("article")).toHaveCount(5);
  await page.getByLabel("Search this Age").fill("Port 003");
  await expect(results.locator("article")).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: "Previous results" }),
  ).toBeDisabled();
  await results
    .getByRole("button", { name: "Open entity", exact: true })
    .click();
  await expect(page.getByLabel("Name", { exact: true })).toHaveValue(
    "Port 003",
  );
});

test("local map drawing works with keyboard controls and survives an Age copy", async ({
  page,
}) => {
  let s = await setup(page);
  s = await put(page, s, {
    id: randomUUID(),
    kind: "map",
    name: "Gatehouse",
    width: 1000,
    height: 700,
  });
  const unit: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Unit",
    name: "Gate guard",
  };
  s = await put(page, s, unit);
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=localmaps`);
  await page.getByLabel("Shape name", { exact: true }).fill("Watch room");
  for (const [x, y] of [
    [10, 10],
    [40, 10],
    [40, 40],
    [10, 40],
  ]) {
    await page.getByLabel("Point X %", { exact: true }).fill(String(x));
    await page.getByLabel("Point Y %", { exact: true }).fill(String(y));
    await page.getByRole("button", { name: "Add point", exact: true }).click();
  }
  await page.getByRole("button", { name: "Add shape to draft" }).click();
  await page.getByLabel("Token entity", { exact: true }).selectOption(unit.id);
  await page.getByRole("button", { name: "Place token in draft" }).click();
  await page.getByRole("button", { name: "Save local map plan" }).click();
  await expect(
    page.getByText("Drawing matches saved map", { exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Remove Watch room", exact: true }),
  ).toBeVisible();
  const source = (await page.evaluate(
    async ({ world, age }) =>
      (await fetch(`/api/v1/projects/${world}/state?age=${age}`)).json(),
    { world: s.root.world.id, age: s.age.id },
  )) as State;
  const copied = (await page.evaluate(async (s) => {
    const r = await fetch(`/api/v1/projects/${s.root.world.id}/commands`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        expected: s.revision,
        age: s.age.id,
        action: "copy-age",
        name: "Siege",
      }),
    });
    return r.json();
  }, source)) as State;
  const sourceMap = Object.values(source.records).find(
    (r) => r.kind === "map",
  )!;
  expect(copied.records[sourceMap.id].properties).toEqual(sourceMap.properties);
});

test("public edition searches offline without unselected secrets or external requests", async ({
  page,
  context,
}, info) => {
  let s = await setup(page);
  const city: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Settlement",
    name: "Rivergate",
    notes: "Public harbor history",
  };
  const secret: Entry = {
    id: randomUUID(),
    kind: "entity",
    type: "Secret",
    name: "PRIVATE_FORTRESS",
    notes: "PRIVATE_NOTES",
  };
  s = await put(page, s, city);
  s = await put(page, s, secret);
  const map: Entry = {
    id: randomUUID(),
    kind: "map",
    name: "Coast",
    width: 1000,
    height: 700,
    pins: [
      { entity: city.id, x: 0.2, y: 0.2 },
      { entity: secret.id, x: 0.8, y: 0.8 },
    ],
  };
  s = await put(page, s, map);
  const html = await page.evaluate(
    async ({ s, ids }) => {
      const r = await fetch(`/api/v1/projects/${s.root.world.id}/export`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          snapshot: s.age.snapshot,
          ids,
          format: "publication",
          title: "The Coast",
        }),
      });
      if (!r.ok) throw Error(await r.text());
      return r.text();
    },
    { s, ids: [city.id, map.id] },
  );
  expect(html).not.toContain("PRIVATE_");
  expect(html).not.toContain(secret.id);
  const file = info.outputPath("edition.html");
  await writeFile(file, html);
  await context.setOffline(true);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  const network: string[] = [];
  page.on("request", (r) => {
    if (/^https?:/.test(r.url())) network.push(r.url());
  });
  await page.goto(pathToFileURL(file).href);
  await expect(page.getByLabel("Search this edition")).toBeVisible();
  await page.getByLabel("Search this edition").fill("Public harbor history");
  await expect(page.locator("article:visible")).toHaveCount(1);
  await expect(page.locator("article:visible")).toContainText("Rivergate");
  await page.getByLabel("Search this edition").fill("");
  await expect(page.locator("article:visible")).toHaveCount(2);
  expect(errors).toEqual([]);
  expect(network).toEqual([]);
});

test("CSV preview creates a new Age and preserves the source", async ({
  page,
}) => {
  const s = await setup(page);
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=project`);
  await page
    .getByLabel("Import text", { exact: true })
    .fill("name,type,notes\nNew city,Settlement,Imported history\n");
  await page
    .getByRole("button", { name: "Preview text import", exact: true })
    .click();
  await expect(page.getByText("1 entries", { exact: true })).toBeVisible();
  await page
    .getByRole("button", { name: "Accept import as a new Age", exact: true })
    .click();
  await expect(page).toHaveURL(
    (url) => url.searchParams.get("age") !== s.age.id,
  );
  const original = (await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  )) as State;
  expect(Object.values(original.records)).toHaveLength(0);
});

test("family paths and storyboard plans remain author controlled", async ({
  page,
}) => {
  let s = await setup(page);
  const parent: Entry = {
      id: randomUUID(),
      kind: "entity",
      type: "Person",
      name: "Ada",
    },
    child: Entry = {
      id: randomUUID(),
      kind: "entity",
      type: "Person",
      name: "Ren",
    };
  s = await put(page, s, parent);
  s = await put(page, s, child);
  s = await put(page, s, {
    id: randomUUID(),
    kind: "relation",
    name: "adoptive parent of",
    from: parent.id,
    to: child.id,
  });
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=families`);
  await page
    .getByLabel("Starting person or entity", { exact: true })
    .selectOption(parent.id);
  await page
    .getByRole("button", { name: "Trace relationships", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "2 connected identities", exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("listitem")
      .filter({ hasText: "Ada → adoptive parent of → Ren" }),
  ).toBeVisible();
  await page
    .getByRole("navigation", { name: "Workspaces" })
    .getByRole("button", { name: "Storyboards", exact: true })
    .click();
  await page.getByLabel("New panel title", { exact: true }).fill("At the gate");
  await page
    .getByRole("button", { name: "Create storyboard panel", exact: true })
    .click();
  await page
    .getByLabel("Panel action", { exact: true })
    .fill("Ren raises the lantern.");
  await page
    .getByLabel("Panel public description", { exact: true })
    .fill("A lantern beside the gate.");
  await page
    .getByRole("button", { name: "Save storyboard panel", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Save storyboard panel", exact: true }),
  ).toBeEnabled();
  await page.reload();
  await expect(page.getByLabel("Panel action", { exact: true })).toHaveValue(
    "Ren raises the lantern.",
  );
});

test("failed storyboard save retains a reviewable draft across reload", async ({
  page,
}) => {
  let s = await setup(page);
  for (const name of ["Arrival", "Departure"])
    s = await put(page, s, {
      id: randomUUID(),
      kind: "entity",
      type: "Storyboard panel",
      name,
      properties: { _domain: "storyboard" },
    });
  await page.goto(
    `/?world=${s.root.world.id}&age=${s.age.id}&view=storyboards`,
  );
  const panel = page.getByRole("article").filter({
    has: page.getByRole("heading", { name: "Arrival", exact: true }),
  });
  await panel
    .getByLabel("Panel action", { exact: true })
    .fill("Keep this unsaved crossing.");
  await expect(
    page
      .getByRole("navigation", { name: "Workspaces" })
      .getByRole("button", { name: "Overview", exact: true }),
  ).toBeDisabled();
  await expect(
    panel.getByRole("button", { name: "Move panel later", exact: true }),
  ).toBeDisabled();
  await page.route("**/commands", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "Test storage unavailable" }),
    }),
  );
  await panel
    .getByRole("button", { name: "Save storyboard panel", exact: true })
    .click();
  await expect(page.locator(".error-banner[role=alert]")).toContainText(
    "Test storage unavailable",
  );
  await page.unroute("**/commands");
  page.on("dialog", (dialog) => dialog.accept());
  await page.reload();
  await expect(
    panel.getByText("Unsaved panel recovered from this browser.", {
      exact: true,
    }),
  ).toBeVisible();
  await panel
    .getByRole("button", {
      name: "Restore panel draft for review",
      exact: true,
    })
    .click();
  await expect(panel.getByLabel("Panel action", { exact: true })).toHaveValue(
    "Keep this unsaved crossing.",
  );
  await panel
    .getByRole("button", { name: "Save storyboard panel", exact: true })
    .click();
  await expect(
    page
      .getByRole("navigation", { name: "Workspaces" })
      .getByRole("button", { name: "Overview", exact: true }),
  ).toBeEnabled();
  await page.reload();
  await expect(panel.getByLabel("Panel action", { exact: true })).toHaveValue(
    "Keep this unsaved crossing.",
  );
  await expect(
    page.getByText("Unsaved panel recovered from this browser.", {
      exact: true,
    }),
  ).toHaveCount(0);
});

test("scene plan recovery preserves manuscript text and blocks accidental reordering", async ({
  page,
}) => {
  let s = await setup(page);
  const scene: Entry = {
    id: randomUUID(),
    kind: "scene",
    name: "Planning recovery",
    settingAge: s.age.id,
    settingSnapshot: s.age.snapshot,
    document: {
      type: "doc",
      content: [
        {
          type: "paragraph",
          content: [
            { type: "text", text: "The original manuscript stays intact." },
          ],
        },
      ],
    },
  };
  s = await put(page, s, scene);
  s = await put(page, s, {
    ...scene,
    id: randomUUID(),
    name: "Second scene",
    order: 2,
  });
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=outline`);
  await page
    .getByLabel("goal", { exact: true })
    .fill("Retain this planning goal.");
  await expect(
    page.getByRole("button", { name: "Move later", exact: true }).first(),
  ).toBeDisabled();
  await page.route("**/commands", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "Test planning save failure" }),
    }),
  );
  await page
    .getByRole("button", { name: "Save scene plan", exact: true })
    .click();
  await expect(page.locator(".error-banner[role=alert]")).toContainText(
    "Test planning save failure",
  );
  await page.unroute("**/commands");
  page.on("dialog", (dialog) => dialog.accept());
  await page.reload();
  await expect(
    page.getByText("Unsaved scene plan recovered from this browser.", {
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByRole("button", {
      name: "Restore scene plan draft for review",
      exact: true,
    })
    .click();
  await expect(page.getByLabel("goal", { exact: true })).toHaveValue(
    "Retain this planning goal.",
  );
  await page
    .getByRole("button", { name: "Save scene plan", exact: true })
    .click();
  await expect(
    page
      .getByRole("navigation", { name: "Workspaces" })
      .getByRole("button", { name: "Overview", exact: true }),
  ).toBeEnabled();
  const saved = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(saved.records[scene.id].properties.goal).toBe(
    "Retain this planning goal.",
  );
  expect(saved.records[scene.id].document).toEqual(scene.document);
});

test("an unsaved new domain dossier can be recovered without changing canon", async ({
  page,
}) => {
  const s = await setup(page);
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=domains`);
  await page
    .getByRole("button", { name: "New domain entity", exact: true })
    .click();
  await page
    .getByLabel("Domain entity name", { exact: true })
    .fill("Unwritten harbor");
  await expect(
    page.getByRole("button", { name: "Return to overview", exact: true }),
  ).toBeDisabled();
  await page.route("**/commands", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "Dossier save unavailable" }),
    }),
  );
  await page
    .getByRole("button", { name: "Save domain entity", exact: true })
    .click();
  await expect(page.locator(".error-banner[role=alert]")).toContainText(
    "Dossier save unavailable",
  );
  await page.unroute("**/commands");
  page.on("dialog", (dialog) => dialog.accept());
  await page.reload();
  await page
    .getByRole("button", { name: "New domain entity", exact: true })
    .click();
  await expect(
    page.getByText("Unsaved dossier recovered from this browser.", {
      exact: true,
    }),
  ).toBeVisible();
  const before = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(before.revision).toBe(s.revision);
  await page
    .getByRole("button", {
      name: "Restore dossier draft for review",
      exact: true,
    })
    .click();
  await expect(
    page.getByLabel("Domain entity name", { exact: true }),
  ).toHaveValue("Unwritten harbor");
  await page
    .getByRole("button", { name: "Save domain entity", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "New domain entity", exact: true }),
  ).toBeEnabled();
  const after = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(
    Object.values(after.records).filter(
      (r: any) => r.name === "Unwritten harbor",
    ),
  ).toHaveLength(1);
});

test("Unicode names and right-to-left manuscript text survive saving and reload", async ({
  page,
}) => {
  const s = await setup(page);
  const name = "Місто".repeat(40);
  await put(page, s, { id: randomUUID(), kind: "entity", type: "Place", name });
  await page.goto(`/?world=${s.root.world.id}&age=${s.age.id}&view=writing`);
  await page
    .getByLabel("New scene title", { exact: true })
    .fill("حكاية النهر — Річкова оповідь");
  await page
    .getByRole("button", { name: "+ Create scene", exact: true })
    .click();
  const text =
    "عند النهر تبدأ الحكاية. Київ — місто історій. 日本語の物語。 a\u0301 e\u0308";
  const manuscript = page.getByRole("textbox", {
    name: "Manuscript",
    exact: true,
  });
  await manuscript.fill(text);
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  await expect(manuscript).toHaveCSS("direction", "rtl");
  await page.reload();
  await expect(manuscript).toHaveText(text);
  const saved = await page.evaluate(
    async (s) =>
      (
        await fetch(`/api/v1/projects/${s.root.world.id}/state?age=${s.age.id}`)
      ).json(),
    s,
  );
  expect(Object.values(saved.records).some((r: any) => r.name === name)).toBe(
    true,
  );
});
