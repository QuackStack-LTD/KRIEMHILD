import { createContext, useContext, useEffect, useRef, useState } from "react";
import {
  TerrainPyramid,
  viewportRequest,
  renderViewport,
} from "./terrain-pyramid.js";
import { previewRequest } from "./preview-quality.js";
import { compositePreview } from "./preview-composite.js";

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
  return (
    <g
      aria-label="Continuous terrain tiles"
      data-terrain-style={style}
      pointerEvents="none"
    >
      <image
        href={image.url}
        x={frame.bounds.x}
        y={frame.bounds.y}
        width={frame.bounds.w}
        height={frame.bounds.h}
        preserveAspectRatio="none"
        data-generation={frame.generation}
        data-revision={frame.revision}
        data-lod={frame.z}
        data-raster-style={image.style}
      />
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
  previewHillshade,
}) {
  const [frame, setFrame] = useState(null),
    generation = useRef(0),
    committed = useRef(null);
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
    retry,
    enabled,
    active,
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
    onLoading?.(1);
    record({
      event: "request",
      generation: current,
      ...request,
      preview: previewQuality
        ? { ...request.preview, hillshade: previewHillshade || 0 }
        : undefined,
      invalidations: undefined,
    });
    const timer = setTimeout(
      () =>
        renderViewport(
          pyramid,
          { ...request, generation: current },
          controller.signal,
          {
            record,
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
                images: displayed.images.map((i) => ({
                  style: i.style,
                  url: URL.createObjectURL(i.blob),
                })),
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
                  return;
                }
                setFrame(next);
                return {
                  layers: next.images.length,
                  bytes: displayed.images.reduce((n, i) => n + i.blob.size, 0),
                };
              } catch (error) {
                release(next);
                throw error;
              }
            },
          },
        )
          .then(() => {
            if (live) onLoading?.(0);
          })
          .catch((error) => {
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
          }),
      80,
    );
    return () => {
      live = false;
      clearTimeout(timer);
      controller.abort();
    };
  }, [signature]);
  useEffect(() => {
    const previous = committed.current;
    committed.current = frame;
    release(previous);
  }, [frame]);
  useEffect(() => () => release(committed.current), []);
  if (!active) return children;
  const current = frame?.identity === identity ? frame : null;
  return (
    <FrameContext.Provider value={current}>
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
