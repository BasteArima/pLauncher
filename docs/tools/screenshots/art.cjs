// Генератор SFW-арта для скриншотов README: постеры-обложки (600x900) и «скриншоты» (1280x720).
const fs = require('fs');
const path = require('path');
const OUT = path.join(__dirname, 'out', 'media', 'covers');
fs.mkdirSync(OUT, { recursive: true });

function rng(seed) { let s = seed >>> 0 || 1; return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296); }
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');

const PAL = {
  sunset: { sky: ['#1b1f4b', '#5b2a68', '#d4566a', '#f7a86a'], sun: '#ffd48a', far: '#6b3a6e', near: '#2a1636', sea: ['#3a2356', '#1a1030'] },
  night:  { sky: ['#05081a', '#10173a', '#252056'], sun: '#e9e7ff', far: '#1a2150', near: '#0a0d22', sea: ['#101634', '#05081a'] },
  day:    { sky: ['#3f8fe0', '#8cc6ff', '#e8f4ff'], sun: '#fffbe0', far: '#7aa7c9', near: '#3c6b8f', sea: ['#2f86c4', '#1b5b8f'] },
  dusk:   { sky: ['#101438', '#352a60', '#a4527a', '#eb9b77'], sun: '#ffcf9a', far: '#4a3466', near: '#1b1230', sea: ['#2b2350', '#120c24'] },
  synth:  { sky: ['#090420', '#26083f', '#6a0f5e', '#ff3d7f'], sun: '#ffcc4d', far: '#3a0d52', near: '#12031f', sea: ['#1a0530', '#07010f'] },
  space:  { sky: ['#01020a', '#070b26', '#150f3a'], sun: '#9fd3ff', far: '#1b1545', near: '#07061a', sea: ['#000', '#000'] },
  blossom:{ sky: ['#8ec5ff', '#d7ebff', '#ffe6ef'], sun: '#fff8e8', far: '#9bb7d6', near: '#5b6f8f', sea: ['#9cc9ef', '#6fa4d4'] },
  snow:   { sky: ['#17264a', '#4a6a9e', '#b9d3f2'], sun: '#fff4dc', far: '#8aa6cf', near: '#e9f1ff', sea: ['#3b5a8a', '#1f3560'] },
  tropic: { sky: ['#1f9bd6', '#71d0f0', '#fdf2cf'], sun: '#fffbe6', far: '#3fae9a', near: '#0f5d57', sea: ['#16b3c9', '#0b6f9a'] },
};

function defs(id, p, w, h) {
  const stops = p.sky.map((c, i) => `<stop offset="${(i / (p.sky.length - 1)).toFixed(2)}" stop-color="${c}"/>`).join('');
  return `<defs>
    <linearGradient id="${id}sky" x1="0" y1="0" x2="0" y2="1">${stops}</linearGradient>
    <radialGradient id="${id}glow"><stop offset="0" stop-color="${p.sun}" stop-opacity=".9"/><stop offset=".35" stop-color="${p.sun}" stop-opacity=".35"/><stop offset="1" stop-color="${p.sun}" stop-opacity="0"/></radialGradient>
    <linearGradient id="${id}sea" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${p.sea[0]}"/><stop offset="1" stop-color="${p.sea[1]}"/></linearGradient>
    <radialGradient id="${id}vig" cx=".5" cy=".45" r=".75"><stop offset=".55" stop-color="#000" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".55"/></radialGradient>
    <linearGradient id="${id}fade" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#05040c" stop-opacity=".45"/><stop offset=".25" stop-color="#05040c" stop-opacity=".5"/><stop offset=".48" stop-color="#05040c" stop-opacity="0"/><stop offset=".62" stop-color="#05040c" stop-opacity="0"/><stop offset="1" stop-color="#05040c" stop-opacity=".85"/></linearGradient>
    <filter id="${id}grain"><feTurbulence type="fractalNoise" baseFrequency=".9" numOctaves="2" stitchTiles="stitch"/><feColorMatrix values="0 0 0 0 .5  0 0 0 0 .5  0 0 0 0 .5  0 0 0 .07 0"/></filter>
    <filter id="${id}blur"><feGaussianBlur stdDeviation="${w / 160}"/></filter>
  </defs>`;
}

function ridge(r, w, h, y, amp, color, n = 9, opacity = 1) {
  let d = `M0 ${h} L0 ${y}`;
  for (let i = 1; i <= n; i++) d += ` L${(w * i / n).toFixed(0)} ${(y - r() * amp + amp / 3).toFixed(0)}`;
  return `<path d="${d} L${w} ${h} Z" fill="${color}" opacity="${opacity}"/>`;
}
function stars(r, w, h, n) {
  let s = '';
  for (let i = 0; i < n; i++) s += `<circle cx="${(r() * w).toFixed(0)}" cy="${(r() * h * 0.6).toFixed(0)}" r="${(r() * 1.4 + 0.3).toFixed(1)}" fill="#fff" opacity="${(r() * 0.7 + 0.2).toFixed(2)}"/>`;
  return s;
}
function city(r, w, h, base, color, lit, tall = 0.35) {
  let s = '', x = -10;
  while (x < w) {
    const bw = 30 + r() * 70, bh = h * (0.08 + r() * tall);
    s += `<rect x="${x.toFixed(0)}" y="${(base - bh).toFixed(0)}" width="${bw.toFixed(0)}" height="${(bh + h).toFixed(0)}" fill="${color}"/>`;
    for (let wy = base - bh + 10; wy < base - 8; wy += 14) for (let wx = x + 6; wx < x + bw - 8; wx += 12)
      if (r() < 0.28) s += `<rect x="${wx.toFixed(0)}" y="${wy.toFixed(0)}" width="5" height="7" fill="${lit[Math.floor(r() * lit.length)]}" opacity="${(0.5 + r() * 0.5).toFixed(2)}"/>`;
    x += bw + r() * 6;
  }
  return s;
}
function sea(id, r, w, h, y, sunX, sunCol) {
  let s = `<rect x="0" y="${y}" width="${w}" height="${h - y}" fill="url(#${id}sea)"/>`;
  for (let i = 0; i < 26; i++) {
    const yy = y + 6 + i * (h - y) / 26, ww = (40 + r() * 120) * (1 + i / 14);
    s += `<rect x="${(sunX - ww / 2 + (r() - 0.5) * 30).toFixed(0)}" y="${yy.toFixed(0)}" width="${ww.toFixed(0)}" height="${(2 + i / 8).toFixed(1)}" rx="2" fill="${sunCol}" opacity="${(0.55 - i / 60).toFixed(2)}"/>`;
  }
  return s;
}
function palm(x, y, s, c) {
  const leaf = (a) => `<path d="M0 0 Q ${40 * s} ${-30 * s} ${95 * s} ${8 * s} Q ${45 * s} ${-12 * s} 0 0Z" fill="${c}" transform="translate(${x + 10 * s} ${y - 170 * s}) rotate(${a})"/>`;
  return `<path d="M${x} ${y} Q ${x + 18 * s} ${y - 90 * s} ${x + 10 * s} ${y - 170 * s}" stroke="${c}" stroke-width="${9 * s}" fill="none" stroke-linecap="round"/>` +
    [-160, -120, -60, -20, 20, 200].map(leaf).join('');
}
function pines(r, w, y, c, n, sz) {
  let s = '';
  for (let i = 0; i < n; i++) {
    const x = r() * w, t = sz * (0.6 + r() * 0.8);
    s += `<path d="M${x.toFixed(0)} ${(y - t).toFixed(0)} L${(x + t * 0.32).toFixed(0)} ${y} L${(x - t * 0.32).toFixed(0)} ${y}Z" fill="${c}"/>`;
  }
  return s;
}
function castle(x, y, s, c, lit) {
  const t = (tx, tw, th) => `<rect x="${x + tx * s}" y="${y - th * s}" width="${tw * s}" height="${th * s}" fill="${c}"/><path d="M${x + (tx - 4) * s} ${y - th * s} L${x + (tx + tw / 2) * s} ${y - (th + 40) * s} L${x + (tx + tw + 4) * s} ${y - th * s}Z" fill="${c}"/>`;
  let win = '';
  for (const [wx, wy] of [[30, 120], [95, 170], [160, 110], [98, 90]]) win += `<rect x="${x + wx * s}" y="${y - wy * s}" width="${6 * s}" height="${11 * s}" rx="${3 * s}" fill="${lit}"/>`;
  return `<rect x="${x}" y="${y - 90 * s}" width="${200 * s}" height="${90 * s}" fill="${c}"/>` + t(15, 40, 150) + t(80, 44, 210) + t(150, 36, 140) + win;
}
function cabin(x, y, s, c, lit) {
  return `<path d="M${x} ${y} L${x} ${y - 50 * s} L${x + 55 * s} ${y - 95 * s} L${x + 110 * s} ${y - 50 * s} L${x + 110 * s} ${y}Z" fill="${c}"/>
    <rect x="${x + 20 * s}" y="${y - 40 * s}" width="${22 * s}" height="${18 * s}" fill="${lit}"/><rect x="${x + 66 * s}" y="${y - 40 * s}" width="${22 * s}" height="${18 * s}" fill="${lit}" opacity=".8"/>
    <rect x="${x + 20 * s}" y="${y + 6 * s}" width="${22 * s}" height="${40 * s}" fill="${lit}" opacity=".25"/><rect x="${x + 66 * s}" y="${y + 6 * s}" width="${22 * s}" height="${40 * s}" fill="${lit}" opacity=".2"/>`;
}
function petals(r, w, h, n) {
  let s = '';
  for (let i = 0; i < n; i++) s += `<ellipse cx="${(r() * w).toFixed(0)}" cy="${(r() * h).toFixed(0)}" rx="${(3 + r() * 4).toFixed(1)}" ry="${(2 + r() * 2).toFixed(1)}" fill="#ffb7cf" opacity="${(0.5 + r() * 0.5).toFixed(2)}" transform="rotate(${(r() * 180).toFixed(0)} ${(r() * w).toFixed(0)} ${(r() * h).toFixed(0)})"/>`;
  return s;
}
function blossomTree(x, y, s, r) {
  let s2 = `<path d="M${x} ${y} C ${x - 10 * s} ${y - 80 * s} ${x + 30 * s} ${y - 120 * s} ${x + 10 * s} ${y - 190 * s}" stroke="#5a3b3f" stroke-width="${14 * s}" fill="none" stroke-linecap="round"/>`;
  for (let i = 0; i < 26; i++) s2 += `<circle cx="${(x + (r() - 0.5) * 260 * s).toFixed(0)}" cy="${(y - 170 * s + (r() - 0.5) * 140 * s).toFixed(0)}" r="${(28 + r() * 34) * s}" fill="${['#ffc4d8', '#ffb0c9', '#ffd6e4'][i % 3]}" opacity=".92"/>`;
  return s2;
}
function rain(r, w, h, n) {
  let s = '';
  for (let i = 0; i < n; i++) { const x = r() * w, y = r() * h, l = 14 + r() * 26; s += `<line x1="${x.toFixed(0)}" y1="${y.toFixed(0)}" x2="${(x - 4).toFixed(0)}" y2="${(y + l).toFixed(0)}" stroke="#cfe0ff" stroke-width="1.2" opacity="${(0.15 + r() * 0.35).toFixed(2)}"/>`; }
  return s;
}
function windowFrame(w, h, c) {
  const t = Math.round(w * 0.035);
  return `<rect x="0" y="0" width="${w}" height="${t}" fill="${c}"/><rect x="0" y="${h - t * 2.2}" width="${w}" height="${t * 2.2}" fill="${c}"/>
    <rect x="0" y="0" width="${t}" height="${h}" fill="${c}"/><rect x="${w - t}" y="0" width="${t}" height="${h}" fill="${c}"/>
    <rect x="${w / 2 - t / 2}" y="0" width="${t}" height="${h}" fill="${c}"/><rect x="0" y="${h * 0.45}" width="${w}" height="${t * 0.7}" fill="${c}"/>`;
}
function synthGrid(w, h, y) {
  let s = `<rect x="0" y="${y}" width="${w}" height="${h - y}" fill="#12031f"/>`;
  for (let i = 1; i < 16; i++) { const yy = y + Math.pow(i / 16, 2) * (h - y); s += `<line x1="0" y1="${yy.toFixed(0)}" x2="${w}" y2="${yy.toFixed(0)}" stroke="#ff3dbb" stroke-width="${(1 + i / 8).toFixed(1)}" opacity=".7"/>`; }
  for (let i = -12; i <= 12; i++) s += `<line x1="${w / 2 + i * 12}" y1="${y}" x2="${w / 2 + i * w / 7}" y2="${h}" stroke="#ff3dbb" stroke-width="1.5" opacity=".6"/>`;
  return s;
}
function synthSun(id, x, y, rad) {
  let bars = '';
  for (let i = 0; i < 6; i++) bars += `<rect x="${x - rad}" y="${y + rad * (0.1 + i * 0.16)}" width="${rad * 2}" height="${rad * (0.03 + i * 0.018)}" fill="#26083f"/>`;
  return `<defs><linearGradient id="${id}ss" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ffe066"/><stop offset="1" stop-color="#ff3d7f"/></linearGradient><clipPath id="${id}sc"><circle cx="${x}" cy="${y}" r="${rad}"/></clipPath></defs>
    <circle cx="${x}" cy="${y}" r="${rad * 1.8}" fill="url(#${id}glow)"/><g clip-path="url(#${id}sc)"><circle cx="${x}" cy="${y}" r="${rad}" fill="url(#${id}ss)"/>${bars}</g>`;
}

// Сцена: kind → слои. Возвращает внутренность SVG (без заголовков).
function scene(kind, w, h, seed, id) {
  const r = rng(seed), p = PAL[kind === 'rainy' ? 'night' : kind === 'lake' ? 'dusk' : kind === 'castle' ? 'night' : kind === 'office' ? 'day' : kind === 'resort' ? 'tropic' : kind === 'neon' ? 'synth' : kind === 'mountain' ? 'snow' : kind === 'citydusk' ? 'dusk' : kind];
  let s = defs(id, p, w, h) + `<rect width="${w}" height="${h}" fill="url(#${id}sky)"/>`;
  const sunX = w * (0.3 + r() * 0.4);
  switch (kind) {
    case 'sunset':
      s += `<circle cx="${sunX}" cy="${h * 0.55}" r="${w * 0.4}" fill="url(#${id}glow)"/><circle cx="${sunX}" cy="${h * 0.56}" r="${w * 0.09}" fill="${p.sun}"/>`;
      s += ridge(r, w, h, h * 0.6, h * 0.05, p.far, 7, 0.9) + sea(id, r, w, h, h * 0.6, sunX, p.sun);
      s += palm(w * 0.12, h * 0.98, w / 700, p.near) + palm(w * 0.86, h * 1.02, w / 560, p.near);
      break;
    case 'night':
      s += stars(r, w, h, 160) + `<circle cx="${w * 0.78}" cy="${h * 0.18}" r="${w * 0.25}" fill="url(#${id}glow)" opacity=".6"/><circle cx="${w * 0.78}" cy="${h * 0.18}" r="${w * 0.05}" fill="${p.sun}"/>`;
      s += city(r, w, h, h * 0.78, '#141a3d', ['#ffd27a', '#8fb8ff'], 0.25) + city(r, w, h, h * 0.9, '#0a0d22', ['#ffcf6a', '#ff9ad5', '#9fd0ff'], 0.4);
      break;
    case 'rainy':
      s += city(r, w, h, h * 0.8, '#1a2150', ['#ffd27a', '#7fb2ff'], 0.35) + `<rect width="${w}" height="${h}" fill="#0b1030" opacity=".35" filter="url(#${id}blur)"/>` + rain(r, w, h, 260) + windowFrame(w, h, '#0c0a14');
      break;
    case 'office':
      s += `<circle cx="${sunX}" cy="${h * 0.15}" r="${w * 0.3}" fill="url(#${id}glow)"/>` + city(r, w, h, h * 0.95, '#5f86a8', ['#e8f4ff', '#bcd9f0'], 0.55) + city(r, w, h, h * 1.02, '#3c6b8f', ['#e8f4ff'], 0.7);
      break;
    case 'neon':
      s += stars(r, w, h, 60) + synthSun(id, w / 2, h * 0.47, w * 0.2) + ridge(r, w, h, h * 0.62, h * 0.08, '#3a0d52', 12) + synthGrid(w, h, h * 0.62);
      break;
    case 'blossom':
      s += `<circle cx="${sunX}" cy="${h * 0.2}" r="${w * 0.35}" fill="url(#${id}glow)"/>` + ridge(r, w, h, h * 0.7, h * 0.08, p.far, 6) +
        `<rect x="${w * 0.55}" y="${h * 0.48}" width="${w * 0.4}" height="${h * 0.25}" fill="#e9eef7"/><rect x="${w * 0.53}" y="${h * 0.45}" width="${w * 0.44}" height="${h * 0.04}" fill="#8a5f5f"/>` +
        ridge(r, w, h, h * 0.8, h * 0.03, '#8fbf8f', 5) + blossomTree(w * 0.22, h * 0.92, w / 600, r) + petals(r, w, h, 70);
      break;
    case 'space':
      s += stars(r, w, h / 0.6, 260) + `<circle cx="${w * 0.3}" cy="${h * 0.35}" r="${w * 0.45}" fill="url(#${id}glow)" opacity=".35"/>` +
        `<circle cx="${w * 0.72}" cy="${h * 0.62}" r="${w * 0.28}" fill="#2a3f8a"/><circle cx="${w * 0.66}" cy="${h * 0.56}" r="${w * 0.28}" fill="#3d5fc2" opacity=".55"/>` +
        `<ellipse cx="${w * 0.72}" cy="${h * 0.62}" rx="${w * 0.46}" ry="${w * 0.07}" fill="none" stroke="#9fd3ff" stroke-width="${w / 120}" opacity=".55" transform="rotate(-14 ${w * 0.72} ${h * 0.62})"/>` +
        `<path d="M${w * 0.1} ${h * 0.3} l${w * 0.22} ${-h * 0.04} l${w * 0.04} ${h * 0.02} l${-w * 0.22} ${h * 0.04}z" fill="#c9d6ff"/><rect x="${w * 0.2}" y="${h * 0.265}" width="${w * 0.05}" height="${h * 0.02}" fill="#9fd3ff"/>`;
      break;
    case 'lake':
      s += stars(r, w, h, 50) + `<circle cx="${sunX}" cy="${h * 0.5}" r="${w * 0.35}" fill="url(#${id}glow)"/><circle cx="${sunX}" cy="${h * 0.52}" r="${w * 0.06}" fill="${p.sun}"/>` +
        ridge(r, w, h, h * 0.55, h * 0.12, p.far, 8) + pines(r, w, h * 0.6, '#1b1230', 40, h * 0.12) + sea(id, r, w, h, h * 0.6, sunX, p.sun) + cabin(w * 0.14, h * 0.6, w / 500, '#140d24', '#ffcf7a');
      break;
    case 'castle':
      s += stars(r, w, h, 140) + `<circle cx="${w * 0.25}" cy="${h * 0.2}" r="${w * 0.3}" fill="url(#${id}glow)" opacity=".7"/><circle cx="${w * 0.25}" cy="${h * 0.2}" r="${w * 0.06}" fill="${p.sun}"/>` +
        ridge(r, w, h, h * 0.72, h * 0.18, '#141a3d', 7) + castle(w * 0.45, h * 0.72, w / 480, '#0a0d22', '#ffcf6a') + pines(r, w, h * 0.95, '#05081a', 50, h * 0.2);
      break;
    case 'resort':
      s += `<circle cx="${sunX}" cy="${h * 0.14}" r="${w * 0.3}" fill="url(#${id}glow)"/><circle cx="${sunX}" cy="${h * 0.14}" r="${w * 0.05}" fill="${p.sun}"/>` +
        ridge(r, w, h, h * 0.58, h * 0.1, '#3fae9a', 6) + sea(id, r, w, h, h * 0.6, sunX, '#ffffff') +
        `<path d="M0 ${h * 0.86} Q ${w * 0.5} ${h * 0.78} ${w} ${h * 0.88} L${w} ${h} L0 ${h}Z" fill="#f4dfae"/>` + palm(w * 0.8, h * 0.95, w / 520, '#0f5d57') + palm(w * 0.1, h * 0.98, w / 650, '#0f5d57');
      break;
    case 'citydusk':
      s += stars(r, w, h, 40) + `<circle cx="${sunX}" cy="${h * 0.62}" r="${w * 0.4}" fill="url(#${id}glow)"/><circle cx="${sunX}" cy="${h * 0.64}" r="${w * 0.08}" fill="${p.sun}"/>` +
        city(r, w, h, h * 0.8, '#3b2a5a', ['#ffcf8a', '#ffe2b0'], 0.3) + city(r, w, h, h * 0.92, '#1b1230', ['#ffcf6a', '#ffb0d0'], 0.45);
      break;
    case 'mountain':
      s += stars(r, w, h, 40) + ridge(r, w, h, h * 0.5, h * 0.25, '#8aa6cf', 5) + ridge(r, w, h, h * 0.66, h * 0.2, '#c9dcf5', 6) +
        ridge(r, w, h, h * 0.8, h * 0.08, '#e9f1ff', 8) + pines(r, w, h * 0.95, '#1d2b4f', 30, h * 0.18) + cabin(w * 0.6, h * 0.86, w / 600, '#2a2238', '#ffcf7a');
      break;
  }
  return s;
}

function cover(g) {
  const w = 600, h = 900, id = 'c' + g.id;
  const font = g.font || "'Segoe UI Black', 'Arial Black', sans-serif";
  const size = g.titleSize || 74;
  const lines = g.title.toUpperCase().split('|');
  const titleY = 250 + size * 0.8 - (lines.length - 1) * size * 0.45;
  const title = lines.map((l, i) => `<text x="${w / 2}" y="${titleY + i * size * 0.95}" text-anchor="middle" font-family="${font}" font-size="${size}" letter-spacing="${g.spacing ?? 4}" fill="#fff" style="paint-order:stroke" stroke="#000" stroke-opacity=".25" stroke-width="3">${esc(l)}</text>`).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" preserveAspectRatio="xMidYMid slice">${scene(g.scene, w, h, g.seed, id)}
    <rect width="${w}" height="${h}" fill="url(#${id}fade)"/>
    <text x="${w / 2}" y="70" text-anchor="middle" font-family="Segoe UI, sans-serif" font-size="20" letter-spacing="8" fill="#fff" opacity=".8">${esc(g.studio.toUpperCase())}</text>
    ${title}
    <text x="${w / 2}" y="${titleY + lines.length * size * 0.95 - size * 0.25}" text-anchor="middle" font-family="Segoe UI, sans-serif" font-size="24" letter-spacing="6" fill="${g.accent || '#ffd48a'}">${esc(g.tagline.toUpperCase())}</text>
    <rect width="${w}" height="${h}" fill="url(#${id}vig)"/><rect width="${w}" height="${h}" filter="url(#${id}grain)"/></svg>`;
}

// Кадр «из игры»: фон + окно диалога в духе Ren'Py с быстрым меню.
function shot(g, i, line) {
  const w = 1280, h = 720, id = `s${g.id}${i}`;
  const [who, text] = line;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" preserveAspectRatio="xMidYMid slice">${scene(g.scene, w, h, g.seed * 7 + i * 13, id)}
    <rect width="${w}" height="${h}" fill="url(#${id}vig)"/>
    <rect x="0" y="${h - 190}" width="${w}" height="190" fill="#07060f" opacity=".72"/>
    <rect x="150" y="${h - 214}" width="${Math.max(140, who.length * 16 + 50)}" height="44" rx="6" fill="${g.accent || '#ffd48a'}" opacity=".95"/>
    <text x="175" y="${h - 184}" font-family="Segoe UI Semibold, Segoe UI, sans-serif" font-size="24" fill="#1a1024">${esc(who)}</text>
    <text x="175" y="${h - 125}" font-family="Segoe UI, sans-serif" font-size="27" fill="#f4f1ff">${esc(text)}</text>
    <text x="${w / 2}" y="${h - 18}" text-anchor="middle" font-family="Segoe UI, sans-serif" font-size="15" letter-spacing="1" fill="#fff" opacity=".6">Back    History    Skip    Auto    Save    Q.Save    Q.Load    Prefs</text>
    <rect width="${w}" height="${h}" filter="url(#${id}grain)"/></svg>`;
}

module.exports = { cover, shot };

if (require.main === module) {
  const games = require('./games.cjs');
  for (const g of games) {
    fs.writeFileSync(path.join(OUT, `${g.id}.svg`), cover(g));
    g.lines.forEach((l, i) => fs.writeFileSync(path.join(OUT, `${g.id}-${i + 1}.svg`), shot(g, i + 1, l)));
  }
  console.log('art: ' + fs.readdirSync(OUT).length + ' files');
}
