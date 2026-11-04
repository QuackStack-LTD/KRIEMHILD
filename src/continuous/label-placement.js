import { contains } from "./geometry.js";

// Text stays legible at the illustration's export scale. Small countries use
// an external label and a leader instead of an unreadable or missing name.
export function placeLabel(label, extent, pixel, occupied, override = {}) {
  const [x, y, w, h] = extent;
  const boxAt = (p, size) => {
    const width = Math.max(12, label.text.length * size * 0.57) * pixel;
    const height = size * 1.25 * pixel;
    return { x: p[0] - width / 2, y: p[1] - height / 2, w: width, h: height };
  };
  const fits = (b) =>
    b.x >= x && b.y >= y && b.x + b.w <= x + w && b.y + b.h <= y + h;
  const collides = (b) =>
    occupied.some(
      (a) =>
        b.x < a.x + a.w &&
        b.x + b.w > a.x &&
        b.y < a.y + a.h &&
        b.y + b.h > a.y,
    );
  const inside = (b) =>
    !label.polygon ||
    [
      [b.x, b.y],
      [b.x + b.w, b.y],
      [b.x, b.y + b.h],
      [b.x + b.w, b.y + b.h],
      [b.x + b.w / 2, b.y + b.h / 2],
    ].every((p) => contains(p, label.polygon));
  const origin = label.position;
  if (override.position) {
    const size = override.size || pixel * (label.country ? 15 : 11);
    return {
      position: override.position,
      size,
      box: boxAt(override.position, size / pixel),
    };
  }
  for (const size of label.country ? [15, 13, 11, 9] : [11]) {
    const b = boxAt(origin, size),
      dx = b.w / 2 + 6 * pixel;
    const offsets = label.country
      ? [
          [0, 0],
          [-b.w * 0.2, 0],
          [b.w * 0.2, 0],
          [0, -b.h],
          [0, b.h],
        ]
      : [
          [dx, -b.h / 2],
          [dx, b.h / 2],
          [-dx, -b.h / 2],
          [0, -b.h],
        ];
    for (const [ox, oy] of offsets) {
      const position = [origin[0] + ox, origin[1] + oy],
        box = boxAt(position, size);
      if (fits(box) && !collides(box) && (!label.country || inside(box)))
        return { position, size: pixel * size, box };
    }
  }
  const size = 11,
    template = boxAt(origin, size);
  const clamp = (p) => [
    Math.max(x + template.w / 2, Math.min(x + w - template.w / 2, p[0])),
    Math.max(y + template.h / 2, Math.min(y + h - template.h / 2, p[1])),
  ];
  for (let ring = 1; ring <= 32; ring++) {
    for (const [dx, dy] of [
      [1, -1],
      [1, 1],
      [-1, -1],
      [-1, 1],
      [1, 0],
      [-1, 0],
      [0, -1],
      [0, 1],
    ]) {
      const position = clamp([
          origin[0] + dx * (template.w / 2 + ring * 6 * pixel),
          origin[1] + dy * (template.h + ring * 6 * pixel),
        ]),
        box = boxAt(position, size);
      if (fits(box) && !collides(box))
        return { position, size: pixel * size, box, leaderFrom: origin };
    }
  }
  const position = clamp(origin),
    box = boxAt(position, size);
  return {
    position,
    size: pixel * size,
    box,
    placementWarning:
      "Labels overlap here; drag this name to adjust its position.",
  };
}
