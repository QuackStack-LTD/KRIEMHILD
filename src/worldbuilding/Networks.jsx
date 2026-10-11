import Dialog from "./ui/Dialog.jsx";
import { useRef, useState } from "react";
import {
  activeEntities,
  entityIn,
  label,
  registry,
  connectionPorts,
  portPoint,
  edgeGeometry,
  familyJunctions,
  isFamilyRelation,
} from "./model.js";
import {
  peopleGraph,
  connectionKinds,
  reconnectRelations,
} from "./connections.js";
import EntityGraphIcon from "./EntityGraphIcon.jsx";
import CharacterRelationEditor from "./CharacterRelationEditor.jsx";
import ConnectionEditor from "./ConnectionEditor.jsx";

function nodeName(name) {
  const words = name.split(/\s+/);
  let first = words.shift() || "";
  while (words.length && first.length + words[0].length < 12)
    first += " " + words.shift();
  const trim = (s) => (s.length > 13 ? s.slice(0, 12) + "..." : s);
  return [trim(first), trim(words.join(" "))];
}

export default function Networks({
  doc,
  age,
  query = new URLSearchParams(),
  command,
  onProfile,
  onBack,
  combined = false,
}) {
  const mode = combined
    ? "all"
    : query.get("mode") ||
      (query.get("genealogy") === "1" ? "family" : "social");
  const [center, setCenter] = useState(query.get("center") || ""),
    [search, setSearch] = useState(""),
    [type, setType] = useState(""),
    [hidden, setHidden] = useState([]),
    [extras, setExtras] = useState([]),
    [zoom, setZoom] = useState(1),
    [edge, setEdge] = useState(null),
    [positions, setPositions] = useState({}),
    [draft, setDraft] = useState(null),
    [status, setStatus] = useState(""),
    [create, setCreate] = useState(false),
    [name, setName] = useState(""),
    [keyboardPort, setKeyboardPort] = useState(null),
    [transfer, setTransfer] = useState(null);
  const host = useRef(null),
    svg = useRef(null),
    gesture = useRef(null),
    all = activeEntities(doc, age.id),
    characters = all.filter((e) => e.kind === "character");
  const data = peopleGraph(doc, age.id, {
    mode,
    center,
    search,
    type,
    extras,
    hidden,
  });
  for (const [id, p] of Object.entries(positions))
    if (data.positions.has(id))
      data.positions.set(id, { ...data.positions.get(id), ...p });
  data.width = Math.max(
    data.width,
    ...[...data.positions.values()].map((p) => p.x + 180),
  );
  data.height = Math.max(
    data.height,
    ...[...data.positions.values()].map((p) => p.y + 140),
  );
  const junctions = familyJunctions(data),
    pairs = new Map(),
    colors = {
      systems: "#c2b4e5",
      genealogical: "#d9b983",
      social: "#a9b9df",
      organizational: "#8cc7b3",
      geographical: "#88bcc8",
      historical: "#ca9ece",
      cultural: "#d0b1d9",
      political: "#ccab91",
    };
  const point = (ev) => {
    const m = svg.current?.getScreenCTM();
    if (!m) return { x: 0, y: 0 };
    return new DOMPoint(ev.clientX, ev.clientY).matrixTransform(m.inverse());
  };
  function connect(from, to) {
    let a = entityIn(doc, age.id, from.id),
      b = entityIn(doc, age.id, to.id),
      kinds = connectionKinds(a, b, mode);
    if (!kinds.length) {
      kinds = connectionKinds(b, a, mode);
      if (kinds.length) {
        [a, b] = [b, a];
        [from, to] = [to, from];
      }
    }
    if (!kinds.length) {
      setStatus(
        "These nodes have no compatible personal connection. Manage places in the builders and institutions in Organizations.",
      );
      return;
    }
    setEdge({
      source: a.id,
      destination: b.id,
      sourcePort: from.port,
      destinationPort: to.port,
      kind: kinds.includes("friend")
        ? "friend"
        : kinds.includes("child-of")
          ? "child-of"
          : kinds[0],
      description: "",
    });
    setKeyboardPort(null);
  }
  function attached(id, port) {
    const at = portPoint(data.positions.get(id), port),
      found = [],
      counts = new Map();
    for (const r of data.edges) {
      const key = [r.source, r.destination].sort().join("/"),
        i = counts.get(key) || 0;
      counts.set(key, i + 1);
      if (junctions.hidden.has(r.id) || r.automatic) continue;
      const g = edgeGeometry(
          data.positions.get(r.source),
          data.positions.get(r.destination),
          r,
          i,
        ),
        end = r.source === id ? g.start : r.destination === id ? g.end : null;
      if (end && Math.hypot(end.x - at.x, end.y - at.y) < 1)
        found.push({
          records: [r],
          name:
            label(r.kind) +
            " ? " +
            entityIn(doc, age.id, r.source === id ? r.destination : r.source)
              ?.name,
          start: r.source === id ? g.end : g.start,
        });
    }
    for (const j of junctions.links)
      if (j.child === id && Math.hypot(j.end.x - at.x, j.end.y - at.y) < 1)
        found.push({
          records: j.records,
          name: "Parentage of " + entityIn(doc, age.id, id)?.name,
          start: j.start,
        });
    for (const u of junctions.unions) {
      if (u.partner || !u.parents.includes(id)) continue;
      const end = id === u.parents[0] ? u.geometry.start : u.geometry.end;
      if (Math.hypot(end.x - at.x, end.y - at.y) < 1) {
        const records = junctions.links
          .filter((l) => l.parents.join("/") === u.parents.join("/"))
          .flatMap((l) =>
            l.records.filter((r) => r.source === id || r.destination === id),
          );
        found.push({
          records,
          name: "Parent connections of " + entityIn(doc, age.id, id)?.name,
          start: id === u.parents[0] ? u.geometry.end : u.geometry.start,
        });
      }
    }
    return found;
  }
  async function reconnect(item, from, to) {
    if (from.id === to.id && from.port === to.port) return;
    const changes = reconnectRelations(item.records, from.id, to);
    if (changes.some((r) => r.source === r.destination)) {
      setStatus("A connection cannot point to its own entity.");
      return;
    }
    if (
      await command({ kind: "relations", ageId: age.id, relations: changes })
    ) {
      setStatus("Connection transferred.");
      setTransfer(null);
    } else
      setStatus(
        "Connection unchanged: the selected endpoint is incompatible or contradicts an existing relationship.",
      );
  }
  function start(ev, id, port) {
    if (ev.button !== 0) return;
    ev.preventDefault();
    ev.stopPropagation();
    const p = point(ev);
    gesture.current = port
      ? {
          kind: "port",
          from: { id, port },
          start: portPoint(data.positions.get(id), port),
        }
      : id
        ? {
            kind: "node",
            id,
            origin: data.positions.get(id),
            start: p,
            moved: false,
          }
        : {
            kind: "pan",
            x: ev.clientX,
            y: ev.clientY,
            left: host.current.scrollLeft,
            top: host.current.scrollTop,
          };
    if (port) {
      const links = ev.shiftKey ? [] : attached(id, port);
      gesture.current.links = links;
      if (links.length === 1) gesture.current.start = links[0].start;
      setDraft({ ...gesture.current, point: p });
    }
    ev.currentTarget.setPointerCapture(ev.pointerId);
  }
  function move(ev) {
    const g = gesture.current;
    if (!g) return;
    const p = point(ev);
    if (g.kind === "port") setDraft({ ...g, point: p });
    else if (g.kind === "pan") {
      host.current.scrollLeft = g.left + g.x - ev.clientX;
      host.current.scrollTop = g.top + g.y - ev.clientY;
    } else {
      const dx = p.x - g.start.x,
        dy = p.y - g.start.y;
      if (Math.hypot(dx, dy) > 5) g.moved = true;
      if (g.moved) {
        g.next = {
          x: Math.max(110, g.origin.x + dx),
          y: Math.max(65, g.origin.y + dy),
        };
        setPositions((v) => ({ ...v, [g.id]: g.next }));
      }
    }
  }
  async function finish(ev) {
    const g = gesture.current;
    gesture.current = null;
    setDraft(null);
    if (!g) return;
    if (g.kind === "port") {
      const target = document
        .elementFromPoint(ev.clientX, ev.clientY)
        ?.closest("[data-port]");
      if (target && svg.current.contains(target)) {
        const to = { id: target.dataset.entity, port: target.dataset.port };
        if (g.links?.length === 1) await reconnect(g.links[0], g.from, to);
        else if (g.links?.length)
          setTransfer({ items: g.links, from: g.from, to });
        else connect(g.from, to);
      }
    } else if (g.kind === "node") {
      const e = entityIn(doc, age.id, g.id);
      if (!g.moved) onProfile(e);
      else {
        const ok = await command({
          kind: "entity",
          ageId: age.id,
          entity: {
            ...e,
            properties: { ...e.properties, graphPosition: g.next },
          },
        });
        if (!ok) {
          setPositions((p) => {
            const next = { ...p };
            delete next[g.id];
            return next;
          });
          setStatus("Position could not be saved.");
        }
      }
    }
  }
  function cancel() {
    const g = gesture.current;
    if (g?.kind === "node")
      setPositions((v) => {
        const next = { ...v };
        delete next[g.id];
        return next;
      });
    gesture.current = null;
    setDraft(null);
    setKeyboardPort(null);
  }
  const title = {
    all: "Characters",
    family: "Family Graph",
    social: "Relationship Graph",
    places: "Places & Affiliations",
  }[mode];
  function renderEdge(r, geometry) {
    return (
      <g
        key={r.id}
        role="button"
        tabIndex={0}
        aria-label={`${label(r.kind)}: ${entityIn(doc, age.id, r.source)?.name} to ${entityIn(doc, age.id, r.destination)?.name}`}
        onClick={() => setEdge(r)}
        onKeyDown={(e) => {
          if (e.key === "Enter") setEdge(r);
        }}
      >
        <path
          d={geometry.path}
          fill="none"
          stroke={colors[registry.relations[r.kind]?.family] || "#a9b9df"}
          strokeWidth="2"
          strokeDasharray={r.automatic ? "5 4" : undefined}
        />
        <path
          d={geometry.path}
          fill="none"
          stroke="transparent"
          strokeWidth="16"
        />
        <text
          className="relationship-edge-label"
          x={geometry.label.x}
          y={geometry.label.y - 5}
          textAnchor="middle"
        >
          {label(r.kind)}
        </text>
      </g>
    );
  }
  return (
    <main className="world-page workspace relationship-workspace">
      <div className="workspace-heading">
        <div>
          <span className="eyebrow">
            {doc.world.name} / {age.name}
          </span>
          <h1>{title}</h1>
        </div>
        <div className="history-actions">
          {combined ? (
            <button
              className="primary"
              onClick={() => {
                setName("");
                setCreate(true);
              }}
            >
              Create Character
            </button>
          ) : (
            <button onClick={onBack}>Back to Characters</button>
          )}
        </div>
      </div>
      <div className="collection-toolbar">
        <input
          type="text"
          aria-label="Search graph"
          placeholder="Find a character or connection"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <label>
          Focus
          <select value={center} onChange={(e) => setCenter(e.target.value)}>
            <option value="">All characters</option>
            {characters.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Connections
          <select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="">All relevant types</option>
            {[
              ...new Set(
                peopleGraph(doc, age.id, { mode }).edges.map((r) => r.kind),
              ),
            ].map((k) => (
              <option key={k} value={k}>
                {label(k)}
              </option>
            ))}
          </select>
        </label>
        <label>
          Zoom
          <input
            aria-label="Character graph zoom"
            type="range"
            min=".35"
            max="2"
            step=".05"
            value={zoom}
            onChange={(e) => setZoom(+e.target.value)}
          />
        </label>
        {
          <label>
            Add an existing node
            <select
              value=""
              onChange={(e) =>
                setExtras((v) => [...new Set([...v, e.target.value])])
              }
            >
              <option value="">Choose a character, place or affiliation</option>
              {all
                .filter(
                  (e) =>
                    e.kind === "character" ||
                    characters.some((c) => connectionKinds(c, e, mode).length),
                )
                .map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.name} · {label(e.kind)}
                  </option>
                ))}
            </select>
          </label>
        }
        <details className="graph-filters">
          <summary>Visible characters</summary>
          {characters.map((e) => (
            <label key={e.id}>
              <input
                type="checkbox"
                checked={!hidden.includes(e.id)}
                onChange={(ev) =>
                  setHidden((v) =>
                    ev.target.checked
                      ? v.filter((id) => id !== e.id)
                      : [...v, e.id],
                  )
                }
              />
              {e.name}
            </label>
          ))}
        </details>
      </div>
      <div className="graph-legend">
        {Object.entries(colors)
          .filter(([k]) =>
            data.edges.some((r) => registry.relations[r.kind]?.family === k),
          )
          .map(([k, c]) => (
            <span key={k}>
              <i style={{ background: c }} />
              {k}
            </span>
          ))}
        <span>
          Click a node to open its profile. Drag nodes to arrange them; drag
          free dots to connect or occupied ends to transfer. Shift-drag always
          creates a connection. Drag empty space to pan.
        </span>
      </div>
      <output role="status">{status}</output>
      <div
        className="relationship-canvas combined-people-canvas"
        ref={host}
        tabIndex={0}
        onPointerDown={(ev) => {
          if (!ev.target.closest('[role="button"]')) start(ev);
        }}
        onPointerMove={move}
        onPointerUp={finish}
        onPointerCancel={cancel}
        onKeyDown={(e) => {
          if (e.key === "Escape") cancel();
        }}
      >
        <svg
          ref={svg}
          width={data.width * zoom}
          height={data.height * zoom}
          viewBox={`0 0 ${data.width} ${data.height}`}
          aria-label={title}
        >
          {data.edges.map((r) => {
            const key = [r.source, r.destination].sort().join("/"),
              index = pairs.get(key) || 0;
            pairs.set(key, index + 1);
            return junctions.hidden.has(r.id)
              ? null
              : renderEdge(
                  r,
                  edgeGeometry(
                    data.positions.get(r.source),
                    data.positions.get(r.destination),
                    r,
                    index,
                  ),
                );
          })}
          {junctions.unions
            .filter((u) => !u.partner)
            .map((u) => (
              <path
                key={u.key}
                d={u.geometry.path}
                fill="none"
                stroke={colors.genealogical}
                strokeWidth="2"
              >
                <title>Co-parents</title>
              </path>
            ))}
          {junctions.links.map((l) => (
            <g
              key={l.child}
              role="button"
              tabIndex={0}
              aria-label={`Parentage of ${entityIn(doc, age.id, l.child)?.name}`}
              onClick={() => setEdge(l.relation)}
              onKeyDown={(e) => {
                if (e.key === "Enter") setEdge(l.relation);
              }}
            >
              <path
                d={l.path}
                fill="none"
                stroke={colors.genealogical}
                strokeWidth="2"
              />
              <path
                d={l.path}
                fill="none"
                stroke="transparent"
                strokeWidth="16"
              />
              <circle
                cx={l.start.x}
                cy={l.start.y}
                r="3"
                fill={colors.genealogical}
              />
            </g>
          ))}
          {data.nodes.map((e) => {
            const p = data.positions.get(e.id),
              portrait = e.properties?.portrait || e.properties?.emblem;
            return (
              <g key={e.id}>
                <g
                  className="person-node"
                  role="button"
                  tabIndex={0}
                  aria-label={`Open profile ${e.name}`}
                  onPointerDown={(ev) => start(ev, e.id)}
                  onKeyDown={(ev) => {
                    if (ev.key === "Enter") onProfile(e);
                  }}
                >
                  {e.kind !== "character" ? (
                    <>
                      <rect
                        className="entity-node-square"
                        x={p.x - p.width / 2}
                        y={p.y - p.height / 2}
                        width={p.width}
                        height={p.height}
                        rx="7"
                      />
                      <g
                        transform={`translate(${p.x - 16},${p.y - 29})`}
                        pointerEvents="none"
                      >
                        <EntityGraphIcon kind={e.kind} />
                      </g>
                      <text
                        className="entity-node-name"
                        textAnchor="middle"
                        x={p.x}
                        y={p.y + 17}
                        pointerEvents="none"
                      >
                        {nodeName(e.name)[0]}
                      </text>
                      <text
                        className="entity-node-name"
                        textAnchor="middle"
                        x={p.x}
                        y={p.y + 29}
                        pointerEvents="none"
                      >
                        {nodeName(e.name)[1]}
                      </text>
                    </>
                  ) : (
                    <>
                      <rect
                        x={p.x - 80}
                        y={p.y - 45}
                        width="160"
                        height="90"
                        rx="8"
                      />
                      {portrait ? (
                        <image
                          href={portrait}
                          x={p.x - 71}
                          y={p.y - 34}
                          width="38"
                          height="68"
                          preserveAspectRatio="xMidYMid slice"
                          pointerEvents="none"
                        />
                      ) : (
                        <g pointerEvents="none">
                          <rect
                            className="node-placeholder"
                            x={p.x - 71}
                            y={p.y - 34}
                            width="38"
                            height="68"
                            rx="3"
                          />
                          <circle
                            cx={p.x - 52}
                            cy={p.y - 12}
                            r="9"
                            fill="#adad9c"
                          />
                          <path
                            d={`M${p.x - 68} ${p.y + 25}q0 -25 16 -25t16 25`}
                            fill="#adad9c"
                          />
                        </g>
                      )}
                      <text pointerEvents="none" x={p.x - 25} y={p.y - 8}>
                        {e.name.length > 12
                          ? e.name.slice(0, 11) + "…"
                          : e.name}
                      </text>
                      <text
                        pointerEvents="none"
                        className="person-node-status"
                        x={p.x - 25}
                        y={p.y + 14}
                      >
                        {label(e.kind)}
                      </text>
                    </>
                  )}
                  <title>{e.name}</title>
                </g>
                {connectionPorts.map((port) => {
                  const at = portPoint(p, port);
                  return (
                    <g
                      key={port}
                      data-entity={e.id}
                      data-port={port}
                      className="character-port"
                      role="button"
                      tabIndex={0}
                      aria-label={`Connect ${e.name} ${port}`}
                      onPointerDown={(ev) => start(ev, e.id, port)}
                      onKeyDown={(ev) => {
                        if (ev.key === "Enter") {
                          const next = { id: e.id, port };
                          if (keyboardPort) connect(keyboardPort, next);
                          else {
                            setKeyboardPort(next);
                            setStatus(
                              "Select another connection dot and press Enter.",
                            );
                          }
                        }
                      }}
                    >
                      <circle cx={at.x} cy={at.y} r="10" fill="transparent" />
                      <circle
                        className="port-dot"
                        cx={at.x}
                        cy={at.y}
                        r="3.7"
                      />
                    </g>
                  );
                })}
              </g>
            );
          })}
          {draft && (
            <line
              x1={draft.start.x}
              y1={draft.start.y}
              x2={draft.point.x}
              y2={draft.point.y}
              stroke="#f0d9a0"
              strokeWidth="2"
              pointerEvents="none"
            />
          )}
        </svg>
        {!data.nodes.length && (
          <p className="workspace-empty">
            Create a character or adjust your filters.
          </p>
        )}
      </div>
      {edge &&
        (entityIn(doc, age.id, edge.source)?.kind === "character" &&
        entityIn(doc, age.id, edge.destination)?.kind === "character" ? (
          <CharacterRelationEditor
            doc={doc}
            age={age.id}
            relation={edge}
            genealogy={isFamilyRelation(edge)}
            mixed={mode === "all"}
            command={command}
            onClose={() => setEdge(null)}
          />
        ) : (
          <ConnectionEditor
            doc={doc}
            age={age.id}
            relation={edge}
            command={command}
            graph
            onClose={() => setEdge(null)}
          />
        ))}
      {transfer && (
        <Dialog
          title={<>Transfer connection</>}
          size="compact"
          className=""
          onClose={() => setTransfer(null)}
          actions={
            <>
              <button onClick={() => setTransfer(null)} data-dialog-dismiss>
                Cancel
              </button>
            </>
          }
        >
          <p>Several connections share this dot. Choose the one to transfer.</p>
          {transfer.items.map((item, i) => (
            <button
              key={i}
              onClick={() => reconnect(item, transfer.from, transfer.to)}
            >
              {item.name}
            </button>
          ))}
          <button
            onClick={() => {
              connect(transfer.from, transfer.to);
              setTransfer(null);
            }}
          >
            Create a new connection instead
          </button>
        </Dialog>
      )}
      {create && (
        <Dialog
          title={<>Create Character</>}
          size="compact"
          className=""
          onClose={() => setCreate(false)}
          onSubmit={async (ev) => {
            ev.preventDefault();
            if (
              await command({
                kind: "entity",
                ageId: age.id,
                entity: {
                  id: crypto.randomUUID(),
                  kind: "character",
                  name,
                  properties: {},
                },
              })
            ) {
              setCreate(false);
              setSearch("");
              setCenter("");
            }
          }}
          actions={
            <>
              <button
                type="button"
                onClick={() => setCreate(false)}
                data-dialog-dismiss
              >
                Cancel
              </button>
              <button className="primary">Create</button>
            </>
          }
        >
          <label>
            Name
            <input
              type="text"
              autoFocus
              required
              maxLength={200}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
        </Dialog>
      )}
    </main>
  );
}
