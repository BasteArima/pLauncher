// Собирает ui/: свежий frontend/dist + арт + мок Wails с витриной и сценариями съёмки (?shot=...).
const fs = require('fs');
const path = require('path');
const games = require('./games.cjs');
const DIST = path.resolve(__dirname, '../../../game-launcher/frontend/dist');
const UI = path.join(__dirname, 'out', 'ui');

fs.mkdirSync(UI, { recursive: true });
for (const f of fs.readdirSync(UI)) fs.rmSync(path.join(UI, f), { recursive: true, force: true }); // саму папку держит http.server
fs.cpSync(DIST, UI, { recursive: true });
fs.cpSync(path.join(__dirname, 'out', 'media'), path.join(UI, 'media'), { recursive: true });

const now = Math.floor(Date.now() / 1000);
const DAY = 86400;
const srcName = { F95Zone: 'F95zone', Pornolab: 'Pornolab', Steam: 'Steam' };
const srcUrl = { F95zone: 'https://f95zone.to/threads/', Pornolab: 'https://pornolab.net/forum/viewtopic.php?t=', Steam: 'https://store.steampowered.com/app/' };
const data = games.map((g, i) => {
  const sources = g.src.map(([s, v]) => ({ source: srcName[s], url: srcUrl[srcName[s]] + (1000 + i), last_version: v }));
  return {
    id: g.id, title: g.title.replace('|', ' ').replace('Room mates', 'Roommates'), description: g.desc, version: g.version,
    author: g.author, engine: g.engine, languages: g.langs, tags: g.tags,
    cover_path: `covers/${g.id}.svg`, cover_fit: '', cover_pos: '50% 50%',
    images: g.lines.map((_, k) => `covers/${g.id}-${k + 1}.svg`),
    exec_path: `D:/Games/${g.title.replace('|', ' ')}/Game.exe`, folder_path: `D:/Games/${g.title.replace('|', ' ')}`,
    favorite: !!g.fav, time_played: g.played, added_at: now - (i + 1) * DAY * 3,
    last_launched_at: g.last ? now - g.last * DAY / 2 : 0,
    sources, primary_source: sources[0].source,
    update_available: !!g.update, update_version: g.update || '', update_source: g.update ? sources[0].source : '',
    last_checked_at: now - 3600, folder_missing: false, size_bytes: Math.round(g.size), size_checked_at: now,
  };
});
const collections = [
  { id: 'c1', name: 'Weekend picks', type: 'manual', game_ids: ['g01', 'g05', 'g07', 'g11'], tags: [] },
  { id: 'c2', name: 'Completed', type: 'dynamic', game_ids: [], tags: ['Completed'] },
  { id: 'c3', name: 'Waiting for updates', type: 'manual', game_ids: ['g02', 'g04', 'g06', 'g09', 'g10'], tags: [] },
];

const mock = `(function () {
  try { localStorage.clear(); localStorage.setItem('plauncher_lang', 'en'); localStorage.setItem('plauncher_sidebar_w', '270'); } catch (e) {}
  const games = ${JSON.stringify(data)};
  let collections = ${JSON.stringify(collections)};
  const priv = { has_pin: true, locked: false, idle_lock_min: 15, panic_enabled: true, panic_mods: 3, panic_vk: 72, panic_label: 'Ctrl+Alt+H', panic_action: 'hide', lock_on_panic: true, blur_mode: 'none', start_discreet: false };
  const impl = {
    IsConfigured: () => true, GetGames: () => games, SearchGames: (q) => games.filter(g => g.title.toLowerCase().includes(q.toLowerCase())),
    GetScanPaths: () => ['D:/Games'], GetCollections: () => collections, SaveCollections: (c) => { collections = c; },
    GetSupportedSources: () => [{ name: 'F95zone', domain: 'f95zone.to' }, { name: 'Pornolab', domain: 'pornolab.net' }, { name: 'Steam', domain: 'store.steampowered.com' }],
    GetCustomLocales: () => ({}), GetAppVersion: () => '1.0.0', GetDataDir: () => 'D:/pLauncher-data', GetPrivacy: () => priv, NotifyWindowShown: () => null,
    GetIgnoredPaths: () => ['D:/Games/_videos'], CheckLauncherUpdate: () => ({ available: false, current: '1.0.0', latest: '1.0.0' }),
  };
  const App = new Proxy({}, { get: (_, name) => (...args) => Promise.resolve(impl[name] ? JSON.parse(JSON.stringify(impl[name](...args) ?? null)) : null) });
  window.go = { main: { App } };
  window.runtime = new Proxy({}, { get: (_, name) => name === 'EventsOnMultiple' ? () => () => {} : () => {} });

  // Сценарии съёмки
  const shot = new URLSearchParams(location.search).get('shot') || 'home';
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const key = (k, o = {}) => window.dispatchEvent(new KeyboardEvent('keydown', Object.assign({ key: k, bubbles: true }, o)));
  const card = (id) => document.querySelector('[data-game-id="' + id + '"]');
  const st = document.createElement('style'); st.textContent = '*,*::before,*::after{animation:none!important;transition:none!important}'; document.head.appendChild(st);
  window.addEventListener('load', async () => {
    await sleep(900);
    if (shot === 'detail') { card('g01').click(); await sleep(600); }
    if (shot === 'discreet') { key('h', { ctrlKey: true, code: 'KeyH' }); await sleep(400); }
    if (shot === 'bulk') {
      [...document.querySelectorAll('button')].find(b => b.textContent.trim() === 'Select').click(); await sleep(200);
      // Выбираем в сетке «All games» (последние экземпляры карточек) и прокручиваем к ней
      const last = (id) => [...document.querySelectorAll('[data-game-id="' + id + '"]')].pop();
      for (const id of ['g02', 'g04', 'g06', 'g10']) { last(id).click(); await sleep(80); }
      const h = [...document.querySelectorAll('h2, h3, span')].find(e => e.textContent.trim().startsWith('All games'));
      if (h) h.scrollIntoView({ block: 'start' });
      await sleep(400);
    }
    if (shot === 'menu') { document.querySelector('.h-9 button').click(); await sleep(400); }
    if (shot === 'settings') {
      key(',', { ctrlKey: true }); await sleep(300);
      [...document.querySelectorAll('.fixed button')].find(b => b.textContent.includes('Privacy')).click(); await sleep(400);
    }
    document.body.dataset.ready = '1';
  });
})();`;
fs.writeFileSync(path.join(UI, 'mock.js'), mock);
const idx = path.join(UI, 'index.html');
fs.writeFileSync(idx, fs.readFileSync(idx, 'utf8').replace('<head>', '<head><script src="/mock.js"></script>'));
console.log('ui ready: ' + data.length + ' games');
