// Read saved categorical cells only. No generation or migration is performed.
export function archivePixels(state) {
  const width = state.W,
    height = state.H;
  if (
    !Number.isInteger(width) ||
    !Number.isInteger(height) ||
    width < 1 ||
    height < 1 ||
    width * height > 65536
  )
    throw Error("Invalid archived map dimensions");
  const pixels = new Uint8ClampedArray(width * height * 4);
  for (let i = 0; i < width * height; i++) {
    const mask = state.dom?.[i] >>> 0;
    const index =
      mask && (mask & (mask - 1)) === 0 ? 31 - Math.clz32(mask) : -1;
    const hex = state.config?.types?.[index]?.color || "#59616c";
    const match = /^#([\da-f]{2})([\da-f]{2})([\da-f]{2})$/i.exec(hex);
    pixels.set(
      match
        ? [...match.slice(1).map((c) => parseInt(c, 16)), 255]
        : [89, 97, 108, 255],
      i * 4,
    );
  }
  return { width, height, pixels };
}
