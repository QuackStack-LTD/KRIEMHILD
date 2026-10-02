import { test, expect, Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import type { State } from "../lib/types";
async function create(page: Page) {
  await page.goto("/");
  await page
    .getByLabel("World name", { exact: true })
    .fill("Authoring " + randomUUID().slice(0, 8));
  await page.getByLabel("First Age", { exact: true }).fill("Founding");
  await page
    .getByRole("button", { name: "Create world →", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Founding", exact: true }),
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
        `/api/v1/projects/${p.get("world")}/state?age=${p.get("age") || ""}`,
      )
    ).json();
  });
}

test("P3 domains accept qualitative and ranged quantities and connect to manuscript plans", async ({
  page,
}) => {
  await create(page);
  await nav(page, "Societies & systems");
  await page
    .getByRole("button", { name: "New domain entity", exact: true })
    .click();
  await page
    .getByLabel("Domain entity name", { exact: true })
    .fill("Rivergate");
  await page.getByLabel("population mode").selectOption("range");
  await page.getByLabel("population min", { exact: true }).fill("1000");
  await page.getByLabel("population max", { exact: true }).fill("1500");
  await page.getByLabel("population unit", { exact: true }).fill("people");
  await page
    .getByRole("button", { name: "Save domain entity", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Connected world" }),
  ).toBeVisible();
  let st = await state(page);
  const city = Object.values(st.records).find((r) => r.name === "Rivergate")!;
  expect(city.properties?.population).toMatchObject({
    mode: "range",
    min: "1000",
    max: "1500",
  });
  await page.getByLabel("Domain template").selectOption("culture");
  await page
    .getByRole("button", { name: "New domain entity", exact: true })
    .click();
  await page
    .getByLabel("Domain entity name", { exact: true })
    .fill("River people");
  await page
    .getByLabel("traditions", { exact: true })
    .fill("Every bridge is named by its builders.");
  await page
    .getByRole("button", { name: "Save domain entity", exact: true })
    .click();
  await page.getByLabel("Relationship role").fill("present in");
  await page.getByLabel("Connection target").selectOption(city.id);
  await page
    .getByRole("button", { name: "Add connection", exact: true })
    .click();
  await expect(
    page.getByRole("button", {
      name: "River people → present in → Rivergate",
      exact: true,
    }),
  ).toBeVisible();
  await nav(page, "Writing");
  await page.getByLabel("New scene title").fill("Crossing the bridge");
  await page.getByRole("button", { name: "+ Create scene" }).click();
  await expect(page.getByLabel("Scene title", { exact: true })).toBeVisible();
  await nav(page, "Story architecture");
  await page
    .getByLabel("goal", { exact: true })
    .fill("Reach the city before nightfall");
  await page.getByLabel("Location", { exact: true }).selectOption(city.id);
  await page.getByRole("button", { name: "Save scene plan" }).click();
  await expect(
    page.getByRole("button", { name: "Save scene plan" }),
  ).toBeEnabled();
  st = await state(page);
  const scene = Object.values(st.records).find((r) => r.kind === "scene")!;
  expect(scene.properties).toMatchObject({
    locationID: city.id,
    goal: "Reach the city before nightfall",
  });
  await page.reload();
  await expect(page.getByLabel("goal", { exact: true })).toHaveValue(
    "Reach the city before nightfall",
  );
});

test("native archive download and preview import creates an independent world", async ({
  page,
}, info) => {
  await create(page);
  const source = await state(page);
  await nav(page, "Project & export");
  const download = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "Download native backup", exact: true })
    .click();
  const file = await download;
  const path = info.outputPath("backup.zip");
  await file.saveAs(path);
  await page.getByLabel("Native archive").setInputFiles(path);
  await expect(
    page.getByRole("button", { name: "Import as a new world" }),
  ).toBeVisible();
  await expect(
    page.getByText("source format 2", { exact: false }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Import as a new world" }).click();
  await expect(page).toHaveURL(
    (url) =>
      !!url.searchParams.get("world") &&
      url.searchParams.get("world") !== source.root.world.id,
  );
  const imported = await state(page);
  expect(imported.root.world.name).toBe(source.root.world.name);
  expect(imported.root.world.id).not.toBe(source.root.world.id);
  expect(Object.keys(imported.root.ages)).toEqual(
    Object.keys(source.root.ages),
  );
});

test("P6 two authors merge shared manuscript edits and checkpoint history", async ({
  page,
  context,
}) => {
  test.setTimeout(90000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await create(page);
  await nav(page, "Writing");
  await page.getByLabel("New scene title").fill("Shared opening");
  await page.getByRole("button", { name: "+ Create scene" }).click();
  await page
    .getByRole("textbox", { name: "Manuscript", exact: true })
    .fill("The gate opens. ");
  await expect(
    page.getByText("Saved on this device", { exact: true }),
  ).toBeVisible();
  await nav(page, "Shared writing");
  const a = page.getByRole("textbox", {
    name: "Shared manuscript",
    exact: true,
  });
  await expect(a).toContainText("The gate opens.");
  const second = await context.newPage();
  second.on("pageerror", (e) => errors.push(e.message));
  await second.goto(page.url());
  const b = second.getByRole("textbox", {
    name: "Shared manuscript",
    exact: true,
  });
  await expect(b).toContainText("The gate opens.");
  await a.click();
  await page.keyboard.press("Control+End");
  await b.click();
  await second.keyboard.press("Control+End");
  await Promise.all([
    page.keyboard.insertText("Alpha arrives. "),
    second.keyboard.insertText("Beta waits. "),
  ]);
  await expect(a).toContainText("Alpha arrives.");
  await expect(a).toContainText("Beta waits.");
  await expect(b).toContainText("Alpha arrives.");
  await expect(b).toContainText("Beta waits.");
  await expect(async () => {
    await page
      .getByRole("button", { name: "Save shared revision", exact: true })
      .click();
    await expect(
      page.getByText("Shared revision saved to Age history", { exact: true }),
    ).toBeVisible();
  }).toPass({ timeout: 15000 });
  const st = await state(page);
  const scene = Object.values(st.records).find((r) => r.kind === "scene")!;
  const raw = JSON.stringify(scene.document);
  expect(raw).toContain("Alpha arrives.");
  expect(raw).toContain("Beta waits.");
  expect(raw.match(/The gate opens/g)).toHaveLength(1);
  // Let one browser fall behind while another checkpoints a compacted log.
  await second.route("**/live**", (route) => route.abort());
  await a.click();
  await page.keyboard.press("Control+End");
  await page.keyboard.insertText("Gamma enters. ");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("Delta follows.");
  await expect(async () => {
    await page
      .getByRole("button", { name: "Save shared revision", exact: true })
      .click();
    await expect(
      page.getByText("Shared revision saved to Age history", { exact: true }),
    ).toBeVisible();
  }).toPass({ timeout: 15000 });
  await second.unroute("**/live**");
  await expect(b).toContainText("Delta follows.");
  const checkpoint = await state(page);
  const sharedScene = Object.values(checkpoint.records).find(
    (r) => r.kind === "scene",
  )!;
  const paragraphs =
    sharedScene.document?.content?.filter((n) => n.type === "paragraph") || [];
  expect(paragraphs.length).toBeGreaterThan(1);
  expect(paragraphs.every((p) => !!p.attrs?.blockId)).toBe(true);
  const blockIDs = paragraphs.map((p) => String(p.attrs!.blockId));
  expect(new Set(blockIDs).size).toBe(blockIDs.length);
  await page
    .getByLabel("Comment on", { exact: true })
    .selectOption(blockIDs[blockIDs.length - 1]);
  await page
    .getByLabel("Comment", { exact: true })
    .fill("Check the new paragraph's timing.");
  await page
    .getByRole("button", { name: "Add manuscript comment", exact: true })
    .click();
  await expect(
    page.getByText("Check the new paragraph's timing.", { exact: true }),
  ).toBeVisible();
  await b.click();
  await second.keyboard.press("Control+End");
  await second.keyboard.insertText(" Epsilon closes the door.");
  await expect(a).toContainText("Epsilon closes the door.");
  await second.close();
  await page.reload();
  await expect(
    page.getByRole("textbox", { name: "Shared manuscript", exact: true }),
  ).toContainText("Beta waits.");
  await expect(
    page.getByRole("textbox", { name: "Shared manuscript", exact: true }),
  ).toContainText("Epsilon closes the door.");
  expect(errors).toEqual([]);
});
