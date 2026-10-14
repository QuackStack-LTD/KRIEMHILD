import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { request } from "./api.js";
import { visibleTiles } from "./camera.js";
export default function Terrain3D({ state, camera, onClose, onPick }) {
  const host = useRef(null),
    close = useRef(null),
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
    controls.minDistance = globe ? 1.001 : 0.0001;
    controls.maxDistance = 8;
    const light = new THREE.DirectionalLight(0xfff3dc, 2.3);
    light.position.set(-3, 5, 4);
    scene.add(light, new THREE.HemisphereLight(0xcbdff5, 0x405038, 1.2));
    const material = new THREE.MeshStandardMaterial({
        vertexColors: true,
        roughness: 0.92,
        side: THREE.DoubleSide,
      }),
      waterMaterial = new THREE.MeshStandardMaterial({
        color: 0x498ea8,
        transparent: true,
        opacity: 0.55,
        roughness: 0.25,
        side: THREE.DoubleSide,
      });
    const span = globe ? 2 : camera.w,
      center = globe
        ? [1, 0.5]
        : [camera.x + camera.w / 2, camera.y + camera.h / 2],
      latitude = (0.5 - center[1]) * Math.PI,
      cos = Math.max(0.15, Math.cos(latitude)),
      radius = state.continuous.spec.radiusKm * 1000;
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
        (h / (radius * Math.PI * span)) * 2 * exaggeration.current,
        ((y - center[1]) / span) * 2,
      ];
    };
    const cache = new Map(),
      pending = new Map();
    let signature = "",
      scheduling = false,
      disposed = false,
      lastCheck = 0,
      wanted = new Set(),
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
            ...m.colors[i].map((v) => v / 255),
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
    function install(key, m) {
      const ground = new THREE.Mesh(geometry(m, false), material),
        water = new THREE.Mesh(geometry(m, true), waterMaterial),
        group = new THREE.Group();
      ground.userData.bounds = m.bounds;
      group.add(ground, water);
      scene.add(group);
      cache.set(key, { group, m, used: performance.now() });
    }
    function disposeTile(t) {
      scene.remove(t.group);
      for (const mesh of t.group.children) mesh.geometry.dispose();
    }
    async function schedule() {
      const distance = view.position.distanceTo(controls.target);
      let region;
      if (globe) {
        const direction = view.position.clone().normalize(),
          x = 1 + Math.atan2(direction.x, direction.z) / Math.PI,
          y = 0.5 - Math.asin(direction.y) / Math.PI,
          w = Math.min(2, Math.max(0.0001, (distance - 1) * 1.2));
        region = { x: x - w / 2, y: y - w / 4, w, h: w / 2 };
      } else {
        const w = Math.max(0.00008, Math.min(2, (span * distance) / 2.5)),
          x = center[0] + (controls.target.x * span) / (2 * cos),
          y = center[1] + (controls.target.z * span) / 2;
        region = { x: x - w / 2, y: y - w / 3, w, h: (w * 2) / 3 };
      }
      const tiles = visibleTiles(region, 768, 1)
        .sort(
          (a, b) =>
            Math.hypot(
              (a.x + 0.5) * a.span - region.x - region.w / 2,
              (a.y + 0.5) * a.span - region.y - region.h / 2,
            ) -
            Math.hypot(
              (b.x + 0.5) * b.span - region.x - region.w / 2,
              (b.y + 0.5) * b.span - region.y - region.h / 2,
            ),
        )
        .slice(0, 24);
      const keys = tiles.map((t) => `${t.z}/${t.x}/${t.y}`),
        sig = keys.join(";");
      if (sig === signature) return;
      wanted = new Set(keys);
      for (const [key, abort] of pending)
        if (!wanted.has(key)) {
          abort.abort();
          pending.delete(key);
        }
      if (scheduling) return;
      signature = sig;
      scheduling = true;
      const queue = tiles.filter(
        (t, i) => !cache.has(keys[i]) && !pending.has(keys[i]),
      );
      setStatus(
        `LOD ${tiles[0]?.z || 0} · ${queue.length} visible mesh tiles loading`,
      );
      const worker = async () => {
        while (queue.length && !disposed) {
          const t = queue.shift(),
            key = `${t.z}/${t.x}/${t.y}`;
          if (!wanted.has(key)) continue;
          const abort = new AbortController();
          pending.set(key, abort);
          try {
            const m = await request(
              `sessions/${state.id}/terrain/mesh?z=${t.z}&x=${t.x}&y=${t.y}`,
              undefined,
              "GET",
              abort.signal,
            );
            if (!disposed && wanted.has(key) && !cache.has(key))
              install(key, m);
          } catch (e) {
            if (e.name !== "AbortError" && !disposed) setStatus(e.message);
          } finally {
            if (pending.get(key) === abort) pending.delete(key);
          }
        }
      };
      await Promise.all([worker(), worker(), worker()]);
      scheduling = false;
      if (disposed || sig !== signature) return;
      for (const [key, t] of cache) {
        if (wanted.has(key)) {
          t.used = performance.now();
          t.group.visible = true;
        } else t.group.visible = !keys.every((k) => cache.has(k));
      }
      while (cache.size > 48) {
        const old = [...cache]
          .filter(([k]) => !wanted.has(k))
          .sort((a, b) => a[1].used - b[1].used)[0];
        if (!old) break;
        disposeTile(old[1]);
        cache.delete(old[0]);
      }
      setStatus(
        `LOD ${tiles[0]?.z || 0} · ${keys.filter((k) => cache.has(k)).length} visible tiles · drag to orbit, scroll to refine; click to inspect`,
      );
    }
    const resize = () => {
      const r = element.getBoundingClientRect();
      renderer.setSize(Math.max(1, r.width), Math.max(1, r.height));
      view.aspect = r.width / Math.max(1, r.height);
      view.updateProjectionMatrix();
    };
    const observer = new ResizeObserver(resize);
    observer.observe(element);
    resize();
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
      }
      if (time - lastCheck > 250) {
        lastCheck = time;
        void schedule();
      }
      renderer.render(scene, view);
    });
    return () => {
      disposed = true;
      for (const abort of pending.values()) abort.abort();
      renderer.setAnimationLoop(null);
      observer.disconnect();
      controls.dispose();
      for (const t of cache.values()) disposeTile(t);
      material.dispose();
      waterMaterial.dispose();
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, [state.id, state.project.revision, camera, globe]);
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
