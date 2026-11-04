export const VECTOR_VERSIONS = {
  engine: "vector-script/1",
  adapter: "sign-layout/1",
  compiler: "vector-script-compiler/1",
};
export const signRoles = [
  "phonetic",
  "syllabic",
  "logographic",
  "morphemic",
  "semantic",
  "classifier",
];
export const emptyWriting = () => ({
  version: "vector-script/1",
  profile: "mixed",
  unitsPerEm: 1000,
  lineHeight: 1400,
  spacing: 80,
  glyphs: [],
  signs: [],
  sequences: [],
});
export const blankGlyph = (id, name = "") => ({
  id,
  name,
  viewBox: [0, 0, 1000, 1000],
  baseline: 800,
  advance: 1000,
  bearing: 0,
  scale: 1,
  paths: [],
});
export const blankPath = (id) => ({
  id,
  commands: [],
  fill: "none",
  stroke: "#182126",
  strokeWidth: 24,
});
export const commandSizes = { M: 2, L: 2, Q: 4, C: 6, Z: 0 };
export function safeGlyph(glyph) {
  if (
    !glyph ||
    !Array.isArray(glyph.viewBox) ||
    glyph.viewBox.length !== 4 ||
    glyph.viewBox.some((v) => !Number.isFinite(v) || Math.abs(v) > 8192) ||
    glyph.viewBox[2] < 1 ||
    glyph.viewBox[3] < 1 ||
    !Array.isArray(glyph.paths) ||
    glyph.paths.length > 128
  )
    return false;
  let count = 0;
  return glyph.paths.every(
    (p) =>
      /^none$|^#[0-9a-fA-F]{6}$/.test(p.fill) &&
      /^none$|^#[0-9a-fA-F]{6}$/.test(p.stroke) &&
      Number.isFinite(p.strokeWidth) &&
      p.strokeWidth >= 0 &&
      p.strokeWidth <= 256 &&
      Array.isArray(p.commands) &&
      p.commands.every((c, i) => {
        count++;
        return (
          count <= 4096 &&
          Object.hasOwn(commandSizes, c.op) &&
          (i !== 0 || c.op === "M") &&
          Array.isArray(c.values) &&
          c.values.length === commandSizes[c.op] &&
          c.values.every((v) => Number.isFinite(v) && Math.abs(v) <= 8192)
        );
      }),
  );
}
export const commandPath = (commands) =>
  commands.map((c) => [c.op, ...c.values].join(" ")).join(" ");
export function boundedHistory(snapshots, limit = 8 * 1024 * 1024) {
  let bytes = 0;
  const result = [];
  for (let i = snapshots.length - 1; i >= 0 && result.length < 32; i--) {
    const size = JSON.stringify(snapshots[i]).length * 2;
    if (bytes + size > limit) break;
    bytes += size;
    result.unshift(snapshots[i]);
  }
  return result;
}
