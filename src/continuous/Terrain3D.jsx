import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { request } from "./api.js";
import { meshCoverage, meshView, sameMeshView } from "./terrain3d-coverage.js";
import {
  drapeSegment,
  drapePolygon,
  meshBounds,
  meshHeight,
} from "./terrain-drape.js";
import { projectToMap } from "./geometry.js";
import { bounds as geographicBounds } from "./geometry.js";
export default function Terrain3D({
  state,
  camera,
  entities = [],
  rivers = [],
  onClose,
  onPick,
}) {
  const host = useRef(null),
    close = useRef(null),
    overhead = useRef(null),
    exaggeration = useRef(2);
  const [globe, setGlobe] = useState(false),
    [vertical, setVertical] = useState(2),
    [status, setStatus] = useState("Sampling visible terrain…");
  exaggeration.current = vertical;
  useEffect(() => {
    const previous = document.activeElement;
    close.current.focus();
    return () => previous?.focus();
  }, []);
  useEffect(() => {
    const element = host.current,
      renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setPixelRatio(Math.min(3, devicePixelRatio || 1));
    renderer.setClearColor(0x101d2a);
    element.appendChild(renderer.domElement);
    const scene = new THREE.Scene(),
      view = new THREE.PerspectiveCamera(42, 1, 0.00001, 100),
      controls = new OrbitControls(view, renderer.domElement);
    view.position.set(0, globe ? 0.3 : 1.5, globe ? 3.2 : 2.2);
    controls.enableDamping = true;
    controls.minDistance = globe ? 1.00001 : 0.000001;
    controls.maxDistance = 8;
    controls.screenSpacePanning = false;
    controls.zoomToCursor = true;
    controls.enablePan = !globe;
    if (!globe) controls.maxPolarAngle = Math.PI * 0.49;
    const material = new THREE.MeshBasicMaterial({
        vertexColors: true,
        toneMapped: false,
        side: THREE.DoubleSide,
      }),
      waterMaterial = new THREE.MeshBasicMaterial({
        vertexColors: true,
        toneMapped: false,
        side: THREE.DoubleSide,
      });
    const span = globe ? 2 : camera.w,
      center = globe
        ? [1, 0.5]
        : [camera.x + camera.w / 2, camera.y + camera.h / 2],
      latitude = (0.5 - center[1]) * Math.PI,
      cos = Math.max(0.15, Math.cos(latitude)),
      radius = state.continuous.spec.radiusKm * 1000;
    let overlayLimited = false;
    let labelBytes = 0,
      fillBytes = 0,
      lineBytes = 0;
    let datum = 0,
      ready = globe;
    const point = (x, y, h) => {
      if (globe) {
        const lon = (x - 1) * Math.PI,
          lat = (0.5 - y) * Math.PI,
          r = 1 + (h / radius) * exaggeration.current;
        return [
          r * Math.cos(lat) * Math.sin(lon),
          r * Math.sin(lat),
          r * Math.cos(lat) * Math.cos(lon),
        ];
      }
      return [
        ((x - center[0]) / span) * 2 * cos,
        ((h - datum) / (radius * Math.PI * span)) * 2 * exaggeration.current,
        ((y - center[1]) / span) * 2,
      ];
    };
    const cache = new Map(),
      pending = new Map();
    overhead.current = () => {
      const distance = view.position.distanceTo(controls.target);
      view.position
        .copy(controls.target)
        .add(new THREE.Vector3(0, distance, 0.0000001));
      controls.update();
    };
    const overlayFeatures = entities
      .filter(
        (e) =>
          !e.deleted &&
          state.project.layers.find((l) => l.id === e.layer)?.visible,
      )
      .map((e) => ({
        ...e,
        points: e.points.map((p) => projectToMap(p, state)),
      }))
      .concat(
        rivers.map((r) => ({
          ...r,
          geometry: "line",
          color: "#71b7d3",
          name: "",
          minDetail: ["major", "distributary"].includes(r.river?.class)
            ? 0
            : r.river?.class === "regional"
              ? 2
              : 4,
        })),
      )
      .filter((f) => f.points?.length)
      .map((f) => ({ ...f, bounds: geographicBounds(f.points) }));
    let renderedFrames = 0;
    let needsRender = true;
    const requestRender = () => {
      needsRender = true;
    };
    controls.addEventListener("change", requestRender);
    let lastPlan = null;
    let signature = "",
      scheduling = false,
      queued = [],
      disposed = false,
      lastCheck = 0,
      wanted = new Set(),
      cover = new Set(),
      lastVertical = vertical;
    function geometry(m, wet) {
      const positions = [],
        colors = [],
        uv = [],
        index = [];
      const [w, s, e, n] = m.bounds;
      for (let y = 0; y < m.rows; y++)
        for (let x = 0; x < m.columns; x++) {
          const i = y * m.columns + x,
            u = x / (m.columns - 1),
            v = y / (m.rows - 1),
            mx = (w + (e - w) * u) / Math.PI + 1,
            my = 0.5 - (n - (n - s) * v) / Math.PI;
          positions.push(...point(mx, my, wet ? m.surface[i] : m.bed[i]));
          uv.push(u, 1 - v);
          const c = new THREE.Color().setRGB(
            ...(m.litColors || m.colors)[i].map((v) => v / 255),
            THREE.SRGBColorSpace,
          );
          colors.push(c.r, c.g, c.b);
          if (x + 1 < m.columns && y + 1 < m.rows) {
            const a = i,
              b = i + 1,
              c = i + m.columns,
              d = c + 1;
            for (const tri of [
              [a, c, b],
              [b, c, d],
            ])
              if (!wet || tri.every((k) => m.wet[k])) index.push(...tri);
          }
        }
      // Skirts cover mixed-LOD T junctions; canonical top vertices are unchanged.
      if (!wet) {
        const edge = [];
        for (let x = 0; x < m.columns; x++) edge.push(x);
        for (let y = 1; y < m.rows; y++)
          edge.push(y * m.columns + m.columns - 1);
        for (let x = m.columns - 2; x >= 0; x--)
          edge.push((m.rows - 1) * m.columns + x);
        for (let y = m.rows - 2; y > 0; y--) edge.push(y * m.columns);
        const depth = Math.max(20, ((radius * (e - w)) / m.columns) * 0.15),
          start = positions.length / 3;
        for (const at of edge) {
          const u = (at % m.columns) / (m.columns - 1),
            v = Math.floor(at / m.columns) / (m.rows - 1);
          positions.push(
            ...point(
              (w + (e - w) * u) / Math.PI + 1,
              0.5 - (n - (n - s) * v) / Math.PI,
              m.bed[at] - depth,
            ),
          );
          colors.push(...colors.slice(at * 3, at * 3 + 3));
          uv.push(u, 1 - v);
        }
        for (let i = 0; i < edge.length; i++) {
          const j = (i + 1) % edge.length;
          index.push(
            edge[i],
            start + i,
            edge[j],
            edge[j],
            start + i,
            start + j,
          );
        }
      }
      const g = new THREE.BufferGeometry();
      g.setAttribute(
        "position",
        new THREE.Float32BufferAttribute(positions, 3),
      );
      g.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));
      g.setAttribute("uv", new THREE.Float32BufferAttribute(uv, 2));
      g.setIndex(index);
      g.computeVertexNormals();
      if (!wet && m.gradients) {
        const normals = g.getAttribute("normal");
        for (let i = 0; i < m.bed.length; i++) {
          const lon = w + ((e - w) * (i % m.columns)) / (m.columns - 1),
            lat = n - ((n - s) * Math.floor(i / m.columns)) / (m.rows - 1);
          const [east, north] = m.gradients[i].map(
            (v) => v * exaggeration.current,
          );
          const normal = globe
            ? new THREE.Vector3(
                Math.cos(lat) * Math.sin(lon) -
                  east * Math.cos(lon) +
                  north * Math.sin(lat) * Math.sin(lon),
                Math.sin(lat) - north * Math.cos(lat),
                Math.cos(lat) * Math.cos(lon) +
                  east * Math.sin(lon) +
                  north * Math.sin(lat) * Math.cos(lon),
              )
            : new THREE.Vector3((-east * Math.cos(lat)) / cos, 1, north);
          normal.normalize();
          normals.setXYZ(i, normal.x, normal.y, normal.z);
        }
      }
      return g;
    }
    function overlay(m, key) {
      const group = new THREE.Group(),
        b = meshBounds(m);
      for (const f of overlayFeatures) {
        if (
          f.bounds.x > b.x + b.w ||
          f.bounds.x + f.bounds.w < b.x ||
          f.bounds.y > b.y + b.h ||
          f.bounds.y + f.bounds.h < b.y
        )
          continue;
        if (f.minDetail > Number(key.split("/")[0]) + 1 || !f.points?.length)
          continue;
        const positions = [];
        const paths = [
          f.geometry === "polygon" ? [...f.points, f.points[0]] : f.points,
        ];
        for (const path of paths)
          for (let i = 1; i < path.length; i++) {
            const line = drapeSegment(m, path[i - 1], path[i]);
            for (let j = 1; j < line.length; j++)
              positions.push(
                ...point(line[j - 1][0], line[j - 1][1], line[j - 1][2] + 2),
                ...point(line[j][0], line[j][1], line[j][2] + 2),
              );
          }
        const color = f.color || "#e4c989";
        if (f.geometry === "polygon") {
          const fill = drapePolygon(m, f.points).flatMap((p) =>
            point(p[0], p[1], p[2] + 1),
          );
          if (fill.length && fillBytes + fill.length * 4 <= 8 * 1024 * 1024) {
            const geometry = new THREE.BufferGeometry();
            geometry.setAttribute(
              "position",
              new THREE.Float32BufferAttribute(fill, 3),
            );
            const mesh = new THREE.Mesh(
              geometry,
              new THREE.MeshBasicMaterial({
                color,
                transparent: true,
                opacity: f.kind === "house" ? 0.7 : 0.13,
                side: THREE.DoubleSide,
                depthWrite: false,
                toneMapped: false,
                polygonOffset: true,
                polygonOffsetFactor: -1,
                polygonOffsetUnits: -1,
              }),
            );
            mesh.userData.featureId = f.id;
            mesh.userData.fillBytes = fill.length * 4;
            fillBytes += mesh.userData.fillBytes;
            group.add(mesh);
          } else if (fill.length) overlayLimited = true;
        }
        if (
          positions.length &&
          lineBytes + positions.length * 4 <= 24 * 1024 * 1024
        ) {
          const geometry = new THREE.BufferGeometry();
          geometry.setAttribute(
            "position",
            new THREE.Float32BufferAttribute(positions, 3),
          );
          const line = new THREE.LineSegments(
            geometry,
            new THREE.LineBasicMaterial({
              color,
              toneMapped: false,
              depthTest: true,
            }),
          );
          line.userData.featureId = f.id;
          line.userData.lineBytes = positions.length * 4;
          lineBytes += line.userData.lineBytes;
          group.add(line);
        } else if (positions.length) overlayLimited = true;
        const p = f.points[0];
        if (p[0] < b.x || p[0] >= b.x + b.w || p[1] < b.y || p[1] >= b.y + b.h)
          continue;
        if (f.geometry === "point") {
          const mark = new THREE.Sprite(
            new THREE.SpriteMaterial({
              color,
              toneMapped: false,
              depthTest: false,
            }),
          );
          mark.position.set(...point(...p, meshHeight(m, p) + 3));
          const size = globe ? b.w * 0.035 : (b.w / span) * 0.035;
          mark.scale.set(size, size, 1);
          mark.userData.featureId = f.id;
          group.add(mark);
        }
        if (
          f.name &&
          !["house", "road", "district"].includes(f.kind) &&
          labelBytes + 512 * 64 * 4 <= 16 * 1024 * 1024
        ) {
          const canvas = document.createElement("canvas");
          canvas.width = 512;
          canvas.height = 64;
          const ctx = canvas.getContext("2d");
          ctx.font = "28px sans-serif";
          ctx.textAlign = "center";
          ctx.lineWidth = 5;
          ctx.strokeStyle = "#101d2a";
          ctx.strokeText(f.name, 256, 40, 500);
          ctx.fillStyle = "#f0e6c8";
          ctx.fillText(f.name, 256, 40, 500);
          const texture = new THREE.CanvasTexture(canvas);
          labelBytes += 512 * 64 * 4;
          const label = new THREE.Sprite(
            new THREE.SpriteMaterial({
              map: texture,
              toneMapped: false,
              depthTest: false,
            }),
          );
          label.position.set(...point(...p, meshHeight(m, p) + 5));
          const size = globe ? b.w * 0.5 : (b.w / span) * 0.5;
          label.scale.set(size, size / 8, 1);
          label.center.set(0.5, 0);
          label.userData.featureId = f.id;
          group.add(label);
        }
      }
      return group;
    }
    function install(key, m) {
      const ground = new THREE.Mesh(geometry(m, false), material),
        water = new THREE.Mesh(geometry(m, true), waterMaterial),
        group = new THREE.Group();
      ground.userData.bounds = m.bounds;
      group.add(ground, water, overlay(m, key));
      group.visible = false;
      scene.add(group);
      cache.set(key, { group, m, used: performance.now() });
      trimCache();
    }
    function disposeTile(t) {
      scene.remove(t.group);
      t.group.traverse((object) => {
        fillBytes -= object.userData.fillBytes || 0;
        lineBytes -= object.userData.lineBytes || 0;
        object.geometry?.dispose();
        if (
          object.material &&
          object.material !== material &&
          object.material !== waterMaterial
        ) {
          if (object.material.map) labelBytes -= 512 * 64 * 4;
          object.material.map?.dispose();
          object.material.dispose();
        }
      });
    }
    function trimCache() {
      // Keep two bounded covers during replacement, even if navigation never
      // goes idle. Obsolete invisible tiles cannot accumulate between batches.
      while (cache.size > 132) {
        const old = [...cache]
          .filter(([key, t]) => !wanted.has(key) && !t.group.visible)
          .sort((a, b) => a[1].used - b[1].used)[0];
        if (!old) break;
        disposeTile(old[1]);
        cache.delete(old[0]);
      }
    }
    async function schedule() {
      if (!ready || disposed) return;
      const rect = element.getBoundingClientRect();
      const pose = meshView(view, controls.target, rect.width, rect.height);
      if (signature && sameMeshView(pose, lastPlan)) return;
      lastPlan = pose;
      const tiles = meshCoverage({
        view,
        point,
        width: rect.width,
        height: rect.height,
        budget: 64,
      });
      const keys = tiles.map((t) => `${t.z}/${t.x}/${t.y}`),
        sig = [...keys].sort().join(";");
      if (sig === signature) return;
      signature = sig;
      cover = new Set(keys);
      wanted = new Set(["0/0/0", "0/1/0", ...keys]);
      for (const [key, abort] of pending)
        if (!wanted.has(key)) {
          abort.abort();
          pending.delete(key);
        }
      queued = [{ z: 0, x: 0, y: 0 }, { z: 0, x: 1, y: 0 }, ...tiles].filter(
        (t) => !cache.has(`${t.z}/${t.x}/${t.y}`),
      );
      if (scheduling) return;
      scheduling = true;
      setStatus(
        `LOD ${tiles[0]?.z || 0} · ${queued.length} visible mesh tiles loading`,
      );
      const worker = async () => {
        while (queued.length && !disposed) {
          const t = queued.shift(),
            key = `${t.z}/${t.x}/${t.y}`;
          if (!wanted.has(key) || cache.has(key) || pending.has(key)) continue;
          const abort = new AbortController();
          pending.set(key, abort);
          try {
            const m = await request(
              `sessions/${state.id}/terrain/mesh?z=${t.z}&x=${t.x}&y=${t.y}&revision=${state.project.revision}`,
              undefined,
              "GET",
              abort.signal,
            );
            if (!disposed && wanted.has(key) && !cache.has(key)) {
              install(key, m);
              publish();
            }
          } catch (e) {
            if (e.status === 409) {
              ready = false;
              queued = [];
              for (const abort of pending.values()) abort.abort();
            }
            if (e.name !== "AbortError" && !disposed) setStatus(e.message);
          } finally {
            if (pending.get(key) === abort) pending.delete(key);
          }
        }
      };
      await Promise.all([worker(), worker(), worker()]);
      scheduling = false;
      if (disposed) return;
      publish();
      while (cache.size > 128) {
        const old = [...cache]
          .filter(([k, t]) => !wanted.has(k) && !t.group.visible)
          .sort((a, b) => a[1].used - b[1].used)[0];
        if (!old) break;
        disposeTile(old[1]);
        cache.delete(old[0]);
      }
      const complete = [...wanted].every((k) => cache.has(k));
      if (!complete) signature = ""; // Retry failures; never freeze an incomplete cover.
      setStatus(
        overlayLimited
          ? "Dense geographic overlays reached the 32 MiB geometry budget. Zoom to a smaller region for additional overlay detail."
          : `${cache.size} cached tiles · ${pending.size} loading · drag to orbit, scroll to refine; click to inspect`,
      );
    }
    function publish() {
      let changed = false;
      const keys = [...wanted],
        complete = keys.every((k) => cache.has(k));
      for (const [key, t] of cache) {
        const previous = t.group.visible;
        if (
          (complete && cover.has(key)) ||
          (!complete && key.startsWith("0/"))
        ) {
          t.used = performance.now();
          t.group.visible = true;
        } else if (complete) t.group.visible = false;
        if (previous !== t.group.visible) changed = true;
      }
      if (changed) needsRender = true;
      element.dataset.coverComplete = String(complete);
      element.dataset.visibleTiles = String(
        [...cache.values()].filter((t) => t.group.visible).length,
      );
      element.dataset.wantedTiles = String(cover.size);
      element.dataset.labelBytes = String(labelBytes);
      element.dataset.overlayBytes = String(fillBytes + lineBytes);
      element.dataset.overlayLimited = String(overlayLimited);
      element.dataset.cachedTiles = String(cache.size);
      element.dataset.pendingTiles = String(pending.size);
      element.dataset.queuedTiles = String(queued.length);
      element.dataset.featureSegments = String(
        [...cache.values()]
          .filter((t) => t.group.visible)
          .reduce(
            (n, t) =>
              n +
              t.group.children[2].children.filter((c) => c.isLineSegments)
                .length,
            0,
          ),
      );
    }
    const resize = () => {
      needsRender = true;
      const r = element.getBoundingClientRect();
      renderer.setSize(Math.max(1, r.width), Math.max(1, r.height));
      view.aspect = r.width / Math.max(1, r.height);
      view.updateProjectionMatrix();
    };
    const observer = new ResizeObserver(resize);
    observer.observe(element);
    resize();
    const initialize = new AbortController();
    if (!globe) {
      request(
        `sessions/${state.id}/terrain/sample?lon=${center[0] * 180 - 180}&lat=${90 - center[1] * 180}&revision=${state.project.revision}`,
        undefined,
        "GET",
        initialize.signal,
      )
        .then((sample) => {
          if (disposed) return;
          datum =
            sample.water === "dry"
              ? sample.bed
              : Math.max(sample.bed, sample.waterSurface);
          ready = true;
          void schedule();
        })
        .catch((e) => {
          if (e.name !== "AbortError") setStatus(e.message);
        });
    }
    const ray = new THREE.Raycaster();
    let down = null;
    const start = (e) => {
      down = [e.clientX, e.clientY];
    };
    const pick = (event) => {
      if (
        !down ||
        Math.hypot(event.clientX - down[0], event.clientY - down[1]) > 4
      )
        return;
      const r = renderer.domElement.getBoundingClientRect();
      ray.setFromCamera(
        new THREE.Vector2(
          ((event.clientX - r.left) / r.width) * 2 - 1,
          1 - ((event.clientY - r.top) / r.height) * 2,
        ),
        view,
      );
      const hit = ray.intersectObjects(
        [...cache.values()]
          .filter((t) => t.group.visible)
          .map((t) => t.group.children[0]),
      )[0];
      if (hit?.uv) {
        const [w, s, e, n] = hit.object.userData.bounds,
          p = [
            (w + (e - w) * hit.uv.x) / Math.PI + 1,
            0.5 - (s + (n - s) * hit.uv.y) / Math.PI,
          ];
        const screenRadius =
          (hit.distance * 2 * Math.tan((view.fov * Math.PI) / 360) * 8) /
          r.height;
        const tolerance = globe
          ? screenRadius /
            Math.PI /
            Math.max(0.15, Math.cos((0.5 - p[1]) * Math.PI))
          : (screenRadius * span) / (2 * cos);
        onPick?.(p, tolerance);
        setStatus(
          `Selected ${(p[0] * 180 - 180).toFixed(4)}°, ${(90 - p[1] * 180).toFixed(4)}°`,
        );
      }
    };
    renderer.domElement.addEventListener("pointerdown", start);
    renderer.domElement.addEventListener("pointerup", pick);
    renderer.setAnimationLoop((time) => {
      controls.update();
      // Resolve metre-scale separation between water and bed even in a planetary
      // overview. A fixed microscopic near plane exhausts the depth buffer.
      const distance = view.position.distanceTo(controls.target);
      view.near = Math.max(
        1e-7,
        (globe ? Math.max(0.00001, distance - 1) : distance) * 0.02,
      );
      view.far = Math.max(10, distance + 4);
      view.updateProjectionMatrix();
      if (lastVertical !== exaggeration.current) {
        lastVertical = exaggeration.current;
        for (const [key, t] of [...cache]) {
          disposeTile(t);
          cache.delete(key);
          install(key, t.m);
        }
        publish();
        signature = "";
      }
      if (time - lastCheck > 250) {
        lastCheck = time;
        void schedule();
      }
      if (needsRender) {
        renderer.render(scene, view);
        element.dataset.renderedFrames = String(++renderedFrames);
        element.dataset.renderedTriangles = String(
          renderer.info.render.triangles,
        );
        needsRender = false;
      }
    });
    return () => {
      disposed = true;
      initialize.abort();
      for (const abort of pending.values()) abort.abort();
      renderer.setAnimationLoop(null);
      observer.disconnect();
      controls.dispose();
      controls.removeEventListener("change", requestRender);
      for (const t of cache.values()) disposeTile(t);
      material.dispose();
      waterMaterial.dispose();
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, [state.id, state.project.revision, camera, globe, entities, rivers]);
  return (
    <div
      className="studio-3d"
      role="dialog"
      aria-modal="true"
      aria-label="Terrain in 3D"
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
        if (e.key === "Tab") {
          const controls = [
              ...e.currentTarget.querySelectorAll("button,input"),
            ],
            i = controls.indexOf(document.activeElement);
          e.preventDefault();
          controls[
            (i + (e.shiftKey ? -1 : 1) + controls.length) % controls.length
          ].focus();
        }
      }}
    >
      <header>
        <strong>3D Terrain</strong>
        <button onClick={() => setGlobe(!globe)}>
          {globe ? "View current region" : "View globe"}
        </button>
        {!globe && (
          <button onClick={() => overhead.current?.()}>Look from above</button>
        )}
        <label>
          Vertical exaggeration{" "}
          <input
            aria-label="Vertical exaggeration"
            type="range"
            min="1"
            max="30"
            step="1"
            value={vertical}
            onChange={(e) => setVertical(+e.target.value)}
          />
          {vertical}×
        </label>
        <button onClick={() => setVertical(1)}>Natural scale</button>
        <button ref={close} onClick={onClose}>
          Close
        </button>
      </header>
      <div ref={host} className="studio-3d-canvas" />
      <p role="status">{status}</p>
    </div>
  );
}
