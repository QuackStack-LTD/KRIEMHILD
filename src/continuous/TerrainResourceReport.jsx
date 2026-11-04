import { useEffect, useState } from "react";
import { request } from "./api.js";
import "./terrain-resources.css";

const title = (s) => s.replaceAll("-", " ").replaceAll("_", " ");
const number = (n) =>
  Number.isFinite(n)
    ? new Intl.NumberFormat(undefined, {
        maximumFractionDigits: 2,
        notation: Math.abs(n) >= 1e7 ? "compact" : "standard",
      }).format(n)
    : "—";

export function useResourceReport(path) {
  const [state, setState] = useState({
    path: "",
    report: null,
    error: "",
    loading: false,
  });
  useEffect(() => {
    if (!path) return;
    const controller = new AbortController();
    setState({ path, report: null, error: "", loading: true });
    const timer = setTimeout(
      () =>
        request(path, undefined, "GET", controller.signal)
          .then((report) => {
            if (!controller.signal.aborted)
              setState({ path, report, error: "", loading: false });
          })
          .catch((error) => {
            if (!controller.signal.aborted)
              setState({
                path,
                report: null,
                error: error.message,
                loading: false,
              });
          }),
      220,
    );
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [path]);
  // Never present the previous circle's inventory as the current selection.
  return state.path === path
    ? state
    : { report: null, error: "", loading: !!path };
}
export function ResourceReport({ report, error, loading }) {
  if (loading) return <p role="status">Reading this terrain area…</p>;
  if (error) return <p role="alert">{error}</p>;
  if (!report)
    return (
      <p>
        Click the terrain to choose a centre, or drag to draw an observed
        circle.
      </p>
    );
  const water = report.freshWater,
    materials = Object.groupBy(report.resources, (r) => r.type),
    organisms = Object.groupBy(report.biosphere.lifeforms, (r) => r.group);
  return (
    <div className="terrain-resource-report">
      <p className="resource-coordinates">
        {number(report.latitude)}° latitude · {number(report.longitude)}°
        longitude
        <br />
        {number(report.radiusKm)} km radius · {number(report.areaKm2)} km²
        observed
      </p>
      {(report.stale || report.biosphere.stale) && (
        <p className="resource-warning" role="status">
          Terrain or habitat edits have made this analysis stale. These are the
          saved generated ranges; regenerate and accept the affected
          terrain/Biosphere to update them.
        </p>
      )}
      <section className="resource-water" aria-label="Fresh water supply">
        <h3>Fresh water supply</h3>
        <dl>
          <dt>Rivers intersecting the circle</dt>
          <dd>{water.rivers.length}</dd>
          <dt>Lake footprints nearby</dt>
          <dd>{water.lakes.length}</dd>
          <dt>Spring emergence zones</dt>
          <dd>{water.springs.length}</dd>
          <dt>Estimated annual recharge</dt>
          <dd>{number(water.rechargeMmYear)} mm/year</dd>
          <dt>Recharge over observed area</dt>
          <dd>{number(water.rechargeM3Year)} m³/year</dd>
          <dt>Sampled fresh surface water</dt>
          <dd>{number(water.freshSurfaceFraction * 100)}%</dd>
        </dl>
        {!water.rivers.length &&
          !water.lakes.length &&
          !water.springs.length && (
            <p>
              No mapped surface source intersects this selection. Regional
              groundwater may still be present.
            </p>
          )}
        {water.rivers.map((r) => (
          <div key={r.id} className="resource-item">
            <strong>{title(r.id)}</strong>
            <p>
              {number(r.dischargeM3s)} m³/s model flow ·{" "}
              {number(r.drySeasonProxyM3s)} m³/s dry-season proxy ·{" "}
              {number(r.distanceKm)} km from centre
            </p>
          </div>
        ))}
        {water.springs.length > 0 && (
          <details>
            <summary>Spring temperature and mineral composition</summary>
            {water.springs.map((s) => (
              <p key={s.id}>
                {title(s.origin)} · {number(s.temperature)} °C ·{" "}
                {Object.entries(s.minerals || {})
                  .map(([k, v]) => `${title(k)} ${number(v)}`)
                  .join(", ")}
              </p>
            ))}
          </details>
        )}
        <p className="resource-note">{water.explanation}</p>
      </section>
      <section aria-label="Terrain and climate">
        <h3>Terrain and climate</h3>
        <p>
          {Object.entries(report.biomes || {})
            .map(([kind, share]) => `${title(kind)} ${number(share * 100)}%`)
            .join(" · ")}
        </p>
        <dl>
          {[
            ["elevation", "Elevation", "m"],
            ["temperature", "Temperature", "°C"],
            ["precipitation", "Annual precipitation", "mm/year"],
            ["pet", "Potential evaporation", "mm/year"],
          ].map(
            ([key, label, unit]) =>
              report.fields[key] && (
                <div className="resource-field-summary" key={key}>
                  <dt>{label}</dt>
                  <dd>
                    {number(report.fields[key].min)}–
                    {number(report.fields[key].max)} {unit}
                  </dd>
                </div>
              ),
          )}
        </dl>
      </section>
      <section aria-label="Natural resource inventory">
        <h3>
          Natural resources <small>({report.totalResources} occurrences)</small>
        </h3>
        {report.truncated && (
          <p role="status">
            Showing the first {report.resources.length} occurrences. Reduce the
            radius to inspect a smaller inventory.
          </p>
        )}
        {!report.resources.length && (
          <p>No generated resource footprint intersects this circle.</p>
        )}
        {Object.entries(materials).map(([kind, items]) => (
          <details key={kind}>
            <summary>
              {title(kind)} <small>· {items.length}</small>
            </summary>
            {items.map((r) => (
              <article className="resource-item" key={r.id}>
                <strong>
                  {title(r.variant)} ·{" "}
                  {title(r.availability || "regional potential")}
                </strong>
                <p>{r.cause}</p>
                {r.extentKm2 > 0 && (
                  <dl>
                    <dt>Whole deposit extent</dt>
                    <dd>{number(r.extentKm2)} km²</dd>
                    <dt>Quantity</dt>
                    <dd>
                      {number(r.quantity)} {r.quantityUnit}
                    </dd>
                    <dt>Grade</dt>
                    <dd>
                      {number(r.grade)} {r.gradeUnit}
                    </dd>
                    <dt>Depth</dt>
                    <dd>{number(r.depthMetres)} m</dd>
                  </dl>
                )}
                <small>
                  {r.provenance || "procedural/inferred"} · Confidence{" "}
                  {number(r.confidence * 100)}%<br />
                  {r.reasonCodes?.map(title).join(" → ")}
                </small>
                <details>
                  <summary>Source formations and system</summary>
                  <p>
                    {r.systemId}
                    <br />
                    {r.districtId}
                  </p>
                  {r.references.map((ref, i) => (
                    <p key={i}>
                      {ref.kind}: {ref.id}
                    </p>
                  ))}
                </details>
              </article>
            ))}
          </details>
        ))}
      </section>
      <section aria-label="Geological features">
        <h3>Geological features and history</h3>
        {report.geology.map((p) => (
          <details key={p.id}>
            <summary>
              {title(p.type || p.setting)} · {number(p.ageMa)} Ma
            </summary>
            <p>
              {title(p.dominantRock)} · {p.process}
            </p>
            <p>
              {title(p.crust || "generated crust")} · Heat flow{" "}
              {number(p.history.heatFlowMwM2)} mW/m² ·{" "}
              {title(p.history.maturity || "maturity unknown")}
            </p>
            <p>
              {title(p.history.sourceEnvironment || p.history.paleoclimate)} ·
              Maximum burial {number(p.history.maxBurialMetres)} m · Maximum
              temperature {number(p.history.maxTemperatureC)} °C
            </p>
            <ul>
              {p.events.map((v, i) => (
                <li key={i}>
                  {title(v.kind)} · {number(v.ageMa)} Ma ·{" "}
                  {title(v.environment)}
                </li>
              ))}
            </ul>
          </details>
        ))}
        <details>
          <summary>
            Rock formations ({report.formations.length}) and faults (
            {report.structures.length})
          </summary>
          {report.formations.map((f) => (
            <article className="resource-item" key={f.id}>
              <strong>
                {title(f.rock)} · {title(f.kind)}
              </strong>
              <p>
                {f.process} · {number(f.ageMa)} Ma
              </p>
              {f.strata?.map((s) => (
                <p key={s.id}>
                  {title(s.rock)} · {number(s.topMetres)}–
                  {number(s.bottomMetres)} m · {title(s.environment)}
                </p>
              ))}
            </article>
          ))}
          {report.structures.map((f) => (
            <p key={f.id}>
              {title(f.kind)} · {number(f.ageMa)} Ma
            </p>
          ))}
        </details>
      </section>
      <section aria-label="Biosphere in observed area">
        <h3>Habitats and lifeforms</h3>
        {!report.biosphere.available ? (
          <p>
            No accepted Biosphere range intersects this circle. Generate and
            accept a Biosphere for this terrain to populate these ranges.
          </p>
        ) : (
          <>
            <p>
              Saved habitat ranges intersecting the circle, rather than
              individual organism locations.
            </p>
            <details open>
              <summary>Habitats ({report.biosphere.habitats.length})</summary>
              {report.biosphere.habitats.map((h) => (
                <p key={h.id}>
                  <strong>{h.name}</strong>
                  <br />
                  {h.description || title(h.kind)}
                </p>
              ))}
            </details>
            {Object.entries(organisms).map(([group, items]) => (
              <details key={group} open>
                <summary>
                  {title(group)} ({items.length})
                </summary>
                {items.map((l) => (
                  <p key={l.id}>
                    <strong>{l.name}</strong> · {title(l.status)}
                    <br />
                    {l.roles?.map(title).join(", ")} · relative abundance{" "}
                    {number(l.abundance)}
                  </p>
                ))}
              </details>
            ))}
            {!report.biosphere.lifeforms.length && (
              <p>No present lifeforms are recorded in these habitat ranges.</p>
            )}
          </>
        )}
      </section>
      <details>
        <summary>All generated scalar fields</summary>
        <p className="resource-note">
          Regional sample minimum / mean / maximum. Field names preserve
          generator terminology. Indices and categories are dimensionless;
          temperatures °C, elevation/depth m, precipitation mm/year,
          recharge/runoff m/year, geothermal mW/m².
        </p>
        <table>
          <thead>
            <tr>
              <th>Field</th>
              <th>Min</th>
              <th>Mean</th>
              <th>Max</th>
            </tr>
          </thead>
          <tbody>
            {Object.entries(report.fields)
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([k, v]) => (
                <tr key={k}>
                  <th>{k}</th>
                  <td>{number(v.min)}</td>
                  <td>{number(v.mean)}</td>
                  <td>{number(v.max)}</td>
                </tr>
              ))}
          </tbody>
        </table>
      </details>
      <details>
        <summary>Resolution and provenance</summary>
        {report.notes.map((n, i) => (
          <p key={i}>{n}</p>
        ))}
        <p>
          Regional cell height ≈ {number(report.regionalCellKm)} km. Source:{" "}
          {report.source || "legacy saved geology"}. Generator:{" "}
          {report.generator || "legacy resource inventory"}.
          {report.derived &&
            " Derived read-only from an older terrain snapshot."}
        </p>
      </details>
    </div>
  );
}
export default function TerrainResourceReport({ path }) {
  return <ResourceReport {...useResourceReport(path)} />;
}
