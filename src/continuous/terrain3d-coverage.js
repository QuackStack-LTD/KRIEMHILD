import * as THREE from "three";

// Refine a complete frustum cover, never a truncated list of nearest tiles.
// The budget limits refinement: an unrefined parent still covers its children.
export function meshCoverage({
  view,
  point,
  width,
  height,
  budget = 96,
  maxLevel = 18,
}) {
  view.updateMatrixWorld();
  const frustum = new THREE.Frustum().setFromProjectionMatrix(
    new THREE.Matrix4().multiplyMatrices(
      view.projectionMatrix,
      view.matrixWorldInverse,
    ),
  );
  const eye = view.position;
  function node(z, x, y) {
    const span = 2 ** -z,
      box = new THREE.Box3();
    for (const u of [0, 0.25, 0.5, 0.75, 1])
      for (const v of [0, 0.25, 0.5, 0.75, 1])
        for (const h of [-12000, 16000])
          box.expandByPoint(
            new THREE.Vector3(...point((x + u) * span, (y + v) * span, h)),
          );
    if (!frustum.intersectsBox(box)) return null;
    const center = new THREE.Vector3(
      ...point((x + 0.5) * span, (y + 0.5) * span, 0),
    );
    const edge =
      new THREE.Vector3(
        ...point((x + 1) * span, (y + 0.5) * span, 0),
      ).distanceTo(center) * 2;
    // Tall conservative bounds are for culling, not a claim that a 16km peak
    // stands beside the camera. Refine projected ground coverage first.
    const distance = Math.max(view.near, center.distanceTo(eye) - edge * 0.71);
    const pixels =
      (edge * height) / (2 * Math.tan((view.fov * Math.PI) / 360) * distance);
    return {
      z,
      x,
      y,
      span,
      size: 256,
      pixels,
      distance: center.distanceTo(eye),
    };
  }
  const leaves = [node(0, 0, 0), node(0, 1, 0)].filter(Boolean);
  const blocked = new Set();
  for (;;) {
    const candidate = leaves
      .filter((n) => n.z < maxLevel && n.pixels > 192 && !blocked.has(n))
      .sort((a, b) => b.pixels - a.pixels)[0];
    if (!candidate) break;
    const children = [];
    for (let y = 0; y < 2; y++)
      for (let x = 0; x < 2; x++) {
        const child = node(
          candidate.z + 1,
          candidate.x * 2 + x,
          candidate.y * 2 + y,
        );
        if (child) children.push(child);
      }
    if (!children.length || leaves.length + children.length - 1 > budget) {
      blocked.add(candidate);
      continue;
    }
    leaves.splice(leaves.indexOf(candidate), 1, ...children);
  }
  return leaves.sort((a, b) => a.distance - b.distance);
}

export function meshView(view, target, width, height) {
  return {
    position: view.position.clone(),
    rotation: view.quaternion.clone(),
    target: target.clone(),
    width,
    height,
  };
}
export function sameMeshView(a, b) {
  if (!a || !b || a.width !== b.width || a.height !== b.height) return false;
  const epsilon = Math.max(1e-11, a.position.distanceTo(a.target) * 1e-7);
  return (
    a.position.distanceToSquared(b.position) < epsilon ** 2 &&
    a.target.distanceToSquared(b.target) < epsilon ** 2 &&
    a.rotation.angleTo(b.rotation) < 1e-7
  );
}
