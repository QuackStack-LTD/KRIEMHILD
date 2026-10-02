"use client";
import { useState } from "react";
import { api, Entry, State } from "../lib/types";
type Graph = {
  nodes: { id: string; name: string; type: string; depth: number }[];
  edges: Entry[];
  truncated: boolean;
  unresolved: string[];
};
export default function FamilyGraph({ state }: { state: State }) {
  const [start, setStart] = useState(""),
    [direction, setDirection] = useState("descendants"),
    [depth, setDepth] = useState(4),
    [tick, setTick] = useState(""),
    [roles, setRoles] = useState<string[]>([]),
    [graph, setGraph] = useState<Graph | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const entities = Object.values(state.records).filter(
      (r) => r.kind === "entity",
    ),
    available = [
      ...new Set(
        Object.values(state.records)
          .filter((r) => r.kind === "relation")
          .map((r) => r.name),
      ),
    ].sort();
  const counts: Record<number, number> = {};
  const positions: Record<string, { x: number; y: number }> = {};
  for (const n of graph?.nodes || []) {
    const row = counts[n.depth] || 0;
    counts[n.depth] = row + 1;
    positions[n.id] = { x: 30 + n.depth * 230, y: 35 + row * 90 };
  }
  const height = Math.max(
      160,
      ...Object.values(counts).map((n) => n * 90 + 30),
    ),
    width = Math.max(
      500,
      ...(graph?.nodes || []).map((n) => n.depth * 230 + 240),
    );
  return (
    <div>
      <div className="eyebrow">Relationships / {state.age.name}</div>
      <h2>Families & relationship paths</h2>
      <p>
        Choose the roles that define this view. Biological, adoptive, guardian,
        ritual and disputed links can coexist. Shared ancestors retain one
        identity, and cycles remain visible.
      </p>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            setGraph(
              await api<Graph>(`/projects/${state.root.world.id}/graph`, {
                method: "POST",
                body: JSON.stringify({
                  snapshot: state.age.snapshot,
                  start,
                  direction,
                  depth,
                  tick,
                  roles,
                }),
              }),
            );
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <div className="two-col">
          <label>
            Starting person or entity
            <select
              aria-label="Starting person or entity"
              value={start}
              onChange={(e) => setStart(e.target.value)}
            >
              <option value="">Choose</option>
              {entities.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Relationship direction
            <select
              aria-label="Relationship direction"
              value={direction}
              onChange={(e) => setDirection(e.target.value)}
            >
              <option value="descendants">
                Follow outgoing roles (parent to child)
              </option>
              <option value="ancestors">
                Follow incoming roles (child to parent)
              </option>
              <option value="connected">Follow both directions</option>
            </select>
          </label>
          <label>
            Generations / steps
            <input
              type="number"
              min="1"
              max="8"
              value={depth}
              onChange={(e) => setDepth(Number(e.target.value))}
            />
          </label>
          <label>
            Optional historical tick
            <input value={tick} onChange={(e) => setTick(e.target.value)} />
          </label>
        </div>
        <details open>
          <summary>Relationship roles (none selected means all)</summary>
          {available.map((role) => (
            <label className="check-label" key={role}>
              <input
                type="checkbox"
                checked={roles.includes(role)}
                onChange={(e) =>
                  setRoles(
                    e.target.checked
                      ? [...roles, role]
                      : roles.filter((v) => v !== role),
                  )
                }
              />
              {role}
            </label>
          ))}
        </details>
        <button disabled={busy || !start}>Trace relationships</button>
      </form>
      {error && <p role="alert">{error}</p>}
      {graph && (
        <section>
          <h3>{graph.nodes.length} connected identities</h3>
          {graph.unresolved.map((u, i) => (
            <p key={i} className="notice">
              {u}
            </p>
          ))}
          {graph.truncated && (
            <p className="notice">
              This view reached its 200-identity or 500-link limit. Narrow the
              roles or start from another person.
            </p>
          )}
          <div style={{ overflow: "auto", maxHeight: "65vh" }}>
            <svg
              width={width}
              height={height}
              role="img"
              aria-label="Relationship graph. The equivalent named links are listed below."
            >
              {graph.edges.map((r) => {
                const a = positions[r.from!],
                  b = positions[r.to!];
                return (
                  <path
                    key={r.id}
                    d={`M${a.x + 85} ${a.y + 25} Q${(a.x + b.x) / 2 + 85} ${Math.min(a.y, b.y) - 30} ${b.x + 85} ${b.y + 25}`}
                    fill="none"
                    stroke="#94754e"
                    strokeWidth="2"
                  >
                    <title>
                      {graph.nodes.find((n) => n.id === r.from)?.name} →{" "}
                      {r.name} → {graph.nodes.find((n) => n.id === r.to)?.name}
                    </title>
                  </path>
                );
              })}
              {graph.nodes.map((n) => (
                <g
                  key={n.id}
                  transform={`translate(${positions[n.id].x} ${positions[n.id].y})`}
                >
                  <rect
                    width="180"
                    height="54"
                    rx="7"
                    fill="#f4ecda"
                    stroke="#867557"
                  />
                  <text x="9" y="23" fontSize="14">
                    {n.name.length > 23 ? n.name.slice(0, 22) + "…" : n.name}
                  </text>
                  <text x="9" y="43" fontSize="11">
                    {n.type} · step {n.depth}
                  </text>
                  <title>{n.name}</title>
                </g>
              ))}
            </svg>
          </div>
          <ul>
            {graph.edges.map((r) => (
              <li key={r.id}>
                {graph.nodes.find((n) => n.id === r.from)?.name} → {r.name} →{" "}
                {graph.nodes.find((n) => n.id === r.to)?.name}
                {r.notes && ` — ${r.notes}`}
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
