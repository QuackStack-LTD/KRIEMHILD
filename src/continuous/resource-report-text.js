const label = (key) =>
  key.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/[_.-]/g, " ");
// Preserve every supplied value, including provenance, field ranges and species
// hidden behind the panel's disclosure controls. Clipboard text is one paragraph.
export function resourceReportParagraph(report) {
  const format = (value) => {
    if (Array.isArray(value))
      return value.map(format).filter(Boolean).join("; ");
    if (value && typeof value === "object")
      return Object.entries(value)
        .filter(([key]) => !["fingerprint", "context"].includes(key))
        .map(([key, v]) => {
          const text = format(v);
          return text ? `${label(key)}: ${text}` : "";
        })
        .filter(Boolean)
        .join("; ");
    if (value == null || value === "") return "";
    return String(value).replace(/\s+/g, " ").trim();
  };
  return `Terrain resource report. ${format(report)}.`;
}

export function resourceAreas(reports, ageId) {
  const areas = new Map();
  for (const report of reports) {
    const c = report.survey.circle,
      key = [
        report.survey.terrainId,
        c.longitude.toFixed(7),
        c.latitude.toFixed(7),
        c.radiusKm.toFixed(6),
        c.planetRadiusKm,
      ].join("/");
    const old = areas.get(key);
    const a = report.survey,
      b = old?.survey;
    const year = a.year ?? Infinity,
      previousYear = b?.year ?? Infinity;
    const preferred =
      !old ||
      (a.sourceAge === ageId && b.sourceAge !== ageId) ||
      (a.sourceAge === b.sourceAge &&
        (year > previousYear ||
          (year === previousYear && a.recordedAt > b.recordedAt))) ||
      (a.sourceAge !== b.sourceAge &&
        b.sourceAge !== ageId &&
        a.recordedAt > b.recordedAt);
    areas.set(key, {
      ...(preferred ? report : old),
      reportCount: (old?.reportCount || 0) + 1,
    });
  }
  // Small overlapping disks stay clickable above larger disks. Repeated
  // documents for exactly the same area share one map entry and its history.
  return [...areas.values()].sort(
    (a, b) => b.survey.circle.radiusKm - a.survey.circle.radiusKm,
  );
}

export const surveyLabel = (survey) =>
  `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(4 * Math.PI * survey.circle.planetRadiusKm ** 2 * Math.sin(survey.circle.radiusKm / survey.circle.planetRadiusKm / 2) ** 2)} km² at ${survey.circle.latitude.toFixed(3)}°, ${survey.circle.longitude.toFixed(3)}°`;
