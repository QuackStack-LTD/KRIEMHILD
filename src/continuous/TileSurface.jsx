import { useEffect, useRef, useState } from "react";
import { visibleTiles } from "./camera.js";
const sharedCache = new Map();
export default function TileSurface({
  session,
  identity = session,
  revision,
  invalidations = [],
  style,
  camera,
  width,
  onLoading,
  enabled = true,
}) {
  const cache = useRef(sharedCache),
    [images, setImages] = useState([]);
  const baseRevision = useRef(revision);
  const wanted = visibleTiles(
    camera,
    width,
    Math.min(3, devicePixelRatio || 1),
  );
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
    const stamp = (t) =>
      invalidations.reduce(
        (v, b) =>
          t.x * t.span <= b.x + b.w &&
          (t.x + 1) * t.span >= b.x &&
          t.y * t.span <= b.y + b.h &&
          (t.y + 1) * t.span >= b.y
            ? Math.max(v, b.revision)
            : v,
        0,
      );
    const key = (t) =>
      `${identity}/${baseRevision.current}/${stamp(t)}/${style}/${t.z}/${t.x}/${t.y}/${t.size}`;
    const publish = () => {
      if (!active) return;
      setImages(
        [...cache.current.values()]
          .filter(
            (t) =>
              t.identity === identity &&
              t.baseRevision === baseRevision.current &&
              t.style === style &&
              t.stamp === stamp(t) &&
              t.z <= wanted[0]?.z &&
              t.x * t.span < camera.x + camera.w + t.span &&
              (t.x + 1) * t.span > camera.x - t.span &&
              t.y * t.span < camera.y + camera.h + t.span &&
              (t.y + 1) * t.span > camera.y - t.span,
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
          const r = await fetch(
            `/api/sessions/${session}/terrain/tiles/${t.z}/${t.x}/${t.y}?style=${style}&revision=${revision}&size=${t.size}`,
            { signal: abort.signal },
          );
          if (!r.ok) throw Error("Terrain tile unavailable");
          const blob = await r.blob();
          if (!active) break;
          cache.current.set(k, {
            ...t,
            key: k,
            session,
            identity,
            baseRevision: baseRevision.current,
            revision,
            stamp: stamp(t),
            style,
            url: URL.createObjectURL(blob),
          });
          while (cache.current.size > 192) {
            const [old, entry] = cache.current.entries().next().value;
            URL.revokeObjectURL(entry.url);
            cache.current.delete(old);
          }
          publish();
        } catch (e) {
          if (e.name !== "AbortError" && active) failed = true;
        } finally {
          if (active) onLoading?.(failed ? -1 : --remaining);
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
    };
  }, [session, revision, invalidations, style, signature, identity, enabled]);
  return (
    <g aria-label="Continuous terrain tiles" data-terrain-style={style}>
      {images.map((t) => (
        <image
          key={t.key}
          href={t.url}
          x={t.x * t.span}
          y={t.y * t.span}
          width={t.span}
          height={t.span}
          preserveAspectRatio="none"
        />
      ))}
    </g>
  );
}
