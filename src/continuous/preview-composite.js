// Preview-only composition at the explicitly requested raster dimensions. The
// Illustrator and exports keep their independent, original layer configuration.
export async function compositePreview(frame, opacity, signal) {
  const shade = frame.images.find((i) => i.style === "hillshade");
  const base = frame.images.find((i) => i.style !== "hillshade");
  if (!shade || !base || !(opacity > 0)) return frame;
  signal.throwIfAborted();
  const canvas = new OffscreenCanvas(frame.pixels.width, frame.pixels.height);
  let color, relief;
  try {
    // Join both decodes even if one fails, so every successful bitmap is closed.
    const results = await Promise.allSettled([
      createImageBitmap(base.blob),
      createImageBitmap(shade.blob),
    ]);
    if (results[0].status === "fulfilled") color = results[0].value;
    if (results[1].status === "fulfilled") relief = results[1].value;
    const failure = results.find((r) => r.status === "rejected");
    if (failure) throw failure.reason;
    signal.throwIfAborted();
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    // This is the same backdrop as Studio's preview SVG. Include it before
    // multiplying, including partially transparent pixels at the world boundary.
    ctx.fillStyle = "#101d2a";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(color, 0, 0);
    ctx.globalCompositeOperation = "multiply";
    ctx.globalAlpha = Math.max(0, Math.min(1, opacity));
    ctx.drawImage(relief, 0, 0);
    const blob = await canvas.convertToBlob({ type: "image/png" });
    signal.throwIfAborted();
    return {
      ...frame,
      images: [
        { style: base.style, blob },
        ...frame.images.filter((i) => i !== base && i !== shade),
      ],
      compositedStyles: ["hillshade"],
    };
  } finally {
    color?.close();
    relief?.close();
    canvas.width = canvas.height = 1;
  }
}
