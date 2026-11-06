// Serialized into build-ui.cjs's browser mock. No network or real library data.
module.exports = function statsFixture({ games, collections, priv, enabled }) {
  const DAY = 86400;
  let clock = Date.parse('2026-11-06T12:00:00Z');
  if (enabled) {
    const NativeDate = Date;
    window.Date = class extends NativeDate {
      constructor(...args) { super(...(args.length ? args : [clock])); }
      static now() { return clock; }
    };
    collections = [...collections,
      { id: 'private-manual', name: 'Private manual', type: 'manual', game_ids: ['g10'], tags: [], hidden: true },
      { id: 'private-dynamic', name: 'Private dynamic', type: 'dynamic', game_ids: ['g12'], tags: ['PrivateTag'], hidden: true },
    ];
    collections[0].game_ids.push('g10'); // A public membership must not override a hidden one.
    games.find(g => g.id === 'g11').tags.push('PrivateTag');
    games.forEach((g, i) => {
      g.time_played = [1800, 900, 6000, 0, 140, 90, 120, 30, 60, 50000, 60000, 70000][i];
      g.engine = ["Ren'Py", 'Unity', 'Godot', '', 'RPG Maker', 'HTML', 'Unreal', 'QSP', 'Flash', 'SecretEngine', 'SecretEngine', 'SecretEngine'][i];
      g.size_bytes = (i + 1) * 1024 ** 3;
      g.size_checked_at = Math.floor(clock / 1000);
    });
    Object.assign(games.find(g => g.id === 'g04'), { size_bytes: 0, size_checked_at: 0, cover_path: '', description: '', images: [], sources: [], primary_source: '', exec_path: '' });
    Object.assign(games.find(g => g.id === 'g05'), { folder_missing: true });
    Object.assign(games.find(g => g.id === 'g06'), { size_bytes: 0 }); // Measured empty folder.
    games.find(g => g.id === 'g07').exec_path = 'D:/Games/Game/index.html';
  }
  const start = Math.floor(clock / 1000);
  const sessions = games.flatMap((g, i) => ['g03', 'g04'].includes(g.id) ? [] : [0, 1, 2, 3, 8, 40].map((days, j) => ({
    id: g.id + '-' + j, game_id: g.id, started_at: start - days * DAY - (i + 2) * 3600,
    duration_seconds: (i >= 9 ? 30000 : (10 - i) * 600) + j * 60,
    interrupted: g.id === 'g01' && j === 1,
  })));
  // A real boundary-crossing fixture, not just already aggregated arrays.
  sessions.push({ id: 'boundary', game_id: 'g01', started_at: Date.parse('2026-10-31T23:30:00Z') / 1000, duration_seconds: 7200, interrupted: false });
  const control = window.__statsMock = {
    delay: 100, error: false, sessionError: false, emptyActivity: false, revision: 1, calls: [],
    advanceDay() { clock += DAY * 1000; window.dispatchEvent(new Event('focus')); },
    complete(gameID = 'g01') {
      sessions.unshift({ id: 'new-' + this.revision, game_id: gameID, started_at: Math.floor(clock / 1000) - 120, duration_seconds: 120, interrupted: false });
      games.find(g => g.id === gameID).time_played += 2; this.revision++;
      window.__mockEmit('games-updated'); window.__mockEmit('sessions-updated');
    },
    emptyLibrary() { games.splice(0); window.__mockEmit('games-updated'); },
  };
  const clone = v => JSON.parse(JSON.stringify(v));
  const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
  function week(date) { const d = new Date(date); d.setHours(0, 0, 0, 0); d.setDate(d.getDate() - (d.getDay() + 6) % 7); return d; }
  function weeks(n) {
    const d = week(clock); d.setDate(d.getDate() - (n - 1) * 7);
    return Array.from({ length: n }, () => { const start = d.getTime() / 1000; d.setDate(d.getDate() + 7); return { start, end: d.getTime() / 1000, seconds: 0 }; });
  }
  const overlap = (s, from, to) => Math.max(0, Math.min(s.started_at + s.duration_seconds, to) - Math.max(s.started_at, from));
  function aggregate(ids, period) {
    const selected = games.filter(g => ids.includes(g.id));
    const rows = control.emptyActivity ? [] : sessions.filter(s => selected.some(g => g.id === s.game_id));
    const now = clock / 1000, monday = week(clock).getTime() / 1000;
    const month = new Date(clock); month.setDate(1); month.setHours(0, 0, 0, 0);
    const out = { all_time_seconds: selected.reduce((n, g) => n + g.time_played * 60, 0), month_seconds: 0, week_seconds: 0,
      history_since: start - 100 * DAY, weeks: weeks(52), top_games: [], statuses: [], engines: [],
      disk: { bytes: 0, measured: 0, unknown: 0, missing: 0 }, updates: 0, no_cover: 0, no_source: 0 };
    const times = new Map(selected.map(g => [g.id, period === 'all' ? g.time_played * 60 : 0]));
    for (const s of rows) {
      const m = overlap(s, month.getTime() / 1000, now), w = overlap(s, monday, now);
      out.month_seconds += m; out.week_seconds += w;
      if (period !== 'all') times.set(s.game_id, times.get(s.game_id) + (period === 'week' ? w : m));
      for (const b of out.weeks) b.seconds += overlap(s, b.start, Math.min(b.end, now));
    }
    out.top_games = [...times].filter(([, seconds]) => seconds > 0).map(([game_id, seconds]) => ({ game_id, seconds }))
      .sort((a, b) => b.seconds - a.seconds || selected.find(g => g.id === a.game_id).title.localeCompare(selected.find(g => g.id === b.game_id).title) || a.game_id.localeCompare(b.game_id)).slice(0, 10);
    out.statuses = ['playing', 'want', 'finished', 'dropped', ''].map(name => ({ name, count: selected.filter(g => (g.status || '') === name).length }));
    out.engines = [...new Set(selected.map(g => g.engine))].map(name => ({ name, count: selected.filter(g => g.engine === name).length })).sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
    for (const g of selected) {
      if (g.folder_missing) out.disk.missing++;
      else if (!g.size_checked_at) out.disk.unknown++;
      else { out.disk.bytes += g.size_bytes; out.disk.measured++; }
      if (g.update_available) out.updates++;
      if (!g.cover_path) out.no_cover++;
      if (!g.sources.some(s => s.url)) out.no_source++;
    }
    return out;
  }
  return {
    collections,
    async library(ids, period, zone) {
      control.calls.push({ method: 'library', ids, period, zone });
      const result = priv.locked ? null : clone(aggregate(ids, period)), fail = control.error;
      await pause(control.delay); if (fail) throw '[sessions.storage] fixture statistics error'; return result;
    },
    async game(id, ids, zone) {
      control.calls.push({ method: 'game', id, ids, zone });
      const g = games.find(g => g.id === id);
      const rows = control.emptyActivity ? [] : sessions.filter(s => s.game_id === id).sort((a, b) => b.started_at - a.started_at || a.id.localeCompare(b.id));
      const buckets = weeks(8); for (const b of buckets) for (const s of rows) b.seconds += overlap(s, b.start, Math.min(b.end, clock / 1000));
      const result = priv.locked || !ids.includes(id) || !g ? null : clone({ tracking_supported: g.exec_path.endsWith('.exe'), count: rows.length,
        longest_seconds: Math.max(0, ...rows.map(s => s.duration_seconds)), history_since: start - 100 * DAY, weeks: buckets, latest: rows.slice(0, 5) });
      const fail = control.sessionError; await pause(control.delay); if (fail) throw '[sessions.storage] fixture sessions error'; return result;
    },
    revision: () => priv.locked ? null : control.revision,
  };
};
