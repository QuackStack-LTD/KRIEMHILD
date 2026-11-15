import { useEffect, useId, useMemo, useState } from "react";
import { request } from "../continuous/api.js";
import { resourceRing } from "../continuous/terrain-resources.js";
import {
  surveyLabel,
  resourceAreas,
} from "../continuous/resource-report-text.js";
import {
  ResourceReport,
  LifeformResults,
} from "../continuous/TerrainResourceReport.jsx";
import "./resource-archive.css";

const number = (n) =>
  Number.isFinite(n)
    ? new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(n)
    : "—";
const title = (s) => s.replaceAll("-", " ").replaceAll("_", " ");
const palette = [
  "#d8b975",
  "#5aabb2",
  "#b58cc4",
  "#87ad7b",
  "#cf8973",
  "#789dcb",
  "#b8b37b",
  "#db9bb4",
  "#7fc6ad",
];

export function ResourcePie({ title: heading, values, unit = "" }) {
  const entries = Object.entries(values)
    .filter(([, v]) => v > 0)
    .sort((a, b) => b[1] - a[1]);
  const total = entries.reduce((s, [, v]) => s + v, 0);
  let cursor = 0;
  return (
    <section className="resource-chart">
      <h3>{heading}</h3>
      {!total ? (
        <p>No recorded values.</p>
      ) : (
        <>
          <svg
            viewBox="0 0 220 220"
            role="img"
            aria-label={`${heading}: ${entries.map(([k, v]) => `${title(k)} ${number(v)} ${unit}`).join(", ")}`}
          >
            {entries.map(([k, v], i) => {
              const start = cursor;
              cursor += (v / total) * Math.PI * 2;
              const end = cursor;
              const large = end - start > Math.PI ? 1 : 0;
              const d =
                v === total
                  ? null
                  : `M110 110 L${110 + 90 * Math.cos(start)} ${110 + 90 * Math.sin(start)} A90 90 0 ${large} 1 ${110 + 90 * Math.cos(end)} ${110 + 90 * Math.sin(end)} Z`;
              return d ? (
                <path key={k} d={d} fill={palette[i % palette.length]}>
                  <title>
                    {title(k)}: {number(v)} {unit}
                  </title>
                </path>
              ) : (
                <circle
                  key={k}
                  cx="110"
                  cy="110"
                  r="90"
                  fill={palette[i % palette.length]}
                >
                  <title>
                    {title(k)}: {number(v)} {unit}
                  </title>
                </circle>
              );
            })}
            <circle cx="110" cy="110" r="49" fill="#17232b" />
            <text x="110" y="114" textAnchor="middle" fill="#eee4d2">
              {number(total)}
              {unit}
            </text>
          </svg>
          <ul className="resource-chart-legend">
            {entries.map(([k, v], i) => (
              <li key={k}>
                <i style={{ background: palette[i % palette.length] }} />
                <span>{title(k)}</span>
                <strong>
                  {number(v)} {unit}
                </strong>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}
function ResourceLine({ history, metric }) {
  const points = history.points
    .map((p) => ({ ...p, value: metricValue(p.statistics, metric) }))
    .filter((p) => p.statistics.samples > 0 && Number.isFinite(p.value));
  const lo = Math.min(...points.map((p) => p.year)),
    hi = Math.max(...points.map((p) => p.year)),
    min = Math.min(0, ...points.map((p) => p.value)),
    max = Math.max(1, ...points.map((p) => p.value));
  const x = (p) => 64 + ((p.year - lo) / Math.max(1, hi - lo)) * 600,
    y = (p) => 190 - ((p.value - min) / Math.max(1, max - min)) * 160;
  return (
    <section className="resource-chart resource-line">
      <h3>{history.name}</h3>
      {!points.length ? (
        <p>No dated values for this measure in this Age.</p>
      ) : (
        <>
          <svg
            viewBox="0 0 720 240"
            role="img"
            aria-label={`${history.name} historical resource chart`}
          >
            <path d="M64 25 V190 H680" fill="none" stroke="#a8b5bd" />
            <text x="58" y="32" textAnchor="end">
              {number(max)}
            </text>
            <text x="58" y="193" textAnchor="end">
              {number(min)}
            </text>
            <polyline
              points={points.map((p) => `${x(p)},${y(p)}`).join(" ")}
              fill="none"
              stroke="#d8b975"
              strokeWidth="2"
            />
            {points.map((p) => (
              <circle key={p.year} cx={x(p)} cy={y(p)} r="4" fill="#d8b975">
                <title>
                  Year {p.year}: {number(p.value)};{" "}
                  {number(p.statistics.coverageFraction * 100)}% recorded
                  coverage
                </title>
              </circle>
            ))}
            <text x="64" y="219">
              Year {lo}
            </text>
            <text x="664" y="219" textAnchor="end">
              {hi === lo ? "" : `Year ${hi}`}
            </text>
          </svg>
          <details>
            <summary>Recorded chart values</summary>
            <table>
              <thead>
                <tr>
                  <th>Year</th>
                  <th>Value</th>
                  <th>Known coverage</th>
                </tr>
              </thead>
              <tbody>
                {points.map((p) => (
                  <tr key={p.year}>
                    <td>{p.year}</td>
                    <td>{number(p.value)}</td>
                    <td>{number(p.statistics.coverageFraction * 100)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </details>
        </>
      )}
      {history.undated > 0 && (
        <p>
          {history.undated} undated report(s) are available in the current view
          and excluded from dated lines.
        </p>
      )}
    </section>
  );
}
function metricValue(s, m) {
  const [kind, ...rest] = m.split(":");
  const key = rest.join(":");
  return kind === "lifeforms"
    ? s.biologyCoverageFraction > 0
      ? s.lifeformCount
      : undefined
    : kind === "habitats"
      ? s.biologyCoverageFraction > 0
        ? s.habitatCount
        : undefined
      : kind === "lifeformGroup"
        ? s.biologyCoverageFraction > 0
          ? s.lifeformGroups?.[key] || 0
          : undefined
        : kind === "resource"
          ? s.resourceCounts[key] || 0
          : kind === "quantity"
            ? s.quantities[key] || 0
            : kind === "field"
              ? s.fields[key]?.mean
              : kind === "recharge"
                ? s.rechargeMmYear
                : s.freshWaterFraction * 100;
}
function ResourceSnapshotDetails({ path, biology = false }) {
  const [open, setOpen] = useState(false),
    [response, setResponse] = useState(null);
  const data = response?.path === path ? response.data : null,
    error = response?.path === path ? response.error : "";
  useEffect(() => {
    setResponse(null);
    if (!open) return;
    const c = new AbortController();
    request(path, undefined, "GET", c.signal)
      .then((data) => {
        if (!c.signal.aborted) setResponse({ path, data });
      })
      .catch((e) => {
        if (!c.signal.aborted) setResponse({ path, error: e.message });
      });
    return () => c.abort();
  }, [open, path]);
  return (
    <details
      open={open}
      onToggle={(e) => {
        if (e.target === e.currentTarget) setOpen(e.currentTarget.open);
      }}
    >
      <summary>
        {biology
          ? "Recorded habitats and lifeforms"
          : "Complete information in the original document"}
      </summary>
      {open &&
        (error ? (
          <p role="alert">{error}</p>
        ) : !data ? (
          <p role="status">Reading saved information…</p>
        ) : biology ? (
          <div className="terrain-resource-report">
            <p>
              Biology is recorded for {number(data.coverageFraction * 100)}% of
              the known sampled area. Relative abundance is averaged over
              recorded biology and is not a population count.
            </p>
            {data.biosphere.stale && (
              <p className="resource-warning">
                Some observations were marked stale when recorded.
              </p>
            )}
            <p>{data.biosphere.habitats.length} recorded habitats.</p>
            <LifeformResults
              group="recorded lifeforms"
              items={data.biosphere.lifeforms}
            />
          </div>
        ) : (
          <ResourceReport report={data.report} actions={false} />
        ))}
    </details>
  );
}
function SurveyMap({ reports, onSelect }) {
  const clip = useId().replaceAll(":", "");
  const [worldView, setWorldView] = useState(false);
  const first = reports[0]?.survey.circle;
  const rings = useMemo(() => {
    if (!first) return [];
    const origin = first.longitude / 180 + 1;
    return reports.map((r) => {
      let center = r.survey.circle.longitude / 180 + 1;
      center += Math.round((origin - center) / 2) * 2;
      return {
        report: r,
        points: resourceRing(
          [center, 0.5 - r.survey.circle.latitude / 180],
          r.survey.circle.radiusKm,
          r.survey.circle.planetRadiusKm,
        ),
      };
    });
  }, [reports]);
  let left = Infinity,
    right = -Infinity,
    top = Infinity,
    bottom = -Infinity;
  for (const r of rings)
    for (const [x, y] of r.points) {
      left = Math.min(left, x);
      right = Math.max(right, x);
      top = Math.min(top, y);
      bottom = Math.max(bottom, y);
    }
  if (worldView) {
    left = 0;
    right = 2;
    top = 0;
    bottom = 1;
  }
  const spanX = Math.max(0.00002, right - left),
    spanY = Math.max(0.00001, bottom - top),
    w = Math.max(spanX, spanY * 2) * 1.25,
    h = w / 2;
  const box = [(left + right - w) / 2, (top + bottom - h) / 2, w, h];
  if (!reports.length)
    return (
      <p>
        No resource circles have been recorded here. Observe an area in the
        Atlas, then choose Archive information.
      </p>
    );
  return (
    <div className="resource-survey-map">
      <div className="resource-review-controls">
        <span>Saved geographic coverage · select a circle</span>
        <button onClick={() => setWorldView((v) => !v)}>
          {worldView ? "Fit recorded areas" : "Show whole terrain"}
        </button>
      </div>
      <svg
        viewBox={box.join(" ")}
        role="group"
        aria-label="Map of archived terrain resource circles"
      >
        <defs>
          <clipPath id={clip}>
            <rect x={box[0]} y={box[1]} width={w} height={h} />
          </clipPath>
        </defs>
        <rect x={box[0]} y={box[1]} width={w} height={h} fill="#142a35" />
        <g clipPath={`url(#${clip})`}>
          {Array.from({ length: 7 }, (_, i) => {
            const x = box[0] + (w * i) / 6,
              y = box[1] + (h * i) / 6;
            return (
              <g key={i} opacity=".35">
                <path
                  d={`M${x} ${box[1]}v${h} M${box[0]} ${y}h${w}`}
                  stroke="#728e9a"
                  strokeWidth={w / 1300}
                />
                <text
                  x={x + w / 150}
                  y={box[1] + h / 19}
                  fontSize={w / 95}
                  fill="#becbd0"
                >
                  {number(((x * 180 - 180 + 540) % 360) - 180)}°
                </text>
                <text
                  x={box[0] + w / 100}
                  y={y + h / 35}
                  fontSize={w / 95}
                  fill="#becbd0"
                >
                  {number(90 - y * 180)}°
                </text>
              </g>
            );
          })}
          {rings.map(({ report: r, points }, i) => (
            <g key={r.id}>
              {[-2, 0, 2].map((shift) => (
                <path
                  key={shift}
                  d={
                    points
                      .map(([x, y], j) => `${j ? "L" : "M"}${x + shift} ${y}`)
                      .join(" ") + "Z"
                  }
                  fill={palette[i % palette.length]}
                  fillOpacity=".18"
                  stroke={palette[i % palette.length]}
                  strokeWidth={w / 650}
                  onClick={() => onSelect(r.id)}
                  tabIndex={shift === 0 ? 0 : -1}
                  role="button"
                  aria-label={`Open resources: ${surveyLabel(r.survey)}, ${r.survey.year == null ? "undated" : `year ${r.survey.year}`}`}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onSelect(r.id);
                    }
                  }}
                >
                  <title>
                    {surveyLabel(r.survey)} ·{" "}
                    {r.survey.year == null
                      ? "Undated"
                      : `Year ${r.survey.year}`}
                  </title>
                </path>
              ))}
            </g>
          ))}
        </g>
      </svg>
      <p>
        Circle locations and coverage come from retained reports. This map does
        not fetch current terrain resources.
      </p>
    </div>
  );
}

function ArchivedResourceInventory({ items }) {
  const [search, setSearch] = useState(""),
    [page, setPage] = useState(0);
  const matches = useMemo(() => {
    const q = search.trim().toLowerCase();
    return q
      ? items.filter((r) =>
          `${r.type} ${r.variant} ${r.availability || ""}`
            .toLowerCase()
            .includes(q),
        )
      : items;
  }, [items, search]);
  const size = 100,
    current = Math.min(page, Math.max(0, Math.ceil(matches.length / size) - 1));
  return (
    <details open>
      <summary>Saved resource inventory ({items.length})</summary>
      {items.length > size && (
        <label>
          Find resources
          <input
            type="search"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(0);
            }}
          />
        </label>
      )}
      <div className="resource-table-scroll">
        <table>
          <thead>
            <tr>
              <th>Resource</th>
              <th>Variant</th>
              <th>Whole deposit quantity</th>
              <th>Grade</th>
              <th>Confidence</th>
            </tr>
          </thead>
          <tbody>
            {matches.slice(current * size, (current + 1) * size).map((r) => (
              <tr key={r.id}>
                <td>{title(r.type)}</td>
                <td>{r.variant}</td>
                <td>
                  {number(r.quantity)} {r.quantityUnit}
                </td>
                <td>
                  {number(r.grade)} {r.gradeUnit}
                </td>
                <td>{number(r.confidence * 100)}%</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {items.length > size && (
        <div className="resource-review-controls">
          <span>
            {matches.length ? current * size + 1 : 0}–
            {Math.min((current + 1) * size, matches.length)} of {matches.length}
          </span>
          <button disabled={current === 0} onClick={() => setPage(current - 1)}>
            Previous resources
          </button>
          <button
            disabled={(current + 1) * size >= matches.length}
            onClick={() => setPage(current + 1)}
          >
            Next resources
          </button>
        </div>
      )}
    </details>
  );
}

export default function ResourceArchiveReview({
  world,
  ageId,
  archiveId,
  onBack,
  onDocuments,
  initialArea = "",
}) {
  const [list, setList] = useState(null),
    [area, setArea] = useState(initialArea),
    [detailResponse, setDetailResponse] = useState(null),
    [terrain, setTerrain] = useState(""),
    [observedAge, setObservedAge] = useState(""),
    [year, setYear] = useState(""),
    [metric, setMetric] = useState("recharge"),
    [error, setError] = useState("");
  const base = `worlds/${world}/resource-surveys?${new URLSearchParams({ age: ageId, archive: archiveId })}`;
  const detailPath = `${base}&${new URLSearchParams({ area, sourceAge: observedAge, year })}`,
    detail = detailResponse?.path === detailPath ? detailResponse.data : null;
  useEffect(() => {
    const c = new AbortController();
    setList(null);
    setError("");
    request(base, undefined, "GET", c.signal)
      .then((v) => {
        setList(v);
        setTerrain(v.reports[0]?.survey.terrainId || "");
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(e.message);
      });
    return () => c.abort();
  }, [base]);
  useEffect(() => {
    setDetailResponse(null);
    setError("");
    if (!area) return;
    const c = new AbortController();
    request(detailPath, undefined, "GET", c.signal)
      .then((data) => {
        if (!c.signal.aborted) setDetailResponse({ path: detailPath, data });
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(e.message);
      });
    return () => c.abort();
  }, [detailPath, area]);
  const reports = useMemo(
    () =>
      resourceAreas(
        list?.reports.filter((r) => r.survey.terrainId === terrain) || [],
        ageId,
      ),
    [list, terrain, ageId],
  );
  function open(id) {
    setArea(id);
    setObservedAge("");
    setYear("");
    setMetric("recharge");
  }
  const metrics = useMemo(() => {
    const counts = new Set(),
      quantities = new Set();
    for (const h of detail?.history || [])
      for (const p of h.points) {
        Object.keys(p.statistics.resourceCounts).forEach((k) => counts.add(k));
        Object.keys(p.statistics.quantities).forEach((k) => quantities.add(k));
      }
    Object.keys(detail?.current.resourceCounts || {}).forEach((k) =>
      counts.add(k),
    );
    Object.keys(detail?.current.quantities || {}).forEach((k) =>
      quantities.add(k),
    );
    const groups = new Set();
    for (const h of detail?.history || [])
      for (const p of h.points)
        Object.keys(p.statistics.lifeformGroups || {}).forEach((g) =>
          groups.add(g),
        );
    return [
      ["lifeforms", "Recorded lifeform count"],
      ["habitats", "Recorded habitat count"],
      ...[...groups]
        .sort()
        .map((g) => [
          `lifeformGroup:${g}`,
          `${title(g)} — recorded lifeform count`,
        ]),
      ["recharge", "Aquifer recharge (mm/year)"],
      ["water", "Fresh surface water (%)"],
      ...[...counts]
        .sort()
        .map((k) => [`resource:${k}`, `${title(k)} — occurrence count`]),
      ...[...quantities]
        .sort()
        .map((k) => [`quantity:${k}`, `${title(k)} — whole deposits`]),
      ...["elevation", "temperature", "precipitation", "pet"].map((k) => [
        `field:${k}`,
        `${title(k)} — mean`,
      ]),
    ];
  }, [detail]);
  const current = detail?.current;
  return (
    <section
      className="resource-archive-review"
      aria-label="Archive terrain resources"
    >
      <header className="resource-review-heading">
        <div>
          <span className="eyebrow">
            {list?.archive.name || "Archive"} / Resources
          </span>
          <h1>
            {area ? "Terrain resource record" : "Review Terrain Resources"}
          </h1>
          <p>
            Knowledge preserved in this archive · separate timelines for each
            Age
          </p>
        </div>
        <div className="resource-review-controls">
          <button onClick={() => (area ? setArea("") : onBack?.())}>
            {area ? "← All recorded areas" : "← Archive"}
          </button>
          {onDocuments && (
            <button onClick={() => onDocuments(list?.resourcesFolderId || "")}>
              Resource documents
            </button>
          )}
        </div>
      </header>
      {error && <p role="alert">{error}</p>}
      {!list && !error && <p role="status">Reading archived reports…</p>}
      {list && !area && (
        <>
          {!!list.reports.length && (
            <label className="resource-terrain-select">
              Terrain
              <select
                aria-label="Archived resource terrain"
                value={terrain}
                onChange={(e) => setTerrain(e.target.value)}
              >
                {[...new Set(list.reports.map((r) => r.survey.terrainId))].map(
                  (t) => (
                    <option key={t} value={t}>
                      {list.terrains.find((x) => x.id === t)?.name ||
                        "Recorded terrain"}
                    </option>
                  ),
                )}
              </select>
            </label>
          )}
          <SurveyMap reports={reports} onSelect={open} />
          <div className="resource-recorded-areas">
            {reports.map((r) => (
              <button key={r.id} onClick={() => open(r.id)}>
                <strong>{surveyLabel(r.survey)}</strong>
                <span>
                  {list.ages.find((a) => a.id === r.survey.sourceAge)?.name} ·{" "}
                  {r.survey.year == null ? "Undated" : `Year ${r.survey.year}`}
                </span>
                <small>
                  {r.reportCount} report(s) · Open current knowledge and
                  resource history →
                </small>
              </button>
            ))}
          </div>
        </>
      )}
      {area && !detail && !error && (
        <p role="status">Resolving saved observations…</p>
      )}
      {detail && (
        <>
          <p className="resource-area-label">
            {surveyLabel(detail.area.survey)}
          </p>
          <div className="resource-review-controls">
            <label>
              Observed Age
              <select
                value={observedAge || detail.sourceAge}
                onChange={(e) => {
                  setObservedAge(e.target.value);
                  setYear("");
                }}
              >
                {detail.history.map((h) => (
                  <option key={h.ageId} value={h.ageId}>
                    {h.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              As of
              <select
                aria-label="Resource observation year"
                value={year}
                onChange={(e) => setYear(e.target.value)}
              >
                <option value="">Latest recorded knowledge</option>
                {detail.history
                  .find((h) => h.ageId === (observedAge || detail.sourceAge))
                  ?.points.map((p) => (
                    <option key={p.year} value={p.year}>
                      Year {p.year}
                    </option>
                  ))}
              </select>
            </label>
            <a
              href={`/api/worlds/${world}/library/${detail.area.id}?age=${ageId}`}
            >
              Download original report
            </a>
          </div>
          <p className="resource-knowledge-note">
            This view combines retained reports for this circle in the selected
            Age. Newer overlapping observations replace older samples;
            unobserved parts stay unknown.{" "}
            {number(current.coverageFraction * 100)}% sampled coverage ·{" "}
            {number(current.sampleAreaKm2)} km² per sample. Deposit quantities
            describe whole deposits, deduplicated by identity; they are not
            recoverable reserves or quantities clipped to this circle.
          </p>
          {!current.samples ? (
            <p>No observations for this area at the selected date.</p>
          ) : (
            <>
              <div className="resource-stat-cards">
                <article>
                  <span>Known resource occurrences</span>
                  <strong>{current.resources.length}</strong>
                </article>
                <article>
                  <span>Aquifer recharge</span>
                  <strong>
                    {number(current.rechargeMmYear)} <small>mm/year</small>
                  </strong>
                </article>
                <article>
                  <span>Fresh surface water</span>
                  <strong>{number(current.freshWaterFraction * 100)}%</strong>
                </article>
              </div>
              <div className="resource-chart-grid">
                <ResourcePie
                  title="Resources by occurrence count"
                  values={current.resourceCounts}
                />
                <ResourcePie
                  title="Observed biome composition"
                  values={Object.fromEntries(
                    Object.entries(current.biomes).map(([k, v]) => [
                      k,
                      v * 100,
                    ]),
                  )}
                  unit="%"
                />
                {current.biosphere?.available && (
                  <ResourcePie
                    title="Recorded lifeforms by group"
                    values={current.lifeformGroups}
                  />
                )}
              </div>
              {current.biosphere?.available && (
                <ResourceSnapshotDetails
                  biology
                  path={`${base}&${new URLSearchParams({ area, sourceAge: observedAge || detail.sourceAge, year, lifeforms: "1" })}`}
                />
              )}
              <ArchivedResourceInventory key={area} items={current.resources} />
            </>
          )}
          <section className="resource-historical">
            <h2>Historical resources</h2>
            <label>
              Resource or measure
              <select
                aria-label="Historical resource measure"
                value={metric}
                onChange={(e) => setMetric(e.target.value)}
              >
                {metrics.map(([key, name]) => (
                  <option key={key} value={key}>
                    {name}
                  </option>
                ))}
              </select>
            </label>
            <p>
              Each Age has its own year axis. Undated reports do not invent
              historical dates. Coverage can change as knowledge is recorded or
              lost.
            </p>
            {detail.history.map((h) => (
              <ResourceLine key={h.ageId} history={h} metric={metric} />
            ))}
          </section>
          <ResourceSnapshotDetails
            key={area}
            path={`${base}&${new URLSearchParams({ area, original: "1" })}`}
          />
        </>
      )}
    </section>
  );
}
