import { useCallback, useEffect, useRef, useState } from "react";
import { request, defaults, presets } from "./api.js";
import TileSurface from "./TileSurface.jsx";
import { normalizeCamera } from "./camera.js";
import "./studio.css";

const uid = () => crypto.randomUUID().replaceAll("-", "");
const layerFor = (kind) =>
  ({
    road: "roads",
    border: "borders",
    label: "labels",
    landmark: "natural-features",
    house: "buildings",
    country: "countries",
  })[kind] || "settlements";
const initialCamera = { x: 0, y: 0, w: 2, h: 1 };
function download(blob, name) {
  const url = URL.createObjectURL(blob),
    a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}
const toMap = (p, state) => [(p[0] / (state.W - 1)) * 2, p[1] / (state.H - 1)];
const pointsText = (points, state) =>
  points.map((p) => toMap(p, state).join(",")).join(" ");

export default function Studio({ initialState, onExit }) {
  const [state, setState] = useState(initialState || null),
    [spec, setSpec] = useState(initialState?.continuous?.spec || defaults);
  const savedView =
    initialState?.projectUI?.continuousView ||
    initialState?.project?.view ||
    {};
  const [mode, setMode] = useState(
      savedView.mode ||
        (location.pathname.endsWith("/build") ? "map" : "generate"),
    ),
    [style, setStyle] = useState(savedView.style || "natural");
  const [camera, setCamera] = useState(normalizeCamera(savedView.camera)),
    [width, setWidth] = useState(1000);
  const [entities, setEntities] = useState([]),
    [rivers, setRivers] = useState([]),
    [selected, setSelected] = useState(null);
  const [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [job, setJob] = useState(null),
    [loading, setLoading] = useState(0);
  const [tool, setTool] = useState("inspect"),
    [draft, setDraft] = useState([]),
    [cursor, setCursor] = useState(null),
    [sample, setSample] = useState(null);
  const [radius, setRadius] = useState(100),
    [amount, setAmount] = useState(300),
    [name, setName] = useState("New feature");
  const [settlement, setSettlement] = useState(null),
    [settlementSpec, setSettlementSpec] = useState({
      name: "New town",
      kind: "town",
      archetype: "organic",
    });
  const [showRivers, setShowRivers] = useState(savedView.showRivers !== false),
    [panel, setPanel] = useState(true),
    [autoSave, setAutoSave] = useState(true);
  const [invalidations, setInvalidations] = useState([]);
  const svg = useRef(null),
    stage = useRef(null),
    drag = useRef(null),
    jobRef = useRef(null),
    active = useRef(state),
    accepted = useRef(initialState || null),
    cancelled = useRef(false),
    inspectAbort = useRef(null);
  active.current = state;
  const query = useRef(new URLSearchParams(location.search));
  const base = `sessions/${state?.id}`;
  const run = async (work) => {
    setError("");
    setBusy(true);
    try {
      return await work();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  };
  const refresh = useCallback(async (s = active.current) => {
    if (!s) return;
    const result = await request(`sessions/${s.id}/world/entities?detail=8`);
    setEntities(result.entities || []);
  }, []);
  useEffect(() => {
    if (!state?.id) return;
    let alive = true;
    refresh(state).catch((e) => setError(e.message));
    request(`sessions/${state.id}/terrain/features`)
      .then((r) => {
        if (alive) setRivers(r.rivers || []);
      })
      .catch((e) => setError(e.message));
    return () => {
      alive = false;
    };
  }, [state?.id, refresh]);
  useEffect(() => {
    const observer = new ResizeObserver((entries) =>
      setWidth(entries[0].contentRect.width),
    );
    if (stage.current) observer.observe(stage.current);
    return () => observer.disconnect();
  }, []);
  useEffect(
    () => () => {
      cancelled.current = true;
      inspectAbort.current?.abort();
      if (jobRef.current)
        request(
          `terrain/generation-jobs/${jobRef.current}`,
          undefined,
          "DELETE",
        ).catch(() => {});
    },
    [],
  );
  useEffect(() => {
    const warn = (e) => {
      if (active.current?.unsaved) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, []);
  async function command(body) {
    const s = active.current;
    if (!s) return;
    const result = await request(`sessions/${s.id}/world`, {
      ...body,
      revision: s.project.revision,
    });
    if (result.dirty) {
      const b = result.dirty;
      setInvalidations((v) => [
        ...v,
        {
          x: (b.x / (s.W - 1)) * 2,
          y: b.y / (s.H - 1),
          w: (b.width / (s.W - 1)) * 2,
          h: b.height / (s.H - 1),
          revision: result.project.revision,
        },
      ]);
    }
    setState((v) => ({
      ...v,
      project: result.project || v.project,
      unsaved: true,
    }));
    await refresh(s);
    return result;
  }
  async function save() {
    const s = active.current;
    if (!s) return;
    await request(`sessions/${s.id}/world`, {
      kind: "view",
      revision: s.project.revision,
      view: { camera, style, mode, showRivers },
    });
    const result = await request(`sessions/${s.id}/save`, {});
    setState((v) => ({
      ...v,
      project: result.project || v.project,
      unsaved: false,
    }));
  }
  useEffect(() => {
    if (!state?.id) return;
    const id = state.id,
      timer = setTimeout(
        () =>
          request(`sessions/${id}/view`, {
            ui: { continuousView: { camera, style, mode, showRivers } },
            merge: true,
          })
            .then((result) => {
              if (active.current?.id === id)
                setState((v) => ({ ...v, unsaved: result.unsaved }));
            })
            .catch((e) => setError(e.message)),
        700,
      );
    return () => clearTimeout(timer);
  }, [state?.id, camera, style, mode, showRivers]);
  useEffect(() => {
    if (!autoSave || !state?.historicalMap || !state.unsaved || busy) return;
    const timer = setTimeout(() => run(save), 30000);
    return () => clearTimeout(timer);
  }, [state?.project?.revision, state?.unsaved, autoSave, busy]);
  useEffect(() => {
    const key = (e) => {
      if (e.target.matches("input,textarea,select")) return;
      if (e.key === "Escape") {
        setDraft([]);
        setSelected(null);
        setSettlement(null);
        setTool("inspect");
      }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") {
        e.preventDefault();
        run(() => command({ kind: e.shiftKey ? "redo" : "undo" }));
      }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        run(save);
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  });
  async function generate() {
    setError("");
    setBusy(true);
    setSelected(null);
    setSettlement(null);
    cancelled.current = false;
    try {
      if (
        active.current?.unsaved &&
        (active.current.historicalMap || entities.length)
      )
        await save();
      const started = await request("terrain/generation-jobs", { spec });
      jobRef.current = started.id;
      while (!cancelled.current) {
        const progress = await request(`terrain/generation-jobs/${started.id}`);
        setJob(progress);
        if (progress.status === "failed") throw Error(progress.error);
        if (progress.status === "cancelled") break;
        if (progress.status === "ready") {
          const next = await request(
            `sessions/${progress.session}/world/state`,
          );
          if (active.current && !active.current.historicalMap)
            await request(`sessions/${active.current.id}`, undefined, "DELETE");
          setState(next);
          setCamera(initialCamera);
          setEntities([]);
          setInvalidations([]);
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 250));
      }
    } catch (e) {
      setError(e.message);
    } finally {
      jobRef.current = null;
      setBusy(false);
      setJob(null);
    }
  }
  async function accept() {
    const s = active.current;
    if (!s) return;
    let worldId = query.current.get("world"),
      ageId = query.current.get("age");
    if (!worldId) {
      const doc = await request("worlds", { name: "New World" });
      worldId = doc.world.id;
      ageId = doc.world.currentAge;
      query.current.set("world", worldId);
      query.current.set("age", ageId);
    }
    await request(`worlds/${worldId}/accept`, {
      session: s.id,
      ageId,
      name: `Terrain · ${s.continuous.spec.seed}`,
    });
    const next = await request(`sessions/${s.id}/world/state`);
    setState(next);
    accepted.current = next;
    setMode("map");
    setTool("inspect");
    history.replaceState(
      {},
      "",
      `/world/${next.worldId}/build?${query.current}`,
    );
  }
  async function discard() {
    if (state && !state.historicalMap)
      await request(`sessions/${state.id}`, undefined, "DELETE");
    setState(accepted.current);
    setSpec(accepted.current?.continuous.spec || spec);
    setSettlement(null);
  }
  function position(e) {
    const point = svg.current.createSVGPoint();
    point.x = e.clientX;
    point.y = e.clientY;
    const p = point.matrixTransform(svg.current.getScreenCTM().inverse());
    return [p.x, p.y];
  }
  function toWorld(p) {
    return [
      Math.max(0, Math.min(state.W - 1, (p[0] / 2) * (state.W - 1))),
      Math.max(0, Math.min(state.H - 1, p[1] * (state.H - 1))),
    ];
  }
  function inspect(p) {
    inspectAbort.current?.abort();
    const abort = new AbortController();
    inspectAbort.current = abort;
    request(
      `${base}/terrain/sample?lon=${p[0] * 180 - 180}&lat=${90 - p[1] * 180}`,
      undefined,
      "GET",
      abort.signal,
    )
      .then(setSample)
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      });
  }
  async function clickMap(p) {
    if (!state || p[0] < 0 || p[0] > 2 || p[1] < 0 || p[1] > 1) return;
    setCursor(p);
    inspect(p);
    if (mode === "settlement" && tool === "site") {
      await run(async () => {
        const d = await request(`${base}/settlements/preview`, {
          ...settlementSpec,
          longitude: p[0] * 180 - 180,
          latitude: 90 - p[1] * 180,
          seed: 12345,
        });
        setSettlement(d);
        const span =
          (d.preview.radiusMetres /
            (state.continuous.spec.radiusKm * 1000 * Math.PI)) *
          3.8;
        setCamera({
          x: p[0] - span / 2,
          y: p[1] - span / 4,
          w: span,
          h: span / 2,
        });
      });
      return;
    }
    if (["road", "country", "border"].includes(tool)) {
      setDraft((d) => [...d, toWorld(p)]);
      return;
    }
    if (["landmark", "label", "house"].includes(tool)) {
      await run(() =>
        command({
          kind: "entity",
          entity: {
            id: uid(),
            name,
            kind: tool,
            geometry: "point",
            points: [toWorld(p)],
            layer: layerFor(tool),
            color: "#e4c989",
            minDetail: 0,
          },
        }),
      );
      return;
    }
    if (
      [
        "raise",
        "lower",
        "flatten",
        "smooth",
        "crater",
        "valley",
        "island",
        "water",
        "drain",
        "plant",
        "clear",
      ].includes(tool)
    ) {
      await run(() =>
        command({
          kind: "operation",
          operation: {
            id: uid(),
            kind: tool,
            layer:
              tool === "water" || tool === "drain"
                ? "water"
                : tool === "plant" || tool === "clear"
                  ? "vegetation"
                  : "elevation",
            center: toWorld(p),
            radius:
              (radius / (Math.PI * state.continuous.spec.radiusKm)) *
              (state.H - 1),
            amount,
            target: amount,
          },
        }),
      );
    }
  }
  async function finish() {
    const polygon = tool === "country";
    if (draft.length < (polygon ? 3 : 2)) return;
    await command({
      kind: "entity",
      entity: {
        id: uid(),
        name,
        kind: tool,
        geometry: polygon ? "polygon" : "line",
        points: draft,
        layer: layerFor(tool),
        color: polygon ? "#c49d65" : "#e7cfaa",
        minDetail: 0,
      },
    });
    setDraft([]);
  }
  function down(e) {
    if (e.button !== 0) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    drag.current = {
      start: [e.clientX, e.clientY],
      camera,
      point: position(e),
      moved: false,
    };
  }
  function move(e) {
    const p = position(e);
    setCursor(p);
    if (!drag.current) return;
    const d = drag.current,
      dx = e.clientX - d.start[0],
      dy = e.clientY - d.start[1];
    if (Math.hypot(dx, dy) > 4) d.moved = true;
    if (d.vertex != null) {
      setSelected((v) => ({
        ...v,
        points: v.points.map((q, i) => (i === d.vertex ? toWorld(p) : q)),
      }));
      return;
    }
    if (d.entity) {
      const worldP = toWorld(p),
        original = toWorld(d.point);
      setSelected({
        ...d.entity,
        points: d.entity.points.map((q) => [
          q[0] + worldP[0] - original[0],
          q[1] + worldP[1] - original[1],
        ]),
      });
      return;
    }
    if (d.moved)
      setCamera({
        ...d.camera,
        x: d.camera.x - (dx / width) * d.camera.w,
        y: d.camera.y - (dy / width) * d.camera.w,
      });
  }
  function up(e) {
    const d = drag.current;
    drag.current = null;
    if (!d) return;
    if ((d.vertex != null || d.entity) && d.moved) {
      run(() => command({ kind: "entity", entity: selected }));
      return;
    }
    if (!d.moved && !d.entity && d.vertex == null) void clickMap(position(e));
  }
  function zoom(e) {
    e.preventDefault();
    const p = position(e),
      factor = Math.exp(Math.max(-0.4, Math.min(0.4, e.deltaY * 0.0015)));
    setCamera((c) => {
      const w = Math.max(0.00008, Math.min(3, c.w * factor)),
        f = w / c.w;
      return {
        x: p[0] - (p[0] - c.x) * f,
        y: p[1] - (p[1] - c.y) * f,
        w,
        h: c.h * f,
      };
    });
  }
  useEffect(() => {
    const element = svg.current;
    if (!element) return;
    element.addEventListener("wheel", zoom, { passive: false });
    return () => element.removeEventListener("wheel", zoom);
  }, [camera, width]);
  async function exportMap(format) {
    if (!svg.current) return;
    const clone = svg.current.cloneNode(true);
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", "2048");
    clone.setAttribute("height", String((2048 * camera.h) / camera.w));
    for (const img of clone.querySelectorAll("image")) {
      const blob = await (await fetch(img.getAttribute("href"))).blob();
      const uri = await new Promise((resolve) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result);
        reader.readAsDataURL(blob);
      });
      img.setAttribute("href", uri);
    }
    const blob = new Blob([new XMLSerializer().serializeToString(clone)], {
      type: "image/svg+xml",
    });
    if (format === "svg") {
      download(blob, "KRIEMHILD-map.svg");
      return;
    }
    const url = URL.createObjectURL(blob),
      img = new Image();
    try {
      await new Promise((resolve, reject) => {
        img.onload = resolve;
        img.onerror = reject;
        img.src = url;
      });
      const c = document.createElement("canvas");
      c.width = 2048;
      c.height = Math.round((2048 * camera.h) / camera.w);
      c.getContext("2d").drawImage(img, 0, 0, c.width, c.height);
      const png = await new Promise((resolve) =>
        c.toBlob(resolve, "image/png"),
      );
      if (format === "png") {
        download(png, "KRIEMHILD-map.png");
        return;
      }
      const { PDFDocument } = await import("pdf-lib");
      const pdf = await PDFDocument.create(),
        image = await pdf.embedPng(await png.arrayBuffer()),
        page = pdf.addPage([c.width, c.height]);
      page.drawImage(image, { x: 0, y: 0, width: c.width, height: c.height });
      download(
        new Blob([await pdf.save()], { type: "application/pdf" }),
        "KRIEMHILD-map.pdf",
      );
    } finally {
      URL.revokeObjectURL(url);
    }
  }
  const diagnostics = state?.continuous?.diagnostics;
  const visibleEntities = entities.filter(
    (e) =>
      !e.deleted &&
      state.project.layers.find((l) => l.id === e.layer)?.visible &&
      e.minDetail <= Math.max(0, Math.log2(2 / camera.w)),
  );
  function feature(e, preview = false) {
    const isSelected = selected?.id === e.id;
    const f = isSelected ? selected : e;
    const common = {
      key: e.id,
      stroke: isSelected ? "#fff4b1" : e.color || "#ddba7b",
      strokeWidth: (camera.w / width) * (e.kind === "road" ? 2 : 1.2),
      fill:
        f.geometry === "polygon"
          ? e.kind === "house"
            ? "#d7bc91"
            : e.kind === "district"
              ? "#ceae6830"
              : "#bda67e25"
          : "none",
      onPointerDown: (event) => {
        if (preview) return;
        event.stopPropagation();
        if (state.project.layers.find((l) => l.id === e.layer)?.locked) {
          setError("This layer is locked.");
          return;
        }
        setSelected(e);
        setTool("select");
        drag.current = {
          start: [event.clientX, event.clientY],
          point: position(event),
          entity: e,
          moved: false,
        };
        svg.current.setPointerCapture(event.pointerId);
      },
    };
    return (
      <g key={e.id} opacity={preview ? 0.65 : 1}>
        {f.geometry === "polygon" ? (
          <polygon {...common} points={pointsText(f.points, state)} />
        ) : f.geometry === "line" ? (
          <polyline {...common} points={pointsText(f.points, state)} />
        ) : (
          <circle
            {...common}
            cx={toMap(f.points[0], state)[0]}
            cy={toMap(f.points[0], state)[1]}
            r={(camera.w / width) * 4}
            fill={e.color}
          />
        )}
        {f.name &&
          f.kind !== "house" &&
          f.kind !== "road" &&
          f.kind !== "district" && (
            <text
              x={toMap(f.points[0], state)[0]}
              y={toMap(f.points[0], state)[1] - (camera.w / width) * 8}
              fill="#f0e6c8"
              fontSize={(camera.w / width) * 12}
              textAnchor="middle"
              pointerEvents="none"
            >
              {f.name}
            </text>
          )}
      </g>
    );
  }
  return (
    <div className="terrain-studio">
      <header className="studio-top">
        <button
          className="studio-brand"
          onClick={() =>
            run(async () => {
              if (state?.unsaved) await save();
              onExit?.(query.current.get("world"));
            })
          }
        >
          KRIEMHILD <span>/ Atlas</span>
        </button>
        <span className="studio-title">
          {state?.project?.name || "A new natural world"}
          <small>
            {state?.historicalMap
              ? "Accepted terrain · current Age"
              : "Generation preview"}
          </small>
        </span>
        <nav aria-label="World editors">
          {[
            ["generate", "Terrain Generator"],
            ["map", "Map Builder"],
            ["settlement", "Settlement Builder"],
          ].map(([id, label]) => (
            <button
              key={id}
              aria-current={mode === id ? "page" : undefined}
              disabled={id !== "generate" && !state}
              onClick={() => {
                setMode(id);
                setTool(id === "settlement" ? "site" : "inspect");
                setDraft([]);
              }}
            >
              {label}
            </button>
          ))}
        </nav>
        <span className="studio-save" role="status">
          {busy
            ? "Working…"
            : state?.unsaved
              ? "Unsaved changes"
              : state
                ? "Saved"
                : ""}
        </span>
        <button disabled={!state || busy} onClick={() => run(save)}>
          Save
        </button>
        <button
          aria-label="Toggle settings panel"
          onClick={() => setPanel(!panel)}
        >
          ☷
        </button>
      </header>
      {error && (
        <div className="studio-error" role="alert">
          {error}
          <button onClick={() => setError("")}>Dismiss</button>
        </div>
      )}
      <div className="studio-body">
        <nav className="studio-rail" aria-label="Map tools">
          {(mode === "generate"
            ? ["inspect"]
            : mode === "settlement"
              ? ["site", "select", "road", "house", "landmark"]
              : [
                  "inspect",
                  "select",
                  "road",
                  "country",
                  "border",
                  "landmark",
                  "label",
                  "raise",
                  "lower",
                  "smooth",
                  "water",
                  "plant",
                ]
          ).map((t) => (
            <button
              title={t}
              key={t}
              aria-pressed={tool === t}
              onClick={() => {
                setTool(t);
                setDraft([]);
              }}
            >
              {t}
            </button>
          ))}
          <hr />
          <button onClick={() => setCamera(initialCamera)}>Fit world</button>
          <button
            disabled={!state || busy}
            onClick={() => run(() => command({ kind: "undo" }))}
          >
            Undo
          </button>
          <button
            disabled={!state || busy}
            onClick={() => run(() => command({ kind: "redo" }))}
          >
            Redo
          </button>
        </nav>
        <main className="studio-stage" ref={stage}>
          <svg
            ref={svg}
            role="img"
            aria-label="Interactive continuous world map"
            tabIndex={0}
            viewBox={`${camera.x} ${camera.y} ${camera.w} ${camera.h}`}
            onPointerDown={down}
            onPointerMove={move}
            onPointerUp={up}
            onPointerCancel={() => {
              drag.current = null;
            }}
          >
            <rect x="-100" y="-100" width="200" height="200" fill="#101d2a" />
            {state && (
              <>
                <TileSurface
                  session={state.id}
                  revision={state.project.revision}
                  invalidations={invalidations}
                  style={style}
                  camera={camera}
                  width={width}
                  onLoading={setLoading}
                />
                {showRivers && (
                  <g
                    fill="none"
                    stroke="#71b7d3"
                    strokeOpacity=".7"
                    pointerEvents="none"
                  >
                    {rivers.map((r) => (
                      <polyline
                        key={r.id}
                        strokeWidth={Math.max(
                          (camera.w / width) * 0.7,
                          Math.min(
                            (camera.w / width) * 3,
                            (r.width?.at(-1) || 20) /
                              (state.continuous.spec.radiusKm * 1000 * Math.PI),
                          ),
                        )}
                        points={r.path
                          .map((p) =>
                            [
                              (p[0] + Math.PI) / Math.PI,
                              0.5 - p[1] / Math.PI,
                            ].join(","),
                          )
                          .join(" ")}
                      />
                    ))}
                  </g>
                )}
                {visibleEntities.map((e) => feature(e))}
                {settlement?.entities.map((e) => feature(e, true))}
                {selected?.points.map((p, i) => (
                  <circle
                    key={i}
                    cx={toMap(p, state)[0]}
                    cy={toMap(p, state)[1]}
                    r={(camera.w / width) * 4}
                    fill="#fff0a3"
                    stroke="#473c28"
                    strokeWidth={camera.w / width}
                    onPointerDown={(e) => {
                      e.stopPropagation();
                      drag.current = {
                        start: [e.clientX, e.clientY],
                        vertex: i,
                        moved: false,
                      };
                      svg.current.setPointerCapture(e.pointerId);
                    }}
                  />
                ))}
                {draft.length > 0 && (
                  <polyline
                    points={pointsText(draft, state)}
                    fill={tool === "country" ? "#ddc98c33" : "none"}
                    stroke="#ffe3a2"
                    strokeWidth={(camera.w / width) * 2}
                  />
                )}
                {cursor &&
                  ["raise", "lower", "smooth", "water", "plant"].includes(
                    tool,
                  ) && (
                    <circle
                      cx={cursor[0]}
                      cy={cursor[1]}
                      r={radius / (Math.PI * state.continuous.spec.radiusKm)}
                      fill="#ffe5a522"
                      stroke="#ffe5a5"
                      strokeWidth={camera.w / width}
                      pointerEvents="none"
                    />
                  )}
              </>
            )}
          </svg>
          {!state && (
            <div className="studio-empty">
              <span>THE SHAPE OF A WORLD</span>
              <h1>Start with the land.</h1>
              <p>
                Generate a seeded planet, explore its coastlines,
                <br />
                then build its places and histories.
              </p>
              <button className="primary" disabled={busy} onClick={generate}>
                Generate preview
              </button>
            </div>
          )}
          {job && (
            <div className="studio-progress" role="status">
              <strong>{job.stage}</strong>
              <progress max="1" value={job.progress} />
              <button
                onClick={() => {
                  cancelled.current = true;
                  request(
                    `terrain/generation-jobs/${jobRef.current}`,
                    undefined,
                    "DELETE",
                  ).catch((e) => setError(e.message));
                }}
              >
                Cancel generation
              </button>
            </div>
          )}
          {draft.length > 0 && (
            <div className="studio-floating">
              <span>{draft.length} vertices · click to add</span>
              <button onClick={() => run(finish)}>Finish {tool}</button>
              <button onClick={() => setDraft([])}>Cancel</button>
            </div>
          )}
        </main>
        {panel && (
          <aside className="studio-panel">
            {mode === "generate" && (
              <>
                <div className="studio-eyebrow">NATURAL WORLD</div>
                <h2>Shape the planet</h2>
                <p className="studio-muted">
                  Preview a new terrain version. Accepted maps and their history
                  stay saved.
                </p>
                <label>
                  Preset
                  <select
                    value={spec.preset}
                    onChange={(e) =>
                      setSpec({
                        ...spec,
                        ...presets[e.target.value],
                        preset: e.target.value,
                      })
                    }
                  >
                    {Object.keys(presets).map((p) => (
                      <option key={p}>{p}</option>
                    ))}
                  </select>
                </label>
                <label>
                  Seed
                  <div className="studio-inline">
                    <input
                      value={spec.seed}
                      onChange={(e) =>
                        setSpec({ ...spec, seed: e.target.value })
                      }
                    />
                    <button
                      title="Choose a new seed"
                      onClick={() =>
                        setSpec({ ...spec, seed: uid().slice(0, 12) })
                      }
                    >
                      ↻
                    </button>
                  </div>
                </label>
                {[
                  ["landCoverage", "Land preference", 0.03, 0.65, 0.01, "%"],
                  ["continents", "Major landmasses", 1, 10, 1, ""],
                  ["complexity", "Coast complexity", 0, 2, 0.1, ""],
                  ["islands", "Island intensity", 0, 3, 0.1, ""],
                  ["relief", "Relief strength", 0.1, 2, 0.1, ""],
                ].map(([key, label, min, max, step, unit]) => (
                  <label key={key}>
                    {label}
                    <output>
                      {unit === "%" ? Math.round(spec[key] * 100) : spec[key]}
                      {unit}
                    </output>
                    <input
                      type="range"
                      min={min}
                      max={max}
                      step={step}
                      value={spec[key]}
                      onChange={(e) =>
                        setSpec({ ...spec, [key]: Number(e.target.value) })
                      }
                    />
                  </label>
                ))}
                <details>
                  <summary>Climate & scale</summary>
                  <label>
                    Rainfall multiplier
                    <input
                      type="number"
                      min=".1"
                      max="3"
                      step=".1"
                      value={spec.rainfall}
                      onChange={(e) =>
                        setSpec({ ...spec, rainfall: +e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Axial tilt (degrees)
                    <input
                      type="number"
                      min="0"
                      max="60"
                      value={spec.tilt}
                      onChange={(e) =>
                        setSpec({ ...spec, tilt: +e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Planet radius (km)
                    <input
                      type="number"
                      min="100"
                      max="100000"
                      value={spec.radiusKm}
                      onChange={(e) =>
                        setSpec({ ...spec, radiusKm: +e.target.value })
                      }
                    />
                  </label>
                  <button
                    onClick={() =>
                      setSpec({
                        ...spec,
                        layoutVariant: spec.layoutVariant + 1,
                      })
                    }
                  >
                    Vary layout stream ({spec.layoutVariant})
                  </button>
                </details>
                <button
                  className="primary studio-full"
                  disabled={busy}
                  onClick={generate}
                >
                  Generate preview
                </button>
                {state && !state.historicalMap && (
                  <div className="studio-actions">
                    <button disabled={busy} onClick={() => run(accept)}>
                      Accept terrain
                    </button>
                    <button disabled={busy} onClick={() => run(discard)}>
                      Discard preview
                    </button>
                  </div>
                )}
                {diagnostics && (
                  <details open>
                    <summary>Geography diagnostics</summary>
                    <p>
                      {diagnostics.majorContinents} major continents ·{" "}
                      {(diagnostics.landCoverage * 100).toFixed(1)}% land
                    </p>
                    <p>
                      Largest landmass:{" "}
                      {(diagnostics.largestShare * 100).toFixed(0)}% of land
                    </p>
                    {diagnostics.warnings?.map((w) => (
                      <p className="studio-warning" key={w}>
                        {w}
                      </p>
                    ))}
                    <button
                      onClick={() =>
                        download(
                          new Blob([JSON.stringify(diagnostics, null, 2)], {
                            type: "application/json",
                          }),
                          "terrain-diagnostics.json",
                        )
                      }
                    >
                      Download diagnostics
                    </button>
                  </details>
                )}
              </>
            )}
            {mode === "map" && (
              <>
                <div className="studio-eyebrow">MAP BUILDER</div>
                <h2>Places & geography</h2>
                <label>
                  New feature name
                  <input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                  />
                </label>
                <p className="studio-muted">
                  Select a tool and click the map. Drag to pan. Scroll to zoom.
                  Escape cancels drawing.
                </p>
                {["raise", "lower", "smooth", "water", "plant"].includes(
                  tool,
                ) && (
                  <>
                    <label>
                      Brush radius (km)
                      <input
                        type="number"
                        min="1"
                        max="2000"
                        value={radius}
                        onChange={(e) => setRadius(+e.target.value)}
                      />
                    </label>
                    <label>
                      Height / water level (metres)
                      <input
                        type="number"
                        min="-10000"
                        max="10000"
                        value={amount}
                        onChange={(e) => setAmount(+e.target.value)}
                      />
                    </label>
                    <p className="studio-muted">
                      Click applies a local edit. Undo restores the generated
                      surface.
                    </p>
                  </>
                )}
              </>
            )}
            {mode === "settlement" && (
              <>
                <div className="studio-eyebrow">SETTLEMENT BUILDER</div>
                <h2>A place to belong</h2>
                <label>
                  Settlement name
                  <input
                    value={settlementSpec.name}
                    onChange={(e) =>
                      setSettlementSpec({
                        ...settlementSpec,
                        name: e.target.value,
                      })
                    }
                  />
                </label>
                <label>
                  Size
                  <select
                    value={settlementSpec.kind}
                    onChange={(e) =>
                      setSettlementSpec({
                        ...settlementSpec,
                        kind: e.target.value,
                      })
                    }
                  >
                    {["village", "town", "city", "capital"].map((k) => (
                      <option key={k}>{k}</option>
                    ))}
                  </select>
                </label>
                <label>
                  Layout
                  <select
                    value={settlementSpec.archetype}
                    onChange={(e) =>
                      setSettlementSpec({
                        ...settlementSpec,
                        archetype: e.target.value,
                      })
                    }
                  >
                    {["organic", "market", "fortified", "port", "planned"].map(
                      (k) => (
                        <option key={k}>{k}</option>
                      ),
                    )}
                  </select>
                </label>
                <p className="studio-muted">
                  Choose Site, then click dry land to preview districts, streets
                  and building footprints.
                </p>
                {settlement && (
                  <>
                    <p>
                      Site suitability:{" "}
                      {(settlement.preview.score * 100).toFixed(0)}% ·{" "}
                      {settlement.entities.length} features
                    </p>
                    {settlement.preview.warnings.map((w) => (
                      <p className="studio-warning" key={w}>
                        {w}
                      </p>
                    ))}
                    <button
                      className="primary"
                      disabled={busy}
                      onClick={() =>
                        run(async () => {
                          const r = await request(`${base}/settlements`, {
                            id: settlement.id,
                            revision: settlement.revision,
                          });
                          setState((v) => ({
                            ...v,
                            project: r.project,
                            unsaved: true,
                          }));
                          setSettlement(null);
                          await refresh();
                        })
                      }
                    >
                      Accept settlement
                    </button>
                    <button onClick={() => setSettlement(null)}>Discard</button>
                  </>
                )}
              </>
            )}
            {selected && (
              <section className="studio-inspector">
                <h3>Object inspector</h3>
                <label>
                  Name
                  <input
                    value={selected.name}
                    onChange={(e) =>
                      setSelected({ ...selected, name: e.target.value })
                    }
                  />
                </label>
                <label>
                  Notes
                  <textarea
                    value={selected.notes || ""}
                    onChange={(e) =>
                      setSelected({ ...selected, notes: e.target.value })
                    }
                  />
                </label>
                <label>
                  Parent
                  <select
                    value={selected.parentId || ""}
                    onChange={(e) =>
                      setSelected({ ...selected, parentId: e.target.value })
                    }
                  >
                    <option value="">None</option>
                    {entities
                      .filter(
                        (e) => !e.deleted && e.id !== selected.id && e.name,
                      )
                      .map((e) => (
                        <option key={e.id} value={e.id}>
                          {e.name}
                        </option>
                      ))}
                  </select>
                </label>
                <label>
                  Minimum detail
                  <input
                    type="number"
                    min="0"
                    max="8"
                    value={selected.minDetail}
                    onChange={(e) =>
                      setSelected({ ...selected, minDetail: +e.target.value })
                    }
                  />
                </label>
                <p className="studio-muted">
                  Drag the shape to move it, or drag a vertex to reshape it.
                </p>
                <button
                  onClick={() =>
                    run(() => command({ kind: "entity", entity: selected }))
                  }
                >
                  Apply properties
                </button>
                <button
                  onClick={() =>
                    run(async () => {
                      await command({
                        kind: "entity",
                        entity: { ...selected, deleted: true },
                      });
                      setSelected(null);
                    })
                  }
                >
                  Delete object
                </button>
              </section>
            )}
            <section>
              <h3>Map appearance</h3>
              <label>
                Style
                <select
                  value={style}
                  onChange={(e) => setStyle(e.target.value)}
                >
                  {[
                    "natural",
                    "elevation",
                    "biome",
                    "atlas",
                    "mask",
                    "bathymetry",
                    "temperature",
                    "precipitation",
                    "moisture",
                  ].map((v) => (
                    <option key={v}>{v}</option>
                  ))}
                </select>
              </label>
              <label className="studio-check">
                <input
                  type="checkbox"
                  checked={showRivers}
                  onChange={(e) => setShowRivers(e.target.checked)}
                />{" "}
                Rivers
              </label>
              <p className="studio-muted">
                Styles change presentation without regenerating terrain.
              </p>
            </section>
            {state && (
              <details>
                <summary>Layers & objects</summary>
                {state.project.layers.map((l, i) => (
                  <div className="studio-layer" key={l.id}>
                    <input
                      type="checkbox"
                      aria-label={`${l.name} visible`}
                      checked={l.visible}
                      onChange={(e) =>
                        run(() =>
                          command({
                            kind: "layers",
                            layers: state.project.layers.map((x) =>
                              x.id === l.id
                                ? { ...x, visible: e.target.checked }
                                : x,
                            ),
                          }),
                        )
                      }
                    />
                    <span>{l.name}</span>
                    <button
                      aria-label={`${l.name} lock`}
                      aria-pressed={l.locked}
                      onClick={() =>
                        run(() =>
                          command({
                            kind: "layers",
                            layers: state.project.layers.map((x) =>
                              x.id === l.id ? { ...x, locked: !x.locked } : x,
                            ),
                          }),
                        )
                      }
                    >
                      {l.locked ? "Locked" : "Lock"}
                    </button>
                  </div>
                ))}
                <label>
                  Find an object
                  <input
                    type="search"
                    placeholder="Name…"
                    onChange={(e) => {
                      const found = entities.find(
                        (x) =>
                          x.name.toLowerCase() === e.target.value.toLowerCase(),
                      );
                      if (found) setSelected(found);
                    }}
                  />
                </label>
              </details>
            )}
            {sample && (
              <section>
                <h3>Geographic inspection</h3>
                <dl>
                  <dt>Surface</dt>
                  <dd>
                    {sample.biome} · {sample.water}
                  </dd>
                  <dt>Bed elevation</dt>
                  <dd>{sample.bed.toFixed(0)} m</dd>
                  <dt>Water depth</dt>
                  <dd>{sample.waterDepth.toFixed(1)} m</dd>
                  <dt>Temperature</dt>
                  <dd>{sample.temperature.toFixed(1)} °C</dd>
                  <dt>Rainfall</dt>
                  <dd>{sample.precipitation.toFixed(0)} mm/year</dd>
                  <dt>Moisture</dt>
                  <dd>{(sample.moisture * 100).toFixed(0)}%</dd>
                </dl>
              </section>
            )}
            {state && (
              <section>
                <h3>Save & export</h3>
                <label className="studio-check">
                  <input
                    type="checkbox"
                    checked={autoSave}
                    onChange={(e) => setAutoSave(e.target.checked)}
                  />{" "}
                  Autosave accepted terrain after 30 seconds
                </label>
                <div className="studio-actions">
                  {["png", "svg", "pdf"].map((f) => (
                    <button key={f} onClick={() => run(() => exportMap(f))}>
                      {f.toUpperCase()}
                    </button>
                  ))}
                  <button
                    onClick={() =>
                      run(async () => {
                        await save();
                        const worldId = query.current.get("world");
                        const r = await fetch(
                          state.historicalMap && worldId
                            ? `/api/worlds/${worldId}/export`
                            : `/api/${base}/project`,
                          state.historicalMap && worldId
                            ? {}
                            : {
                                method: "POST",
                                headers: { "Content-Type": "application/json" },
                                body: JSON.stringify({ ui: { camera, style } }),
                              },
                        );
                        if (!r.ok) throw Error("Archive export failed");
                        download(await r.blob(), "KRIEMHILD.world.zip");
                      })
                    }
                  >
                    World ZIP
                  </button>
                </div>
              </section>
            )}
          </aside>
        )}
      </div>
      <footer className="studio-status">
        <span>
          Equirectangular · {state?.continuous.spec.radiusKm || spec.radiusKm}{" "}
          km radius
        </span>
        <span>
          {cursor
            ? `${(cursor[0] * 180 - 180).toFixed(3)}° E, ${(90 - cursor[1] * 180).toFixed(3)}° N`
            : "Move over the map"}
        </span>
        <span>{Math.max(0, Math.log2(2 / camera.w)).toFixed(1)}× detail</span>
        <span>
          {state
            ? `${((camera.w / 5) * Math.PI * state.continuous.spec.radiusKm).toFixed(1)} km ━━━━━`
            : ""}
        </span>
        <span role="status">
          {loading > 0
            ? `Sampling ${loading} tiles…`
            : loading < 0
              ? "Tile loading failed"
              : "Ready"}
        </span>
      </footer>
    </div>
  );
}
