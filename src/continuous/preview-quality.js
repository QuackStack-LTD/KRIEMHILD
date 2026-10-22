import { viewportRequest, pyramidNodes } from "./terrain-pyramid.js";

export const previewPreferenceKey = "kriemhild.preview-quality.v1";
export const previewPresets = {
  performance: { label: "Performance", scale: 0.5 },
  balanced: { label: "Balanced", scale: 0.75 },
  high: { label: "High quality", scale: 1 },
};
export const previewHelp =
  "Controls the quality and responsiveness of the interactive preview. Does not change saved terrain, climate generation, the Map Illustrator, or exported maps. Preview density is measured per CSS pixel, independently of screen DPR.";

export function normalizePreviewQuality(value) {
  const preset =
    value?.preset === "custom" || Object.hasOwn(previewPresets, value?.preset)
      ? value.preset
      : "balanced";
  const customScale = Number.isFinite(value?.customScale)
    ? Math.max(0.5, Math.min(1.5, value.customScale))
    : 1;
  return { preset, customScale };
}
export function readPreviewQuality(storage) {
  try {
    storage ??= globalThis.localStorage;
    return normalizePreviewQuality(
      JSON.parse(storage?.getItem(previewPreferenceKey)),
    );
  } catch {
    return normalizePreviewQuality(null);
  }
}
export function savePreviewQuality(value, storage) {
  try {
    storage ??= globalThis.localStorage;
    storage?.setItem(
      previewPreferenceKey,
      JSON.stringify(normalizePreviewQuality(value)),
    );
    return true;
  } catch {
    return false;
  }
}

// Interactive callers opt in explicitly. Generic renderers, the Illustrator and
// exports never read this preference. CSS-pixel density makes the workload stable
// across DPR 1/2/3; users can request supersampling explicitly with Custom.
export function previewRequest(options, quality) {
  const q = normalizePreviewQuality(quality);
  const requestedScale =
    q.preset === "custom" ? q.customScale : previewPresets[q.preset].scale;
  let density = Math.min(
    requestedScale,
    Math.sqrt((32 * 1024 * 1024) / (options.width * options.height)),
  );
  let request;
  for (;;) {
    request = viewportRequest({ ...options, dpr: density });
    const nodes = pyramidNodes(request);
    const atlasPixels = nodes.length
      ? (nodes.at(-1).x - nodes[0].x + 1) *
        (nodes.at(-1).y - nodes[0].y + 1) *
        request.size ** 2
      : 1;
    if (
      request.pixels.width * request.pixels.height <= 32 * 1024 * 1024 &&
      atlasPixels <= 64 * 1024 * 1024
    )
      break;
    density *= 0.9;
  }
  return {
    ...request,
    preview: {
      preset: q.preset,
      label: q.preset === "custom" ? "Custom" : previewPresets[q.preset].label,
      requestedScale,
      effectiveScale: density,
      clamped: density < requestedScale - 1e-9,
    },
  };
}
