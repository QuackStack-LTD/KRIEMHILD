import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useMemo,
  useState,
} from "react";
import { request, defaults, presets } from "./api.js";
import TileSurface from "./TileSurface.jsx";
import { normalizeCamera, visibleTiles } from "./camera.js";
import {
  screenToMap,
  viewport,
  projectToMap,
  mapToProject,
  FeatureIndex,
} from "./geometry.js";
import { riverFeatures, riverWidth, riverVisible } from "./rivers.js";
import { generationSpec } from "./seed.js";
import { bounds as geometryBounds } from "./geometry.js";
import { newIllustration, buildIllustration } from "./illustration.js";
import IllustrationScene from "./IllustrationScene.jsx";
import IllustrationPanel from "./IllustrationPanel.jsx";
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
const Terrain3D = lazy(() => import("./Terrain3D.jsx"));
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
  const [show3D, setShow3D] = useState(false);
  const [exportWidth, setExportWidth] = useState(2048);
  const [state, setState] = useState(initialState || null),
    [spec, setSpec] = useState({
      ...defaults,
      ...initialState?.continuous?.spec,
      version: defaults.version,
      seed: "",
    });
  const [hillshade, setHillshade] = useState(
      initialState?.projectUI?.continuousView?.hillshade ??
        initialState?.project?.view?.hillshade ??
        0.55,
    ),
    [legend, setLegend] = useState({}),
    [riverSelection, setRiverSelection] = useState(null);
  const [illustrating, setIllustrating] = useState(false),
    [art, setArt] = useState(null),
    [artSelected, setArtSelected] = useState(null),
    [artTerrain, setArtTerrain] = useState(null);
  const [worldContext, setWorldContext] = useState(null);
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
    [coasts, setCoasts] = useState(null),
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
  const displayStyle = illustrating ? "atlas" : style;
  const artObjects = useMemo(
    () =>
      art && state
        ? buildIllustration(art, entities, rivers, artTerrain, state, coasts)
        : [],
    [art, entities, rivers, artTerrain, state?.id, coasts],
  );
  useEffect(() => {
    if (!art || !state) return;
    const saved = state.project.illustrations?.[art.id];
    if (saved) setArt(saved);
    else {
      setArt(null);
      setArtSelected(null);
      setIllustrating(false);
    }
  }, [state?.project?.revision]);
  useEffect(() => {
    if (!illustrating || !art || !state) return;
    const abort = new AbortController(),
      [x, y, w, h] = art.bounds;
    setArtTerrain(null);
    request(
      `${base}/terrain/mesh?west=${Math.max(-180, x * 180 - 180)}&east=${Math.min(180, (x + w) * 180 - 180)}&north=${Math.min(90, 90 - y * 180)}&south=${Math.max(-90, 90 - (y + h) * 180)}`,
      undefined,
      "GET",
      abort.signal,
    )
      .then(setArtTerrain)
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      });
    return () => abort.abort();
  }, [illustrating, art?.id, art?.bounds?.join(","), state?.id, invalidations]);
  useEffect(() => {
    const id = state?.historyWorldId || query.current.get("world");
    if (!id) return;
    const abort = new AbortController();
    request(`worlds/${id}`, undefined, "GET", abort.signal)
      .then(setWorldContext)
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      });
    return () => abort.abort();
  }, [state?.historyWorldId, state?.historicalMap?.ageId]);
  function changeArt(next, commit = false) {
    setArt(next);
    if (commit)
      void run(() => command({ kind: "illustration", illustration: next }));
  }
  function overrideArt(o, patch, commit) {
    if (busy) return;
    if (art.layers.find((l) => l.id === o.layer)?.locked) return;
    const item = {
      id: o.id,
      source: o.source || "",
      layer: o.layer,
      kind: o.kind,
      ...art.objects[o.id],
      ...patch,
    };
    const next = { ...art, objects: { ...art.objects, [o.id]: item } };
    setArtSelected({ ...o, ...patch });
    changeArt(next, commit);
  }
  function artScope(scope, focus) {
    let extent = scope === "world" ? initialCamera : camera;
    if (focus) {
      const b = geometryBounds(focus.points.map((p) => projectToMap(p, state))),
        pad = Math.max(b.w, b.h) * 0.12 || 0.0002;
      extent = {
        x: b.x - pad,
        y: b.y - pad,
        w: Math.max(0.0001, b.w + pad * 2),
        h: Math.max(0.0001, b.h + pad * 2),
      };
    }
    const next = newIllustration(scope, extent, focus?.id || "");
    const saved = state.project.illustrations?.[next.id];
    const current =
      saved && focus ? { ...saved, bounds: next.bounds } : saved || next;
    changeArt(current, !saved || !!focus);
    setCamera(
      saved && !focus
        ? {
            x: saved.bounds[0],
            y: saved.bounds[1],
            w: saved.bounds[2],
            h: saved.bounds[3],
          }
        : extent,
    );
    setArtSelected(null);
    setIllustrating(true);
    setTool("select");
  }
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
    const result = await request(
      `sessions/${s.id}/world/entities?detail=8&all=1`,
    );
    if (active.current?.id === s.id) setEntities(result.entities || []);
  }, []);
  useEffect(() => {
    if (!state?.id) return;
    let alive = true;
    refresh(state).catch((e) => setError(e.message));
    request(`sessions/${state.id}/terrain/features`)
      .then((r) => {
        if (alive) {
          setRivers(r.rivers || []);
          setCoasts(r.coasts);
          setLegend(r.legend || {});
        }
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
      view: { camera, style, mode, showRivers, hillshade },
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
            ui: {
              continuousView: { camera, style, mode, showRivers, hillshade },
            },
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
  }, [state?.id, camera, style, mode, showRivers, hillshade]);
  useEffect(() => {
    if (!autoSave || !state?.historicalMap || !state.unsaved || busy) return;
    const timer = setTimeout(() => run(save), 30000);
    return () => clearTimeout(timer);
  }, [state?.project?.revision, state?.unsaved, autoSave, busy]);
  useEffect(() => {
    const key = (e) => {
      if (e.target.matches("input,textarea,select")) return;
      if (e.key === "Escape") {
        setArtSelected(null);
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
      const started = await request("terrain/generation-jobs", {
        spec: generationSpec(spec),
      });
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
    return screenToMap(
      e.clientX,
      e.clientY,
      svg.current.getBoundingClientRect(),
      camera,
    );
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
    if (state && (tool === "select" || tool === "inspect")) {
      const hits = featureIndex.pick(
        position(e),
        7 / viewport(svg.current.getBoundingClientRect(), camera).scale,
      );
      const hit = e.altKey
        ? hits[(hits.findIndex((f) => f.id === selected?.id) + 1) % hits.length]
        : hits[0];
      if (hit?.river) {
        setRiverSelection(hit.river);
        setSelected(null);
        return;
      }
      setRiverSelection(null);
      if (hit) {
        const entity = entities.find((f) => f.id === hit.id);
        setSelected(entity);
        if (
          tool === "select" &&
          !state.project.layers.find((l) => l.id === entity.layer)?.locked
        ) {
          drag.current = {
            start: [e.clientX, e.clientY],
            point: position(e),
            entity,
            moved: false,
          };
          return;
        }
      } else setSelected(null);
    }
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
        x:
          d.camera.x -
          dx / viewport(svg.current.getBoundingClientRect(), d.camera).scale,
        y:
          d.camera.y -
          dy / viewport(svg.current.getBoundingClientRect(), d.camera).scale,
      });
  }
  function up(e) {
    const d = drag.current;
    drag.current = null;
    if (!d) return;
    if ((d.vertex != null || d.entity) && d.moved) {
      run(() =>
        command(
          d.entity
            ? {
                kind: "move-family",
                entity: { id: d.entity.id },
                delta: [
                  selected.points[0][0] - d.entity.points[0][0],
                  selected.points[0][1] - d.entity.points[0][1],
                ],
              }
            : { kind: "entity", entity: selected },
        ),
      );
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
    if ((exportWidth * exportWidth * camera.h) / camera.w > 16777216)
      throw Error("Choose a smaller export width for this tall view.");
    const clone = svg.current.cloneNode(true);
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", String(exportWidth));
    clone.setAttribute("height", String((exportWidth * camera.h) / camera.w));
    for (const selected of clone.querySelectorAll(
      '[aria-label="Illustration selection"], [data-editor-handle]',
    ))
      selected.remove();
    for (const terrain of clone.querySelectorAll(
      '[aria-label="Continuous terrain tiles"]',
    )) {
      terrain.replaceChildren();
      const queue = visibleTiles(camera, exportWidth, 1);
      const worker = async () => {
        while (queue.length) {
          const t = queue.shift();
          const r = await fetch(
            `/api/${base}/terrain/tiles/${t.z}/${t.x}/${t.y}?size=${t.size}&style=${terrain.getAttribute("data-terrain-style") || displayStyle}&strict=1&revision=${state.project.revision}`,
          );
          if (!r.ok)
            throw Error(
              "Export terrain changed or could not be sampled; retry the export.",
            );
          const blob = await r.blob();
          const uri = await new Promise((resolve) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result);
            reader.readAsDataURL(blob);
          });
          const img = document.createElementNS(
            "http://www.w3.org/2000/svg",
            "image",
          );
          for (const [k, v] of Object.entries({
            href: uri,
            x: t.x * t.span,
            y: t.y * t.span,
            width: t.span,
            height: t.span,
            preserveAspectRatio: "none",
          }))
            img.setAttribute(k, String(v));
          terrain.appendChild(img);
        }
      };
      await Promise.all([worker(), worker(), worker()]);
    }
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
      c.width = exportWidth;
      c.height = Math.round((exportWidth * camera.h) / camera.w);
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
  const visibleEntities = entities
    .filter(
      (e) =>
        !e.deleted &&
        state.project.layers.find((l) => l.id === e.layer)?.visible &&
        e.minDetail <= Math.max(0, Math.log2(2 / camera.w)),
    )
    .filter((e) => {
      const b = geometryBounds(e.points.map((p) => projectToMap(p, state)));
      return (
        b.x <= camera.x + camera.w &&
        b.x + b.w >= camera.x &&
        b.y <= camera.y + camera.h &&
        b.y + b.h >= camera.y
      );
    })
    .sort(
      (a, b) =>
        state.project.layers.find((l) => l.id === a.layer).order -
        state.project.layers.find((l) => l.id === b.layer).order,
    );
  const riverShapes = useMemo(() => riverFeatures(rivers), [rivers]);
  const featureIndex = useMemo(
    () =>
      new FeatureIndex([
        ...visibleEntities.map((e) => ({
          ...e,
          points: e.points.map((p) => projectToMap(p, state)),
        })),
        ...(showRivers
          ? riverShapes.filter((f) =>
              riverVisible(
                f.river,
                camera.w > 0.8
                  ? "world"
                  : camera.w > 0.15
                    ? "continent"
                    : "local",
              ),
            )
          : []),
      ]),
    [
      entities,
      state?.project?.revision,
      camera.x,
      camera.y,
      camera.w,
      camera.h,
      showRivers,
      riverShapes,
    ],
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
      pointerEvents: "none",
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
              ? `${worldContext?.world?.name || "World"} · ${worldContext?.ages?.find((a) => a.id === state.historicalMap.ageId)?.name || "Current Age"}`
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
                if (id === "generate") setIllustrating(false);
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
        <button disabled={!state} onClick={() => setShow3D(true)}>
          3D Terrain
        </button>
        {mode !== "generate" && (
          <button
            disabled={!state}
            aria-pressed={illustrating}
            onClick={() => {
              if (illustrating) setIllustrating(false);
              else
                artScope(
                  mode === "settlement" ? "city" : "world",
                  mode === "settlement" && selected ? selected : undefined,
                );
            }}
          >
            {illustrating ? "Construction mode" : "Medieval Illustrator"}
          </button>
        )}
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
                  enabled={!show3D}
                  session={state.id}
                  revision={state.project.revision}
                  invalidations={invalidations}
                  style={displayStyle}
                  identity={`${state.worldId}:${state.id}`}
                  camera={camera}
                  width={width}
                  onLoading={setLoading}
                />
                {hillshade > 0 && !illustrating && (
                  <g
                    style={{ mixBlendMode: "multiply" }}
                    opacity={hillshade}
                    pointerEvents="none"
                  >
                    <TileSurface
                      enabled={!show3D}
                      session={state.id}
                      revision={state.project.revision}
                      invalidations={invalidations}
                      style="hillshade"
                      identity={`${state.worldId}:${state.id}`}
                      camera={camera}
                      width={width}
                    />
                  </g>
                )}
                {showRivers && !illustrating && (
                  <g
                    fill="none"
                    stroke="#71b7d3"
                    strokeOpacity=".7"
                    pointerEvents="none"
                  >
                    {rivers
                      .filter(
                        (r) =>
                          style === "river-network" ||
                          riverVisible(
                            r,
                            camera.w > 0.8
                              ? "world"
                              : camera.w > 0.15
                                ? "continent"
                                : "local",
                          ),
                      )
                      .map((r) => (
                        <path
                          key={r.id}
                          stroke={
                            style === "river-network"
                              ? {
                                  major: "#17488c",
                                  regional: "#328bd3",
                                  stream: "#6fcce4",
                                  ephemeral: "#9b7345",
                                  distributary: "#36aaaa",
                                }[r.class] || "#71b7d3"
                              : undefined
                          }
                          strokeDasharray={
                            r.class === "ephemeral"
                              ? `${(camera.w / width) * 3} ${(camera.w / width) * 2}`
                              : undefined
                          }
                          strokeWidth={riverWidth(
                            r,
                            camera.w / width,
                            state.continuous.spec.radiusKm,
                          )}
                          d={r.path
                            .map(
                              (p, i) =>
                                `${i === 0 || Math.abs(p[0] - r.path[i - 1][0]) > Math.PI ? "M" : "L"}${(p[0] + Math.PI) / Math.PI},${0.5 - p[1] / Math.PI}`,
                            )
                            .join(" ")}
                        />
                      ))}
                  </g>
                )}
                {!illustrating && visibleEntities.map((e) => feature(e))}
                {illustrating && art && (
                  <IllustrationScene
                    config={art}
                    objects={artObjects}
                    selected={artSelected}
                    onSelect={setArtSelected}
                    onOverride={overrideArt}
                    position={position}
                    state={state}
                    pixel={art.bounds[2] / 1200}
                    coastOverlay={
                      state.continuous.analysisStale ? (
                        <TileSurface
                          enabled={!show3D}
                          session={state.id}
                          revision={state.project.revision}
                          invalidations={invalidations}
                          style="shore-ink"
                          identity={`${state.worldId}:${state.id}`}
                          camera={camera}
                          width={width}
                        />
                      ) : null
                    }
                    relief={
                      <g
                        style={{ mixBlendMode: "multiply" }}
                        opacity={hillshade}
                        pointerEvents="none"
                      >
                        <TileSurface
                          enabled={!show3D}
                          session={state.id}
                          revision={state.project.revision}
                          invalidations={invalidations}
                          style="hillshade"
                          identity={`${state.worldId}:${state.id}`}
                          camera={camera}
                          width={width}
                        />
                      </g>
                    }
                  />
                )}
                {settlement?.entities.map((e) => feature(e, true))}
                {!illustrating &&
                  selected?.points.map((p, i) => (
                    <circle
                      key={i}
                      data-editor-handle="vertex"
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
            {illustrating && art && (
              <IllustrationPanel
                busy={busy}
                config={art}
                objects={artObjects}
                onChange={changeArt}
                onScope={artScope}
                selected={artSelected}
                onOverride={overrideArt}
                entities={entities}
                onSelect={setArtSelected}
                onAdd={() => {
                  const o = {
                    id: uid(),
                    kind: "text",
                    layer: "annotations",
                    text: "New annotation",
                    position: [
                      camera.x + camera.w / 2,
                      camera.y + camera.h / 2,
                    ],
                    color: "#503d2a",
                  };
                  changeArt(
                    { ...art, objects: { ...art.objects, [o.id]: o } },
                    true,
                  );
                  setArtSelected(o);
                }}
              />
            )}
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
                  Seed (optional)
                  <div className="studio-inline">
                    <input
                      aria-label="Optional generation seed"
                      value={spec.seed}
                      onChange={(e) =>
                        setSpec({ ...spec, seed: e.target.value })
                      }
                    />
                    <button
                      title="Leave blank for a fresh seed each time"
                      onClick={() => setSpec({ ...spec, seed: "" })}
                    >
                      ↻
                    </button>
                  </div>
                </label>
                {state && (
                  <p>
                    Generated seed: <code>{state.continuous.spec.seed}</code>
                    <br />
                    <button
                      onClick={() =>
                        navigator.clipboard
                          .writeText(state.continuous.spec.seed)
                          .catch((e) => setError(e.message))
                      }
                    >
                      Copy seed
                    </button>
                    <button
                      onClick={() =>
                        setSpec({ ...spec, seed: state.continuous.spec.seed })
                      }
                    >
                      Reuse this seed
                    </button>
                  </p>
                )}
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
                <label>
                  Terrain detail
                  <select
                    value={spec.surfaceWidth || 4096}
                    onChange={(e) =>
                      setSpec({ ...spec, surfaceWidth: +e.target.value })
                    }
                  >
                    <option value={1024}>Standard: 1024 x 512</option>
                    <option value={2048}>Fine: 2048 x 1024</option>
                    <option value={4096}>Detailed: 4096 x 2048</option>
                  </select>
                  <small>
                    All terrain is built before opening. Zoom inspects the saved
                    surface; it does not add detail. At the equator, samples are{" "}
                    {(
                      (Math.PI * 2 * spec.radiusKm) /
                      (spec.surfaceWidth || 4096)
                    ).toFixed(1)}{" "}
                    km apart.
                  </small>
                </label>
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
                  <details>
                    <summary>Geography diagnostics</summary>
                    <p>
                      {diagnostics.majorContinents} major continents ·{" "}
                      {(diagnostics.landCoverage * 100).toFixed(1)}% land
                    </p>
                    <p>
                      Largest landmass:{" "}
                      {(diagnostics.largestShare * 100).toFixed(0)}% of land
                    </p>
                    {diagnostics.hydrology && (
                      <>
                        <p>
                          {diagnostics.hydrology.basins} drainage basins /{" "}
                          {diagnostics.hydrology.lakes} lakes /{" "}
                          {diagnostics.hydrology.wetlands} wetland complexes /{" "}
                          {diagnostics.hydrology.springs} springs
                        </p>
                        {Object.entries(diagnostics.hydrology.classes).map(
                          ([name, value]) => (
                            <p key={name}>
                              {name}: {value.count} rivers / reaches,{" "}
                              {Math.round(value.lengthKm).toLocaleString()} km
                            </p>
                          ),
                        )}
                        <p>
                          {(
                            diagnostics.hydrology.networkCoverage * 100
                          ).toFixed(1)}
                          % of land in channel-bearing drainage cells /{" "}
                          {diagnostics.hydrology.invalidTerminations} invalid
                          terminations
                        </p>
                        <p>
                          {diagnostics.hydrology.lowlandHeadwaters} lowland
                          headwaters / {diagnostics.hydrology.distributaries}{" "}
                          distributary splits
                        </p>
                      </>
                    )}
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
            {mode === "map" && !illustrating && (
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
            {mode === "settlement" && !illustrating && (
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
            {selected && !illustrating && (
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
                  Color
                  <input
                    type="color"
                    value={selected.color}
                    onChange={(e) =>
                      setSelected({ ...selected, color: e.target.value })
                    }
                  />
                </label>
                <label>
                  Created in year
                  <input
                    type="number"
                    value={selected.created ?? ""}
                    onChange={(e) =>
                      setSelected({
                        ...selected,
                        created:
                          e.target.value === "" ? null : Number(e.target.value),
                      })
                    }
                  />
                </label>
                <label>
                  Destroyed in year
                  <input
                    type="number"
                    value={selected.destroyed ?? ""}
                    onChange={(e) =>
                      setSelected({
                        ...selected,
                        destroyed:
                          e.target.value === "" ? null : Number(e.target.value),
                      })
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
              <div className="studio-actions">
                <button
                  onClick={() => {
                    setStyle("natural");
                    setHillshade(0);
                  }}
                >
                  Flat
                </button>
                <button
                  onClick={() => {
                    setStyle("elevation");
                    setHillshade(0.65);
                  }}
                >
                  Relief
                </button>
                <button disabled={!state} onClick={() => setShow3D(true)}>
                  3D Terrain
                </button>
              </div>
              <label>
                Hillshade strength
                <input
                  type="range"
                  min="0"
                  max="1"
                  step=".05"
                  value={hillshade}
                  onChange={(e) => setHillshade(+e.target.value)}
                />
              </label>
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
                    "pet",
                    "aridity",
                    "soil",
                    "seasonality",
                    "rainShadow",
                    "exposure",
                    "slope",
                    "contours",
                    "watersheds",
                    "flow-accumulation",
                    "river-network",
                    "wetlands",
                  ].map((v) => (
                    <option key={v}>{v}</option>
                  ))}
                </select>
              </label>
              {style === "biome" && (
                <div aria-label="Biome legend">
                  {Object.entries(legend).map(([name, rgb]) => (
                    <div key={name}>
                      <span
                        style={{
                          display: "inline-block",
                          width: 12,
                          height: 12,
                          background: `rgb(${rgb.join(",")})`,
                        }}
                      />{" "}
                      {name}
                    </div>
                  ))}
                </div>
              )}
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
                {[...state.project.layers]
                  .sort((a, b) => a.order - b.order)
                  .map((l, i) => (
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
                        aria-label={`Move ${l.name} up`}
                        disabled={l.order === 0}
                        onClick={() =>
                          run(() =>
                            command({
                              kind: "layers",
                              layers: state.project.layers.map((x) =>
                                x.id === l.id
                                  ? { ...x, order: x.order - 1 }
                                  : x.order === l.order - 1
                                    ? { ...x, order: x.order + 1 }
                                    : x,
                              ),
                            }),
                          )
                        }
                      >
                        ↑
                      </button>
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
                  {sample.landmass && (
                    <>
                      <dt>Landmass</dt>
                      <dd>
                        {sample.landmass.id} · {sample.landmass.family}
                      </dd>
                    </>
                  )}
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
                  <dt>Evaporative demand</dt>
                  <dd>{sample.pet.toFixed(0)} mm/year</dd>
                  {sample.climate?.soil != null && (
                    <>
                      <dt>Soil water</dt>
                      <dd>{sample.climate.soil.toFixed(0)} mm</dd>
                      <dt>Precipitation / PET</dt>
                      <dd>{sample.climate.aridity.toFixed(2)}</dd>
                      <dt>Dry months</dt>
                      <dd>{sample.climate.dryMonths.toFixed(1)}</dd>
                      <dt>Seasonality (CV)</dt>
                      <dd>{sample.climate.seasonality.toFixed(2)}</dd>
                    </>
                  )}
                </dl>
              </section>
            )}
            {riverSelection && (
              <section>
                <h3>{riverSelection.name || riverSelection.id}</h3>
                <p>
                  Discharge {riverSelection.discharge?.at(-1)?.toFixed(1)} m³/s
                  · width {riverSelection.width?.at(-1)?.toFixed(1)} m
                </p>
                <p>Outlet: {riverSelection.destination}</p>
                {riverSelection.order > 0 && (
                  <p>Stream order: {riverSelection.order}</p>
                )}
                <p>
                  Source elevation {riverSelection.bed?.[0]?.toFixed(0)} m ·
                  outlet elevation {riverSelection.bed?.at(-1)?.toFixed(0)} m
                </p>
              </section>
            )}
            {state && (
              <section>
                <h3>Save & export</h3>
                <label>
                  Export width
                  <select
                    value={exportWidth}
                    onChange={(e) => setExportWidth(Number(e.target.value))}
                  >
                    {[1024, 2048, 4096].map((n) => (
                      <option key={n} value={n}>
                        {n} pixels
                      </option>
                    ))}
                  </select>
                </label>
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
      {show3D && state && (
        <Suspense fallback={<div role="status">Loading 3D view…</div>}>
          <Terrain3D
            state={state}
            camera={camera}
            onPick={(p, tolerance) => {
              inspect(p);
              const index = new FeatureIndex([
                ...entities
                  .filter(
                    (e) =>
                      !e.deleted &&
                      state.project.layers.find((l) => l.id === e.layer)
                        ?.visible,
                  )
                  .map((e) => ({
                    ...e,
                    points: e.points.map((q) => projectToMap(q, state)),
                  })),
                ...(showRivers ? riverShapes : []),
              ]);
              const hit = index.pick(p, tolerance)[0];
              setRiverSelection(hit?.river || null);
              setSelected(
                hit && !hit.river
                  ? entities.find((e) => e.id === hit.id)
                  : null,
              );
            }}
            onClose={() => setShow3D(false)}
          />
        </Suspense>
      )}
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
