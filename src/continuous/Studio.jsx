import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useMemo,
  useState,
} from "react";
import { request, defaults, presets } from "./api.js";
import TileSurface from "./TileSurface.jsx";
import ViewportSurface, { ViewportLayer } from "./ViewportSurface.jsx";
import { ProjectionContext } from "./MapProjection.js";
import { mapProjection } from "./projection.js";
import { clipLine, clipPolygon, intersects } from "./clip.js";
import {
  normalizeCamera,
  visibleTiles,
  visibleViewport,
  reliefResolution,
} from "./camera.js";
import {
  screenToMap,
  viewport,
  projectToMap,
  mapToProject,
  FeatureIndex,
} from "./geometry.js";
import { riverFeatures, riverWidth } from "./rivers.js";
import { generationSpec } from "./seed.js";
import { GeographicQuery } from "./geographic-query.js";
import {
  CartographicGeometry,
  cartographicLevel,
  cartographicLevels,
} from "./cartography.js";
import { pathText } from "./geographic-path.js";
import { FrameNavigator } from "./frame-navigator.js";
import { prepareGeographicExport } from "./export-geography.js";
import {
  readPreviewQuality,
  savePreviewQuality,
  previewRequest,
  previewPresets,
  previewHelp,
} from "./preview-quality.js";
import { bounds as geometryBounds } from "./geometry.js";
import { newIllustration, buildIllustration } from "./illustration.js";
import IllustrationScene from "./IllustrationScene.jsx";
import IllustrationPanel from "./IllustrationPanel.jsx";
import { exportTerrain, illustrationAssets } from "./illustration-export.js";
import {
  landPaths,
  toggleLand,
  rectangleLand,
  selectedLand,
  illustrationFootprint,
  illustrationMeshBounds,
} from "./land-selection.js";
import Dialog from "../worldbuilding/ui/Dialog.jsx";
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
    state: "countries",
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

export default function Studio({ initialState, onExit }) {
  const [show3D, setShow3D] = useState(false);
  const [exportWidth, setExportWidth] = useState(2048);
  const [previewQuality, setPreviewQuality] = useState(readPreviewQuality);
  const [previewSaved, setPreviewSaved] = useState(true);
  useEffect(() => {
    setPreviewSaved(savePreviewQuality(previewQuality));
  }, [previewQuality]);
  const [state, setState] = useState(initialState || null),
    [spec, setSpec] = useState({
      ...defaults,
      ...initialState?.continuous?.spec,
      version: defaults.version,
      reliefVersion: defaults.reliefVersion,
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
  const [savedIllustration, setSavedIllustration] = useState(null);
  const [illustrationProgress, setIllustrationProgress] = useState(null);
  const imageAssets = useRef(null);
  const [mapSetup, setMapSetup] = useState(null);
  const [landComponents, setLandComponents] = useState([]);
  const [landIDs, setLandIDs] = useState([]);
  const [landBox, setLandBox] = useState(null);
  const setupSequence = useRef(0);
  const savedView =
    initialState?.projectUI?.continuousView ||
    initialState?.project?.view ||
    {};
  const [mode, setMode] = useState(
      (new URLSearchParams(location.search).get("workspace") === "atlas"
        ? "atlas"
        : null) ||
        savedView.mode ||
        (location.pathname.endsWith("/build") ? "map" : "generate"),
    ),
    [style, setStyle] = useState(
      !savedView.style ||
        ["physical-atlas", "atlas", "natural"].includes(savedView.style)
        ? "terrain"
        : savedView.style,
    );
  const [camera, setCamera] = useState(normalizeCamera(savedView.camera)),
    [width, setWidth] = useState(1000),
    [height, setHeight] = useState(500);
  const cameraRef = useRef(camera);
  const constructionCamera = useRef(camera);
  const savedCamera =
    illustrating || mapSetup ? constructionCamera.current : camera;
  cameraRef.current = camera;
  const navigation = useRef(null);
  navigation.current ||= new FrameNavigator(() => cameraRef.current, setCamera);
  useEffect(() => {
    navigation.current.cancel();
  }, [camera]);
  useEffect(() => () => navigation.current.cancel(), []);
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
  const [drawingType, setDrawingType] = useState({
    state: "country",
    settlement: "town",
  });
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
    attachment = useRef(null),
    viewWriter = useRef({ client: crypto.randomUUID(), sequence: 0 }),
    cancelled = useRef(false),
    geographicQuery = useRef(null);
  geographicQuery.current ||= new GeographicQuery();
  const sourceIdentity = `${state?.worldId}:${state?.id}`;
  geographicQuery.current.source(sourceIdentity, state?.project?.revision);
  const [inspectionPoint, setInspectionPoint] = useState(null);
  active.current = state;
  const query = useRef(new URLSearchParams(location.search));
  useEffect(() => {
    if (query.current.get("tileDebug") !== "1") return;
    // Reproducible geographic views for diagnostics, independent of event timing.
    const restore = (event) => setCamera(normalizeCamera(event.detail));
    window.addEventListener("terrain-diagnostic-camera", restore);
    return () =>
      window.removeEventListener("terrain-diagnostic-camera", restore);
  }, []);
  // Keep the original path only as an explicit regression comparison tool.
  const viewportRenderer = query.current.get("terrainRenderer") !== "tiles";
  const [renderRetry, setRenderRetry] = useState(0);
  const [renderError, setRenderError] = useState("");
  const [renderDiagnostics, setRenderDiagnostics] = useState(null);
  const [terrainProgress, setTerrainProgress] = useState(null);
  const [hydrologyDiagnostics, setHydrologyDiagnostics] = useState(null);
  const geometryCache = useRef(new CartographicGeometry());
  const presentation = useRef();
  presentation.current = cartographicLevel(camera.w, presentation.current);
  const detailLevel = presentation.current;
  const base = `sessions/${state?.id}`;
  const displayStyle = illustrating ? "atlas" : style;
  const wrapIllustration = !!(illustrating && art?.regions?.length);
  const illustrationCoverage = useMemo(
    () => illustrationFootprint(art?.regions || []),
    [art?.regions],
  );
  const previewInfo = previewRequest({ camera, width, height }, previewQuality);
  const geographicViewport = visibleViewport(camera, width, height);
  const projection = useMemo(
    () => mapProjection(camera, width, height),
    [camera, width, height],
  );
  const drawPoint = (p) => projection.point(p);
  const landPath = (regions) =>
    regions
      .map(
        (p) =>
          pathText(clipPolygon(p, projection.bounds).map(drawPoint)) + " Z",
      )
      .join(" ");
  const drawProjectPoint = (p) => drawPoint(projectToMap(p, state));
  const drawProjectPoints = (points) =>
    points
      .map(drawProjectPoint)
      .map((p) => p.join(","))
      .join(" ");
  const drawPath = (points) =>
    clipLine(points, projection.bounds)
      .map((part) => pathText(part.map(drawPoint)))
      .join(" ");
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
      if (illustrating) leaveIllustration();
    }
  }, [state?.project?.revision]);
  useEffect(() => {
    if (!illustrating || !art || !state) return;
    const abort = new AbortController();
    setArtTerrain(null);
    Promise.all(
      illustrationMeshBounds(art).map(([west, south, east, north]) =>
        request(
          `${base}/terrain/mesh?west=${west}&east=${east}&north=${north}&south=${south}`,
          undefined,
          "GET",
          abort.signal,
        ),
      ),
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
    setSavedIllustration(null);
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
  function artScope(scope, focus, options = {}) {
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
    extent = options.extent || extent;
    const next = newIllustration(scope, extent, focus?.id || "");
    next.id =
      options.fresh ||
      (art && (art.scope !== scope || (art.focus || "") !== (focus?.id || "")))
        ? `medieval-${uid()}`
        : art?.id || next.id;
    next.title =
      options.title ||
      art?.title ||
      (focus?.name ? `${focus.name} atlas` : next.title);
    if (options.regions) {
      next.regions = options.regions;
      next.landIds = options.landIds;
      next.landKey = options.landKey;
    }
    if (art && !options.fresh) {
      next.style = art.style;
      next.layers = art.layers;
      next.objects = art.objects;
    }
    changeArt(next, true);
    setCamera(extent);
    setArtSelected(null);
    setIllustrating(true);
    setTool("select");
  }
  function startIllustration() {
    constructionCamera.current = camera;
    const focus = selected?.recordStatus !== "draft" ? selected : null;
    const inherited = entities.find(
      (e) => (e.logicalId || e.id) === state.historicalMap?.entityId,
    );
    setMapSetup({
      step: "name",
      title: art?.title || "",
      resume: !!art,
      focus: focus || inherited || null,
      kind: focus || inherited ? "place" : "land",
      camera,
    });
  }
  function leaveIllustration() {
    setIllustrating(false);
    setCamera(constructionCamera.current);
  }
  async function setupIllustration() {
    const setup = { ...mapSetup };
    if (!setup.title.trim()) return;
    if (setup.resume && art && setup.title.trim() === art.title) {
      const currentLand = art.landKey
        ? await request(`${base}/terrain/land-selection?key=1`)
        : null;
      if (currentLand && currentLand.sourceKey !== art.landKey) {
        setError(
          "The landscape changed. Select the connected land for this map again.",
        );
        setup.resume = false;
      } else {
        setCamera({
          x: art.bounds[0],
          y: art.bounds[1],
          w: art.bounds[2],
          h: art.bounds[3],
        });
        setIllustrating(true);
        setMapSetup(null);
        return;
      }
    }
    const serial = ++setupSequence.current;
    if (setup.kind === "place" && setup.focus) {
      artScope(
        ["country", "state", "kingdom", "empire"].includes(setup.focus.kind)
          ? "country"
          : "city",
        setup.focus,
        { fresh: true, title: setup.title.trim() },
      );
      setMapSetup(null);
      return;
    }
    setMapSetup({ ...setup, step: "land", loading: true });
    setLandIDs([]);
    setCamera(initialCamera);
    setIllustrating(false);
    try {
      const data = await request(`${base}/terrain/land-selection`);
      if (serial !== setupSequence.current) return;
      if (active.current?.project.revision !== data.revision)
        throw Error("The terrain changed; start land selection again.");
      setLandComponents(
        data.components
          .map((c) => ({
            ...c,
            paths: landPaths(c.paths, (0.05 * 2) / data.sourceWidth),
          }))
          .filter((c) => c.paths.length),
      );
      setMapSetup({
        ...setup,
        step: "land",
        loading: false,
        revision: data.revision,
        landKey: data.sourceKey,
      });
    } catch (e) {
      if (serial === setupSequence.current) {
        setError(e.message);
        setMapSetup(null);
      }
    }
  }
  function createSelectedMap() {
    if (active.current.project.revision !== mapSetup.revision) {
      setError("The source changed; start land selection again.");
      return;
    }
    const selection = selectedLand(landComponents, landIDs);
    if (!selection) return;
    if (selection.regions.reduce((n, p) => n + p.length, 0) > 80000) {
      setError(
        "This selection exceeds the map geometry budget. Select a smaller region.",
      );
      return;
    }
    artScope("continent", null, {
      fresh: true,
      title: mapSetup.title.trim(),
      extent: selection.bounds,
      regions: selection.regions,
      landIds: landIDs,
      landKey: mapSetup.landKey,
    });
    setMapSetup(null);
    setLandBox(null);
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
          setHydrologyDiagnostics(r.hydrology || null);
        }
      })
      .catch((e) => setError(e.message));
    return () => {
      alive = false;
    };
  }, [state?.id, invalidations, refresh]);
  useLayoutEffect(() => {
    const element = svg.current;
    const measure = () => {
      const rect = element.getBoundingClientRect();
      if (rect.width > 0 && rect.height > 0) {
        setWidth(rect.width);
        setHeight(rect.height);
      }
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, []);
  useEffect(
    () => () => {
      cancelled.current = true;
      geographicQuery.current.cancel();
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
    const s =
      body.kind === "entity" || body.kind === "move-family"
        ? await ensureWorldMap()
        : active.current;
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
    const next = {
      ...active.current,
      project: result.project || s.project,
      continuous: s.continuous
        ? { ...s.continuous, analysisStale: result.analysisStale }
        : s.continuous,
      unsaved: true,
    };
    active.current = next;
    setState(next);
    await refresh(s);
    return result;
  }
  async function save() {
    const s = await ensureWorldMap();
    if (!s) return;
    const sequence = ++viewWriter.current.sequence;
    const result = await request(`sessions/${s.id}/save`, {
      revision: s.project.revision,
      view: { camera: savedCamera, style, mode, showRivers, hillshade },
      client: viewWriter.current.client,
      sequence,
    });
    if (active.current?.id !== s.id) return;
    const next = {
      ...active.current,
      project: result.project || active.current.project,
      projectUI: result.projectUI || active.current.projectUI,
      unsaved: sequence !== viewWriter.current.sequence,
    };
    active.current = next;
    setState(next);
  }
  useEffect(() => {
    if (!state?.id) return;
    const id = state.id,
      sequence = ++viewWriter.current.sequence,
      timer = setTimeout(
        () =>
          request(`sessions/${id}/view`, {
            ui: {
              continuousView: {
                camera: savedCamera,
                style,
                mode,
                showRivers,
                hillshade,
              },
            },
            merge: true,
            client: viewWriter.current.client,
            sequence,
          })
            .then((result) => {
              if (
                active.current?.id === id &&
                sequence === viewWriter.current.sequence
              )
                setState((v) => ({ ...v, unsaved: result.unsaved }));
            })
            .catch((e) => setError(e.message)),
        700,
      );
    return () => clearTimeout(timer);
  }, [state?.id, savedCamera, style, mode, showRivers, hillshade]);
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
  async function ensureWorldMap() {
    const s = active.current;
    if (!s) return;
    if (s.historicalMap) return s;
    if (attachment.current?.session === s.id) return attachment.current.promise;
    const promise = attachWorldMap(s);
    attachment.current = { session: s.id, promise };
    try {
      return await promise;
    } finally {
      if (attachment.current?.promise === promise) attachment.current = null;
    }
  }
  async function attachWorldMap(s) {
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
    active.current = next;
    setState(next);
    accepted.current = next;
    history.replaceState(
      {},
      "",
      `/world/${next.worldId}/build?${query.current}`,
    );
    return next;
  }
  async function accept() {
    await ensureWorldMap();
    setMode("map");
    setTool("inspect");
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
    setInspectionPoint({
      point: p,
      identity: sourceIdentity,
      revision: state.project.revision,
    });
    setSample(null);
    geographicQuery.current
      .select(
        {
          session: state.id,
          identity: sourceIdentity,
          revision: state.project.revision,
          point: p,
        },
        setSample,
      )
      .catch((e) => setError(e.message));
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
        const span = Math.max(
          0.00008,
          (d.preview.radiusMetres /
            (state.continuous.spec.radiusKm * 1000 * Math.PI)) *
            3.8,
        );
        setCamera({
          x: p[0] - span / 2,
          y: p[1] - span / 4,
          w: span,
          h: span / 2,
        });
      });
      return;
    }
    if (["road", "country", "state", "settlement", "border"].includes(tool)) {
      setDraft((d) => [...d, toWorld(p)]);
      return;
    }
    if (["landmark", "label", "house"].includes(tool)) {
      const id = uid();
      const shape = {
        id,
        name: name === "New feature" ? `${tool} ${id.slice(0, 8)}` : name,
        kind: tool,
        geometry: "point",
        points: [toWorld(p)],
        layer: layerFor(tool),
        color: "#e4c989",
        minDetail: 0,
      };
      await run(async () => {
        await command({ kind: "entity", entity: shape });
        setSelected({ ...shape, recordStatus: "draft" });
        setTool("select");
      });
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
    const polygon = ["country", "state", "settlement"].includes(tool);
    if (draft.length < (polygon ? 3 : 2)) return;
    const id = uid();
    const kind = drawingType[tool] || tool;
    const shape = {
      id,
      name: name === "New feature" ? `${kind} ${id.slice(0, 8)}` : name,
      kind,
      geometry: polygon ? "polygon" : "line",
      points: draft,
      layer: layerFor(tool),
      color: polygon ? "#c49d65" : "#e7cfaa",
      minDetail: 0,
    };
    await command({ kind: "entity", entity: shape });
    setSelected({ ...shape, recordStatus: "draft" });
    setTool("select");
    setDraft([]);
  }
  function down(e) {
    if (e.button !== 0) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    if (mapSetup?.step === "land") {
      drag.current = { landStart: position(e), start: [e.clientX, e.clientY] };
      return;
    }
    if (state && (tool === "select" || tool === "inspect")) {
      const hits = featureIndex.pick(
        position(e),
        7 / viewport(svg.current.getBoundingClientRect(), camera).scale,
      );
      const hit = e.altKey
        ? hits[(hits.findIndex((f) => f.id === selected?.id) + 1) % hits.length]
        : hits[0];
      if (hit?.river) {
        inspect(position(e));
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
    if (d.landStart) {
      const a = d.landStart;
      setLandBox({
        x: Math.min(a[0], p[0]),
        y: Math.min(a[1], p[1]),
        w: Math.abs(a[0] - p[0]),
        h: Math.abs(a[1] - p[1]),
      });
      return;
    }
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
      navigation.current.transform(() => ({
        ...d.camera,
        x:
          d.camera.x -
          dx / viewport(svg.current.getBoundingClientRect(), d.camera).scale,
        y:
          d.camera.y -
          dy / viewport(svg.current.getBoundingClientRect(), d.camera).scale,
      }));
  }
  function up(e) {
    const d = drag.current;
    drag.current = null;
    if (!d) return;
    if (d.landStart) {
      if (!mapSetup?.loading) {
        const p = position(e);
        if (Math.hypot(e.clientX - d.start[0], e.clientY - d.start[1]) > 4)
          setLandIDs((ids) =>
            rectangleLand(landComponents, ids, {
              x: Math.min(d.landStart[0], p[0]),
              y: Math.min(d.landStart[1], p[1]),
              w: Math.abs(p[0] - d.landStart[0]),
              h: Math.abs(p[1] - d.landStart[1]),
            }),
          );
        else setLandIDs((ids) => toggleLand(landComponents, ids, p));
      }
      setLandBox(null);
      return;
    }
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
    const rect = svg.current.getBoundingClientRect(),
      x = e.clientX,
      y = e.clientY,
      factor = Math.exp(Math.max(-0.4, Math.min(0.4, e.deltaY * 0.0015)));
    navigation.current.transform((c) => {
      // Batched wheel events can share a listener from an earlier render. Use
      // the SAME current camera for both pointer conversion and zooming.
      const p = screenToMap(x, y, rect, c);
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
  async function exportMap(
    format,
    outputCamera = camera,
    outputState = active.current,
  ) {
    if (!svg.current) return;
    if (format === "assets" && exportWidth > 4096)
      throw Error(
        "Choose an export width up to 4096 pixels to save a square map and banner.",
      );
    if (
      (exportWidth * exportWidth * outputCamera.h) / outputCamera.w >
      16777216
    )
      throw Error("Choose a smaller export width for this tall view.");
    const clone = svg.current.cloneNode(true);
    if (!illustrating)
      prepareGeographicExport(
        clone,
        geometryCache.current.derive({
          identity: sourceIdentity,
          revision: outputState.project.revision,
          rivers,
          coasts,
          level: detailLevel,
          fast: false,
        }),
        projection.point,
        outputCamera,
      );
    clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    clone.setAttribute("width", String(exportWidth));
    clone.setAttribute(
      "height",
      String((exportWidth * outputCamera.h) / outputCamera.w),
    );
    const exportBounds = projection.box(outputCamera);
    const landMask = clone.querySelector("#selected-illustration-land path");
    if (landMask && art?.regions)
      landMask.setAttribute(
        "d",
        illustrationCoverage
          .map(
            (p) =>
              pathText(clipPolygon(p, outputCamera).map(projection.point)) +
              " Z",
          )
          .join(" "),
      );
    clone.setAttribute(
      "viewBox",
      `${exportBounds.x} ${exportBounds.y} ${exportBounds.w} ${exportBounds.h}`,
    );
    for (const mask of clone.querySelectorAll("mask")) {
      mask.setAttribute("x", String(exportBounds.x));
      mask.setAttribute("y", String(exportBounds.y));
      mask.setAttribute("width", String(exportBounds.w));
      mask.setAttribute("height", String(exportBounds.h));
    }
    // Preview transactions are editor state, not export quality or content.
    for (const frame of clone.querySelectorAll(
      '[aria-label="Committed terrain viewport"]',
    ))
      for (const key of [
        "data-generation",
        "data-phase",
        "data-pixel-width",
        "data-pixel-height",
      ])
        frame.removeAttribute(key);
    for (const selected of clone.querySelectorAll(
      '[aria-label="Illustration selection"], [data-editor-handle]',
    ))
      selected.remove();
    await exportTerrain(clone, {
      camera: outputCamera,
      width: exportWidth,
      projection,
      session: outputState.id,
      revision: outputState.project.revision,
      identity: JSON.stringify([outputState.id, outputState.project.revision]),
      wrapLongitude: wrapIllustration,
      onProgress: setIllustrationProgress,
    });
    if (format === "assets") return illustrationAssets(clone, exportWidth);
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
      c.height = Math.round((exportWidth * outputCamera.h) / outputCamera.w);
      c.getContext("2d").drawImage(img, 0, 0, c.width, c.height);
      const png = await new Promise((resolve) =>
        c.toBlob(resolve, "image/png"),
      );
      if (!png) throw Error("The illustration image could not be rendered.");
      if (format === "snapshot") return png;
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
  async function saveIllustration() {
    if (!art) return;
    const started = performance.now();
    setSavedIllustration(null);
    const design = art;
    await ensureWorldMap();
    if (
      JSON.stringify(active.current.project.illustrations?.[design.id]) !==
      JSON.stringify(design)
    )
      await command({ kind: "illustration", illustration: design });
    if (active.current.unsaved) await save();
    const source = active.current;
    const sourceMs = performance.now() - started;
    const assetKey = JSON.stringify([
      source.id,
      source.project.revision,
      design,
      invalidations,
      entities,
      rivers.length,
      exportWidth,
      hillshade,
      artTerrain?.bounds,
    ]);
    setIllustrationProgress({
      phase: "Preparing map images",
      completed: 0,
      total: 1,
    });
    const cached = imageAssets.current?.key === assetKey;
    const assets = cached
      ? imageAssets.current.assets
      : await exportMap("assets", {
          x: design.bounds[0],
          y: design.bounds[1],
          w: design.bounds[2],
          h: design.bounds[3],
        });
    imageAssets.current = { key: assetKey, assets };
    const rasterMs = performance.now() - started - sourceMs;
    if (
      active.current?.id !== source.id ||
      active.current.project.revision !== source.project.revision
    )
      throw Error(
        "The map changed while rendering; save the illustration again.",
      );
    const form = new FormData();
    form.append(
      "metadata",
      JSON.stringify({
        sourceRevision: source.project.revision,
        illustration: design,
      }),
    );
    form.append("file", assets.map, "illustration.png");
    form.append("banner", assets.banner, "banner.png");
    setIllustrationProgress({
      phase: "Saving map and banner",
      completed: 0,
      total: 1,
    });
    const response = await fetch(
      `/api/sessions/${source.id}/illustrated-maps`,
      { method: "POST", body: form },
    );
    const result = await response.json();
    if (!response.ok)
      throw Error(result.error || "Could not save the illustration.");
    setWorldContext(result.document);
    const saves = (window.__illustrationSaveLog ||= []);
    saves.push({
      sourceMs,
      rasterMs,
      totalMs: performance.now() - started,
      serverTiming: response.headers.get("Server-Timing"),
      cached,
    });
    if (saves.length > 20) saves.shift();
    setIllustrationProgress(null);
    const folder = result.document.entities.find(
      (e) =>
        e.id === result.archiveId && e.ageId === source.historicalMap.ageId,
    );
    setSavedIllustration({
      name: design.title,
      folder: folder?.name || "Illustrated maps",
      href: `/worlds/${source.historyWorldId}/library?age=${source.historicalMap.ageId}&record=${result.fileId}`,
    });
  }
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
        b.x <= geographicViewport.x + geographicViewport.w &&
        b.x + b.w >= geographicViewport.x &&
        b.y <= geographicViewport.y + geographicViewport.h &&
        b.y + b.h >= geographicViewport.y
      );
    })
    .sort(
      (a, b) =>
        state.project.layers.find((l) => l.id === a.layer).order -
        state.project.layers.find((l) => l.id === b.layer).order,
    );
  const riverShapes = useMemo(() => riverFeatures(rivers), [rivers]);
  const geography = useMemo(
    () =>
      geometryCache.current.derive({
        identity: sourceIdentity,
        revision: state?.project?.revision,
        rivers,
        coasts,
        level: detailLevel,
        fast: previewQuality.preset === "performance",
        pixel: 1 / projection.scale,
      }),
    [
      sourceIdentity,
      state?.project?.revision,
      rivers,
      coasts,
      detailLevel,
      previewQuality.preset,
      projection.scale,
    ],
  );
  const displayedRivers = useMemo(
    () =>
      (style === "river-network" ? riverShapes : geography.rivers)
        .filter((f) => intersects(f.bounds, projection.bounds))
        .map((f) => ({
          feature: f,
          path: clipLine(f.points, projection.bounds)
            .map((part) => pathText(part.map(projection.point)))
            .join(" "),
        }))
        .filter((f) => f.path),
    [style, riverShapes, geography, projection],
  );
  const featureIndex = useMemo(
    () =>
      new FeatureIndex([
        ...visibleEntities.map((e) => ({
          ...e,
          points: e.points.map((p) => projectToMap(p, state)),
        })),
        ...(showRivers ? displayedRivers.map((f) => f.feature) : []),
      ]),
    [
      entities,
      state?.project?.revision,
      camera.x,
      camera.y,
      camera.w,
      camera.h,
      width,
      height,
      showRivers,
      riverShapes,
      geography,
      displayedRivers,
    ],
  );
  function feature(e, preview = false) {
    const isSelected = selected?.id === e.id;
    const f = isSelected ? selected : e;
    const common = {
      key: e.id,
      stroke: isSelected ? "#fff4b1" : e.color || "#ddba7b",
      strokeWidth: e.kind === "road" ? 2 : 1.2,
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
      <g key={e.id} data-map-feature={e.id} opacity={preview ? 0.65 : 1}>
        {f.geometry === "polygon" ? (
          <>
            <polygon
              {...common}
              stroke="none"
              points={projection.points(
                clipPolygon(
                  f.points.map((p) => projectToMap(p, state)),
                  projection.bounds,
                ),
              )}
            />
            <path
              {...common}
              fill="none"
              d={drawPath(
                [...f.points, f.points[0]].map((p) => projectToMap(p, state)),
              )}
            />
          </>
        ) : f.geometry === "line" ? (
          <path
            {...common}
            d={drawPath(f.points.map((p) => projectToMap(p, state)))}
          />
        ) : (
          <circle
            {...common}
            cx={drawProjectPoint(f.points[0])[0]}
            cy={drawProjectPoint(f.points[0])[1]}
            r={4}
            fill={e.color}
          />
        )}
        {f.name &&
          f.kind !== "house" &&
          f.kind !== "road" &&
          f.kind !== "district" && (
            <text
              x={drawProjectPoint(f.points[0])[0]}
              y={drawProjectPoint(f.points[0])[1] - 8}
              fill="#f0e6c8"
              fontSize={12}
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
      {mapSetup?.step === "name" && (
        <Dialog
          title="Name illustrated map"
          onClose={() => {
            setupSequence.current++;
            setMapSetup(null);
          }}
          onSubmit={setupIllustration}
          trackChanges={false}
          actions={
            <>
              <button type="button" data-dialog-dismiss>
                Cancel
              </button>
              <button
                type="submit"
                className="primary"
                disabled={!mapSetup.title.trim()}
              >
                Continue
              </button>
            </>
          }
        >
          <label>
            Map name
            <input
              aria-label="Illustrated map name"
              autoFocus
              required
              maxLength={160}
              value={mapSetup.title}
              onChange={(e) =>
                setMapSetup({ ...mapSetup, title: e.target.value })
              }
            />
          </label>
          {Object.values(state.project.illustrations || {}).length > 0 && (
            <label>
              Saved map
              <select
                aria-label="Saved illustrated map"
                value={mapSetup.resume ? art?.id || "" : ""}
                onChange={(e) => {
                  const saved = state.project.illustrations[e.target.value];
                  if (saved) {
                    setArt(saved);
                    setMapSetup({
                      ...mapSetup,
                      resume: true,
                      title: saved.title,
                      kind: saved.focus ? "place" : "land",
                      focus: entities.find((e) => e.id === saved.focus) || null,
                    });
                  } else setMapSetup({ ...mapSetup, resume: false, title: "" });
                }}
              >
                <option value="">Create a new map</option>
                {Object.values(state.project.illustrations).map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.title}
                  </option>
                ))}
              </select>
            </label>
          )}
          {mapSetup.focus && (
            <label>
              Map content
              <select
                aria-label="Illustrated map content"
                value={mapSetup.kind}
                onChange={(e) =>
                  setMapSetup({
                    ...mapSetup,
                    kind: e.target.value,
                    resume: false,
                  })
                }
              >
                <option value="place">{mapSetup.focus.name}</option>
                <option value="land">Choose continents and islands</option>
              </select>
            </label>
          )}
        </Dialog>
      )}
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
            ["atlas", "Atlas"],
            ["generate", "Terrain Generator"],
            ["map", "Map Builder"],
            ["settlement", "Settlement Builder"],
          ].map(([id, label]) => (
            <button
              key={id}
              aria-current={mode === id ? "page" : undefined}
              disabled={busy || (id !== "generate" && !state)}
              onClick={() =>
                run(async () => {
                  if (id !== "generate") await ensureWorldMap();
                  setMode(id);
                  if (id === "generate" || id === "atlas")
                    if (illustrating) leaveIllustration();
                  setTool(id === "settlement" ? "settlement" : "inspect");
                  setDraft([]);
                })
              }
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
            aria-pressed={illustrating || !!mapSetup}
            aria-describedby={
              mapSetup?.step === "land"
                ? "illustrator-land-instructions"
                : undefined
            }
            className={
              illustrating || mapSetup ? "studio-illustrator-active" : ""
            }
            onClick={() => {
              if (mapSetup) {
                setupSequence.current++;
                setCamera(mapSetup.camera);
                setMapSetup(null);
                setLandBox(null);
              } else if (illustrating) leaveIllustration();
              else startIllustration();
            }}
          >
            {mapSetup?.step === "land"
              ? "Illustrator: select land"
              : illustrating
                ? "Illustrator active · return to construction"
                : "Medieval Illustrator"}
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
          {(mode === "generate" || mode === "atlas"
            ? ["inspect"]
            : mode === "settlement"
              ? ["settlement", "select", "road", "house", "landmark", "site"]
              : [
                  "inspect",
                  "select",
                  "road",
                  "state",
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
              title={t === "site" ? "Generate settlement layout" : t}
              key={t}
              aria-pressed={tool === t}
              onClick={() => {
                setTool(t);
                setDraft([]);
              }}
            >
              {t === "site"
                ? "Generate layout"
                : t === "state"
                  ? "State"
                  : t === "settlement"
                    ? "Settlement"
                    : t}
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
          <ProjectionContext.Provider value={projection}>
            <svg
              ref={svg}
              role="img"
              aria-label="Interactive continuous world map"
              tabIndex={0}
              viewBox={`0 0 ${width} ${height}`}
              data-camera={JSON.stringify(camera)}
              onPointerDown={down}
              onPointerMove={move}
              onPointerUp={up}
              onPointerCancel={() => {
                drag.current = null;
              }}
            >
              <rect
                x="0"
                y="0"
                width={width}
                height={height}
                fill={illustrating ? "#e9d9af" : "#101d2a"}
              />
              {state && (
                <ViewportSurface
                  wrapLongitude={wrapIllustration}
                  active={viewportRenderer}
                  enabled={!show3D}
                  session={state.id}
                  revision={state.project.revision}
                  invalidations={invalidations}
                  identity={`${state.worldId}:${state.id}`}
                  camera={camera}
                  width={width}
                  height={height}
                  previewQuality={illustrating ? undefined : previewQuality}
                  previewHillshade={
                    illustrating || displayStyle === "terrain"
                      ? undefined
                      : hillshade
                  }
                  onDiagnostics={setRenderDiagnostics}
                  onProgress={setTerrainProgress}
                  detailLevel={detailLevel}
                  retry={renderRetry}
                  onLoading={setLoading}
                  onError={setRenderError}
                  styles={[
                    displayStyle,
                    ...(displayStyle !== "terrain" &&
                    !illustrating &&
                    hillshade > 0 &&
                    previewQuality.preset !== "performance"
                      ? ["hillshade"]
                      : []),
                    ...(illustrating ? ["relief-shadow", "shore-ink"] : []),
                    ...(showRivers || illustrating ? ["river-mask"] : []),
                  ]}
                >
                  <defs>
                    {illustrating && art?.regions?.length > 0 && (
                      <mask
                        id="selected-illustration-land"
                        maskUnits="userSpaceOnUse"
                        x="0"
                        y="0"
                        width={width}
                        height={height}
                      >
                        <path
                          fill="white"
                          fillRule="nonzero"
                          stroke="white"
                          strokeWidth="1"
                          d={landPath(illustrationCoverage)}
                        />
                      </mask>
                    )}
                    <mask
                      id="terrain-river-mask"
                      maskUnits="userSpaceOnUse"
                      x="0"
                      y="0"
                      width={width}
                      height={height}
                    >
                      <g
                        mask={
                          illustrating && art?.regions?.length
                            ? "url(#selected-illustration-land)"
                            : undefined
                        }
                      >
                        {viewportRenderer ? (
                          <ViewportLayer style="river-mask" />
                        ) : (
                          <TileSurface
                            wrapLongitude={wrapIllustration}
                            enabled={!show3D}
                            session={state.id}
                            revision={state.project.revision}
                            invalidations={invalidations}
                            identity={`${state.worldId}:${state.id}`}
                            style="river-mask"
                            camera={camera}
                            width={width}
                            height={height}
                          />
                        )}
                      </g>
                    </mask>
                  </defs>
                  <g
                    mask={
                      illustrating && art?.regions?.length
                        ? "url(#selected-illustration-land)"
                        : undefined
                    }
                  >
                    {viewportRenderer ? (
                      <ViewportLayer style={displayStyle} primary />
                    ) : (
                      <TileSurface
                        wrapLongitude={wrapIllustration}
                        enabled={!show3D}
                        session={state.id}
                        revision={state.project.revision}
                        invalidations={invalidations}
                        style={displayStyle}
                        identity={`${state.worldId}:${state.id}`}
                        camera={camera}
                        width={width}
                        height={height}
                        onLoading={setLoading}
                      />
                    )}
                  </g>
                  {hillshade > 0 &&
                    !illustrating &&
                    displayStyle !== "terrain" && (
                      <g opacity={hillshade} pointerEvents="none">
                        {viewportRenderer ? (
                          <ViewportLayer style="hillshade" />
                        ) : (
                          <TileSurface
                            wrapLongitude={wrapIllustration}
                            enabled={!show3D}
                            session={state.id}
                            revision={state.project.revision}
                            invalidations={invalidations}
                            style="relief-shadow"
                            identity={`${state.worldId}:${state.id}`}
                            camera={camera}
                            width={width}
                            height={height}
                          />
                        )}
                      </g>
                    )}
                  {!illustrating &&
                    displayStyle === "physical-atlas" &&
                    !state.continuous.analysisStale && (
                      <g
                        fill="none"
                        stroke="#556c68"
                        strokeOpacity=".65"
                        strokeWidth={0.65}
                        pointerEvents="none"
                      >
                        {geography.coasts.map((p, i) => (
                          <path
                            key={i}
                            data-coast-index={i}
                            d={pathText(p.map(drawPoint))}
                          />
                        ))}
                      </g>
                    )}
                  {showRivers && !illustrating && (
                    <g
                      mask="url(#terrain-river-mask)"
                      fill="none"
                      stroke="#71b7d3"
                      strokeOpacity=".7"
                      pointerEvents="none"
                    >
                      {displayedRivers.map(({ feature: f, path }) => {
                        const r = f.river;
                        return (
                          <path
                            key={f.id}
                            data-river-id={f.id}
                            strokeLinecap="round"
                            strokeLinejoin="round"
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
                              r.class === "ephemeral" ? "3 2" : undefined
                            }
                            strokeWidth={
                              riverWidth(
                                r,
                                1 / projection.scale,
                                state.continuous.spec.radiusKm,
                              ) * projection.scale
                            }
                            d={path}
                          />
                        );
                      })}
                    </g>
                  )}
                  {!illustrating && visibleEntities.map((e) => feature(e))}
                  {mapSetup?.step === "land" && (
                    <g aria-label="Selected landmasses" pointerEvents="none">
                      {landComponents
                        .filter((c) => landIDs.includes(c.id))
                        .map((c) => (
                          <path
                            key={c.id}
                            d={landPath(c.paths)}
                            fill="#e7c56b66"
                            fillRule="evenodd"
                            stroke="#ffdd83"
                            strokeWidth="1.5"
                          />
                        ))}
                      {landBox && (
                        <rect
                          {...(() => {
                            const b = projection.box(landBox);
                            return { x: b.x, y: b.y, width: b.w, height: b.h };
                          })()}
                          fill="#f8db7044"
                          stroke="#ffdd83"
                        />
                      )}
                    </g>
                  )}
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
                        viewportRenderer ? (
                          <ViewportLayer style="shore-ink" />
                        ) : (
                          <TileSurface
                            wrapLongitude={wrapIllustration}
                            enabled={!show3D}
                            session={state.id}
                            revision={state.project.revision}
                            invalidations={invalidations}
                            style="shore-ink"
                            identity={`${state.worldId}:${state.id}`}
                            camera={camera}
                            width={width}
                            height={height}
                          />
                        )
                      }
                      riverMask="url(#terrain-river-mask)"
                      landMask={
                        art.regions?.length
                          ? "url(#selected-illustration-land)"
                          : undefined
                      }
                      relief={
                        <g opacity={hillshade} pointerEvents="none">
                          {viewportRenderer ? (
                            <ViewportLayer style="relief-shadow" />
                          ) : (
                            <TileSurface
                              wrapLongitude={wrapIllustration}
                              enabled={!show3D}
                              session={state.id}
                              revision={state.project.revision}
                              invalidations={invalidations}
                              style="relief-shadow"
                              identity={`${state.worldId}:${state.id}`}
                              camera={camera}
                              width={width}
                              height={height}
                            />
                          )}
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
                        cx={drawProjectPoint(p)[0]}
                        cy={drawProjectPoint(p)[1]}
                        r={4}
                        fill="#fff0a3"
                        stroke="#473c28"
                        strokeWidth={1}
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
                      points={drawProjectPoints(draft)}
                      fill={tool === "country" ? "#ddc98c33" : "none"}
                      stroke="#ffe3a2"
                      strokeWidth={2}
                    />
                  )}
                  {inspectionPoint?.identity === sourceIdentity &&
                    inspectionPoint.revision === state.project.revision && (
                      <circle
                        data-editor-handle="selection"
                        cx={drawPoint(inspectionPoint.point)[0]}
                        cy={drawPoint(inspectionPoint.point)[1]}
                        r={6}
                        fill="none"
                        stroke="#fff4b1"
                        strokeWidth={1.5}
                        pointerEvents="none"
                      />
                    )}
                  {cursor &&
                    ["raise", "lower", "smooth", "water", "plant"].includes(
                      tool,
                    ) && (
                      <circle
                        cx={drawPoint(cursor)[0]}
                        cy={drawPoint(cursor)[1]}
                        r={
                          (projection.scale * radius) /
                          (Math.PI * state.continuous.spec.radiusKm)
                        }
                        fill="#ffe5a522"
                        stroke="#ffe5a5"
                        strokeWidth={1}
                        pointerEvents="none"
                      />
                    )}
                </ViewportSurface>
              )}
            </svg>
          </ProjectionContext.Provider>
          {mapSetup?.step === "land" && (
            <div
              className="studio-illustration-selection"
              role="status"
              id="illustrator-land-instructions"
            >
              <strong>
                Illustrator selection mode · {landIDs.length} landmasses
                selected
              </strong>
              <span>
                Click land to select or deselect it. Drag a rectangle to add
                islands, then create the illustrated map.
              </span>
            </div>
          )}
          {state && loading > 0 && !show3D && (
            <div
              className="studio-detail-loading"
              role="status"
              aria-live="polite"
            >
              <strong>
                {terrainProgress?.phase === "final"
                  ? "Loading terrain details…"
                  : "Preparing terrain preview…"}
              </strong>
              <progress
                aria-label="Terrain detail loading"
                max={terrainProgress?.total || 1}
                value={
                  terrainProgress?.total ? terrainProgress.completed : undefined
                }
              />
            </div>
          )}
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
            {mapSetup?.step === "land" && (
              <section aria-label="Choose land for the map">
                <h2>{mapSetup.title}</h2>
                <p>
                  Click a connected piece of land to select it; click again to
                  remove it. Drag a rectangle to add islands and archipelagos.
                </p>
                {mapSetup.loading ? (
                  <p role="status">Finding connected land…</p>
                ) : (
                  <p>{landIDs.length} selected</p>
                )}
                <button
                  className="primary"
                  disabled={mapSetup.loading || !landIDs.length || busy}
                  onClick={createSelectedMap}
                >
                  Create illustrated map
                </button>
                <button
                  onClick={() => {
                    setupSequence.current++;
                    setCamera(mapSetup.camera);
                    setMapSetup(null);
                    setLandBox(null);
                  }}
                >
                  Cancel map
                </button>
              </section>
            )}
            {!illustrating && ["state", "settlement"].includes(tool) && (
              <section>
                <h3>{tool === "state" ? "State" : "Settlement"}</h3>
                <label>
                  Type
                  <select
                    aria-label="Drawing place type"
                    value={drawingType[tool]}
                    onChange={(e) =>
                      setDrawingType({ ...drawingType, [tool]: e.target.value })
                    }
                  >
                    {(tool === "state"
                      ? ["country", "state", "kingdom", "empire"]
                      : ["capital", "city", "town", "village", "hamlet"]
                    ).map((k) => (
                      <option key={k}>{k}</option>
                    ))}
                  </select>
                </label>
                <p>
                  Click to draw the boundary. Finish the shape, name it, and
                  Apply changes to publish its record.
                </p>
              </section>
            )}
            {illustrating && art && (
              <IllustrationPanel
                busy={busy}
                progress={illustrationProgress}
                onSave={() => run(saveIllustration)}
                saved={savedIllustration}
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
                  ["continents", "Number of continents", 1, 12, 1, ""],
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
                    Climate model
                    <select
                      value={spec.climateVersion || 1}
                      onChange={(e) =>
                        setSpec({ ...spec, climateVersion: +e.target.value })
                      }
                    >
                      <option value={1}>Version 1</option>
                      <option value={2}>
                        Version 2 — corrected elevation and moisture
                        (experimental)
                      </option>
                      <option value={3}>
                        Version 3 — seasonal heat and rain uplift (experimental)
                      </option>
                    </select>
                    {state?.continuous?.spec && (
                      <small>
                        Displayed terrain: {state.continuous.spec.version},
                        climate version{" "}
                        {state.continuous.spec.climateVersion || 0}. Selecting a
                        model applies to the next generated preview; it does not
                        migrate saved terrain.
                      </small>
                    )}
                  </label>
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
                    aria-label="Settlement size"
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
                    aria-label="Settlement layout"
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
                  Draw a Settlement boundary, then choose its type and name in
                  the selection panel. Generate layout is an optional procedural
                  preview of streets and buildings; it does not publish records.
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
                          await ensureWorldMap();
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
                      Add layout to terrain
                    </button>
                    <button onClick={() => setSettlement(null)}>Discard</button>
                  </>
                )}
              </>
            )}
            {selected && !illustrating && (
              <section className="studio-inspector">
                <h3>
                  {["country", "state", "kingdom", "empire"].includes(
                    selected.kind,
                  )
                    ? "State"
                    : ["capital", "city", "town", "village", "hamlet"].includes(
                          selected.kind,
                        )
                      ? "Settlement"
                      : "Object"}
                </h3>
                {[
                  "country",
                  "state",
                  "kingdom",
                  "empire",
                  "capital",
                  "city",
                  "town",
                  "village",
                  "hamlet",
                ].includes(selected.kind) && (
                  <label>
                    Type
                    <select
                      aria-label="Selected place type"
                      value={selected.kind}
                      onChange={(e) =>
                        setSelected({ ...selected, kind: e.target.value })
                      }
                    >
                      {(["country", "state", "kingdom", "empire"].includes(
                        selected.kind,
                      )
                        ? ["country", "state", "kingdom", "empire"]
                        : ["capital", "city", "town", "village", "hamlet"]
                      ).map((k) => (
                        <option key={k} value={k}>
                          {k}
                        </option>
                      ))}
                    </select>
                  </label>
                )}
                {selected.recordStatus === "draft" && (
                  <p>
                    Terrain object only. Give it a name and Apply changes to
                    create its record.
                  </p>
                )}
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
                    run(async () => {
                      await command({
                        kind: "entity",
                        entity: selected,
                        publish: true,
                      });
                      setSelected((v) => ({ ...v, recordStatus: "published" }));
                    })
                  }
                >
                  Apply changes
                </button>
                {selected.recordStatus !== "draft" && (
                  <button
                    onClick={() =>
                      run(async () => {
                        const s = await ensureWorldMap();
                        onExit?.(
                          s.historyWorldId,
                          selected.logicalId || selected.id,
                          s.historicalMap.ageId,
                        );
                      })
                    }
                  >
                    Open {selected.name || "this place"} record
                  </button>
                )}
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
              {state?.continuous?.analysisStale && (
                <p role="status">
                  Landscape edits changed the terrain. Saved drainage and
                  climate require revalidation; their original graph is
                  retained. Undo restores the original analysis.
                </p>
              )}
              {hydrologyDiagnostics && (
                <details>
                  <summary>River topology diagnostics</summary>
                  <p>
                    {hydrologyDiagnostics.nodes} nodes ·{" "}
                    {hydrologyDiagnostics.reaches} connected reaches
                  </p>
                  <p>Graph {hydrologyDiagnostics.revision.slice(0, 12)}</p>
                  {Object.entries(hydrologyDiagnostics.counts).map(
                    ([key, n]) => (
                      <p key={key}>
                        {key.replaceAll("-", " ")}: {n}
                      </p>
                    ),
                  )}
                </details>
              )}
              {!illustrating && viewportRenderer && (
                <div title={previewHelp}>
                  <label>
                    Preview Quality
                    <select
                      aria-label="Preview Quality"
                      value={previewQuality.preset}
                      onChange={(e) =>
                        setPreviewQuality((q) => ({
                          ...q,
                          preset: e.target.value,
                        }))
                      }
                    >
                      {Object.entries(previewPresets).map(([key, p]) => (
                        <option key={key} value={key}>
                          {p.label} ({p.scale.toFixed(2)}×)
                        </option>
                      ))}
                      <option value="custom">Custom</option>
                    </select>
                  </label>
                  {previewQuality.preset === "custom" && (
                    <label>
                      Custom preview scale (
                      {previewQuality.customScale.toFixed(2)}×)
                      <input
                        type="range"
                        min="0.5"
                        max="1.5"
                        step="0.05"
                        value={previewQuality.customScale}
                        onChange={(e) =>
                          setPreviewQuality((q) => ({
                            ...q,
                            customScale: +e.target.value,
                          }))
                        }
                      />
                    </label>
                  )}
                  <p
                    className="studio-muted"
                    aria-label="Effective preview resolution"
                  >
                    Preview: {previewInfo.pixels.width} ×{" "}
                    {previewInfo.pixels.height} · {previewInfo.preview.label}
                    {previewInfo.preview.clamped &&
                      ` · limited to ${previewInfo.preview.effectiveScale.toFixed(2)}× by render budgets`}
                  </p>
                  <p className="studio-muted">
                    2D preview only; saved terrain and exports are unchanged.
                  </p>
                  <p>
                    Level {detailLevel} ·{" "}
                    {cartographicLevels[detailLevel].label}
                  </p>
                  <details>
                    <summary>Preview diagnostics</summary>
                    <p>
                      Source revision {state?.project?.revision} · geometry{" "}
                      {geography.durationMs.toFixed(1)} ms · geometry cache{" "}
                      {(geometryCache.current.bytes / 1048576).toFixed(1)} MiB
                    </p>
                    {renderDiagnostics && (
                      <>
                        <p>
                          First usable frame{" "}
                          {renderDiagnostics.firstMs.toFixed(0)} ms · final{" "}
                          {renderDiagnostics.finalMs?.toFixed(0) ?? "pending"}{" "}
                          ms
                        </p>
                        <p>
                          Raster cache{" "}
                          {(renderDiagnostics.cacheBytes / 1048576).toFixed(1)}{" "}
                          MiB · {renderDiagnostics.hits} hits /{" "}
                          {renderDiagnostics.misses} misses
                        </p>
                        <p>
                          {renderDiagnostics.outstanding} outstanding ·{" "}
                          {renderDiagnostics.cancelled} cancelled · working
                          canvas{" "}
                          {(
                            renderDiagnostics.workingCanvasBytes / 1048576
                          ).toFixed(1)}{" "}
                          MiB
                        </p>
                      </>
                    )}
                  </details>
                  {!previewSaved && (
                    <p role="status">
                      This browser could not save the preview preference.
                    </p>
                  )}
                </div>
              )}
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
                    "terrain",
                    "natural",
                    "elevation",
                    "biome",
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
            {inspectionPoint?.identity === sourceIdentity &&
              inspectionPoint.revision === state?.project?.revision &&
              !sample && <p role="status">Inspecting geography…</p>}
            {sample?.query?.identity === sourceIdentity &&
              sample.query.revision === state?.project?.revision && (
                <section>
                  <h3>Geographic inspection</h3>
                  <dl>
                    <dt>Surface</dt>
                    <dd>
                      {sample.biome} · {sample.water}
                    </dd>
                    <dt>Coordinates</dt>
                    <dd>
                      {sample.query.longitude.toFixed(5)}°,{" "}
                      {sample.query.latitude.toFixed(5)}°
                    </dd>
                    <dt>Query</dt>
                    <dd>
                      {sample.query.durationMs.toFixed(1)} ms · revision{" "}
                      {sample.query.revision}
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
                                body: JSON.stringify({
                                  ui: { camera: savedCamera, style },
                                }),
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
            entities={entities}
            rivers={showRivers ? riverShapes : []}
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
        <span title="Camera magnification relative to the full world">
          {(2 / camera.w).toFixed(1)}× zoom
        </span>
        {reliefResolution(
          state?.continuous.spec,
          camera,
          width,
          height,
          Math.min(3, devicePixelRatio || 1),
        )?.resolved && (
          <span
            title={`Base relief uses approximately ${reliefResolution(state.continuous.spec, camera, width, height).spacingKM.toFixed(1)} km north–south sample spacing. Closer zoom interpolates relief and reveals saved river and authored geometry; it does not generate new landforms. Preview Quality controls rendering cost, not geography. Finer relief requires a separate local-refinement system.`}
          >
            Base relief resolution limit
          </span>
        )}
        <span>
          {state
            ? `${((camera.w / 5) * Math.PI * state.continuous.spec.radiusKm).toFixed(1)} km ━━━━━`
            : ""}
        </span>
        <span role="status" title={loading < 0 ? renderError : undefined}>
          {loading > 0
            ? viewportRenderer
              ? "Rendering terrain…"
              : `Sampling ${loading} tiles…`
            : loading < 0
              ? viewportRenderer
                ? "Terrain render failed"
                : "Tile loading failed"
              : "Ready"}
          {loading < 0 && viewportRenderer && (
            <button onClick={() => setRenderRetry((v) => v + 1)}>
              Retry terrain
            </button>
          )}
        </span>
      </footer>
    </div>
  );
}
