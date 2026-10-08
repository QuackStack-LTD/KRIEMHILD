// Texture patterns: small icons that repeat across every cell of a terrain type so types are easy
// to tell apart. Each pattern paints one S×S tile; main.js repeats the tile over the map.
(function (global) {
  'use strict';

  // [id, label] in the order shown in the type editor.
  const LIST = [
    ['', 'None'],
    ['waves', 'Waves'],
    ['ripples', 'Ripples'],
    ['coral', 'Coral'],
    ['stipple', 'Dots'],
    ['tufts', 'Grass tufts'],
    ['flowers', 'Flowers'],
    ['hills', 'Hills'],
    ['rows', 'Crop rows'],
    ['houses', 'Houses'],
    ['trees', 'Trees'],
    ['pines', 'Pines'],
    ['palms', 'Palms'],
    ['peaks', 'Peaks'],
    ['sparkle', 'Sparkles'],
    ['dunes', 'Dunes'],
    ['strata', 'Rock layers'],
    ['dashes', 'Dashes'],
    ['cracks', 'Cracks'],
    ['reeds', 'Reeds'],
  ];

  // One icon centred on (x, y) inside a box of size s. Icons are drawn twice per tile, staggered,
  // so the repeat looks less like a grid.
  const ICONS = {
    waves(c, x, y, s) {
      c.beginPath();
      c.moveTo(x - s / 2, y);
      c.quadraticCurveTo(x - s / 4, y - s / 3, x, y);
      c.quadraticCurveTo(x + s / 4, y + s / 3, x + s / 2, y);
      c.stroke();
    },
    ripples(c, x, y, s) {
      for (const dy of [-s / 5, s / 5]) {
        c.beginPath();
        c.moveTo(x - s / 2, y + dy);
        c.quadraticCurveTo(x - s / 4, y + dy - s / 5, x, y + dy);
        c.quadraticCurveTo(x + s / 4, y + dy + s / 5, x + s / 2, y + dy);
        c.stroke();
      }
    },
    coral(c, x, y, s) {
      c.beginPath();
      c.arc(x, y, s / 5, 0, Math.PI * 2);
      c.moveTo(x + s / 2.5 + s / 9, y - s / 4);
      c.arc(x + s / 2.5, y - s / 4, s / 9, 0, Math.PI * 2);
      c.stroke();
      dot(c, x - s / 3, y + s / 4, s);
    },
    stipple(c, x, y, s) {
      dot(c, x - s / 3, y - s / 4, s);
      dot(c, x + s / 4, y - s / 3, s);
      dot(c, x, y + s / 4, s);
    },
    tufts(c, x, y, s) {
      const by = y + s / 4;
      c.beginPath();
      c.moveTo(x - s / 4, y - s / 5);
      c.lineTo(x - s / 12, by);
      c.lineTo(x, y - s / 3);
      c.lineTo(x + s / 12, by);
      c.lineTo(x + s / 4, y - s / 5);
      c.stroke();
    },
    flowers(c, x, y, s) {
      for (const [dx, dy] of [[-s / 3, -s / 5], [s / 4, s / 5]]) {
        c.beginPath();
        c.moveTo(x + dx - s / 6, y + dy);
        c.lineTo(x + dx + s / 6, y + dy);
        c.moveTo(x + dx, y + dy - s / 6);
        c.lineTo(x + dx, y + dy + s / 6);
        c.stroke();
      }
    },
    hills(c, x, y, s) {
      c.beginPath();
      c.arc(x, y + s / 4, s / 2.4, Math.PI * 1.05, Math.PI * 1.95);
      c.stroke();
    },
    houses(c, x, y, s) {
      const w = s * 0.6;
      const h = s * 0.35;
      c.fillRect(x - w / 2, y - h / 4, w, h);
      c.beginPath();
      c.moveTo(x - w / 2 - s / 12, y - h / 4);
      c.lineTo(x, y - h / 4 - s * 0.3);
      c.lineTo(x + w / 2 + s / 12, y - h / 4);
      c.closePath();
      c.fill();
    },
    trees(c, x, y, s) {
      c.beginPath();
      c.moveTo(x, y + s / 2.2);
      c.lineTo(x, y);
      c.stroke();
      c.beginPath();
      c.arc(x, y - s / 8, s / 3.2, 0, Math.PI * 2);
      c.fill();
    },
    pines(c, x, y, s) {
      c.beginPath();
      c.moveTo(x, y - s / 2);
      c.lineTo(x - s / 3, y + s / 4);
      c.lineTo(x + s / 3, y + s / 4);
      c.closePath();
      c.fill();
      c.beginPath();
      c.moveTo(x, y + s / 4);
      c.lineTo(x, y + s / 2.2);
      c.stroke();
    },
    palms(c, x, y, s) {
      const tx = x - s / 10;
      const ty = y - s / 4;
      c.beginPath();
      c.moveTo(x + s / 8, y + s / 2.2);
      c.quadraticCurveTo(x + s / 10, y, tx, ty);
      c.moveTo(tx - s / 2.6, ty + s / 8);
      c.quadraticCurveTo(tx - s / 5, ty - s / 5, tx, ty);
      c.quadraticCurveTo(tx + s / 5, ty - s / 5, tx + s / 2.6, ty + s / 8);
      c.moveTo(tx, ty);
      c.lineTo(tx + s / 12, ty - s / 3.5);
      c.stroke();
    },
    peaks(c, x, y, s) {
      c.beginPath();
      c.moveTo(x - s / 2, y + s / 3);
      c.lineTo(x, y - s / 2.5);
      c.lineTo(x + s / 2, y + s / 3);
      c.stroke();
    },
    sparkle(c, x, y, s) {
      c.beginPath();
      c.moveTo(x - s / 4, y);
      c.lineTo(x + s / 4, y);
      c.moveTo(x, y - s / 4);
      c.lineTo(x, y + s / 4);
      c.moveTo(x - s / 7, y - s / 7);
      c.lineTo(x + s / 7, y + s / 7);
      c.moveTo(x + s / 7, y - s / 7);
      c.lineTo(x - s / 7, y + s / 7);
      c.stroke();
    },
    dunes(c, x, y, s) {
      for (const dx of [-s / 5, s / 4]) {
        c.beginPath();
        c.arc(x + dx, y + s / 3 + (dx > 0 ? -s / 6 : 0), s / 3, Math.PI * 1.15, Math.PI * 1.85);
        c.stroke();
      }
    },
    dashes(c, x, y, s) {
      c.beginPath();
      c.moveTo(x - s / 3, y - s / 8);
      c.lineTo(x, y - s / 8);
      c.moveTo(x, y + s / 5);
      c.lineTo(x + s / 3, y + s / 5);
      c.stroke();
    },
    cracks(c, x, y, s) {
      c.beginPath();
      c.moveTo(x - s / 2, y - s / 6);
      c.lineTo(x - s / 6, y + s / 6);
      c.lineTo(x + s / 6, y - s / 8);
      c.lineTo(x + s / 2, y + s / 6);
      c.moveTo(x + s / 6, y - s / 8);
      c.lineTo(x + s / 5, y - s / 2.4);
      c.stroke();
    },
    reeds(c, x, y, s) {
      for (const dx of [-s / 4, 0, s / 4]) {
        const top = y - s / 4 - (dx === 0 ? s / 8 : 0);
        c.beginPath();
        c.moveTo(x + dx, y + s / 3);
        c.lineTo(x + dx, top);
        c.stroke();
        dot(c, x + dx, top - s / 12, s);
      }
    },
  };

  // Patterns made of lines across the whole tile; they line up seamlessly with the next tile.
  const FILLS = {
    rows(c, S) {
      c.beginPath();
      for (let k = -S; k <= S; k += S / 4) {
        c.moveTo(k, 0);
        c.lineTo(k + S, S);
      }
      c.stroke();
    },
    strata(c, S) {
      c.beginPath();
      for (let y = S / 8; y < S; y += S / 3) {
        c.moveTo(0, y);
        c.lineTo(S, y);
      }
      c.stroke();
    },
  };

  function dot(c, x, y, s) {
    const r = Math.max(0.6, s / 14);
    c.beginPath();
    c.arc(x, y, r, 0, Math.PI * 2);
    c.fill();
  }

  // A darker shade on light colours, a lighter one on dark colours.
  function iconColor([r, g, b]) {
    const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
    const mix = lum > 0.5 ? [0, 0, 0, 0.38] : [255, 255, 255, 0.4];
    const m = (v, i) => Math.round(v + (mix[i] - v) * mix[3]);
    return `rgb(${m(r, 0)}, ${m(g, 1)}, ${m(b, 2)})`;
  }

  // Paints one S×S tile: the base colour plus the pattern. Returns false for "no pattern".
  function paintTile(ctx, id, S, rgb) {
    if (!ICONS[id] && !FILLS[id]) return false;
    ctx.fillStyle = `rgb(${rgb.join(',')})`;
    ctx.fillRect(0, 0, S, S);
    const ink = iconColor(rgb);
    ctx.strokeStyle = ink;
    ctx.fillStyle = ink;
    ctx.lineWidth = Math.max(1, S / 16);
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    if (FILLS[id]) {
      FILLS[id](ctx, S);
    } else {
      const s = S * 0.42;
      ICONS[id](ctx, S * 0.25, S * 0.27, s);
      ICONS[id](ctx, S * 0.75, S * 0.77, s);
    }
    return true;
  }

  global.TerrainPatterns = { LIST, paintTile };
})(window);
