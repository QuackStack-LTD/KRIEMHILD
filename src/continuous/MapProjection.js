import { createContext, useContext, createElement } from "react";
import { rasterCrop } from "./projection.js";
export const ProjectionContext = createContext(null);
export const useMapProjection = () => useContext(ProjectionContext);

export function ProjectedRaster({ frame, image, ...attributes }) {
  const projection = useMapProjection();
  if (!projection)
    return createElement("image", {
      ...attributes,
      href: image.url,
      x: frame.bounds.x,
      y: frame.bounds.y,
      width: frame.bounds.w,
      height: frame.bounds.h,
      preserveAspectRatio: "none",
    });
  const crop = rasterCrop(frame, projection);
  if (!crop) return null;
  const { source: s, destination: d } = crop;
  // Clip in bitmap coordinates, rather than drawing a world overview as a
  // multi-million-pixel SVG image when a close-up is waiting for detail.
  return createElement(
    "svg",
    {
      x: d.x,
      y: d.y,
      width: d.w,
      height: d.h,
      viewBox: `${s.x} ${s.y} ${s.w} ${s.h}`,
      preserveAspectRatio: "none",
      overflow: "hidden",
      "data-world-bounds": JSON.stringify(frame.bounds),
    },
    createElement("image", {
      ...attributes,
      href: image.url,
      x: 0,
      y: 0,
      width: frame.pixels.width,
      height: frame.pixels.height,
      preserveAspectRatio: "none",
    }),
  );
}
