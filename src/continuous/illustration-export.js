import { visibleTiles } from "./camera.js";

// One bounded cache for immutable export rasters; preview quality never enters
// its identity. Visit all styles of a node before the physical tile is evicted.
export class ExportTiles {
  constructor(budget = 48 * 1024 * 1024) {
    this.budget = budget;
    this.bytes = 0;
    this.cache = new Map();
  }
  async get(key, load) {
    if (this.cache.has(key)) {
      const v = this.cache.get(key);
      this.cache.delete(key);
      this.cache.set(key, v);
      return v;
    }
    const value = await load(),
      size = value.length * 2;
    if (size <= this.budget) {
      this.cache.set(key, value);
      this.bytes += size;
      while (this.bytes > this.budget) {
        const k = this.cache.keys().next().value;
        this.bytes -= this.cache.get(k).length * 2;
        this.cache.delete(k);
      }
    }
    return value;
  }
}
const rasterCache = new ExportTiles();
const dataURL = (blob) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
export async function exportTerrain(
  svg,
  {
    camera,
    width,
    projection,
    session,
    revision,
    identity,
    onProgress = () => {},
    fetchTile = fetch,
  },
) {
  const groups = [
    ...svg.querySelectorAll('[aria-label="Continuous terrain tiles"]'),
  ];
  const styles = [...new Set(groups.map((g) => g.dataset.terrainStyle))];
  const nodes = visibleTiles(camera, width, 1),
    layers = new Map(styles.map((s) => [s, []]));
  let next = 0,
    done = 0;
  const total = nodes.length * styles.length;
  for (const g of groups) g.replaceChildren();
  async function worker() {
    while (next < nodes.length) {
      const t = nodes[next++];
      for (const style of styles) {
        const key = `${identity}/${style}/${t.z}/${t.x}/${t.y}/${t.size}`;
        const href = await rasterCache.get(key, async () => {
          const r = await fetchTile(
            `/api/sessions/${session}/terrain/tiles/${t.z}/${t.x}/${t.y}?size=${t.size}&style=${style}&strict=1&revision=${revision}`,
          );
          if (!r.ok)
            throw Error(
              "Export terrain changed or could not be sampled; retry the export.",
            );
          return dataURL(await r.blob());
        });
        const b = projection.box({
          x: t.x * t.span,
          y: t.y * t.span,
          w: t.span,
          h: t.span,
        });
        layers
          .get(style)
          .push({ href, x: b.x, y: b.y, width: b.w, height: b.h });
        onProgress({
          phase: "Rendering illustration",
          completed: ++done,
          total,
        });
      }
    }
  }
  await Promise.all([worker(), worker(), worker()]);
  for (const g of groups)
    for (const attributes of layers
      .get(g.dataset.terrainStyle)
      .sort((a, b) => a.y - b.y || a.x - b.x)) {
      const image = document.createElementNS(
        "http://www.w3.org/2000/svg",
        "image",
      );
      for (const [k, v] of Object.entries({
        ...attributes,
        preserveAspectRatio: "none",
      }))
        image.setAttribute(k, v);
      g.appendChild(image);
    }
  for (const image of svg.querySelectorAll("image"))
    if (!image.getAttribute("href")?.startsWith("data:"))
      image.setAttribute(
        "href",
        await dataURL(
          await (await fetchTile(image.getAttribute("href"))).blob(),
        ),
      );
}

export async function illustrationAssets(svg, width = 2048) {
  const vb = svg.viewBox.baseVal,
    aspect = vb.width / vb.height;
  const paint = async (scene, w, h) => {
    scene.setAttribute("width", w);
    scene.setAttribute("height", h);
    const u = URL.createObjectURL(
      new Blob([new XMLSerializer().serializeToString(scene)], {
        type: "image/svg+xml",
      }),
    );
    try {
      const image = new Image();
      image.src = u;
      await image.decode();
      const c = document.createElement("canvas");
      c.width = w;
      c.height = h;
      const ctx = c.getContext("2d");
      ctx.fillStyle = "#e9d9af";
      ctx.fillRect(0, 0, w, h);
      ctx.drawImage(image, 0, 0, w, h);
      return await new Promise((r) => c.toBlob(r, "image/png"));
    } finally {
      URL.revokeObjectURL(u);
    }
  };
  // SVG meet letterboxes the geographic extent instead of cropping or moving it.
  svg.setAttribute("preserveAspectRatio", "xMidYMid meet");
  const map = await paint(svg, width, width);
  const banner = svg.cloneNode(true);
  for (const e of banner.querySelectorAll(
    '[data-illustration-layer="decoration"]',
  ))
    e.remove();
  return {
    map,
    banner: await paint(
      banner,
      Math.min(4096, Math.round(width * 1.5)),
      Math.min(1365, Math.round(width * 0.5)),
    ),
    aspect,
  };
}
