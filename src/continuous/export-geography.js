import { pathText } from "./geographic-path.js";

// Replace preview simplification with export geometry before serializing. The
// Illustrator supplies its own scene and has none of these preview selectors.
export function prepareGeographicExport(svg, geometry) {
  const rivers = new Map(geometry.rivers.map((f) => [f.id, f.points]));
  for (const path of svg.querySelectorAll("path[data-river-id]")) {
    const points = rivers.get(path.getAttribute("data-river-id"));
    if (points) path.setAttribute("d", pathText(points));
  }
  for (const path of svg.querySelectorAll("path[data-coast-index]")) {
    const points = geometry.coasts[+path.getAttribute("data-coast-index")];
    if (points) path.setAttribute("d", pathText(points));
  }
  for (const handle of svg.querySelectorAll("[data-editor-handle]"))
    handle.remove();
}
