"use client";
import { useState } from "react";
import { api, Command, Entry, newID, State } from "../lib/types";
import { Empty } from "./Records";
import TerrainTools, { MapLayers } from "./Terrain";
export default function Atlas({
  state,
  run,
  busy,
  inspect,
  selected,
}: {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  inspect: (id: string) => void;
  selected?: string;
}) {
  const maps = Object.values(state.records).filter((r) => r.kind === "map");
  const [id, setID] = useState(selected || "");
  const map = maps.find((m) => m.id === id) || maps[0];
  const [name, setName] = useState(""),
    [uploading, setUploading] = useState(false),
    [error, setError] = useState("");
  return (
    <div className="atlas-page">
      <div className="section-heading">
        <div>
          <div className="eyebrow">Atlas / {state.age.name}</div>
          <h2>Your world, located.</h2>
        </div>
        <select
          aria-label="Select map"
          value={map?.id || ""}
          onChange={(e) => setID(e.target.value)}
        >
          {!maps.length && <option value="">No maps yet</option>}
          {maps.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}
            </option>
          ))}
        </select>
      </div>
      <form
        className="map-import"
        onSubmit={async (e) => {
          e.preventDefault();
          const form = e.currentTarget;
          const file = (form.elements.namedItem("image") as HTMLInputElement)
            .files?.[0];
          setUploading(true);
          setError("");
          try {
            let image = { asset: "", width: 1200, height: 800 };
            if (file)
              image = await api(`/projects/${state.root.world.id}/assets`, {
                method: "POST",
                body: file,
              });
            const r: Entry = {
              id: newID(),
              kind: "map",
              name,
              ...image,
              pins: [],
            };
            if (await run({ action: "put", record: r })) {
              setID(r.id);
              setName("");
              form.reset();
            }
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setUploading(false);
          }
        }}
      >
        <label>
          Map name
          <input
            required
            aria-label="Map name"
            placeholder="The northern coast"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label>
          Image (optional)
          <input
            name="image"
            type="file"
            accept="image/png,image/jpeg,image/gif"
          />
        </label>
        <button disabled={busy || uploading}>
          {uploading ? "Importing…" : "+ Create map"}
        </button>
      </form>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {map ? (
        <>
          <MapCanvas
            key={map.id}
            map={map}
            state={state}
            run={run}
            busy={busy}
            inspect={inspect}
          />
          <TerrainTools
            key={map.id + "terrain"}
            map={map}
            state={state}
            run={run}
            busy={busy}
          />
        </>
      ) : (
        <Empty
          title="Give your world a shape."
          text="Import an image or create a blank map, generate editable terrain, and place your world's people and settlements."
        />
      )}
    </div>
  );
}
function MapCanvas({
  map,
  state,
  run,
  busy,
  inspect,
}: {
  map: Entry;
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  inspect: (id: string) => void;
}) {
  const [entity, setEntity] = useState(""),
    [zoom, setZoom] = useState(1),
    [x, setX] = useState("50"),
    [y, setY] = useState("50");
  const entities = Object.values(state.records).filter(
    (r) => r.kind === "entity",
  );
  const place = (px: number, py: number) => {
    if (!entity || busy) return;
    void run({
      action: "put",
      record: {
        ...map,
        pins: [
          ...(map.pins || []).filter((p) => p.entity !== entity),
          { entity, x: px, y: py },
        ],
      },
    });
  };
  return (
    <>
      <div className="map-tools">
        <select
          aria-label="Entity to place"
          value={entity}
          onChange={(e) => setEntity(e.target.value)}
        >
          <option value="">Select an entity to place or move</option>
          {entities.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>
        <span className="muted">Then click the map</span>
        <button
          aria-label="Zoom out"
          onClick={() => setZoom(Math.max(0.5, zoom - 0.25))}
        >
          −
        </button>
        <span>{Math.round(zoom * 100)}%</span>
        <button
          aria-label="Zoom in"
          onClick={() => setZoom(Math.min(3, zoom + 0.25))}
        >
          +
        </button>
        <button onClick={() => setZoom(1)}>Reset</button>
      </div>
      <div className="map-viewport">
        <svg
          role="img"
          aria-label={`${map.name} map with ${map.pins?.length || 0} pins`}
          viewBox={`0 0 ${map.width} ${map.height}`}
          style={{ width: `${zoom * 100}%`, minWidth: 400 }}
          onClick={(e) => {
            const svg = e.currentTarget,
              point = svg.createSVGPoint();
            point.x = e.clientX;
            point.y = e.clientY;
            const matrix = svg.getScreenCTM();
            if (matrix) {
              const p = point.matrixTransform(matrix.inverse());
              place(
                Math.max(0, Math.min(1, p.x / map.width!)),
                Math.max(0, Math.min(1, p.y / map.height!)),
              );
            }
          }}
        >
          <defs>
            <pattern
              id="grid"
              width={60}
              height={60}
              patternUnits="userSpaceOnUse"
            >
              <path
                d="M 60 0 L 0 0 0 60"
                fill="none"
                stroke="#b5bcb0"
                strokeWidth="1"
              />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="#e7ebdf" />
          <rect width="100%" height="100%" fill="url(#grid)" />
          {map.asset && (
            <image
              href={`/api/v1/projects/${state.root.world.id}/assets/${map.asset}`}
              width={map.width}
              height={map.height}
            />
          )}
          <MapLayers
            map={map}
            records={state.records}
            layers={[
              "terrain",
              "water",
              "rivers",
              "river",
              "route",
              "border",
              "lake",
            ]}
          />
          {(map.pins || []).map((p) => (
            <g
              key={p.entity}
              transform={`translate(${p.x * map.width!},${p.y * map.height!})`}
              onClick={(e) => {
                e.stopPropagation();
                setEntity(p.entity);
                setX(String(Math.round(p.x * 1000) / 10));
                setY(String(Math.round(p.y * 1000) / 10));
              }}
            >
              <circle
                r={map.width! * 0.008}
                fill={entity === p.entity ? "#a15d2b" : "#25564b"}
                stroke="#fff"
                strokeWidth={map.width! * 0.002}
              />
              <text
                x={map.width! * 0.013}
                y={map.width! * 0.004}
                fontSize={map.width! * 0.017}
                fill="#172e29"
                stroke="#fff"
                strokeWidth={map.width! * 0.003}
                paintOrder="stroke"
              >
                {state.records[p.entity]?.name}
              </text>
            </g>
          ))}
        </svg>
      </div>
      <form
        className="map-tools"
        onSubmit={(e) => {
          e.preventDefault();
          place(Number(x) / 100, Number(y) / 100);
        }}
      >
        <strong>Place precisely</strong>
        <label>
          X %
          <input
            aria-label="Pin X percent"
            type="number"
            min="0"
            max="100"
            step="any"
            required
            value={x}
            onChange={(e) => setX(e.target.value)}
          />
        </label>
        <label>
          Y %
          <input
            aria-label="Pin Y percent"
            type="number"
            min="0"
            max="100"
            step="any"
            required
            value={y}
            onChange={(e) => setY(e.target.value)}
          />
        </label>
        <button disabled={!entity || busy}>Place / move pin</button>
      </form>
      <table>
        <caption>Places on {map.name}</caption>
        <thead>
          <tr>
            <th>Entity</th>
            <th>Position</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {(map.pins || []).map((p) => (
            <tr key={p.entity}>
              <td>
                <button
                  className="text-button"
                  onClick={() => inspect(p.entity)}
                >
                  {state.records[p.entity]?.name}
                </button>
              </td>
              <td>
                {(p.x * 100).toFixed(1)}%, {(p.y * 100).toFixed(1)}%
              </td>
              <td>
                <button
                  disabled={busy}
                  onClick={() =>
                    void run({
                      action: "put",
                      record: {
                        ...map,
                        pins: map.pins?.filter(
                          (pin) => pin.entity !== p.entity,
                        ),
                      },
                    })
                  }
                >
                  Remove pin
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
