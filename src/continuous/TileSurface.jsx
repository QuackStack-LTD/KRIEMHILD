import { useEffect, useId, useRef, useState } from "react";
import { visibleTiles, visibleViewport } from "./camera.js";
import { requestTile } from "./tile-request.js";
import { ProjectedRaster, useMapProjection } from "./MapProjection.js";
const sharedCache = new Map();
const activeTiles = new Map();
function trimCache() {
  const pinned = new Set(
    [...activeTiles.values()].flatMap((keys) => [...keys]),
  );
  // Bound the reusable cache, but never evict the current viewport's working
  // set. A large/DPR-scaled viewport can legitimately contain over 192 tiles.
  for (const [key, entry] of sharedCache) {
    if (sharedCache.size <= Math.max(192, pinned.size)) break;
    if (pinned.has(key)) continue;
    URL.revokeObjectURL(entry.url);
    sharedCache.delete(key);
  }
}
const noInvalidations = [];
function recordTile(event) {
  const log = (window.__terrainTileLog ||= []);
  log.push(event);
  if (log.length > 1000) log.splice(0, log.length - 1000);
}
export default function TileSurface({
  session,
  identity = session,
  revision,
  invalidations = noInvalidations,
  style,
  camera,
  width,
  height,
  onLoading,
  enabled = true,
  debug = typeof location !== "undefined" &&
    new URLSearchParams(location.search).has("tileDebug"),
}) {
  const debugId = useId();
  const projection = useMapProjection();
  const owner = useRef(Symbol("tile surface"));
  const cache = useRef(sharedCache),
    [images, setImages] = useState([]);
  const lineage = useRef({
    session,
    identity,
    revision,
    base: revision,
    invalidations,
  });
  const previous = lineage.current;
  if (
    session !== previous.session ||
    identity !== previous.identity ||
    revision !== previous.revision ||
    invalidations !== previous.invalidations
  ) {
    // Local, contiguous dirty rectangles prove that unaffected cached content
    // is still valid. A reload, remote edit or missing revision does not.
    const known =
      session === previous.session &&
      identity === previous.identity &&
      revision >= previous.revision &&
      previous.invalidations.every((b, i) => invalidations[i] === b) &&
      new Set(
        invalidations
          .filter(
            (b) => b.revision > previous.revision && b.revision <= revision,
          )
          .map((b) => b.revision),
      ).size ===
        revision - previous.revision;
    lineage.current = {
      session,
      identity,
      revision,
      base: known ? previous.base : revision,
      invalidations,
    };
  }
  const baseRevision = lineage.current.base;
  const stamp = (t) =>
    invalidations.reduce((v, b) => {
      const halo = t.span / t.size;
      return t.x * t.span - halo <= b.x + b.w &&
        (t.x + 1) * t.span + halo >= b.x &&
        t.y * t.span - halo <= b.y + b.h &&
        (t.y + 1) * t.span + halo >= b.y
        ? Math.max(v, b.revision)
        : v;
    }, 0);
  const wanted = visibleTiles(
    camera,
    width,
    Math.min(3, devicePixelRatio || 1),
    height,
  );
  const view = visibleViewport(camera, width, height);
  const signature = wanted
    .map((t) => `${t.z}/${t.x}/${t.y}/${t.size}`)
    .join(";");
  useEffect(() => {
    if (!session || !enabled) {
      onLoading?.(0);
      return;
    }
    const abort = new AbortController();
    let active = true;
    const ordered = [...wanted].sort(
      (a, b) =>
        Math.hypot(
          (a.x + 0.5) * a.span - camera.x - camera.w / 2,
          (a.y + 0.5) * a.span - camera.y - camera.h / 2,
        ) -
        Math.hypot(
          (b.x + 0.5) * b.span - camera.x - camera.w / 2,
          (b.y + 0.5) * b.span - camera.y - camera.h / 2,
        ),
    );
    const parents = ordered
      .filter((t) => t.z > 0)
      .map((t) => ({
        z: t.z - 1,
        x: Math.floor(t.x / 2),
        y: Math.floor(t.y / 2),
        span: t.span * 2,
        size: 256,
      }));
    const queue = [
      ...new Map(
        [...parents, ...ordered].map((t) => [
          `${t.z}/${t.x}/${t.y}/${t.size}`,
          t,
        ]),
      ).values(),
    ];
    const key = (t) =>
      `${identity}/${baseRevision}/${stamp(t)}/${style}/${t.z}/${t.x}/${t.y}/${t.size}`;
    activeTiles.set(owner.current, new Set(queue.map(key)));
    const publish = () => {
      if (!active) return;
      setImages(
        [...cache.current.values()]
          .filter(
            (t) =>
              t.identity === identity &&
              t.baseRevision === baseRevision &&
              t.style === style &&
              t.stamp === stamp(t) &&
              t.z <= wanted[0]?.z &&
              t.x * t.span < view.x + view.w + t.span &&
              (t.x + 1) * t.span > view.x - t.span &&
              t.y * t.span < view.y + view.h + t.span &&
              (t.y + 1) * t.span > view.y - t.span,
          )
          .sort((a, b) => a.z - b.z),
      );
    };
    let remaining = queue.filter((t) => !cache.current.has(key(t))).length;
    onLoading?.(remaining);
    let failed = false;
    const worker = async () => {
      while (queue.length && active) {
        const t = queue.shift(),
          k = key(t);
        if (cache.current.has(k)) {
          const t = cache.current.get(k);
          cache.current.delete(k);
          cache.current.set(k, t);
          continue;
        }
        try {
          const blob = await requestTile(
            `/api/sessions/${session}/terrain/tiles/${t.z}/${t.x}/${t.y}?style=${style}&revision=${revision}&size=${t.size}&strict=1`,
            revision,
            abort.signal,
            { record: debug ? recordTile : undefined },
          );
          if (!active) break;
          cache.current.set(k, {
            ...t,
            key: k,
            session,
            identity,
            baseRevision,
            revision,
            stamp: stamp(t),
            style,
            url: URL.createObjectURL(blob),
          });
          if (debug)
            recordTile({
              event: "accepted",
              key: k,
              revision,
              time: performance.now(),
            });
          trimCache();
          publish();
        } catch (e) {
          if (e.name !== "AbortError" && active) failed = true;
        } finally {
          if (active) {
            remaining--;
            onLoading?.(failed ? -1 : remaining);
          }
        }
      }
    };
    publish();
    const timer = setTimeout(
      () =>
        Promise.all([worker(), worker(), worker()]).then(() => {
          if (active) onLoading?.(failed ? -1 : 0);
        }),
      80,
    );
    return () => {
      active = false;
      clearTimeout(timer);
      abort.abort();
      activeTiles.delete(owner.current);
    };
  }, [
    session,
    revision,
    invalidations,
    style,
    signature,
    identity,
    enabled,
    debug,
  ]);
  return (
    <g aria-label="Continuous terrain tiles" data-terrain-style={style}>
      {images
        .filter(
          (t) =>
            t.identity === identity &&
            t.baseRevision === baseRevision &&
            t.style === style &&
            t.stamp === stamp(t),
        )
        .map((t) => (
          <ProjectedRaster
            key={t.key}
            frame={{
              bounds: {
                x: t.x * t.span,
                y: t.y * t.span,
                w: t.span,
                h: t.span,
              },
              pixels: { width: t.size, height: t.size },
            }}
            image={{ url: t.url }}
            data-tile={`${t.z}/${t.x}/${t.y}`}
            data-revision={t.revision}
          />
        ))}
      {debug && style !== "hillshade" && (
        <g pointerEvents="none" aria-label="Tile diagnostics">
          {wanted.map((tile) => {
            const bounds = {
              x: tile.x * tile.span,
              y: tile.y * tile.span,
              w: tile.span,
              h: tile.span,
            };
            const box = projection ? projection.box(bounds) : bounds;
            return (
              <g key={`${debugId}/${tile.z}/${tile.x}/${tile.y}`}>
                <rect
                  x={box.x}
                  y={box.y}
                  width={box.w}
                  height={box.h}
                  fill="none"
                  stroke="#ff4fa3"
                  strokeWidth={projection ? 1 : view.w / width}
                />
                <text
                  x={box.x + box.w * 0.02}
                  y={box.y + box.h * 0.08}
                  fontSize={box.w * 0.055}
                  fill="#fff"
                  stroke="#111"
                  strokeWidth={box.w * 0.003}
                  paintOrder="stroke"
                >
                  {`${tile.z}/${tile.x}/${tile.y} r${revision}`}
                </text>
                <text
                  x={box.x + box.w * 0.02}
                  y={box.y + box.h * 0.15}
                  fontSize={box.w * 0.035}
                  fill="#fff"
                  stroke="#111"
                  strokeWidth={box.w * 0.002}
                  paintOrder="stroke"
                >
                  {`[${(bounds.x * 180 - 180).toFixed(3)},${(90 - (bounds.y + bounds.h) * 180).toFixed(3)}]–[${((bounds.x + bounds.w) * 180 - 180).toFixed(3)},${(90 - bounds.y * 180).toFixed(3)}]`}
                </text>
              </g>
            );
          })}
        </g>
      )}
    </g>
  );
}
