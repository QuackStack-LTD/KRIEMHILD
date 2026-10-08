// SVG export: the map as true Voronoi polygons, merged into one outline per terrain region.
// main.js loads this module on demand the first time "Save map as SVG" is used.
import Delaunator from 'delaunator';

const GHOST = 3; // rings of extra points around the map, so edge cells get closed polygons

// map: { W, H, cellPx, ptX, ptY (cell points in cell units), typeAt(cell) -> type index,
//        types: [{ id, name, color }], wrapX }
// Returns the SVG document as a string.
export function mapToSvg({ W, H, cellPx, ptX, ptY, typeAt, types, wrapX }) {
  // Real cells plus ghost cells around them. A ghost copies the type and jitter of the nearest
  // real cell (or, on a sphere, the cell across the seam, so the left and right edges match).
  // The outermost ring has no type and is never drawn; it only closes everything inside it.
  const GW = W + 2 * GHOST;
  const GH = H + 2 * GHOST;
  const coords = new Float64Array(GW * GH * 2);
  const type = new Int16Array(GW * GH);
  for (let gy = 0; gy < GH; gy++) {
    for (let gx = 0; gx < GW; gx++) {
      const i = gy * GW + gx;
      const x = gx - GHOST;
      const y = gy - GHOST;
      const sy = Math.min(H - 1, Math.max(0, y));
      const sx = x >= 0 && x < W ? x : wrapX ? ((x % W) + W) % W : Math.min(W - 1, Math.max(0, x));
      const c = sy * W + sx;
      coords[i * 2] = x + (ptX[c] - sx);
      coords[i * 2 + 1] = y + (ptY[c] - sy);
      const outer = gx === 0 || gy === 0 || gx === GW - 1 || gy === GH - 1;
      type[i] = outer ? -1 : typeAt(c);
    }
  }

  const { triangles, halfedges } = new Delaunator(coords);
  const centers = circumcenters(coords, triangles);

  // Every Voronoi edge between two cells of different types is part of a region outline. The
  // edge between points p and q runs between the circumcentres of the two triangles that share
  // the Delaunay edge p→q; walking it from the opposite triangle to this one keeps p's cell on
  // the same side every time, so outlines (and holes) come out consistently wound.
  const byType = types.map(() => ({ from: [], to: [] }));
  for (let e = 0; e < triangles.length; e++) {
    const o = halfedges[e];
    if (o < 0) continue;
    const p = triangles[e];
    const q = triangles[e % 3 === 2 ? e - 2 : e + 1];
    const tp = type[p];
    if (tp < 0 || tp === type[q]) continue;
    byType[tp].from.push((o / 3) | 0);
    byType[tp].to.push((e / 3) | 0);
  }

  const fmt = (v) => (Math.round(v * cellPx * 100) / 100).toString();
  const paths = [];
  types.forEach((t, ti) => {
    const loops = chainLoops(byType[ti]);
    if (!loops.length) return;
    let d = '';
    for (const loop of loops) {
      let prev = '';
      loop.forEach((v, k) => {
        const pt = `${fmt(centers[v * 2])} ${fmt(centers[v * 2 + 1])}`;
        if (pt === prev) return; // zero-length edge
        d += (k === 0 ? 'M' : 'L') + pt;
        prev = pt;
      });
      d += 'Z';
    }
    // A hairline stroke in the same colour hides anti-aliasing seams between neighbouring regions.
    paths.push(
      `  <path id="${escapeAttr(t.id)}" fill="${t.color}" stroke="${t.color}" stroke-width="0.5" stroke-linejoin="round" d="${d}">` +
        `<title>${escapeText(t.name)}</title></path>`
    );
  });

  const w = W * cellPx;
  const h = H * cellPx;
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">\n` +
    `  <title>Terrain map</title>\n` +
    `  <defs><clipPath id="map-bounds"><rect width="${w}" height="${h}"/></clipPath></defs>\n` +
    `  <g clip-path="url(#map-bounds)" fill-rule="nonzero">\n` +
    paths.map((p) => '  ' + p).join('\n') +
    `\n  </g>\n</svg>\n`
  );
}

function circumcenters(coords, triangles) {
  const out = new Float64Array((triangles.length / 3) * 2);
  for (let t = 0; t < triangles.length / 3; t++) {
    const a = triangles[t * 3] * 2;
    const b = triangles[t * 3 + 1] * 2;
    const c = triangles[t * 3 + 2] * 2;
    const ax = coords[a], ay = coords[a + 1];
    const bx = coords[b] - ax, by = coords[b + 1] - ay;
    const cx = coords[c] - ax, cy = coords[c + 1] - ay;
    const d = 2 * (bx * cy - by * cx);
    const bl = bx * bx + by * by;
    const cl = cx * cx + cy * cy;
    out[t * 2] = ax + (cy * bl - by * cl) / d;
    out[t * 2 + 1] = ay + (bx * cl - cx * bl) / d;
  }
  return out;
}

// Joins directed edges (from[i] → to[i]) into closed loops of vertex ids.
function chainLoops({ from, to }) {
  const outgoing = new Map();
  from.forEach((v, i) => {
    if (!outgoing.has(v)) outgoing.set(v, []);
    outgoing.get(v).push(i);
  });
  const used = new Uint8Array(from.length);
  const loops = [];
  for (let i = 0; i < from.length; i++) {
    if (used[i]) continue;
    const loop = [from[i]];
    let cur = i;
    for (;;) {
      used[cur] = 1;
      const v = to[cur];
      if (v === from[i]) break;
      loop.push(v);
      const next = (outgoing.get(v) || []).find((j) => !used[j]);
      if (next === undefined) break; // can't happen for a closed outline; stop rather than loop
      cur = next;
    }
    loops.push(loop);
  }
  return loops;
}

function escapeText(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escapeAttr(s) {
  return escapeText(s).replace(/"/g, '&quot;');
}
