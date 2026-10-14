import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { request } from "./api.js";

export default function Terrain3D({ state, camera, onClose }) {
  const host = useRef(null),
    close = useRef(null);
  const [globe, setGlobe] = useState(true),
    [status, setStatus] = useState("Sampling terrain…");
  useEffect(() => {
    const previous = document.activeElement;
    close.current.focus();
    return () => previous?.focus();
  }, []);
  useEffect(() => {
    const abort = new AbortController();
    let dispose;
    setStatus("Sampling terrain…");
    const west = Math.max(-180, camera.x * 180 - 180),
      east = Math.min(180, (camera.x + camera.w) * 180 - 180);
    const north = Math.min(90, 90 - camera.y * 180),
      south = Math.max(-90, 90 - (camera.y + camera.h) * 180);
    const query = globe
      ? ""
      : `?west=${west}&east=${east}&south=${south}&north=${north}`;
    request(
      `sessions/${state.id}/terrain/mesh${query}`,
      undefined,
      "GET",
      abort.signal,
    )
      .then((mesh) => {
        if (abort.signal.aborted) return;
        const element = host.current;
        const renderer = new THREE.WebGLRenderer({ antialias: true });
        renderer.setPixelRatio(Math.min(2, window.devicePixelRatio));
        renderer.setClearColor(0x0a1721);
        element.appendChild(renderer.domElement);
        const scene = new THREE.Scene(),
          view = new THREE.PerspectiveCamera(42, 1, 0.001, 100);
        view.position.set(0, globe ? 0.6 : 2, globe ? 3.4 : 2.3);
        const controls = new OrbitControls(view, renderer.domElement);
        controls.enableDamping = true;
        controls.minDistance = 0.15;
        controls.maxDistance = 12;
        const light = new THREE.DirectionalLight(0xffffff, 2.2);
        light.position.set(-3, 5, 4);
        scene.add(light, new THREE.HemisphereLight(0xc8e5ff, 0x566044, 1.5));
        const positions = [],
          waterPositions = [],
          colors = [],
          indices = [],
          waterIndices = [];
        const [w, s, e, n] = mesh.bounds;
        const physicalScale = globe
          ? state.continuous.spec.radiusKm * 1000
          : state.continuous.spec.radiusKm * 1000 * Math.max(0.00001, e - w);
        const exaggeration = globe ? 12 : 2;
        const middleLatitude = (s + n) / 2;
        function point(lon, lat, height) {
          const z = (height / physicalScale) * exaggeration;
          if (globe) {
            const r = 1 + z;
            return [
              r * Math.cos(lat) * Math.sin(lon),
              r * Math.sin(lat),
              r * Math.cos(lat) * Math.cos(lon),
            ];
          }
          return [
            ((lon - (w + e) / 2) / (e - w)) * Math.cos(middleLatitude) * 2,
            z * 2,
            (-(lat - middleLatitude) / (e - w)) * 2,
          ];
        }
        for (let y = 0; y < mesh.rows; y++)
          for (let x = 0; x < mesh.columns; x++) {
            const i = y * mesh.columns + x,
              lon = w + ((e - w) * x) / (mesh.columns - 1),
              lat = n - ((n - s) * y) / (mesh.rows - 1);
            positions.push(...point(lon, lat, mesh.bed[i]));
            waterPositions.push(
              ...point(lon, lat, mesh.wet[i] ? mesh.surface[i] : mesh.bed[i]),
            );
            const c = new THREE.Color().setRGB(
              ...mesh.colors[i].map((v) => v / 255),
              THREE.SRGBColorSpace,
            );
            colors.push(c.r, c.g, c.b);
            if (x + 1 < mesh.columns && y + 1 < mesh.rows) {
              const a = i,
                b = i + 1,
                c = i + mesh.columns,
                d = c + 1;
              indices.push(a, c, b, b, c, d);
              for (const tri of [
                [a, c, b],
                [b, c, d],
              ])
                if (tri.every((k) => mesh.wet[k])) waterIndices.push(...tri);
            }
          }
        function geometry(points, triangles, withColor) {
          const g = new THREE.BufferGeometry();
          g.setAttribute(
            "position",
            new THREE.Float32BufferAttribute(points, 3),
          );
          g.setIndex(triangles);
          if (withColor)
            g.setAttribute(
              "color",
              new THREE.Float32BufferAttribute(colors, 3),
            );
          g.computeVertexNormals();
          return g;
        }
        const ground = geometry(positions, indices, true),
          water = geometry(waterPositions, waterIndices, false);
        const material = new THREE.MeshStandardMaterial({
          vertexColors: true,
          roughness: 0.92,
          side: THREE.DoubleSide,
        });
        const waterMaterial = new THREE.MeshStandardMaterial({
          color: 0x438da4,
          transparent: true,
          opacity: 0.65,
          roughness: 0.35,
          side: THREE.DoubleSide,
        });
        scene.add(
          new THREE.Mesh(ground, material),
          new THREE.Mesh(water, waterMaterial),
        );
        const resizeView = () => {
          const { width, height } = element.getBoundingClientRect();
          renderer.setSize(Math.max(1, width), Math.max(1, height));
          view.aspect = width / Math.max(1, height);
          view.updateProjectionMatrix();
          controls.update();
          renderer.render(scene, view);
        };
        resizeView();
        const resize = new ResizeObserver(resizeView);
        resize.observe(element);
        renderer.setAnimationLoop(() => {
          controls.update();
          renderer.render(scene, view);
        });
        dispose = () => {
          renderer.setAnimationLoop(null);
          resize.disconnect();
          controls.dispose();
          ground.dispose();
          water.dispose();
          material.dispose();
          waterMaterial.dispose();
          renderer.dispose();
          renderer.domElement.remove();
        };
        setStatus(
          `${globe ? "Globe · 12×" : "Region · 2×"} vertical exaggeration. Drag to orbit; scroll to zoom.`,
        );
      })
      .catch((e) => {
        if (e.name !== "AbortError") setStatus(e.message);
      });
    return () => {
      abort.abort();
      dispose?.();
    };
  }, [state.id, state.continuous.spec.radiusKm, camera, globe]);
  return (
    <div
      className="studio-3d"
      role="dialog"
      aria-modal="true"
      aria-label="Terrain in 3D"
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
        if (e.key === "Tab") {
          const buttons = [...e.currentTarget.querySelectorAll("button")];
          const i = buttons.indexOf(document.activeElement);
          e.preventDefault();
          buttons[
            (i + (e.shiftKey ? -1 : 1) + buttons.length) % buttons.length
          ].focus();
        }
      }}
    >
      <header>
        <strong>Terrain in 3D</strong>
        <button onClick={() => setGlobe(!globe)}>
          {globe ? "View current region" : "View globe"}
        </button>
        <button ref={close} onClick={onClose}>
          Close
        </button>
      </header>
      <div ref={host} className="studio-3d-canvas" />
      <p role="status">{status}</p>
    </div>
  );
}
