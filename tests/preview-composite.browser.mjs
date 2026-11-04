import { chromium } from "@playwright/test";
import { build } from "esbuild";
import { mkdir, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
const bundle = (
  await build({
    stdin: {
      contents:
        "export {compositePreview} from './src/continuous/preview-composite.js'",
      resolveDir: process.cwd(),
    },
    bundle: true,
    format: "iife",
    globalName: "PreviewCompositor",
    write: false,
  })
).outputFiles[0].text;
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.KRIEMHILD_BROWSER_EXECUTABLE,
});
try {
  const page = await browser.newPage();
  await page.addScriptTag({ content: bundle });
  const result = await page.evaluate(async () => {
    const canvas = (w, h) => new OffscreenCanvas(w, h),
      width = 64,
      height = 32;
    const base = canvas(width, height),
      shade = canvas(width, height);
    for (const [c, isShade] of [
      [base, false],
      [shade, true],
    ]) {
      const ctx = c.getContext("2d");
      const a = ctx.createImageData(width, height);
      for (let y = 0; y < height; y++)
        for (let x = 0; x < width; x++) {
          const i = (y * width + x) * 4;
          a.data[i] = isShade ? x * 4 : 40 + x * 3;
          a.data[i + 1] = isShade ? x * 4 : 100 + y * 3;
          a.data[i + 2] = isShade ? x * 4 : 180 - y * 3;
          a.data[i + 3] = x < 3 ? 0 : x < 6 ? 128 : 255;
        }
      ctx.putImageData(a, 0, 0);
    }
    const images = [
      { style: "natural", blob: await base.convertToBlob() },
      { style: "hillshade", blob: await shade.convertToBlob() },
    ];
    const frame = {
      images,
      pixels: { width, height },
      bounds: { x: 1.2, y: 0.2, w: 0.1, h: 0.05 },
      revision: 17,
      generation: 3,
    };
    const dataURL = (blob) =>
      new Promise((r) => {
        const f = new FileReader();
        f.onload = () => r(f.result);
        f.readAsDataURL(blob);
      });
    const urls = await Promise.all(images.map((i) => dataURL(i.blob))),
      comparisons = [];
    for (const opacity of [0.05, 0.55, 1]) {
      const actual = await PreviewCompositor.compositePreview(
        frame,
        opacity,
        new AbortController().signal,
      );
      const xml = `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="32" viewBox="0 0 64 32"><rect width="64" height="32" fill="#101d2a"/><image href="${urls[0]}" width="64" height="32"/><g opacity="${opacity}" style="mix-blend-mode:multiply"><image href="${urls[1]}" width="64" height="32"/></g></svg>`;
      const ref = new Image();
      ref.src = URL.createObjectURL(new Blob([xml], { type: "image/svg+xml" }));
      await ref.decode();
      const reference = canvas(width, height),
        observed = canvas(width, height);
      reference.getContext("2d").drawImage(ref, 0, 0);
      URL.revokeObjectURL(ref.src);
      const bitmap = await createImageBitmap(actual.images[0].blob);
      observed.getContext("2d").drawImage(bitmap, 0, 0);
      bitmap.close();
      const a = reference
          .getContext("2d")
          .getImageData(0, 0, width, height).data,
        b = observed.getContext("2d").getImageData(0, 0, width, height).data;
      let max = 0,
        sum = 0;
      for (let i = 0; i < a.length; i++) {
        const d = Math.abs(a[i] - b[i]);
        max = Math.max(max, d);
        sum += d;
      }
      comparisons.push({
        opacity,
        maxError: max,
        meanChannelError: sum / a.length,
        bounds: actual.bounds,
        revision: actual.revision,
        images: actual.images.length,
        compositedStyles: actual.compositedStyles,
      });
    }
    return comparisons;
  });
  await mkdir("docs/preview-climate-investigation/preview-composite", {
    recursive: true,
  });
  await writeFile(
    "docs/preview-climate-investigation/preview-composite/pixels.json",
    JSON.stringify(result, null, 2),
  );
  for (const r of result) {
    assert.ok(r.maxError <= 1, JSON.stringify(r));
    assert.equal(r.images, 1);
    assert.equal(r.revision, 17);
    assert.deepEqual(r.bounds, { x: 1.2, y: 0.2, w: 0.1, h: 0.05 });
  }
  console.log(result);
} finally {
  await browser.close();
}
