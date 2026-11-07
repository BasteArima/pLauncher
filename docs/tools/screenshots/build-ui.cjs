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
    update_found_at: g.update ? now - 3 * DAY : 0,
    last_checked_at: now - 3600, folder_missing: false, size_bytes: Math.round(g.size), size_checked_at: now,
    // Статусы и оценки — чтобы на скриншотах были видны чипсы статусов и звёзды
    status: ['playing', 'playing', 'finished', 'want', '', 'playing', 'finished', 'want', '', 'dropped', 'finished', ''][i % 12],
    rating: [5, 4, 5, 0, 0, 4, 3, 0, 0, 2, 4, 0][i % 12], notes: i === 0 ? 'Save before the fireworks night. ||The choice on day 23 locks the lake path.||' : '', notes_updated_at: i === 0 ? now - 2 * DAY : 0,
  };
});
const collections = [
  { id: 'c1', name: 'Weekend picks', type: 'manual', game_ids: ['g01', 'g05', 'g07', 'g11'], tags: [] },
  { id: 'c2', name: 'Completed', type: 'dynamic', game_ids: [], tags: ['Completed'] },
  { id: 'c3', name: 'Waiting for updates', type: 'manual', game_ids: ['g02', 'g04', 'g06', 'g09', 'g10'], tags: [] },
];

const mock = `(function () {
  const params = new URLSearchParams(location.search);
  try { if (!params.has('keep')) localStorage.clear(); localStorage.setItem('plauncher_lang', 'en'); localStorage.setItem('plauncher_sidebar_w', '270'); } catch (e) {}
  const games = ${JSON.stringify(data)};
  let collections = ${JSON.stringify(collections)};
  const priv = { has_pin: true, locked: false, idle_lock_min: 15, panic_enabled: true, panic_mods: 3, panic_vk: 72, panic_label: 'Ctrl+Alt+H', panic_action: 'hide', lock_on_panic: true, blur_mode: 'none', start_discreet: false };
  if (params.get('shot') === 'find') {
    for (const id of ['g01', 'g02']) Object.assign(games.find(g => g.id === id), {
      title: id === 'g01' ? 'MoonlitHarbor_v0.3' : 'QuietTown_v1.0', folder_path: 'D:/Games/' + (id === 'g01' ? 'MoonlitHarbor_v0.3' : 'QuietTown_v1.0'),
      description: '', cover_path: '', images: [], tags: ['Manual'], version: '', author: '', sources: [], primary_source: '',
      update_available: false, update_version: '', update_source: '', update_found_at: 0,
    });
  }
  const controls = window.__findMock = { failedSource: 'F95zone', delay: 180, previewDelay: 180, saveDelay: 180, previewError: false, saveError: false, empty: false, calls: [] };
  const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
  const results = ['Steam', 'F95zone', 'VNDB'].flatMap((source, i) => [0, 1].map(j => ({
    source, url: source === 'VNDB' ? 'https://vndb.org/v' + (1 + j) : source === 'Steam' ? 'https://store.steampowered.com/app/' + (1 + j) + '/' : 'https://f95zone.to/threads/' + (1 + j) + '/',
    title: j ? 'Moonlit Harbor: Afterlight' : 'Moonlit Harbor', author: 'Harbor Studio', version: source === 'F95zone' ? '0.3' : '', year: '2026', engine: source === 'F95zone' ? "Ren'Py" : '', updated_at: source === 'F95zone' ? '2026-10-30T12:00:00Z' : '',
    thumb: location.origin + '/media/covers/g01.svg', score: j ? 0.45 - i * 0.02 : 0.96 - i * 0.18, best: i === 0 && j === 0,
  }))).sort((a, b) => b.score - a.score);
  const previewData = url => ({
    title: results.find(h => h.url === url)?.title || 'Moonlit Harbor', description: 'A quiet harbor, an unexpected letter, and a summer that changes everything. Explore the town and choose your own path.',
    author: 'Harbor Studio', engine: '', version: url.includes('f95zone') ? '0.3' : '', languages: ['English', 'Русский'], tags: ['Visual Novel', 'Mystery'],
    cover_path: location.origin + '/media/covers/g01.svg', images: [1, 2].map(n => location.origin + '/media/covers/g01-' + n + '.svg'),
  });
  const stats = (${require('./stats-fixture.cjs').toString()})({ games, collections, priv, enabled: params.get('shot') === 'stats' });
  collections = stats.collections;
  const impl = {
    GetLibraryStats: stats.library, GetGameSessions: stats.game, GetSessionRevision: stats.revision,
    Unlock: () => { priv.locked = false; }, VerifyPin: () => true,
    Lock: () => { priv.locked = true; window.__mockEmit('privacy-locked'); },
    IsConfigured: () => true, GetGames: () => games, SearchGames: (q) => games.filter(g => g.title.toLowerCase().includes(q.toLowerCase())),
    GetScanPaths: () => ['D:/Games'], GetCollections: () => collections, SaveCollections: (c) => { collections = c; },
    GetSupportedSources: () => [{ name: 'F95zone', domain: 'f95zone.to' }, { name: 'Pornolab', domain: 'pornolab.net' }, { name: 'Steam', domain: 'store.steampowered.com' }, { name: 'VNDB', domain: 'vndb.org' }],
    SuggestSearchName: (id) => ({ title: id === 'g01' ? 'Moonlit Harbor' : id === 'g02' ? 'Quiet Town' : games.find(g => g.id === id)?.title || '', version: id === 'g01' ? '0.3' : id === 'g02' ? '1.0' : '' }),
    SearchGameInfo: async (query, sources) => {
      await pause(controls.delay);
      return { hits: controls.empty ? [] : results.filter(h => sources.includes(h.source) && h.source !== controls.failedSource),
        failed: sources.includes(controls.failedSource) ? [{ source: controls.failedSource, error: '[parse.request] mock timeout' }] : [] };
    },
    PreviewGameInfo: async url => { await pause(controls.previewDelay); if (controls.previewError) throw '[parse.request {"site":"Steam"}] mock error'; return previewData(url); },
    ApplyGameInfo: async (id, url) => {
      await pause(controls.saveDelay);
      if (controls.saveError) throw '[app.library_save] mock error';
      const g = games.find(g => g.id === id), metadata = previewData(url), source = results.find(h => h.url === url).source;
      const tags = [...new Set([...(g.tags || []), ...metadata.tags])];
      Object.assign(g, metadata, { tags, engine: metadata.engine || g.engine, version: metadata.version || g.version || '0.3',
        cover_path: 'covers/g01.svg', images: ['covers/g01-1.svg', 'covers/g01-2.svg'],
        sources: [{ source, url, last_version: metadata.version }], primary_source: source, update_available: false, update_version: '', update_source: '', update_found_at: 0 });
    },
    GetCustomLocales: () => ({}), GetAppVersion: () => '1.0.0', GetDataDir: () => 'D:/pLauncher-data', GetPrivacy: () => priv, NotifyWindowShown: () => null,
    GetIgnoredPaths: () => ['D:/Games/_videos'], CheckLauncherUpdate: () => ({ available: false, current: '1.0.0', latest: '1.0.0' }),
    UpdateGame: (g) => {
      const i = games.findIndex(x => x.id === g.id);
      if (i >= 0) games[i] = { ...g, notes_updated_at: (games[i].notes || '') === (g.notes || '') ? games[i].notes_updated_at : Math.floor(Date.now() / 1000) };
      return null;
    },
    UpdateGameMetadata: (id) => {
      const g = games.find(g => g.id === id);
      Object.assign(g, { version: g.update_version || g.version, update_available: false, update_version: '', update_source: '', update_found_at: 0 });
    },
  };
  Object.assign(impl, (${require('./launch-files-fixture.cjs').toString()})({ games, priv, params, saveGame: impl.UpdateGame }));
  Object.assign(impl, (${require('./smart-lists-fixture.cjs').toString()})({ games, collections, priv, params, impl }));
  const App = new Proxy({}, { get: (_, name) => (...args) => Promise.resolve().then(() => { controls.calls.push({ name, args }); return impl[name]?.(...args); }).then(value => JSON.parse(JSON.stringify(value ?? null))) });
  window.go = { main: { App } };
  const events = {};
  window.__mockEmit = (name, ...args) => { if (name === 'privacy-locked') priv.locked = true; if (name === 'privacy-locked' || name === 'privacy-panic') impl.StopSmartSearch(0); (events[name] || []).forEach(fn => fn(...args)); };
  window.runtime = new Proxy({}, { get: (_, name) => name === 'EventsOnMultiple' ? (event, fn) => { (events[event] ||= []).push(fn); return () => {}; } : () => {} });

  // Сценарии съёмки
  const shot = new URLSearchParams(location.search).get('shot') || 'home';
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const key = (k, o = {}) => window.dispatchEvent(new KeyboardEvent('keydown', Object.assign({ key: k, bubbles: true }, o)));
  const card = (id) => document.querySelector('[data-game-id="' + id + '"]');
  const st = document.createElement('style'); st.textContent = '*,*::before,*::after{animation:none!important;transition:none!important}'; document.head.appendChild(st);
  window.addEventListener('load', async () => {
    await sleep(900);
    if (shot === 'detail' || shot === 'find') { card('g01').click(); await sleep(600); }
    if (shot === 'discreet') { key('h', { ctrlKey: true, code: 'KeyH' }); await sleep(400); }
    if (shot === 'bulk') {
      document.querySelector('button[aria-label="View options"]').click(); await sleep(80);
      [...document.querySelectorAll('button')].find(b => b.textContent.trim() === 'Select').click(); await sleep(200);
      document.querySelector('button[aria-label="View options"]').click(); await sleep(80);
      // Выбираем в сетке «All games» (последние экземпляры карточек) и прокручиваем к ней
      const last = (id) => [...document.querySelectorAll('[data-game-id="' + id + '"]')].pop();
      for (const id of ['g02', 'g04', 'g06', 'g10']) { last(id).click(); await sleep(80); }
      const h = [...document.querySelectorAll('h2, h3, span')].find(e => e.textContent.trim().startsWith('All games'));
      if (h) h.scrollIntoView({ block: 'start' });
      await sleep(400);
    }

    if (shot === 'settings') {
      key(',', { ctrlKey: true }); await sleep(300);
      [...document.querySelectorAll('.fixed button')].find(b => b.textContent.includes('Privacy')).click(); await sleep(400);
    }
    if (params.has('readme')) {
      const waitFor = async test => { for (let i=0;i<100;i++) { if (test()) return; await sleep(50); } throw Error('README scenario timed out: '+shot); };
      const clickText = (text, scope='body') => [...document.querySelectorAll(scope+' button')].find(b=>b.textContent.trim()===text).click();
      if (shot === 'find') {
        controls.failedSource = ''; document.querySelector('[data-find-search]').click();
        await waitFor(()=>document.querySelectorAll('[data-find-card]').length);
        document.querySelector('[data-metadata-panel]').scrollIntoView({block:'start'});
      }
      if (shot === 'stats') {
        document.querySelector('[data-nav-statistics]').click();
        await waitFor(()=>document.querySelector('[data-stats-top]'));
      }
      if (shot === 'launch' || shot === 'editor') {
        card('g01').click(); await waitFor(()=>document.querySelector('[data-launch-menu]'));
        document.querySelector('[data-launch-menu]').click(); await waitFor(()=>document.querySelector('[data-context-menu]'));
        if (shot === 'editor') {
          clickText('Edit','[data-context-menu]'); await waitFor(()=>document.querySelector('[data-launch-find]'));
          document.querySelector('[data-launch-find]').click();
          await waitFor(()=>document.querySelector('[data-launch-candidates] input:not(:disabled)'));
          document.querySelector('[data-launch-candidates] input:not(:disabled)').click();
          document.querySelector('[data-launch-editor]').scrollIntoView({block:'center'});
        }
      }
      if (shot === 'smart') {
        document.querySelector('[data-smart-list=nosource]').click(); await sleep(80);
        document.querySelector('[data-smart-banner] button').click();
        await waitFor(()=>window.__smartMock.applies.length===1);
        window.__smartMock.delay = 60000;
        await waitFor(()=>document.querySelector('[data-smart-banner]').textContent.includes('1 of 3'));
      }
      await sleep(200);
    }
    document.body.dataset.ready = '1';
  });
})();`;
fs.writeFileSync(path.join(UI, 'mock.js'), mock);
const idx = path.join(UI, 'index.html');
fs.writeFileSync(idx, fs.readFileSync(idx, 'utf8').replace('<head>', '<head><script src="/mock.js"></script>'));
console.log('ui ready: ' + data.length + ' games');
