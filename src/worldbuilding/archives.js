export const archiveCategories = [
  "chronicles",
  "administrative",
  "economic",
  "cultural",
  "geographical",
  "personal",
  "genealogies",
  "compilations",
];
export const archivePurposes = [
  "personal",
  "household",
  "religious",
  "civic",
  "military",
  "diplomatic",
  "commercial",
  "royal",
  "scholarly",
  "general",
];
export function archiveDefaults(owner) {
  const kind = owner?.kind || "";
  const purpose =
    kind === "character"
      ? "personal"
      : ["country", "state", "kingdom", "empire"].includes(kind)
        ? "royal"
        : ["city", "village", "town"].includes(kind)
          ? "civic"
          : ["religion", "religious-order"].includes(kind)
            ? "religious"
            : "general";
  const categories = {
    personal: ["chronicles", "personal", "genealogies"],
    royal: ["chronicles", "administrative", "economic", "genealogies"],
    civic: ["chronicles", "administrative", "economic", "cultural"],
    religious: ["chronicles", "cultural", "genealogies"],
  }[purpose] || ["chronicles", "administrative"];
  return {
    version: 1,
    ownerId: owner?.id || "",
    purpose,
    intensity: "moderate",
    categories,
    knowledgeScope: purpose === "personal" ? "direct" : "institutional",
    retention: "preserve",
    sources: [],
    coverage: [],
  };
}
export function matchesArchiveFilters(file, filters) {
  const r = file.record;
  return Boolean(
    (!filters.age || r?.sourceAge === filters.age) &&
      (!filters.type || r?.category === filters.type) &&
      (!filters.entity || r?.relatedEntities?.includes(filters.entity)) &&
      (filters.year === "" ||
        (r &&
          r.fromYear <= Number(filters.year) &&
          r.toYear >= Number(filters.year))) &&
      (filters.recorded === "" ||
        r?.recordingYear === Number(filters.recorded)),
  );
}
export async function archiveRequest(world, endpoint, body) {
  const r = await fetch(`/api/worlds/${world}/archives/${endpoint}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const value = await r.json();
  if (!r.ok) throw Error(value.error || "Archive action failed");
  return value;
}
