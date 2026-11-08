// Fictional collection flow; all media is served by the local screenshot server.
module.exports = function pornolabFixture({ games, collections, priv, params, impl }) {
  const enabled = params.get('shot') === 'collection';
  const state = window.__collectionMock = {
    count: 2, best: true, fallback: false, resolveDelay: 40, previewDelay: 40, saveDelay: 40,
    resolveError: false, previewError: '', saveError: '', changed: false, expired: false,
    pageGets: 0, saved: [], manual: {}, games, calls: [], cached: false, generation: 0,
  };
  const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
  const failure = code => '[' + code + (code === 'parse.request' ? ' {"site":"Pornolab"}' : '') + '] Mock failure';
  const raw = 'https://pornolab.net/forum/viewtopic.php?t=3006469';
  const base = url => url.split('#')[0];
  const isCollection = url => /[?&]t=300646[9]/.test(url) || enabled;
  if (enabled) {
    Object.assign(games[0], { title: 'MoonlitHarbor_v0.3', folder_path: 'D:/Games/MoonlitHarbor_v0.3', description: '', cover_path: '', images: [], sources: [], primary_source: '', update_available: false });
    Object.assign(games[1], { cover_path: '', sources: [{ source: 'Pornolab', url: raw, last_version: 'Final' }], primary_source: 'Pornolab' });
    Object.assign(games[2], { cover_path: '', sources: [{ source: 'Steam', url: 'https://store.steampowered.com/app/1/', last_version: '' }], primary_source: 'Steam' });
    Object.assign(games[3], { cover_path: '', sources: [{ source: 'Pornolab', url: raw, last_version: 'Final' }], primary_source: 'Pornolab' });
    collections.push({ id: 'collection-hidden', name: 'Private collection', type: 'manual', game_ids: [games[3].id], tags: [], hidden: true });
  }
  const candidates = url => Array.from({ length: state.count }, (_, n) => {
    const index = n + 1;
    return { index, source: 'Pornolab', url: base(url) + '#game=' + index,
      title: index === 2 ? 'Moonlit Harbor' : index === 1 ? 'Quiet Town' : 'Mountain Trail',
      author: 'Harbor Studio', version: index === 2 ? '0.3' : '1.0', year: '2026',
      thumb: location.origin + '/media/covers/x0' + (index === 2 ? 1 : 2) + '-wide.svg',
      best: state.best && index === 2 && !state.fallback, score: index === 2 ? .95 : .3,
      fallback_title: state.fallback, description: 'A quiet adventure in location ' + index + '.',
    };
  }).sort((a, b) => b.score - a.score);
  const resolve = (url, refresh) => {
    if (refresh || !state.cached || state.expired) { state.pageGets++; state.cached = true; state.expired = false; }
    return { collection: true, candidates: candidates(url), snapshot: 'snapshot-' + state.count };
  };
  const metadata = (url, index) => {
    const h = candidates(url).find(c => c.index === index);
    if (!h) throw '[parse.collection_game_missing] Game missing';
    return { title: h.title, version: h.version, author: h.author, description: h.description, tags: ['Adventure'], languages: ['English'],
      cover_path: h.thumb, images: [location.origin + '/media/covers/g01-1.svg'],
      sources: [{ source: 'Pornolab', url: h.url, selected_title: h.title, last_version: h.version }] };
  };
  function validate(snapshot) {
    if (state.changed) { state.changed = false; state.count = 3; state.pageGets++; throw '[parse.collection_changed] Release changed'; }
    if (snapshot !== 'snapshot-' + state.count) throw '[parse.collection_changed] Release changed';
    if (state.expired) { state.pageGets++; state.expired = false; }
  }
  return {
    ResolvePornolabCollection: async (id, url, refresh) => {
      state.calls.push({ name: 'resolve', id, url, refresh }); await pause(state.resolveDelay);
      if (!isCollection(url)) return { collection: false, candidates: [], snapshot: '' };
      if (state.resolveError) throw '[parse.request {"site":"Pornolab"}] Request failed';
      return resolve(url, refresh);
    },
    PreviewPornolabGame: async (id, url, snapshot, index) => {
      state.calls.push({ name: 'preview', id, url, index }); await pause(state.previewDelay);
      if (state.previewError) throw failure(state.previewError);
      validate(snapshot); return metadata(url, index);
    },
    ApplyPornolabGame: async (id, url, snapshot, index) => {
      const generation = state.generation;
      state.calls.push({ name: 'save', id, url, index }); await pause(state.saveDelay);
      if (priv.locked || generation !== state.generation) return;
      if (state.saveError) throw failure(state.saveError);
      validate(snapshot);
      const g = games.find(g => g.id === id), m = metadata(url, index);
      Object.assign(g, m, { cover_path: 'covers/x01-wide.svg', images: ['covers/g01-1.svg'], primary_source: 'Pornolab', update_available: false });
      state.saved.push({ id, index }); delete state.manual[id];
    },
    ReparseGameInfo: async (token, id) => {
      state.calls.push({ name: 'reparse', id }); await pause(state.saveDelay);
      if (priv.locked || !window.__smartMock.active || token !== window.__smartMock.token) return { game_id: id, applied: false, manual: false };
      const g = games.find(g => g.id === id), source = g.sources[0];
      if (source.source === 'Pornolab' && !source.url.includes('#game=')) {
        state.manual[id] = { game_id: id, url: source.url, collection: resolve(source.url, false) };
        return { game_id: id, applied: false, manual: true };
      }
      g.cover_path = 'covers/x01-wide.svg'; return { game_id: id, applied: true, manual: false };
    },
    GetManualGameChoices: () => Object.values(state.manual),
  };
};
