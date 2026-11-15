// Chromium can defer asynchronous canvas PNG encoding by a full second per
// layer, even with a visible tab. Small interactive frames have bounded work;
// encode them immediately. Large rasters keep the asynchronous path.
export const immediateRasterLimit = 1024 * 1024;
export function previewRaster(width, height, fallback, immediate = true) {
  if (
    immediate &&
    width * height <= immediateRasterLimit &&
    typeof HTMLCanvasElement !== "undefined"
  ) {
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    canvas.convertToBlob = () => {
      const data = atob(canvas.toDataURL("image/png").split(",")[1]);
      const bytes = Uint8Array.from(data, (c) => c.charCodeAt(0));
      return Promise.resolve(new Blob([bytes], { type: "image/png" }));
    };
    return canvas;
  }
  return fallback(width, height);
}
