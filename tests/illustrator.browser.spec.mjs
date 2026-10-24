import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import {
  landPaths,
  landContains,
  selectedLand,
} from "../src/continuous/land-selection.js";
import { bounds } from "../src/continuous/geometry.js";

// Explicitly opt in with an expendable continuous project. This test creates
// an illustration draft and exports it; never point it at an author's project.
const project = process.env.KRIEMHILD_ILLUSTRATOR_TEST_PROJECT;
test("selected seam-crossing land, inland water and export share one fitted extent", async ({
  page,
  request,
}) => {
  test.skip(
    !project,
    "Set KRIEMHILD_ILLUSTRATOR_TEST_PROJECT to an expendable project with seam-crossing land and lakes.",
  );
  test.setTimeout(180000);
  const opened = await request.post(`/api/projects/${project}/open`, {
    data: {},
  });
  expect(opened.ok()).toBeTruthy();
  const before = await opened.json();
  const source = await (
    await request.get(`/api/sessions/${before.id}/terrain/land-selection`)
  ).json();
  const component = source.components.find(
    (c) =>
      c.paths.some((r) => r.some((p) => p[0] < 0.01)) &&
      c.paths.some((r) => r.some((p) => p[0] > 1.99)),
  );
  expect(
    component,
    "fixture must include a seam-crossing continent",
  ).toBeTruthy();
  const prepared = {
    ...component,
    paths: landPaths(component.paths, 0.1 / source.sourceWidth),
  };
  const fit = selectedLand([prepared], [prepared.id]);
  const point = prepared.paths
    .flat()
    .map(([x, y]) => [x + 0.001, y + 0.001])
    .find((p) => p[0] > 0.01 && p[0] < 0.2 && landContains(prepared, p));
  expect(
    point,
    "fixture must have clickable land near the left seam",
  ).toBeTruthy();
  const errors = [];
  const meshes = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("response", (response) => {
    if (response.url().includes("/terrain/mesh?") && response.ok())
      meshes.push(new URL(response.url()).searchParams);
  });
  await page.goto(`/world/${project}/build?tileDebug=1`);
  const svg = page.getByRole("img", {
    name: "Interactive continuous world map",
    exact: true,
  });
  await expect(svg).toBeVisible();
  const originalCamera = { x: 0, y: 0, w: 2, h: 1 };
  await page.evaluate(
    (camera) =>
      window.dispatchEvent(
        new CustomEvent("terrain-diagnostic-camera", { detail: camera }),
      ),
    originalCamera,
  );
  await page
    .getByRole("button", { name: "Medieval Illustrator", exact: true })
    .click();
  const savedMap = page.getByRole("combobox", {
    name: "Saved illustrated map",
  });
  if (await savedMap.count()) await savedMap.selectOption("");
  await page
    .getByRole("textbox", { name: "Illustrated map name" })
    .fill("Disposable seam and lake map");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(page.getByText("0 selected", { exact: true })).toBeVisible();
  const click = await svg.evaluate((s, p) => {
    const r = s.getBoundingClientRect(),
      c = JSON.parse(s.dataset.camera),
      scale = Math.min(r.width / c.w, r.height / c.h);
    return {
      x: (r.width - c.w * scale) / 2 + (p[0] - c.x) * scale,
      y: (r.height - c.h * scale) / 2 + (p[1] - c.y) * scale,
    };
  }, point);
  await svg.click({ position: click });
  await expect(page.getByText("1 selected", { exact: true })).toBeVisible();
  await page
    .getByRole("button", { name: "Create illustrated map", exact: true })
    .click();
  await expect(
    svg.locator('[aria-label="Editable medieval illustration"]'),
  ).toBeVisible();
  const camera = JSON.parse(await svg.getAttribute("data-camera"));
  expect(camera.x).toBeCloseTo(fit.bounds.x, 10);
  expect(camera.w).toBeCloseTo(fit.bounds.w, 10);
  expect(camera.w).toBeLessThan(1);
  await page.waitForFunction(() => {
    const log = window.__terrainViewportLog || [],
      request = log.filter((e) => e.event === "request").at(-1);
    return log.some(
      (e) =>
        e.event === "frame" &&
        e.phase === "final" &&
        e.generation === request?.generation,
    );
  });
  expect(await svg.textContent()).not.toContain("km at this latitude");
  expect(meshes.some((q) => +q.get("west") > 0 && +q.get("east") === 180)).toBe(
    true,
  );
  expect(
    meshes.some((q) => +q.get("west") === -180 && +q.get("east") < 0),
  ).toBe(true);
  const lakeCandidates = fit.regions
    .map((r) => {
      const b = bounds(r);
      return [b.x + b.w / 2, b.y + b.h / 2];
    })
    .filter((p) => !landContains({ paths: fit.regions }, p));
  let lake;
  for (const p of lakeCandidates.slice(0, 20)) {
    const lon = ((((p[0] * 180) % 360) + 360) % 360) - 180,
      lat = 90 - p[1] * 180;
    const sample = await (
      await request.get(
        `/api/sessions/${before.id}/terrain/sample?lon=${lon}&lat=${lat}`,
      )
    ).json();
    if (sample.water === "lake") {
      lake = p;
      break;
    }
  }
  expect(lake, "fixture must include a genuine inland lake").toBeTruthy();
  const covered = await svg.evaluate((s, p) => {
    const c = JSON.parse(s.dataset.camera),
      r = s.getBoundingClientRect(),
      scale = Math.min(r.width / c.w, r.height / c.h),
      path = s.querySelector("#selected-illustration-land path");
    const ctx = document.createElement("canvas").getContext("2d");
    return ctx.isPointInPath(
      new Path2D(path.getAttribute("d")),
      (r.width - c.w * scale) / 2 + (p[0] - c.x) * scale,
      (r.height - c.h * scale) / 2 + (p[1] - c.y) * scale,
      path.getAttribute("fill-rule"),
    );
  }, lake);
  expect(covered).toBe(true);
  await page
    .getByRole("combobox", { name: /^Export width/ })
    .selectOption("1024");
  const downloaded = page.waitForEvent("download");
  await page.getByRole("button", { name: "SVG", exact: true }).click();
  const download = await downloaded,
    xml = await readFile(await download.path(), "utf8");
  expect(xml).not.toContain("km at this latitude");
  expect(xml).toContain('fill-rule="nonzero"');
  const pixel = await page.evaluate(
    async ({ xml, lake, camera }) => {
      const doc = new DOMParser().parseFromString(xml, "image/svg+xml"),
        svg = doc.documentElement,
        vb = svg.getAttribute("viewBox").split(" ").map(Number);
      const url = URL.createObjectURL(
        new Blob([xml], { type: "image/svg+xml" }),
      );
      try {
        const image = new Image();
        image.src = url;
        await image.decode();
        const canvas = document.createElement("canvas");
        canvas.width = image.width;
        canvas.height = image.height;
        const ctx = canvas.getContext("2d");
        ctx.drawImage(image, 0, 0);
        const x = Math.floor(((lake[0] - camera.x) / camera.w) * canvas.width),
          y = Math.floor(((lake[1] - camera.y) / camera.h) * canvas.height);
        return {
          rgba: [...ctx.getImageData(x, y, 1, 1).data],
          vb,
          images: doc.querySelectorAll("image").length,
          external: [...doc.querySelectorAll("image")].some(
            (e) => !e.getAttribute("href").startsWith("data:"),
          ),
        };
      } finally {
        URL.revokeObjectURL(url);
      }
    },
    { xml, lake, camera },
  );
  expect(pixel.images).toBeGreaterThan(0);
  expect(pixel.external).toBe(false);
  expect(pixel.rgba[3]).toBe(255);
  expect(
    Math.hypot(pixel.rgba[0] - 233, pixel.rgba[1] - 217, pixel.rgba[2] - 175),
  ).toBeGreaterThan(20);
  const after = await (
    await request.get(`/api/sessions/${before.id}/world/state`)
  ).json();
  expect(after.continuous).toEqual(before.continuous);
  await page
    .getByRole("button", { name: "Construction mode", exact: true })
    .click();
  await expect
    .poll(async () => JSON.parse(await svg.getAttribute("data-camera")))
    .toEqual(originalCamera);
  await expect
    .poll(async () => {
      const state = await (
        await request.post(`/api/projects/${project}/open`, { data: {} })
      ).json();
      return state.projectUI.continuousView.camera;
    })
    .toEqual(originalCamera);
  expect(errors).toEqual([]);
});
