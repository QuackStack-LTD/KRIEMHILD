import { useEffect, useRef, useState } from "react";
import { visibleTiles } from "./camera.js";
export default function TileSurface({
  session,
  revision,
  invalidations = [],
  style,
  camera,
  width,
  onLoading,
}) {
  const cache = useRef(new Map()),
    [images, setImages] = useState([]);
  useEffect(
    () => () => {
      for (const tile of cache.current.values()) URL.revokeObjectURL(tile.url);
      cache.current.clear();
    },
    [session],
  );
  useEffect(() => {
    if (!session) return;
    const abort = new AbortController();
    let active = true;
    const wanted = visibleTiles(
      camera,
      width,
      Math.min(2, devicePixelRatio || 1),
    );
    const queue = [
      { z: 0, x: 0, y: 0, span: 1, size: 256 },
      { z: 0, x: 1, y: 0, span: 1, size: 256 },
      ...wanted,
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
      `${session}/${stamp(t)}/${style}/${t.z}/${t.x}/${t.y}/${t.size}`;
    const publish = () => {
      if (!active) return;
      setImages(
        [...cache.current.values()]
          .filter(
            (t) =>
              t.session === session &&
              t.style === style &&
              t.stamp === stamp(t),
          )
          .sort((a, b) => a.z - b.z),
      );
    };
    let remaining = queue.filter((t) => !cache.current.has(key(t))).length;
    onLoading?.(remaining);
    const worker = async () => {
      while (queue.length && active) {
        const t = queue.shift(),
          k = key(t);
        if (cache.current.has(k)) continue;
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
          if (e.name !== "AbortError" && active) onLoading?.(-1);
        } finally {
          if (active) onLoading?.(--remaining);
        }
      }
    };
    publish();
    Promise.all([worker(), worker(), worker()]);
    return () => {
      active = false;
      abort.abort();
    };
  }, [
    session,
    revision,
    invalidations,
    style,
    camera.x,
    camera.y,
    camera.w,
    camera.h,
    width,
  ]);
  return (
    <g aria-label="Continuous terrain tiles">
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
