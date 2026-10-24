export function illustratedMapURL(worldId, entity, banner = false) {
  const map = entity.properties?.illustratedMap;
  if (!worldId || !map?.fileId) return null;
  return `/api/worlds/${worldId}/library/${encodeURIComponent(map.fileId)}?age=${entity.ageId}&version=${encodeURIComponent(map.version)}&inline=1${banner ? "&variant=banner" : ""}`;
}
export default function IllustratedMapImage({
  worldId,
  entity,
  banner = false,
}) {
  const src = illustratedMapURL(worldId, entity, banner);
  return src ? (
    <img
      className={
        banner ? "illustrated-map-banner" : "illustrated-map-thumbnail"
      }
      src={src}
      alt={`${entity.name} illustrated map`}
      loading={banner ? "eager" : "lazy"}
    />
  ) : null;
}
