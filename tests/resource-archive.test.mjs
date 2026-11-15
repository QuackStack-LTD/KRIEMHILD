import test from "node:test";
import assert from "node:assert/strict";
import {
  resourceReportParagraph,
  resourceAreas,
} from "../src/continuous/resource-report-text.js";
import { matchesArchiveFilters } from "../src/worldbuilding/archives.js";
import {
  inworldItems,
  crumbs,
} from "../src/worldbuilding/writer/archive-model.js";
import {
  previewRaster,
  immediateRasterLimit,
} from "../src/continuous/preview-raster.js";

test("copied resource information is a single paragraph containing hidden details and zero values", () => {
  const text = resourceReportParagraph({
    longitude: -44.2,
    areaKm2: 314,
    resources: [
      {
        type: "iron",
        quantity: 0,
        quantityUnit: "tonnes",
        confidence: "inferred",
        reasonCodes: ["basement", "fault"],
      },
    ],
    fields: { elevation: { min: 0, max: 12, mean: 6 } },
    biosphere: {
      stale: false,
      lifeforms: [
        { name: "Willow\nby the\triver", group: "plant", roles: ["producer"] },
      ],
    },
    notes: ["Whole deposits only."],
    fingerprint: "internal-digest",
    context: { worldId: "internal-world" },
  });
  for (const value of [
    "314",
    "iron",
    "quantity: 0",
    "tonnes",
    "confidence: inferred",
    "basement",
    "fault",
    "min: 0",
    "mean: 6",
    "stale: false",
    "Willow by the river",
    "producer",
    "Whole deposits only.",
  ])
    assert.ok(text.includes(value), value);
  assert.doesNotMatch(text, /[\r\n\t]/);
  assert.doesNotMatch(text, /internal-digest|internal-world/);
});
test("every linked archive exposes Resources with review and documents, and saved files keep Age/year ancestry", () => {
  const doc = {
    ages: [{ id: "a", name: "First Age" }],
    entities: [
      {
        id: "archive",
        ageId: "a",
        kind: "archive",
        name: "Harbor",
        properties: {},
      },
      {
        id: "empty",
        ageId: "a",
        kind: "archive",
        name: "Empty archive",
        properties: {},
      },
      {
        id: "resources",
        ageId: "a",
        kind: "archive",
        name: "Resources",
        properties: { parentArchive: "archive", resourceFolder: "resources" },
      },
      {
        id: "age-folder",
        ageId: "a",
        kind: "archive",
        name: "First Age",
        properties: {
          parentArchive: "resources",
          resourceFolder: "age",
          sourceAge: "a",
        },
      },
      {
        id: "year-folder",
        ageId: "a",
        kind: "archive",
        name: "Year 10",
        properties: {
          parentArchive: "age-folder",
          resourceFolder: "year",
          sourceAge: "a/10",
        },
      },
      {
        id: "report",
        ageId: "a",
        kind: "document",
        name: "Survey",
        properties: {},
      },
    ],
    libraryFiles: [
      {
        id: "report",
        ageId: "a",
        archiveId: "year-folder",
        versions: [{ name: "survey.md" }],
      },
    ],
  };
  const items = inworldItems(doc);
  assert.equal(new Set(items.map((n) => n.key)).size, items.length);
  for (const archive of ["archive", "empty"]) {
    assert.ok(
      items.some((n) => n.resourceReview && n.resourceArchiveId === archive),
    );
    assert.ok(
      items.some(
        (n) =>
          n.title === "Resource documents" && n.resourceArchiveId === archive,
      ),
    );
  }
  assert.equal(
    items.filter((n) => n.resourceReview).length,
    2,
    "generated resource/year folders must not recursively acquire resource subfolders",
  );
  assert.deepEqual(
    crumbs(items, "inworld/a/report").map((n) => n.title),
    [
      "Inworld Archive",
      "First Age",
      "Harbor",
      "Resources",
      "Resource documents",
      "First Age",
      "Year 10",
      "Survey",
    ],
  );
});
test("repeated reports share one map area while different overlapping circles remain selectable", () => {
  const report = (id, radius, age, at) => ({
    id,
    survey: {
      terrainId: "terrain",
      sourceAge: age,
      recordedAt: at,
      circle: {
        longitude: 0,
        latitude: 0,
        radiusKm: radius,
        planetRadiusKm: 6371,
      },
    },
  });
  const areas = resourceAreas(
    [
      report("old", 10, "old", 10),
      report("current", 10, "current", 20),
      report("late copy", 10, "old", 30),
      report("small", 5, "current", 40),
      report("large", 12, "current", 50),
    ],
    "current",
  );
  assert.equal(areas.length, 3);
  assert.deepEqual(
    areas.map((a) => a.survey.circle.radiusKm),
    [12, 10, 5],
  );
  assert.equal(areas[1].id, "current");
  assert.equal(areas[1].reportCount, 3);
});
test("archive filters recognize resource report Ages and year zero without assigning dates to undated reports", () => {
  const filters = {
    age: "age",
    type: "geographical",
    year: "0",
    recorded: "0",
    entity: "",
  };
  const file = { resourceSurvey: { sourceAge: "age", year: 0 } };
  assert.equal(matchesArchiveFilters(file, filters), true);
  file.resourceSurvey.year = null;
  assert.equal(matchesArchiveFilters(file, filters), false);
  assert.equal(
    matchesArchiveFilters(file, { ...filters, year: "", recorded: "" }),
    true,
  );
});
test("small preview PNGs avoid delayed asynchronous encoding; large rasters keep the bounded asynchronous path", async () => {
  const oldCanvas = globalThis.HTMLCanvasElement,
    oldDocument = globalThis.document;
  class Canvas {
    toDataURL() {
      return "data:image/png;base64,AAD/";
    }
  }
  globalThis.HTMLCanvasElement = Canvas;
  globalThis.document = { createElement: () => new Canvas() };
  try {
    let fallbacks = 0;
    const fallback = (w, h) => {
      fallbacks++;
      return { width: w, height: h, async: true };
    };
    const small = previewRaster(512, 256, fallback);
    assert.equal(fallbacks, 0);
    assert.deepEqual(
      new Uint8Array(await (await small.convertToBlob()).arrayBuffer()),
      new Uint8Array([0, 0, 255]),
    );
    assert.ok(previewRaster(immediateRasterLimit + 1, 1, fallback).async);
    assert.ok(previewRaster(512, 256, fallback, false).async);
    assert.equal(fallbacks, 2);
  } finally {
    globalThis.HTMLCanvasElement = oldCanvas;
    globalThis.document = oldDocument;
  }
});
