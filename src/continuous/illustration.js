import { anchor, contains, bounds, projectToMap } from "./geometry.js";
import { riverFeatures, riverVisible } from "./rivers.js";
export const layerNames = [
  "Paper",
  "Relief",
  "Coasts",
  "Landforms",
  "Vegetation",
  "Waterways",
  "Countries",
  "Settlements",
  "Routes",
  "Districts and buildings",
  "Labels",
  "Decoration",
  "Annotations",
];
export function newIllustration(scope, extent, focus = "") {
  return {
    version: 1,
    id: `medieval-${scope}${focus ? "-" + focus : ""}`,
    scope,
    focus,
    title:
      scope === "world"
        ? "A World of Histories"
        : `${scope[0].toUpperCase() + scope.slice(1)} atlas`,
    style: "medieval",
    bounds: [extent.x, extent.y, extent.w, extent.h],
    layers: layerNames.map((name, order) => ({
      id: name.toLowerCase().replaceAll(" ", "-"),
      name,
      order,
      visible: true,
      locked: order < 6,
      opacity: 1,
    })),
    objects: {},
  };
}
const countryKinds = new Set(["country", "state", "kingdom", "empire"]);
const cityKinds = new Set(["capital", "city", "town", "village"]);
const hash = (s) => {
  let n = 2166136261;
  for (const c of s) n = Math.imul(n ^ c.charCodeAt(0), 16777619);
  return (n >>> 0) / 4294967295;
};
export function scopeEntities(entities, state, config) {
  const all = entities
      .filter((e) => !e.deleted)
      .map((e) => ({
        ...e,
        points: e.points.map((p) => projectToMap(p, state)),
      })),
    focus = all.find((e) => e.id === config.focus),
    whole = ["world", "continent"].includes(config.scope),
    extent = {
      x: config.bounds[0],
      y: config.bounds[1],
      w: config.bounds[2],
      h: config.bounds[3],
    };
  return all.filter((e) => {
    const b = bounds(e.points);
    if (
      b.x > extent.x + extent.w ||
      b.x + b.w < extent.x ||
      b.y > extent.y + extent.h ||
      b.y + b.h < extent.y
    )
      return false;
    if (whole) return countryKinds.has(e.kind) || e.kind === "capital";
    if (
      config.scope === "country" &&
      ["house", "building", "district", "lot", "street"].includes(e.kind)
    )
      return false;
    if (
      focus?.geometry === "polygon" &&
      cityKinds.has(e.kind) &&
      e.id !== focus.id
    )
      return (
        contains(anchor(e.points, e.geometry), focus.points) ||
        e.parentId === focus.id
      );
    return true;
  });
}
export function buildIllustration(config, entities, rivers, mesh, state) {
  const [x, y, w, h] = config.bounds,
    pixel = w / 1200,
    objects = [],
    sources = scopeEntities(entities, state, config),
    labels = [],
    ink = "#503d2a";
  const push = (o) => {
    const saved = config.objects[o.id];
    objects.push({
      ...o,
      ...saved,
      points: saved?.points?.length ? saved.points : o.points,
      position: saved?.position || o.position,
      text: saved?.text ?? o.text,
    });
  };
  for (const e of sources) {
    let layer = countryKinds.has(e.kind)
      ? "countries"
      : cityKinds.has(e.kind)
        ? "settlements"
        : ["road", "path", "bridge", "border"].includes(e.kind)
          ? "routes"
          : "districts-and-buildings";
    push({
      id: e.id,
      source: e.id,
      kind:
        e.geometry === "point" ||
        (cityKinds.has(e.kind) && config.scope !== "city")
          ? "symbol"
          : e.geometry,
      geometry: e.geometry,
      layer,
      points: e.points,
      position: anchor(e.points, e.geometry),
      symbol: e.kind,
      color: ink,
      dash: e.kind === "border" ? "4 3" : "",
    });
    if (
      e.name &&
      !["house", "building", "road", "district", "lot", "street"].includes(
        e.kind,
      )
    )
      labels.push({
        id: "label-" + e.id,
        source: e.id,
        kind: "text",
        layer: "labels",
        text: e.name,
        position: anchor(e.points, e.geometry),
        country: countryKinds.has(e.kind),
        polygon: e.geometry === "polygon" ? e.points : null,
        priority: countryKinds.has(e.kind) ? 0 : e.kind === "capital" ? 1 : 3,
        color: ink,
      });
  }
  for (const f of riverFeatures(rivers)) {
    if (!riverVisible(f.river, config.scope)) continue;
    push({
      id: f.id.replace(":", "-"),
      source: f.sourceId,
      kind: "line",
      geometry: "line",
      layer: "waterways",
      points: f.points,
      color: "#426e7c",
      river: f.river,
    });
    if (f.river.name)
      labels.push({
        id: "label-" + f.id.replace(":", "-"),
        source: f.sourceId,
        kind: "text",
        layer: "labels",
        text: f.river.name,
        position: anchor(f.points, "line"),
        priority: 2,
        color: "#426e7c",
      });
  }
  if (mesh) {
    const [west, south, east, north] = mesh.bounds;
    const pt = (i) => [
      (west + ((east - west) * (i % mesh.columns)) / (mesh.columns - 1)) /
        Math.PI +
        1,
      0.5 -
        (north -
          ((north - south) * Math.floor(i / mesh.columns)) / (mesh.rows - 1)) /
          Math.PI,
    ];
    for (let j = 0; j + 1 < mesh.rows; j++)
      for (let i = 0; i + 1 < mesh.columns; i++) {
        const a = j * mesh.columns + i,
          ids = [a, a + 1, a + 1 + mesh.columns, a + mesh.columns],
          edges = [];
        for (let k = 0; k < 4; k++) {
          const u = ids[k],
            v = ids[(k + 1) % 4];
          if (mesh.wet[u] !== mesh.wet[v]) {
            const p = pt(u),
              q = pt(v),
              ratio =
                Math.abs(mesh.bed[u]) /
                (Math.abs(mesh.bed[u]) + Math.abs(mesh.bed[v]) || 1);
            edges.push([
              p[0] + (q[0] - p[0]) * ratio,
              p[1] + (q[1] - p[1]) * ratio,
            ]);
          }
        }
        for (let k = 0; k + 1 < edges.length; k += 2)
          push({
            id: `coast-${a}-${k}`,
            kind: "line",
            geometry: "line",
            layer: "coasts",
            points: [edges[k], edges[k + 1]],
            color: ink,
          });
      }
    if (config.style === "medieval")
      for (let i = 0; i < mesh.bed.length; i++) {
        if (mesh.wet[i]) continue;
        const p = pt(i),
          id = `relief-${Math.round(p[0] * 1e7)}-${Math.round(p[1] * 1e7)}`;
        if (hash(id) > 0.07) continue;
        const biome = mesh.biomes?.[i] || "",
          symbol =
            mesh.bed[i] > 1400
              ? "mountain"
              : [
                    "forest",
                    "rainforest",
                    "woodland",
                    "tropical-woodland",
                    "temperate-rainforest",
                    "taiga",
                  ].includes(biome)
                ? "tree"
                : biome === "wetland"
                  ? "marsh"
                  : null;
        if (symbol)
          push({
            id,
            kind: "symbol",
            layer: symbol === "tree" ? "vegetation" : "landforms",
            position: p,
            symbol,
            color: ink,
            size: pixel * (symbol === "mountain" ? 8 : 5),
          });
      }
  }
  // Deterministic priority-first placement, with full text-box containment.
  const occupied = [];
  for (const label of labels.sort(
    (a, b) => a.priority - b.priority || a.id.localeCompare(b.id),
  )) {
    const override = config.objects[label.id],
      size = label.country ? 15 : 11,
      ww = Math.max(12, label.text.length * size * 0.57) * pixel,
      hh = size * 1.25 * pixel;
    const origin = label.position;
    const offsets = label.country
      ? [
          [0, 0],
          [-ww * 0.2, 0],
          [ww * 0.2, 0],
          [0, -hh],
          [0, hh],
        ]
      : [
          [ww / 2 + 6 * pixel, -hh / 2],
          [ww / 2 + 6 * pixel, hh / 2],
          [-ww / 2 - 6 * pixel, -hh / 2],
          [0, -hh],
        ];
    let chosen = null;
    for (const [dx, dy] of offsets) {
      const p = [origin[0] + dx, origin[1] + dy],
        box = { x: p[0] - ww / 2, y: p[1] - hh / 2, w: ww, h: hh };
      if (box.x < x || box.y < y || box.x + ww > x + w || box.y + hh > y + h)
        continue;
      if (
        label.country &&
        label.polygon &&
        ![
          [box.x, box.y],
          [box.x + ww, box.y],
          [box.x, box.y + hh],
          [box.x + ww, box.y + hh],
          p,
        ].every((q) => contains(q, label.polygon))
      )
        continue;
      if (
        occupied.some(
          (b) =>
            box.x < b.x + b.w &&
            box.x + ww > b.x &&
            box.y < b.y + b.h &&
            box.y + hh > b.y,
        )
      )
        continue;
      chosen = { p, box };
      break;
    }
    if (override?.position)
      chosen = {
        p: override.position,
        box: {
          x: override.position[0] - ww / 2,
          y: override.position[1] - hh / 2,
          w: ww,
          h: hh,
        },
      };
    if (chosen) {
      occupied.push(chosen.box);
      push({ ...label, position: chosen.p, size: pixel * size });
    } else {
      push({
        ...label,
        position: origin,
        size: pixel * size,
        hidden: true,
        placementWarning:
          "No collision-free label position fits; place it manually.",
      });
    }
  }
  for (const [id, o] of Object.entries(config.objects))
    if (!objects.some((e) => e.id === id) && !o.source) objects.push(o);
  return objects.sort(
    (a, b) =>
      (config.layers.find((l) => l.id === a.layer)?.order || 0) -
      (config.layers.find((l) => l.id === b.layer)?.order || 0),
  );
}
