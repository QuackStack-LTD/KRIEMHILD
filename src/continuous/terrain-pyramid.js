import { visibleTiles, visibleViewport } from "./camera.js";
import { requestTile } from "./tile-request.js";

// Lazy, immutable raster representations of the authoritative terrain sampler.
// Levels share world coordinates and sample centers with the server's physical
// halo pyramid. These nodes are never mounted as browser images.
export function viewportRequest({ camera, width, height, dpr = 1, ...source }) {
  const bounds = visibleViewport(camera, width, height);
  const pixels = {
    width: Math.max(1, Math.round(width * dpr)),
    height: Math.max(1, Math.round(height * dpr)),
  };
  const tiles = visibleTiles(camera, width, dpr, height);
  return {
    ...source,
    bounds,
    pixels,
    z: tiles[0]?.z ?? 0,
    size: tiles[0]?.size ?? 256,
  };
}

export function pyramidNodes(request, z = request.z) {
  const { bounds: b, size } = request,
    span = 2 ** -z,
    nodes = [];
  for (
    let y = Math.max(0, Math.floor(b.y / span));
    y < Math.min(2 ** z, Math.ceil((b.y + b.h) / span));
    y++
  )
    for (
      let x = Math.max(0, Math.floor(b.x / span));
      x < Math.min(2 ** (z + 1), Math.ceil((b.x + b.w) / span));
      x++
    )
      nodes.push({ z, x, y, size, span });
  return nodes;
}

export class TerrainPyramid {
  constructor({ budget = 64 * 1024 * 1024, fetchTile = requestTile } = {}) {
    this.budget = budget;
    this.fetchTile = fetchTile;
    this.cache = new Map();
    this.bytes = 0;
    this.lineage = new Map();
    this.stats = { hits: 0, misses: 0, evictions: 0 };
  }
  prepare(request) {
    const { identity, revision, invalidations = [] } = request;
    const prior = this.lineage.get(identity);
    const known =
      prior &&
      revision >= prior.revision &&
      prior.invalidations.every((b, i) => invalidations[i] === b) &&
      new Set(
        invalidations
          .filter((b) => b.revision > prior.revision && b.revision <= revision)
          .map((b) => b.revision),
      ).size ===
        revision - prior.revision;
    const base = known ? prior.base : revision;
    this.lineage.set(identity, { revision, invalidations, base });
    // Source metadata is tiny, but must not retain every visited session forever.
    if (this.lineage.size > 32)
      this.lineage.delete(this.lineage.keys().next().value);
    return { ...request, base };
  }
  key(r, n, style) {
    const halo = n.span / n.size;
    const stamp = (r.invalidations || []).reduce(
      (v, b) =>
        b.revision <= r.revision &&
        n.x * n.span - halo <= b.x + b.w &&
        (n.x + 1) * n.span + halo >= b.x &&
        n.y * n.span - halo <= b.y + b.h &&
        (n.y + 1) * n.span + halo >= b.y
          ? Math.max(v, b.revision)
          : v,
      0,
    );
    return `${r.identity}/${r.base}/${stamp}/${style}/${n.z}/${n.x}/${n.y}/${n.size}`;
  }
  async get(r, n, style, signal, record) {
    signal.throwIfAborted();
    const key = this.key(r, n, style),
      hit = this.cache.get(key);
    if (hit) {
      this.cache.delete(key);
      this.cache.set(key, hit);
      this.stats.hits++;
      return hit;
    }
    this.stats.misses++;
    const blob = await this.fetchTile(
      `/api/sessions/${r.session}/terrain/tiles/${n.z}/${n.x}/${n.y}?style=${style}&revision=${r.revision}&size=${n.size}&strict=1`,
      r.revision,
      signal,
      { record },
    );
    signal.throwIfAborted();
    if (blob.size <= this.budget) {
      const prior = this.cache.get(key);
      if (prior) this.bytes -= prior.size;
      this.cache.set(key, blob);
      this.bytes += blob.size;
      while (this.bytes > this.budget) {
        const first = this.cache.keys().next().value;
        this.bytes -= this.cache.get(first).size;
        this.cache.delete(first);
        this.stats.evictions++;
      }
    }
    return blob;
  }
  discard(r, n, style, blob) {
    const key = this.key(r, n, style);
    if (this.cache.get(key) === blob) {
      this.cache.delete(key);
      this.bytes -= blob.size;
    }
  }
}

const canvas = (width, height) => new OffscreenCanvas(width, height);

// Stitch integer-addressed pyramid samples first; resample the complete atlas
// once. Independent fractional drawImage rectangles would create new seams.
export async function renderViewport(
  pyramid,
  request,
  signal,
  {
    onFrame,
    record = () => {},
    createCanvas = canvas,
    decode = (blob) => createImageBitmap(blob),
  } = {},
) {
  const r = pyramid.prepare(request),
    started = performance.now();
  if (r.pixels.width * r.pixels.height > 32 * 1024 * 1024)
    throw Error(
      "Viewport exceeds the 32-megapixel render budget. Reduce display pixel density or window size.",
    );
  async function level(z, phase, overview = false) {
    const levelRequest = overview
      ? { ...r, bounds: { x: 0, y: 0, w: 2, h: 1 } }
      : r;
    signal.throwIfAborted();
    // A coarse preview needs coarse output pixels too. Upscaling it into a full
    // DPR raster would retain a second full-size decoded frame during refinement.
    // The final transaction always uses the complete requested output density.
    const scale = phase === "coarse" ? 2 ** (r.z - z) : 1;
    const pixels = overview
      ? { width: 512, height: 256 }
      : {
          width: Math.max(1, Math.round(r.pixels.width / scale)),
          height: Math.max(1, Math.round(r.pixels.height / scale)),
        };
    const nodes = pyramidNodes(levelRequest, z),
      span = 2 ** -z;
    const minX = Math.min(...nodes.map((n) => n.x)),
      maxX = Math.max(...nodes.map((n) => n.x));
    const minY = Math.min(...nodes.map((n) => n.y)),
      maxY = Math.max(...nodes.map((n) => n.y));
    const atlasWidth = nodes.length ? (maxX - minX + 1) * r.size : 1;
    const atlasHeight = nodes.length ? (maxY - minY + 1) * r.size : 1;
    if (atlasWidth * atlasHeight > 64 * 1024 * 1024)
      throw Error(
        "Terrain atlas exceeds the 64-megapixel render budget. Reduce display pixel density or window size.",
      );
    const images = [];
    let completed = 0;
    const total = nodes.length * r.styles.length;
    record({
      event: "progress",
      generation: r.generation,
      phase,
      z,
      completed,
      total,
    });
    const timings = {
      fetch: 0,
      decode: 0,
      assemble: 0,
      resample: 0,
      encode: 0,
      publish: 0,
    };
    for (const style of r.styles) {
      const atlas = createCanvas(atlasWidth, atlasHeight),
        // These render targets are read back into PNGs, not displayed canvases.
        // CPU backing avoids retained GPU tile/atlas copies (measured at DPR 3).
        ctx = atlas.getContext("2d", { willReadFrequently: true });
      let output;
      try {
        let next = 0;
        const worker = async () => {
          while (next < nodes.length) {
            const n = nodes[next++];
            signal.throwIfAborted();
            let startedNode = performance.now();
            const blob = await pyramid.get(r, n, style, signal, (event) =>
              record({ ...event, generation: r.generation, phase, style }),
            );
            // The backend retains eight physical halo nodes. Fetch all required
            // styles of this node while its physical samples are still resident,
            // rather than finishing the whole planet in one style first. Only
            // bounded encoded blobs are retained; no extra decoded atlas exists.
            if (style === r.styles[0]) {
              for (const related of r.styles.slice(1)) {
                await pyramid.get(r, n, related, signal, (event) =>
                  record({
                    ...event,
                    generation: r.generation,
                    phase,
                    style: related,
                  }),
                );
              }
            }
            timings.fetch += performance.now() - startedNode;
            startedNode = performance.now();
            let bitmap;
            try {
              bitmap = await decode(blob);
            } catch (error) {
              // Failed bytes must not poison every subsequent manual retry.
              // Discard only this exact cached object, never a newer replacement.
              pyramid.discard(r, n, style, blob);
              throw error;
            }
            timings.decode += performance.now() - startedNode;
            try {
              signal.throwIfAborted();
              startedNode = performance.now();
              ctx.drawImage(
                bitmap,
                (n.x - minX) * r.size,
                (n.y - minY) * r.size,
              );
              record({
                event: "progress",
                generation: r.generation,
                phase,
                z,
                completed: ++completed,
                total,
              });
              timings.assemble += performance.now() - startedNode;
            } finally {
              bitmap.close();
            }
          }
        };
        // One shared three-worker queue, including every style. All work is joined
        // before a failed transaction releases its render targets.
        const results = await Promise.allSettled([
          worker(),
          worker(),
          worker(),
        ]);
        const failure = results.find((v) => v.status === "rejected");
        if (failure) {
          atlas.width = atlas.height = 1;
          throw failure.reason;
        }
        signal.throwIfAborted();
        output = createCanvas(pixels.width, pixels.height);
        const dest = output.getContext("2d", { willReadFrequently: true });
        let startedStage = performance.now();
        if (nodes.length)
          dest.drawImage(
            atlas,
            ((minX * span - levelRequest.bounds.x) / levelRequest.bounds.w) *
              pixels.width,
            ((minY * span - levelRequest.bounds.y) / levelRequest.bounds.h) *
              pixels.height,
            (((atlasWidth / r.size) * span) / levelRequest.bounds.w) *
              pixels.width,
            (((atlasHeight / r.size) * span) / levelRequest.bounds.h) *
              pixels.height,
          );
        timings.resample += performance.now() - startedStage;
        startedStage = performance.now();
        const blob = await output.convertToBlob({ type: "image/png" });
        timings.encode += performance.now() - startedStage;
        signal.throwIfAborted();
        images.push({ style, blob });
      } finally {
        atlas.width = atlas.height = 1;
        if (output) output.width = output.height = 1;
      }
    }
    const frame = {
      bounds: levelRequest.bounds,
      revision: r.revision,
      identity: r.identity,
      generation: r.generation,
      z,
      phase,
      pixels,
      images,
    };
    signal.throwIfAborted();
    const publishStart = performance.now();
    const publication = await onFrame(frame);
    signal.throwIfAborted();
    timings.publish = performance.now() - publishStart;
    record({
      event: "frame",
      generation: r.generation,
      phase,
      revision: r.revision,
      bounds: levelRequest.bounds,
      z,
      pixels,
      timings,
      nodes: nodes.length,
      duration: performance.now() - started,
      bytes: images.reduce((n, i) => n + i.blob.size, 0),
      cacheBytes: pyramid.bytes,
      workingCanvasBytes:
        4 * (atlasWidth * atlasHeight + pixels.width * pixels.height),
      decodedFrameBytes:
        4 *
        pixels.width *
        pixels.height *
        (publication?.layers ?? images.length),
      publishedBytes: publication?.bytes,
      ...pyramid.stats,
    });
  }
  // A complete fine cache needs no fallback transaction. In measured revisits,
  // coarse canvas encoding delayed already available fine detail by up to 959 ms.
  const fineCached = pyramidNodes(r).every((n) =>
    r.styles.every((style) => pyramid.cache.has(pyramid.key(r, n, style))),
  );
  if (r.z > 0 && !fineCached) {
    if (r.preview) {
      try {
        await level(0, "overview", true);
      } catch (error) {
        if (signal.aborted) throw error;
        record({
          event: "overview-failed",
          generation: r.generation,
          message: error.message,
        });
      }
    }
    try {
      await level(Math.max(0, r.z - 2), "coarse");
    } catch (error) {
      if (signal.aborted) throw error;
      record({
        event: "coarse-failed",
        generation: r.generation,
        message: error.message,
      });
    }
  }
  await level(r.z, "final");
}
