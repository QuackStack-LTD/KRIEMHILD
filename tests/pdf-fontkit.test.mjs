import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { PDFDocument } from "pdf-lib";
import fontkit from "../src/worldbuilding/writer/pdf-fontkit.js";

test("the modular font engine exports subset PDFs with Latin, Cyrillic, Greek and ligatures", async () => {
  const bytes = await readFile(
    new URL("../src/assets/fonts/NotoSans-Regular.ttf", import.meta.url),
  );
  const pdf = await PDFDocument.create();
  pdf.registerFontkit(fontkit);
  const font = await pdf.embedFont(bytes, { subset: true });
  const text = "KRIEMHILD — café, Київ, Ελληνικά, office";
  assert.ok(font.widthOfTextAtSize(text, 12) > 0);
  pdf.addPage().drawText(text, { font, size: 12, x: 30, y: 700 });
  const saved = await pdf.save();
  assert.ok(
    saved.length < bytes.length / 4,
    "PDF should contain a font subset",
  );
  assert.equal((await PDFDocument.load(saved)).getPageCount(), 1);
  assert.equal(new TextDecoder().decode(saved.slice(0, 4)), "%PDF");
});

test("font subset errors reach the PDF writer instead of leaving its save pending", async () => {
  const bytes = await readFile(
    new URL("../src/assets/fonts/NotoSans-Regular.ttf", import.meta.url),
  );
  const font = fontkit.create(bytes),
    subset = font.createSubset();
  const expected = new Error("subset failure");
  subset.encode = () => {
    throw expected;
  };
  await assert.rejects(
    new Promise((resolve, reject) =>
      subset
        .encodeStream()
        .on("data", () => {})
        .on("end", resolve)
        .on("error", reject),
    ),
    (e) => e === expected,
  );
});
