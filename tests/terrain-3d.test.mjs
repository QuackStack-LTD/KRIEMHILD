import test from "node:test";
import assert from "node:assert/strict";
import * as THREE from "three";
import {
  meshCoverage,
  meshView,
  sameMeshView,
} from "../src/continuous/terrain3d-coverage.js";
import {
  drapeSegment,
  meshHeight,
  clipSegment,
  drapePolygon,
} from "../src/continuous/terrain-drape.js";

test("idle orbit roundoff does not supersede loading; real pan, rotation and resize do", () => {
  const camera = new THREE.PerspectiveCamera(42, 1, 0.01, 20),
    target = new THREE.Vector3();
  camera.position.set(0, 1.5, 2.2);
  camera.lookAt(target);
  const original = meshView(camera, target, 1000, 700);
  for (let i = 0; i < 1000; i++) {
    camera.position.y += 1e-14;
    assert.ok(sameMeshView(original, meshView(camera, target, 1000, 700)));
  }
  assert.equal(
    sameMeshView(original, meshView(camera, target, 1001, 700)),
    false,
  );
  camera.position.x += 0.001;
  assert.equal(
    sameMeshView(original, meshView(camera, target, 1000, 700)),
    false,
  );
});

test("globe frustum cover includes seam and polar surface hits", () => {
  const point = (x, y, h) => {
    const lon = (x - 1) * Math.PI,
      lat = (0.5 - y) * Math.PI,
      r = 1 + h / 6371000;
    return [
      r * Math.cos(lat) * Math.sin(lon),
      r * Math.sin(lat),
      r * Math.cos(lat) * Math.cos(lon),
    ];
  };
  for (const lon of [-Math.PI, -0.4, Math.PI])
    for (const lat of [-1.3, 0, 1.3]) {
      const view = new THREE.PerspectiveCamera(42, 1.6, 0.00001, 10);
      view.position
        .set(...point(lon / Math.PI + 1, 0.5 - lat / Math.PI, 0))
        .multiplyScalar(1.1);
      view.lookAt(0, 0, 0);
      view.updateProjectionMatrix();
      const tiles = meshCoverage({
          view,
          point,
          width: 1000,
          height: 625,
          budget: 64,
        }),
        ray = new THREE.Raycaster();
      for (let i = -4; i <= 4; i++)
        for (let j = -4; j <= 4; j++) {
          ray.setFromCamera(new THREE.Vector2(i / 4, j / 4), view);
          const p = ray.ray.intersectSphere(
            new THREE.Sphere(new THREE.Vector3(), 1),
            new THREE.Vector3(),
          );
          if (!p) continue;
          const x = 1 + Math.atan2(p.x, p.z) / Math.PI,
            y = 0.5 - Math.asin(p.y) / Math.PI;
          assert.ok(
            tiles.some(
              (t) =>
                x >= t.x * t.span - 1e-12 &&
                x <= (t.x + 1) * t.span + 1e-12 &&
                y >= t.y * t.span - 1e-12 &&
                y <= (t.y + 1) * t.span + 1e-12,
            ),
          );
        }
    }
});

test("frustum coverage retains every visible ground point across aspect, pan, tilt and close zoom", () => {
  for (const aspect of [0.5, 1, 2.7])
    for (const distance of [2, 0.1, 0.0001])
      for (const tilt of [0.05, 0.9]) {
        const view = new THREE.PerspectiveCamera(
          42,
          aspect,
          distance * 0.01,
          20,
        );
        const target = new THREE.Vector3(0.21, 0, -0.13);
        view.position
          .copy(target)
          .add(new THREE.Vector3(0, distance, distance * tilt));
        view.lookAt(target);
        view.updateProjectionMatrix();
        const point = (x, y, h) => [(x - 1) * 2, h / 20000000, (y - 0.5) * 2];
        const tiles = meshCoverage({
          view,
          point,
          width: 800 * aspect,
          height: 800,
          budget: 64,
        });
        assert.ok(tiles.length <= 64 && tiles.length > 0);
        const ray = new THREE.Raycaster(),
          plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
        for (let j = 0; j <= 12; j++)
          for (let i = 0; i <= 12; i++) {
            ray.setFromCamera(new THREE.Vector2(i / 6 - 1, j / 6 - 1), view);
            const p = ray.ray.intersectPlane(plane, new THREE.Vector3());
            if (!p) continue;
            const x = p.x / 2 + 1,
              y = p.z / 2 + 0.5;
            if (x < 0 || x > 2 || y < 0 || y > 1) continue;
            assert.ok(
              tiles.some(
                (t) =>
                  x >= t.x * t.span - 1e-12 &&
                  x <= (t.x + 1) * t.span + 1e-12 &&
                  y >= t.y * t.span - 1e-12 &&
                  y <= (t.y + 1) * t.span + 1e-12,
              ),
              `uncovered ${x},${y},aspect ${aspect},distance ${distance}`,
            );
          }
        for (let i = 0; i < tiles.length; i++)
          for (let j = i + 1; j < tiles.length; j++) {
            const a = tiles[i],
              b = tiles[j];
            assert.ok(
              Math.min((a.x + 1) * a.span, (b.x + 1) * b.span) -
                Math.max(a.x * a.span, b.x * b.span) <
                1e-12 ||
                Math.min((a.y + 1) * a.span, (b.y + 1) * b.span) -
                  Math.max(a.y * a.span, b.y * b.span) <
                  1e-12,
              "overlapping parent and child",
            );
          }
      }
});

test("concave polygon fills keep their planar footprint while following relief across tiles", () => {
  const points = [
    [0.6, 0.1],
    [1.4, 0.1],
    [1.4, 0.4],
    [1, 0.25],
    [0.6, 0.4],
  ];
  const area = (path) =>
    Math.abs(
      path.reduce((n, p, i) => {
        const q = path[(i + 1) % path.length];
        return n + p[0] * q[1] - q[0] * p[1];
      }, 0),
    ) / 2;
  let filled = 0;
  for (const [left, right] of [
    [0.5, 1],
    [1, 1.5],
  ]) {
    const m = {
      columns: 3,
      rows: 3,
      bounds: [(left - 1) * Math.PI, 0, (right - 1) * Math.PI, Math.PI / 2],
      bed: [0, 100, 0, 200, 800, 200, 0, 100, 0],
      surface: Array(9).fill(0),
      wet: Array(9).fill(false),
    };
    const vertices = drapePolygon(m, points);
    for (let i = 0; i < vertices.length; i += 3) {
      const tri = vertices.slice(i, i + 3);
      filled += area(tri);
      const mid = [
        tri.reduce((n, p) => n + p[0], 0) / 3,
        tri.reduce((n, p) => n + p[1], 0) / 3,
      ];
      assert.ok(
        Math.abs(tri.reduce((n, p) => n + p[2], 0) / 3 - meshHeight(m, mid)) <
          1e-8,
        "fill cuts through terrain",
      );
    }
  }
  assert.ok(
    Math.abs(filled - area(points)) < 1e-12,
    "fill changed the stored country outline",
  );
});

test("draping preserves geographic footprint and every terrain triangle, including shared tile boundaries", () => {
  const mesh = (left, right) => ({
    columns: 3,
    rows: 3,
    bounds: [(left - 1) * Math.PI, 0, (right - 1) * Math.PI, Math.PI / 2],
    bed: Array.from({ length: 9 }, (_, i) => {
      const x = left + ((i % 3) / 2) * (right - left),
        y = Math.floor(i / 3) / 4;
      return 100 + 800 * Math.max(0, 1 - Math.abs(x - 1) * 2) + y * 100;
    }),
    surface: Array(9).fill(0),
    wet: Array(9).fill(false),
  });
  const left = mesh(0.5, 1),
    right = mesh(1, 1.5),
    a = [0.6, 0.1],
    b = [1.4, 0.4];
  const l = drapeSegment(left, a, b),
    r = drapeSegment(right, a, b);
  assert.deepEqual(l[0].slice(0, 2), a);
  assert.deepEqual(r.at(-1).slice(0, 2), b);
  assert.deepEqual(l.at(-1), r[0]);
  assert.ok(l.length > 2 && r.length > 2);
  for (const [m, path] of [
    [left, l],
    [right, r],
  ])
    for (let i = 1; i < path.length; i++) {
      const p = path[i - 1],
        q = path[i],
        mid = [(p[0] + q[0]) / 2, (p[1] + q[1]) / 2];
      assert.ok(
        Math.abs((p[2] + q[2]) / 2 - meshHeight(m, mid)) < 1e-8,
        "line cuts through a ridge",
      );
      assert.ok(
        Math.abs(
          (p[1] - a[1]) * (b[0] - a[0]) - (p[0] - a[0]) * (b[1] - a[1]),
        ) < 1e-12,
        "footprint moved sideways",
      );
    }
  assert.equal(clipSegment([0, 0], [0, 1], { x: 1, y: 0, w: 1, h: 1 }), null);
});
