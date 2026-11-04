import { useEffect, useState } from "react";
import {
  ResourceReport,
  useResourceReport,
} from "../continuous/TerrainResourceReport.jsx";
import "../continuous/terrain-resources.css";

export default function SettlementResources({ doc, entity }) {
  const [radius, setRadius] = useState(null),
    [limits, setLimits] = useState(null),
    [imageError, setImageError] = useState(false);
  useEffect(() => {
    setRadius(null);
    setLimits(null);
    setImageError(false);
  }, [entity.id, entity.ageId, doc.world.revision]);
  const base = `worlds/${doc.world.id}/settlements/${entity.id}/resources`,
    query = new URLSearchParams({
      age: entity.ageId,
      revision: doc.world.revision,
    });
  if (radius !== null) query.set("radiusKm", radius);
  const state = useResourceReport(`${base}?${query}`),
    report = state.report;
  useEffect(() => {
    if (report)
      setLimits({ min: report.minimumRadiusKm, max: report.maximumRadiusKm });
  }, [report]);
  const current = radius ?? report?.radiusKm ?? limits?.min;
  return (
    <section
      className="settlement-resources"
      aria-label="Resources around this settlement"
    >
      <div className="settlement-resource-map">
        <h2>Resources around {entity.name}</h2>
        {limits && (
          <label className="resource-radius-label">
            Observed radius <output>{current?.toFixed(2)} km</output>
            <input
              type="range"
              aria-label="Observed radius around settlement"
              min={limits.min}
              max={limits.max}
              step="any"
              value={current}
              onChange={(e) => setRadius(Number(e.target.value))}
            />
            <span>Whole settlement ({limits.min.toFixed(2)} km) — 50 km</span>
            <button type="button" onClick={() => setRadius(limits.min)}>
              Fit whole settlement
            </button>
          </label>
        )}
        {current != null && (
          <>
            <img
              className="resource-circle-preview"
              key={`${entity.id}:${current}:${doc.world.revision}`}
              src={`/api/${base}/preview?${new URLSearchParams({ age: entity.ageId, radiusKm: current, revision: doc.world.revision })}`}
              alt={`Terrain within ${current.toFixed(2)} km of ${entity.name}, with its complete footprint outlined in gold`}
              onLoad={() => setImageError(false)}
              onError={() => setImageError(true)}
            />
            {imageError && (
              <p role="alert">The terrain preview could not be loaded.</p>
            )}
          </>
        )}
      </div>
      <div className="settlement-resource-data">
        <ResourceReport {...state} />
      </div>
    </section>
  );
}
