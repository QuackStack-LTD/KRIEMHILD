// Bundle into dist with esbuild; run in the shared browser against a disposable
// server. Uses production rendering, no synthetic tile bytes or altered camera.
import {
  TerrainPyramid,
  viewportRequest,
  renderViewport,
} from "../src/continuous/terrain-pyramid.js";
import { previewRequest } from "../src/continuous/preview-quality.js";
import { compositePreview } from "../src/continuous/preview-composite.js";

window.qualityAudit = { runs: [], status: "idle" };
window.runQualityAudit = async ({
  session,
  revision = 0,
  dprs = [1, 2, 3],
  repeats = 1,
  widths = [2, 0.25, 0.03, 0.003, 0.00008],
  presets = ["reference", "performance", "balanced", "high"],
  styles = ["natural", "hillshade"],
  width = 1200,
  height = 800,
}) => {
  const audit = window.qualityAudit;
  audit.status = "running";
  performance.setResourceTimingBufferSize(30000);
  try {
    for (let repeat = 0; repeat < repeats; repeat++)
      for (const dpr of dprs)
        for (const w of widths)
          for (const preset of presets) {
            // A fresh encoded-node cache for each cold-browser case, then repeat
            // with the exact same request and cache. Backend state is reported
            // from actual response headers, never assumed cold after navigation.
            const pyramid = new TerrainPyramid();
            const camera = {
              x: w === 2 ? 0 : 1.3 - w / 2,
              y: w === 2 ? 0 : 0.22 - w / 4,
              w,
              h: w / 2,
            };
            const options = {
              session,
              identity: session,
              revision,
              camera,
              width,
              height,
              dpr,
              styles,
            };
            const request =
              preset === "reference"
                ? viewportRequest(options)
                : previewRequest(options, { preset });
            for (const cache of ["cold-browser", "warm-browser"]) {
              const events = [],
                started = performance.now();
              await renderViewport(
                pyramid,
                request,
                new AbortController().signal,
                {
                  record: (e) => events.push(e),
                  onFrame: async (frame) => {
                    const published =
                      preset === "reference"
                        ? frame
                        : await compositePreview(
                            frame,
                            0.55,
                            new AbortController().signal,
                          );
                    // Isolated renderer publication decode, with explicit bitmap
                    // release. SVG Image.decode/paint and compositor memory are
                    // measured separately in the actual Studio browser cases.
                    const images = await Promise.all(
                      published.images.map(async (i) =>
                        createImageBitmap(i.blob),
                      ),
                    );
                    images.forEach((i) => i.close());
                    return {
                      layers: published.images.length,
                      bytes: published.images.reduce(
                        (s, i) => s + i.blob.size,
                        0,
                      ),
                    };
                  },
                },
              );
              const frames = events.filter((e) => e.event === "frame"),
                responses = events.filter((e) => e.event === "complete");
              audit.runs.push({
                repeat,
                dpr,
                preset,
                cache,
                actualDeviceDPR: devicePixelRatio,
                camera,
                request,
                frames,
                elapsed: performance.now() - started,
                responses,
                hits: pyramid.stats.hits,
                misses: pyramid.stats.misses,
                obsolete: events.filter((e) => e.event === "obsolete").length,
                browserHeap: performance.memory
                  ? {
                      used: performance.memory.usedJSHeapSize,
                      total: performance.memory.totalJSHeapSize,
                    }
                  : null,
              });
              audit.latest = {
                repeat,
                dpr,
                w,
                preset,
                cache,
                elapsed: audit.runs.at(-1).elapsed,
              };
            }
          }
    audit.status = "complete";
  } catch (e) {
    audit.status = "failed";
    audit.error = e.stack;
  }
};
