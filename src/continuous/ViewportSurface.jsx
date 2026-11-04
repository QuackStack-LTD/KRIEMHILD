import { createContext, useContext, useEffect, useRef, useState } from "react";
import {
  TerrainPyramid,
  viewportRequest,
  renderViewport,
} from "./terrain-pyramid.js";
import { previewRequest } from "./preview-quality.js";
import { compositePreview } from "./preview-composite.js";
import { ProjectedRaster } from "./MapProjection.js";

const pyramid = new TerrainPyramid();
const FrameContext = createContext(null);

export function ViewportLayer({ style, primary = false }) {
  const frame = useContext(FrameContext);
  const image =
    frame?.images.find((i) => i.style === style) ||
    (primary && frame?.images[0]);
  if (!image)
    return (
      // Declare export layers independently of preview readiness/composition.
      // exportMap replaces these children with its own resolution and raw styles.
      <g
        aria-label="Continuous terrain tiles"
        data-terrain-style={style}
        pointerEvents="none"
      />
    );
  const underlay = frame.underlay;
  const underlayImage =
    underlay?.images.find((i) => i.style === style) ||
    (primary && underlay?.images[0]);
  const layers = underlayImage
    ? [
        { ...underlay, image: underlayImage },
        { ...frame, image },
      ]
    : [{ ...frame, image }];
  return (
    <g
      aria-label="Continuous terrain tiles"
      data-terrain-style={style}
      pointerEvents="none"
    >
      {layers.map((layer) => (
        <ProjectedRaster
          key={layer.image.url}
          frame={layer}
          image={layer.image}
          data-generation={layer.generation}
          data-revision={layer.revision}
          data-lod={layer.z}
          data-raster-style={layer.image.style}
          data-overview={layer.phase === "overview" ? "true" : undefined}
        />
      ))}
    </g>
  );
}
function record(event) {
  const log = (window.__terrainViewportLog ||= []);
  log.push({ time: performance.now(), ...event });
  if (log.length > 1000) log.splice(0, log.length - 1000);
}
const release = (frame) =>
  frame?.images.forEach((i) => URL.revokeObjectURL(i.url));

export default function ViewportSurface({
  session,
  identity,
  revision,
  invalidations,
  camera,
  width,
  height,
  enabled = true,
  retry = 0,
  onLoading,
  onError,
  styles,
  children,
  active = true,
  previewQuality,
  wrapLongitude = false,
  previewHillshade,
  detailLevel,
  onDiagnostics,
  onProgress,
}) {
  const [frame, setFrame] = useState(null),
    [overview, setOverview] = useState(null),
    generation = useRef(0),
    owned = useRef(new Set()),
    staging = useRef(new Set()),
    latest = useRef({ frame: null, overview: null }),
    counters = useRef({ cancelled: 0, outstanding: 0 });
  const options = {
    session,
    identity,
    revision,
    invalidations,
    camera,
    width,
    height,
    dpr: Math.min(3, devicePixelRatio || 1),
    styles,
    wrapLongitude,
  };
  const request = previewQuality
    ? previewRequest(options, previewQuality)
    : viewportRequest(options);
  const signature = JSON.stringify([
    identity,
    revision,
    request.bounds,
    request.pixels,
    request.z,
    request.size,
    previewQuality ? previewHillshade : undefined,
    request.styles,
    wrapLongitude,
    retry,
    enabled,
    active,
    previewQuality ? detailLevel : undefined,
  ]);
  useEffect(() => {
    if (!active) return;
    if (!enabled || !session) {
      onLoading?.(0);
      return;
    }
    const controller = new AbortController(),
      current = ++generation.current;
    let live = true;
    let completed = false;
    let entered = false;
    counters.current.outstanding++;
    const started = performance.now();
    let firstMs;
    let lastProgress = 0;
    const report = (event) => {
      if (event.event === "progress") {
        const now = performance.now();
        if (
          live &&
          (now - lastProgress > 75 ||
            event.completed === 0 ||
            event.completed === event.total)
        ) {
          onProgress?.(event);
          lastProgress = now;
        }
        return;
      }
      record(event);
      if (event.event !== "frame") return;
      firstMs ??= performance.now() - started;
      onDiagnostics?.({
        ...event,
        identity,
        detailLevel,
        quality: request.preview?.label,
        firstMs,
        finalMs:
          event.phase === "final" ? performance.now() - started : undefined,
        cancelled: counters.current.cancelled,
        outstanding: Math.max(
          0,
          counters.current.outstanding - (event.phase === "final" ? 1 : 0),
        ),
      });
    };
    onLoading?.(1);
    onProgress?.({
      generation: current,
      phase: "pending",
      completed: 0,
      total: 0,
    });
    record({
      event: "request",
      generation: current,
      ...request,
      preview: previewQuality
        ? { ...request.preview, hillshade: previewHillshade || 0 }
        : undefined,
      invalidations: undefined,
    });
    const timer = setTimeout(() => {
      entered = true;
      return renderViewport(
        pyramid,
        { ...request, generation: current },
        controller.signal,
        {
          record: report,
          onFrame: async (value) => {
            const compositeStart = performance.now();
            const displayed = previewQuality
              ? await compositePreview(
                  value,
                  previewHillshade,
                  controller.signal,
                )
              : value;
            if (displayed !== value)
              record({
                event: "preview-composite",
                generation: current,
                phase: value.phase,
                duration: performance.now() - compositeStart,
                bytes: displayed.images[0].blob.size,
                pixels: value.pixels,
                workingBytes: 12 * value.pixels.width * value.pixels.height,
                opacity: previewHillshade,
              });
            const next = {
              ...displayed,
              images: displayed.images.map((i) => {
                const url = URL.createObjectURL(i.blob);
                owned.current.add(url);
                staging.current.add(url);
                return { style: i.style, url };
              }),
            };
            try {
              // Decode every layer before one React commit, not one load event each.
              await Promise.all(
                next.images.map(async (i) => {
                  const image = new Image();
                  image.src = i.url;
                  await image.decode();
                }),
              );
              if (!live || generation.current !== current) {
                release(next);
                next.images.forEach((i) => owned.current.delete(i.url));
                return;
              }
              if (next.phase === "overview") {
                latest.current.overview = next;
                setOverview(next);
              } else {
                latest.current.frame = next;
                setFrame(next);
              }
              return {
                layers: next.images.length,
                bytes: displayed.images.reduce((n, i) => n + i.blob.size, 0),
              };
            } catch (error) {
              release(next);
              next.images.forEach((i) => owned.current.delete(i.url));
              throw error;
            } finally {
              next.images.forEach((i) => staging.current.delete(i.url));
            }
          },
        },
      )
        .then(() => {
          completed = true;
          counters.current.outstanding--;
          if (live) onLoading?.(0);
        })
        .catch((error) => {
          completed = true;
          counters.current.outstanding--;
          record({
            event: controller.signal.aborted ? "obsolete" : "failed",
            generation: current,
            message: error.message,
          });
          if (live) {
            onLoading?.(-1);
            onError?.(
              `Terrain render failed: ${error.message}. Retry the terrain render below.`,
            );
          }
        });
    }, 80);
    return () => {
      live = false;
      clearTimeout(timer);
      controller.abort();
      if (!completed) {
        counters.current.cancelled++;
        if (!entered) counters.current.outstanding--;
      }
    };
  }, [signature]);
  useEffect(() => {
    const retained = new Set(
      [frame, overview, latest.current.frame, latest.current.overview].flatMap(
        (f) => f?.images.map((i) => i.url) || [],
      ),
    );
    for (const url of owned.current)
      if (!retained.has(url) && !staging.current.has(url)) {
        URL.revokeObjectURL(url);
        owned.current.delete(url);
      }
  }, [frame, overview]);
  useEffect(
    () => () => {
      for (const url of owned.current) URL.revokeObjectURL(url);
      owned.current.clear();
    },
    [],
  );
  if (!active) return children;
  const current =
    frame?.identity === identity
      ? frame
      : overview?.identity === identity
        ? overview
        : null;
  const b = request.bounds,
    a = current?.bounds;
  const covered =
    a &&
    a.x <= b.x + 1e-12 &&
    a.y <= b.y + 1e-12 &&
    a.x + a.w >= b.x + b.w - 1e-12 &&
    a.y + a.h >= b.y + b.h - 1e-12;
  const context = current && {
    ...current,
    underlay:
      !covered && overview?.identity === identity && overview !== current
        ? overview
        : null,
  };
  return (
    <FrameContext.Provider value={context}>
      <g
        aria-label="Committed terrain viewport"
        data-generation={current?.generation}
        data-phase={current?.phase}
        data-revision={current?.revision}
        data-pixel-width={current?.pixels.width}
        data-pixel-height={current?.pixels.height}
      >
        {children}
      </g>
    </FrameContext.Provider>
  );
}
