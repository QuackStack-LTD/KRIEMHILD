"use client";
import { useEffect, useState } from "react";
import { api, Command, Entry, Feature, newID, State } from "../lib/types";

export function MapLayers({
  map,
  records,
  layers = [
    "terrain",
    "water",
    "rivers",
    "route",
    "border",
    "lake",
    "river",
    "climate",
    "biome",
    "pins",
  ],
  world,
}: {
  map: Entry;
  records: Record<string, Entry>;
  layers?: string[];
  world?: string;
}) {
  const t = map.terrain,
    w = map.width || 1200,
    h = map.height || 800;
  return (
    <>
      {map.asset && world && (
        <image
          href={`/api/v1/projects/${world}/assets/${map.asset}`}
          width={w}
          height={h}
        />
      )}
      {t &&
        (layers.includes("terrain") || layers.includes("water")) &&
        t.heights.map((v, i) => {
          const water = layers.includes("water") && t.water[i];
          const color = water
            ? water === 1
              ? "#619caf"
              : "#80b4c7"
            : layers.includes("terrain")
              ? `hsl(${92 - Math.min(65, Math.max(0, v) / 15)} 23% ${75 - Math.min(40, Math.max(0, v) / 30)}%)`
              : "transparent";
          return (
            <rect
              key={i}
              x={((i % t.columns) * w) / t.columns}
              y={(Math.floor(i / t.columns) * h) / t.rows}
              width={w / t.columns + 0.2}
              height={h / t.rows + 0.2}
              fill={color}
            />
          );
        })}
      {t &&
        layers.includes("rivers") &&
        t.flow.map((j, i) =>
          j < 0 ||
          t.heights[i] < t.sea + 70 ||
          i % 7 !== 0 ||
          Math.abs((j % t.columns) - (i % t.columns)) > 1 ? null : (
            <line
              key={i}
              x1={(((i % t.columns) + 0.5) * w) / t.columns}
              y1={((Math.floor(i / t.columns) + 0.5) * h) / t.rows}
              x2={(((j % t.columns) + 0.5) * w) / t.columns}
              y2={((Math.floor(j / t.columns) + 0.5) * h) / t.rows}
              stroke="#287696"
              strokeWidth={1.5}
            />
          ),
        )}
      {(map.features || [])
        .filter((f) => layers.includes(f.kind))
        .map((f) => {
          const area = ["border", "lake", "climate", "biome"].includes(f.kind);
          const points = f.points.map((p) => `${p.x * w},${p.y * h}`).join(" ");
          const color =
            f.kind === "climate"
              ? "#bb743d"
              : f.kind === "biome"
                ? "#52815c"
                : f.kind === "border"
                  ? "#b46732"
                  : f.kind === "route"
                    ? "#804537"
                    : "#286c91";
          return (
            <g key={f.id}>
              <title>
                {f.name}
                {f.entity
                  ? ` · ${records[f.entity]?.name || "Unknown owner"}`
                  : ""}
              </title>
              {area ? (
                <polygon
                  points={points}
                  fill={color}
                  fillOpacity={0.22}
                  stroke={color}
                  strokeWidth={3}
                />
              ) : (
                <polyline
                  points={points}
                  fill="none"
                  stroke={color}
                  strokeWidth={3}
                  strokeDasharray={f.kind === "route" ? "9 5" : undefined}
                />
              )}
              <text
                x={f.points[0].x * w + 8}
                y={f.points[0].y * h - 8}
                fontSize={15}
                fill={color}
              >
                {f.name}
              </text>
            </g>
          );
        })}
      {layers.includes("pins") &&
        (map.pins || []).map((p) => (
          <g key={p.entity}>
            <circle
              cx={p.x * w}
              cy={p.y * h}
              r={8}
              fill="#203c32"
              stroke="white"
              strokeWidth={2}
            />
            <text
              x={p.x * w + 12}
              y={p.y * h + 5}
              fontSize={18}
              fill="#152d25"
              stroke="white"
              strokeWidth={3}
              paintOrder="stroke"
            >
              {records[p.entity]?.name}
            </text>
          </g>
        ))}
    </>
  );
}
export function MapPicture({
  map,
  records,
  world,
  label,
}: {
  map: Entry;
  records: Record<string, Entry>;
  world: string;
  label: string;
}) {
  return (
    <svg
      role="img"
      aria-label={label}
      viewBox={`0 0 ${map.width || 1200} ${map.height || 800}`}
      className="map-picture"
    >
      <rect width="100%" height="100%" fill="#e7ebdf" />
      <MapLayers map={map} records={records} world={world} />
    </svg>
  );
}

type Preview = {
  record: Entry;
  revision: string;
  changed: number;
  impacts: { entity: string; name: string; change: string }[];
};
export default function TerrainTools({
  map,
  state,
  run,
  busy,
}: {
  map: Entry;
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
}) {
  const [operation, setOperation] = useState("generate"),
    [seed, setSeed] = useState("glass-coast"),
    [x, setX] = useState(50),
    [y, setY] = useState(50),
    [radius, setRadius] = useState(25),
    [strength, setStrength] = useState(300),
    [sea, setSea] = useState(0),
    [wrap, setWrap] = useState(false);
  const [preview, setPreview] = useState<Preview | null>(null),
    [working, setWorking] = useState(false),
    [error, setError] = useState(""),
    [eventName, setEventName] = useState(""),
    [date, setDate] = useState(""),
    [explanation, setExplanation] = useState(""),
    [until, setUntil] = useState("");
  const [featureID, setFeatureID] = useState("");
  const [featureName, setFeatureName] = useState(""),
    [kind, setKind] = useState<Feature["kind"]>("route"),
    [coordinates, setCoordinates] = useState("10,20 40,35 75,60"),
    [owner, setOwner] = useState("");
  const [layers, setLayers] = useState([
    "terrain",
    "water",
    "rivers",
    "river",
    "route",
    "border",
    "lake",
    "pins",
    "climate",
    "biome",
  ]);
  useEffect(() => {
    setPreview(null);
  }, [state.revision]);
  async function propose() {
    setWorking(true);
    setError("");
    try {
      setPreview(
        await api<Preview>(`/projects/${state.root.world.id}/terrain-preview`, {
          method: "POST",
          body: JSON.stringify({
            expected: state.revision,
            age: state.age.id,
            map: map.id,
            operation,
            seed,
            x: x / 100,
            y: y / 100,
            radius: radius / 100,
            strength,
            sea,
            wrap,
          }),
        }),
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setWorking(false);
    }
  }
  const displayed = preview?.record || map;
  return (
    <section className="terrain-panel">
      <h3>Terrain & geographic layers</h3>
      <p className="muted">
        Preview a change, review affected places, then accept it. A submerged
        pin does not automatically destroy its settlement.
      </p>
      <div className="terrain-controls">
        <label>
          Terrain operation
          <select
            value={operation}
            onChange={(e) => setOperation(e.target.value)}
          >
            {[
              ["generate", "Generate from seed"],
              ["raise", "Raise land"],
              ["lower", "Lower land"],
              ["smooth", "Smooth"],
              ["flatten", "Plateau / flatten"],
              ["flood", "Flood region"],
              ["drain", "Drain region"],
              ["sea", "Change sea level"],
            ].map(([v, n]) => (
              <option key={v} value={v}>
                {n}
              </option>
            ))}
          </select>
        </label>
        {operation === "generate" ? (
          <>
            <label>
              Terrain seed
              <input value={seed} onChange={(e) => setSeed(e.target.value)} />
            </label>
            <label className="check-label">
              <input
                type="checkbox"
                checked={wrap}
                onChange={(e) => setWrap(e.target.checked)}
              />
              Wrap east / west
            </label>
          </>
        ) : (
          <>
            <label>
              Brush X %
              <input
                type="number"
                min="0"
                max="100"
                value={x}
                onChange={(e) => setX(Number(e.target.value))}
              />
            </label>
            <label>
              Brush Y %
              <input
                type="number"
                min="0"
                max="100"
                value={y}
                onChange={(e) => setY(Number(e.target.value))}
              />
            </label>
            <label>
              Brush radius %
              <input
                type="number"
                min="1"
                max="100"
                value={radius}
                onChange={(e) => setRadius(Number(e.target.value))}
              />
            </label>
            <label>
              Brush strength
              <input
                type="number"
                min="1"
                max="2000"
                value={strength}
                onChange={(e) => setStrength(Number(e.target.value))}
              />
            </label>
          </>
        )}
        {(operation === "sea" || operation === "flatten") && (
          <label>
            Target elevation
            <input
              type="number"
              min="-2000"
              max="2000"
              value={sea}
              onChange={(e) => setSea(Number(e.target.value))}
            />
          </label>
        )}
        <button disabled={busy || working} onClick={() => void propose()}>
          {working ? "Calculating…" : "Preview terrain"}
        </button>
      </div>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="layer-toggles">
        {[
          "terrain",
          "water",
          "rivers",
          "river",
          "route",
          "border",
          "lake",
          "pins",
          "climate",
          "biome",
        ].map((l) => (
          <label className="check-label" key={l}>
            <input
              type="checkbox"
              checked={layers.includes(l)}
              onChange={(e) =>
                setLayers(
                  e.target.checked
                    ? [...layers, l]
                    : layers.filter((x) => x !== l),
                )
              }
            />
            {l === "rivers"
              ? "Drainage directions"
              : l === "river"
                ? "Authored rivers"
                : l}
          </label>
        ))}
      </div>
      <svg
        role="img"
        aria-label={preview ? "Terrain proposal" : "Terrain layers"}
        className="map-picture"
        viewBox={`0 0 ${map.width} ${map.height}`}
        onClick={(e) => {
          const svg = e.currentTarget,
            p = svg.createSVGPoint();
          p.x = e.clientX;
          p.y = e.clientY;
          const matrix = svg.getScreenCTM();
          if (matrix) {
            const pt = p.matrixTransform(matrix.inverse());
            setX(Math.round((pt.x / map.width!) * 100));
            setY(Math.round((pt.y / map.height!) * 100));
          }
        }}
      >
        <rect width="100%" height="100%" fill="#e7ebdf" />
        <MapLayers
          map={displayed}
          records={state.records}
          world={state.root.world.id}
          layers={layers}
        />
      </svg>
      <p className="muted">
        Click this map to position the terrain brush.{" "}
        {displayed.terrain &&
          `${displayed.terrain.columns} × ${displayed.terrain.rows} cells · ${displayed.terrain.unit} · ${displayed.terrain.wrap ? "wrapping" : "regional"}`}
      </p>
      {preview && (
        <div className="notice">
          <h4>Review terrain proposal</h4>
          <p>
            {preview.changed} changed cells. Water and drainage have been
            recalculated.
          </p>
          {preview.impacts.length ? (
            <ul>
              {preview.impacts.map((p) => (
                <li key={p.entity}>
                  {p.name}: {p.change}
                </li>
              ))}
            </ul>
          ) : (
            <p>No pins change between land and water.</p>
          )}
          <div className="two-col">
            <label>
              Event title (optional)
              <input
                value={eventName}
                onChange={(e) => setEventName(e.target.value)}
                placeholder="The breaking of the coast"
              />
            </label>
            <label>
              Event tick
              <input
                value={date}
                onChange={(e) => setDate(e.target.value)}
                placeholder="120"
              />
            </label>
            <label>
              Valid until tick (optional)
              <input value={until} onChange={(e) => setUntil(e.target.value)} />
            </label>
            <label>
              Change explanation
              <textarea
                value={explanation}
                onChange={(e) => setExplanation(e.target.value)}
              />
            </label>
          </div>
          <p className="muted">
            Without an event, this changes the Age overview only. An exact dated
            event also records the map state from that tick until its optional
            end (exclusive).
          </p>
          <button
            className="primary"
            disabled={
              busy ||
              working ||
              preview.revision !== state.revision ||
              (!!eventName && !/^-?(0|[1-9]\d*)$/.test(date))
            }
            onClick={async () => {
              const command: Command = eventName
                ? {
                    action: "record-event",
                    records: [preview.record],
                    event: {
                      id: newID(),
                      kind: "event",
                      name: eventName,
                      notes: explanation,
                      event: {
                        age: state.age.id,
                        date: { precision: "exact", tick: date },
                        until,
                        track: "Geography",
                      },
                    },
                  }
                : { action: "put", record: preview.record };
              if (await run(command)) setPreview(null);
            }}
          >
            Accept terrain{eventName ? " & record event" : ""}
          </button>{" "}
          <button onClick={() => setPreview(null)}>Discard proposal</button>
        </div>
      )}
      <details>
        <summary>
          Draw rivers, routes, lakes, borders, climate & biome areas
        </summary>
        <form
          className="feature-form"
          onSubmit={async (e) => {
            e.preventDefault();
            setError("");
            const points = coordinates
              .trim()
              .split(/\s+/)
              .map((pair) => {
                const [x, y] = pair.split(",").map(Number);
                return { x: x / 100, y: y / 100 };
              });
            if (
              points.some((p) => !Number.isFinite(p.x) || !Number.isFinite(p.y))
            ) {
              setError("Use pairs such as 10,20 40,35 75,60.");
              return;
            }
            const feature: Feature = {
              id: featureID || newID(),
              name: featureName,
              kind,
              points,
              entity: owner,
            };
            if (
              await run({
                action: "put",
                record: {
                  ...map,
                  features: [
                    ...(map.features || []).filter((f) => f.id !== featureID),
                    feature,
                  ],
                },
              })
            ) {
              setFeatureName("");
              setFeatureID("");
            }
          }}
        >
          <div className="two-col">
            <label>
              Feature name
              <input
                required
                value={featureName}
                onChange={(e) => setFeatureName(e.target.value)}
              />
            </label>
            <label>
              Feature kind
              <select
                value={kind}
                onChange={(e) => setKind(e.target.value as Feature["kind"])}
              >
                {["river", "route", "border", "lake", "climate", "biome"].map(
                  (k) => (
                    <option key={k}>{k}</option>
                  ),
                )}
              </select>
            </label>
            <label>
              Feature owner (optional)
              <select value={owner} onChange={(e) => setOwner(e.target.value)}>
                <option value="">Unassigned</option>
                {Object.values(state.records)
                  .filter((r) => r.kind === "entity")
                  .map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
              </select>
            </label>
            <label>
              Vertices (X,Y percentages)
              <input
                required
                value={coordinates}
                onChange={(e) => setCoordinates(e.target.value)}
              />
            </label>
          </div>
          <button disabled={busy}>
            {featureID ? "Save geographic feature" : "Add geographic feature"}
          </button>
          {featureID && (
            <button
              type="button"
              onClick={() => {
                setFeatureID("");
                setFeatureName("");
              }}
            >
              Cancel feature edit
            </button>
          )}
        </form>
        {(map.features || []).map((f) => (
          <div className="feature-row" key={f.id}>
            <span>
              <strong>{f.name}</strong> · {f.kind} · {f.points.length} vertices
            </span>
            <button
              disabled={busy}
              onClick={() => {
                setFeatureID(f.id);
                setFeatureName(f.name);
                setKind(f.kind);
                setOwner(f.entity || "");
                setCoordinates(
                  f.points.map((p) => `${p.x * 100},${p.y * 100}`).join(" "),
                );
              }}
            >
              Edit feature
            </button>
            <button
              disabled={busy}
              onClick={() =>
                void run({
                  action: "put",
                  record: {
                    ...map,
                    features: map.features?.filter((v) => v.id !== f.id),
                  },
                })
              }
            >
              Remove feature
            </button>
          </div>
        ))}
      </details>
    </section>
  );
}
