import { anchor, contains, bounds, projectToMap } from "./geometry.js";
import { riverFeatures, riverVisible } from "./rivers.js";
import { placeLabel } from "./label-placement.js";
import { landContains } from "./land-selection.js";
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
const cityKinds = new Set(["capital", "city", "town", "village", "hamlet"]);
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
    if (
      config.regions?.length &&
      !e.points.some((p) => landContains({ paths: config.regions }, p)) &&
      !landContains({ paths: config.regions }, anchor(e.points, e.geometry))
    )
      return false;
    const b = bounds(e.points);
    if (
      b.x > extent.x + extent.w ||
      b.x + b.w < extent.x ||
      b.y > extent.y + extent.h ||
      b.y + b.h < extent.y
    )
      return false;
    if (whole)
      return (
        countryKinds.has(e.kind) ||
        e.kind === "capital" ||
        e.kind === "city" ||
        (config.scope === "continent" && e.kind === "town")
      );
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
export function buildIllustration(
  config,
  entities,
  rivers,
  mesh,
  state,
  coasts = null,
) {
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
      points:
        o.source && o.kind !== "text"
          ? o.points
          : saved?.points?.length
            ? saved.points
            : o.points,
      position:
        o.source && o.kind !== "text"
          ? o.position
          : saved?.position || o.position,
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
        priority: countryKinds.has(e.kind)
          ? 0
          : e.kind === "capital"
            ? 1
            : e.kind === "city"
              ? 2
              : 3,
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
  if (coasts) {
    let id = 0;
    for (const coast of coasts) {
      let part = [];
      const emit = () => {
        if (part.length > 1) {
          const b = bounds(part);
          if (b.x <= x + w && b.x + b.w >= x && b.y <= y + h && b.y + b.h >= y)
            push({
              id: `shore-${id}`,
              kind: "line",
              geometry: "line",
              layer: "coasts",
              points: part,
              color: ink,
            });
        }
        id++;
        part = [];
      };
      for (const pt of coast) {
        const p = [pt[0] / Math.PI + 1, 0.5 - pt[1] / Math.PI];
        if (part.length && Math.abs(p[0] - part.at(-1)[0]) > 1) emit();
        part.push(p);
      }
      emit();
    }
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
    if (!coasts)
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
    const { box, ...placement } = placeLabel(
      label,
      config.bounds,
      pixel,
      occupied,
      config.objects[label.id],
    );
    occupied.push(box);
    push({ ...label, ...placement });
  }
  for (const [id, o] of Object.entries(config.objects))
    if (!objects.some((e) => e.id === id) && !o.source) objects.push(o);
  return objects.sort(
    (a, b) =>
      (config.layers.find((l) => l.id === a.layer)?.order || 0) -
      (config.layers.find((l) => l.id === b.layer)?.order || 0),
  );
}
