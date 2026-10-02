"use client";
import { useCallback, useEffect, useState } from "react";
import { Command, Entry, newID, State } from "../lib/types";
type Point = { x: number; y: number };
type Shape = {
  id: string;
  name: string;
  kind: string;
  level: number;
  points: Point[];
};
type Token = {
  id: string;
  entity: string;
  x: number;
  y: number;
  level: number;
  facing: number;
};
type Drawing = {
  parent?: string;
  anchor: Point;
  span: number;
  unit: string;
  grid: string;
  columns: number;
  shapes: Shape[];
  tokens: Token[];
};
type Props = {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  onPending?: (pending: boolean) => void;
};
export default function LocalMaps({ state, run, busy, onPending }: Props) {
  const [pending, setPending] = useState(false);
  const reportPending = useCallback(
    (value: boolean) => {
      setPending(value);
      onPending?.(value);
    },
    [onPending],
  );
  const maps = Object.values(state.records).filter((r) => r.kind === "map");
  const [id, setID] = useState("");
  const map = maps.find((r) => r.id === id) || maps[0];
  return (
    <div>
      <div className="eyebrow">Atlas / {state.age.name}</div>
      <h2>Local maps & tactical plans</h2>
      <p>
        Draw rooms, walls and routes, arrange entity tokens, and connect a
        detail map to a parent map. Plans remain yours: moving a token does not
        resolve a battle or alter a character.
      </p>
      <label>
        Detail map
        <select
          aria-label="Detail map"
          disabled={pending}
          value={map?.id || ""}
          onChange={(e) => setID(e.target.value)}
        >
          {maps.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>
      </label>
      <p>Create blank maps or import a background image in Atlas.</p>
      {map && (
        <DrawingEditor
          key={map.id + JSON.stringify(map.properties?._localMap)}
          {...{ state, run, busy, map, maps }}
          onPending={reportPending}
        />
      )}
    </div>
  );
}
function DrawingEditor({
  state,
  run,
  busy,
  map,
  maps,
  onPending,
}: Props & { map: Entry; maps: Entry[] }) {
  const initial = (map.properties?._localMap as Drawing | undefined) || {
    anchor: { x: 0.5, y: 0.5 },
    span: 100,
    unit: "metres",
    grid: "square",
    columns: 20,
    shapes: [],
    tokens: [],
  };
  const [d, setD] = useState<Drawing>(initial),
    [level, setLevel] = useState(0),
    [kind, setKind] = useState("room"),
    [name, setName] = useState(""),
    [points, setPoints] = useState<Point[]>([]),
    [x, setX] = useState(50),
    [y, setY] = useState(50),
    [entity, setEntity] = useState(""),
    [tokenID, setTokenID] = useState(""),
    [facing, setFacing] = useState(0),
    [error, setError] = useState("");
  const entities = Object.values(state.records).filter(
    (r) => r.kind === "entity",
  );
  const ratio = map.height! / map.width!;
  const height = 800 * ratio;
  const dirty = JSON.stringify(d) !== JSON.stringify(initial);
  const pending = dirty || points.length > 0;
  useEffect(() => {
    onPending?.(pending);
    return () => onPending?.(false);
  }, [pending, onPending]);
  useEffect(() => {
    const guard = (e: BeforeUnloadEvent) => {
      if (pending) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [pending]);
  const point = (p: Point) => {
    const snap = (n: number) =>
      Math.max(
        0,
        Math.min(
          1,
          d.grid === "square" ? Math.round(n * d.columns) / d.columns : n,
        ),
      );
    setPoints([...points, { x: snap(p.x), y: snap(p.y) }]);
  };
  const update = (key: keyof Drawing, value: unknown) =>
    setD({ ...d, [key]: value });
  const lines = [];
  if (d.grid === "square") {
    for (let i = 1; i < d.columns; i++)
      lines.push(
        <path
          key={i}
          d={`M${(800 * i) / d.columns} 0V${height} M0 ${(height * i) / d.columns}H800`}
          stroke="#777"
          strokeOpacity=".25"
          fill="none"
        />,
      );
  }
  if (d.grid === "hex") {
    const size = 800 / (d.columns * 1.5 + 0.5);
    for (let col = 0; col < d.columns; col++) {
      for (
        let row = 0;
        row < Math.min(150, Math.ceil(height / (Math.sqrt(3) * size)));
        row++
      ) {
        const cx = size + col * 1.5 * size,
          cy = (row + 0.5 + (col % 2) / 2) * Math.sqrt(3) * size;
        lines.push(
          <polygon
            key={`${col}-${row}`}
            points={Array.from(
              { length: 6 },
              (_, i) =>
                `${cx + size * Math.cos((i * Math.PI) / 3)},${cy + size * Math.sin((i * Math.PI) / 3)}`,
            ).join(" ")}
            fill="none"
            stroke="#777"
            strokeOpacity=".25"
          />,
        );
      }
    }
  }
  return (
    <section className="card">
      <div className="two-col">
        <label>
          Parent map
          <select
            aria-label="Parent map"
            value={d.parent || ""}
            onChange={(e) => update("parent", e.target.value)}
          >
            <option value="">No parent</option>
            {maps
              .filter((r) => r.id !== map.id)
              .map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
          </select>
        </label>
        <label>
          Map width in units
          <input
            aria-label="Map width in units"
            type="number"
            min="0.001"
            max="1000000000000"
            step="any"
            value={d.span}
            onChange={(e) => update("span", Number(e.target.value))}
          />
        </label>
        <label>
          Distance unit
          <input
            value={d.unit}
            onChange={(e) => update("unit", e.target.value)}
          />
        </label>
        <label>
          Grid
          <select
            aria-label="Grid"
            value={d.grid}
            onChange={(e) => update("grid", e.target.value)}
          >
            {["none", "square", "hex"].map((v) => (
              <option key={v}>{v}</option>
            ))}
          </select>
        </label>
        <label>
          Grid columns
          <input
            type="number"
            min="2"
            max="100"
            value={d.columns}
            onChange={(e) =>
              update(
                "columns",
                Math.max(2, Math.min(100, Number(e.target.value))),
              )
            }
          />
        </label>
        <label>
          Floor / level
          <input
            type="number"
            min="-100"
            max="100"
            value={level}
            onChange={(e) => setLevel(Number(e.target.value))}
          />
        </label>
        {(["x", "y"] as const).map((axis) => (
          <label key={axis}>
            Parent anchor {axis} %
            <input
              type="number"
              min="0"
              max="100"
              value={d.anchor[axis] * 100}
              onChange={(e) =>
                update("anchor", {
                  ...d.anchor,
                  [axis]: Number(e.target.value) / 100,
                })
              }
            />
          </label>
        ))}
      </div>
      <svg
        className="local-drawing"
        viewBox={`0 0 800 ${height}`}
        role="img"
        aria-label={`Drawing for ${map.name}, floor ${level}. Use point controls below for keyboard editing.`}
        onClick={(e) => {
          const box = e.currentTarget.getBoundingClientRect();
          point({
            x: (e.clientX - box.left) / box.width,
            y: (e.clientY - box.top) / box.height,
          });
        }}
      >
        <rect width="800" height={height} fill="#e8dfcb" />
        {map.asset && (
          <image
            href={`/api/v1/projects/${state.root.world.id}/assets/${map.asset}`}
            width="800"
            height={height}
            preserveAspectRatio="none"
          />
        )}
        {lines}
        {d.shapes
          .filter((s) => s.level === level)
          .map((s) =>
            s.kind === "room" ? (
              <polygon
                key={s.id}
                points={s.points
                  .map((p) => `${p.x * 800},${p.y * height}`)
                  .join(" ")}
                fill="#bfb190"
                stroke="#51463b"
                strokeWidth="3"
              >
                <title>{s.name}</title>
              </polygon>
            ) : (
              <polyline
                key={s.id}
                points={s.points
                  .map((p) => `${p.x * 800},${p.y * height}`)
                  .join(" ")}
                fill="none"
                stroke={s.kind === "path" ? "#a14b35" : "#51463b"}
                strokeWidth={s.kind === "wall" ? 5 : 2}
                strokeDasharray={s.kind === "path" ? "6 4" : undefined}
              >
                <title>{s.name}</title>
              </polyline>
            ),
          )}
        {d.tokens
          .filter((t) => t.level === level)
          .map((t) => (
            <g key={t.id} transform={`translate(${t.x * 800} ${t.y * height})`}>
              <circle r="9" fill="#526a7e" />
              <path
                d="M0 -14L-4 -7H4Z"
                fill="#aa4b33"
                transform={`rotate(${t.facing})`}
              />
              <text x="12" y="4" fontSize="14">
                {state.records[t.entity]?.name}
              </text>
            </g>
          ))}
        <polyline
          points={points.map((p) => `${p.x * 800},${p.y * height}`).join(" ")}
          fill="none"
          stroke="#c56c2c"
          strokeWidth="3"
        />
        {points.map((p, i) => (
          <circle
            key={i}
            cx={p.x * 800}
            cy={p.y * height}
            r="4"
            fill="#c56c2c"
          />
        ))}
      </svg>
      <div className="two-col">
        <section>
          <h3>Draw a shape</h3>
          <label>
            Drawing tool
            <select
              aria-label="Drawing tool"
              value={kind}
              onChange={(e) => setKind(e.target.value)}
            >
              {["room", "wall", "path"].map((k) => (
                <option key={k}>{k}</option>
              ))}
            </select>
          </label>
          <label>
            Shape name
            <input
              aria-label="Shape name"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <div className="two-col">
            <label>
              Point X %
              <input
                aria-label="Point X %"
                type="number"
                min="0"
                max="100"
                value={x}
                onChange={(e) => setX(Number(e.target.value))}
              />
            </label>
            <label>
              Point Y %
              <input
                aria-label="Point Y %"
                type="number"
                min="0"
                max="100"
                value={y}
                onChange={(e) => setY(Number(e.target.value))}
              />
            </label>
          </div>
          <button
            onClick={() => point({ x: x / 100, y: y / 100 })}
            disabled={points.length >= 200}
          >
            Add point
          </button>
          <button
            onClick={() => setPoints(points.slice(0, -1))}
            disabled={!points.length}
          >
            Undo point
          </button>
          <p>
            {points.length} points · path length{" "}
            {points
              .reduce(
                (sum, p, i) =>
                  i
                    ? sum +
                      Math.hypot(
                        p.x - points[i - 1].x,
                        (p.y - points[i - 1].y) * ratio,
                      ) *
                        d.span
                    : sum,
                0,
              )
              .toFixed(2)}{" "}
            {d.unit}
          </p>
          <button
            disabled={!name.trim() || points.length < (kind === "room" ? 3 : 2)}
            onClick={() => {
              update("shapes", [
                ...d.shapes,
                { id: newID(), name, kind, points, level },
              ]);
              setName("");
              setPoints([]);
            }}
          >
            Add shape to draft
          </button>
        </section>
        <section>
          <h3>Place or move a token</h3>
          <label>
            Existing token
            <select
              aria-label="Existing token"
              value={tokenID}
              onChange={(e) => {
                setTokenID(e.target.value);
                const t = d.tokens.find((t) => t.id === e.target.value);
                if (t) {
                  setEntity(t.entity);
                  setX(t.x * 100);
                  setY(t.y * 100);
                  setLevel(t.level);
                  setFacing(t.facing);
                }
              }}
            >
              <option value="">New token</option>
              {d.tokens.map((t) => (
                <option key={t.id} value={t.id}>
                  {state.records[t.entity]?.name} · floor {t.level}
                </option>
              ))}
            </select>
          </label>
          <label>
            Token entity
            <select
              aria-label="Token entity"
              value={entity}
              onChange={(e) => setEntity(e.target.value)}
            >
              <option value="">Choose entity</option>
              {entities.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Facing degrees
            <input
              type="number"
              min="0"
              max="359"
              value={facing}
              onChange={(e) => setFacing(Number(e.target.value))}
            />
          </label>
          <p>
            Uses Point X / Y and the selected floor. Multiple tokens may
            represent the same force.
          </p>
          <button
            disabled={!entity}
            onClick={() => {
              const token = {
                id: tokenID || newID(),
                entity,
                x: x / 100,
                y: y / 100,
                level,
                facing,
              };
              update("tokens", [
                ...d.tokens.filter((t) => t.id !== token.id),
                token,
              ]);
              setTokenID(token.id);
            }}
          >
            Place token in draft
          </button>
        </section>
      </div>
      <h3>Drawing objects</h3>
      <ul>
        {d.shapes.map((s) => (
          <li key={s.id}>
            {s.name} · {s.kind} · floor {s.level}{" "}
            <button
              onClick={() =>
                update(
                  "shapes",
                  d.shapes.filter((v) => v.id !== s.id),
                )
              }
            >
              Remove {s.name}
            </button>
          </li>
        ))}
        {d.tokens.map((t) => (
          <li key={t.id}>
            {state.records[t.entity]?.name} · {Math.round(t.x * 100)}%,{" "}
            {Math.round(t.y * 100)}% · floor {t.level}{" "}
            <button
              onClick={() =>
                update(
                  "tokens",
                  d.tokens.filter((v) => v.id !== t.id),
                )
              }
            >
              Remove token
            </button>
          </li>
        ))}
      </ul>
      {error && <p role="alert">{error}</p>}
      <button
        disabled={busy || state.readOnly || !dirty}
        onClick={async () => {
          setError("");
          if (
            await run({
              action: "put",
              record: {
                ...map,
                properties: { ...map.properties, _localMap: d },
              },
            })
          )
            setError("");
          else setError("Drawing was not saved. Your draft is still here.");
        }}
      >
        Save local map plan
      </button>
      <button
        disabled={!pending}
        onClick={() => {
          setD(initial);
          setPoints([]);
        }}
      >
        Discard drawing changes
      </button>
      <p role="status">
        {pending ? "Unsaved drawing draft" : "Drawing matches saved map"}
      </p>
    </section>
  );
}
