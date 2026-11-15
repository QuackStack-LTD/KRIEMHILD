// Page furniture lives outside the geographic frame. No land search or mask
// can put the title/compass back on top of a continent, including all-land maps.
export function illustrationLayout(config) {
  const [x, y, w, h] = config.bounds;
  const decorated = config.layers.some(
    (layer) => layer.id === "decoration" && layer.visible,
  );
  const headerHeight = decorated ? w * 0.095 : 0;
  const centerY = y - headerHeight / 2;
  const titleWidth = w * 0.8;
  const titleSize = Math.min(
    w / 52,
    titleWidth / Math.max(1, Array.from(config.title || "").length * 0.65),
  );
  return {
    page: { x, y: y - headerHeight, w, h: h + headerHeight },
    header: { x, y: y - headerHeight, w, h: headerHeight },
    title: { x: x + w * 0.44, y: centerY, size: titleSize },
    compass: { x: x + w * 0.94, y: centerY, size: w * 0.025 },
  };
}

export const illustrationPageBounds = (config) =>
  illustrationLayout(config).page;
