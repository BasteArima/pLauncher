// Fictional data only. No network search, disk checks, or actual launches.
module.exports = function smartListsFixture({ games, collections, priv, params, impl }) {
  const state = window.__smartMock = { token: 0, active: false, delay: 30, saveDelay: 30, searches: [], applies: [], detections: [], games };
  const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
  const enabled = params.get('shot') === 'smart';
  for (const g of games) g.launch_missing = !g.folder_missing && !g.exec_path;
  if (enabled) {
    for (const g of games.slice(0, 3)) { g.sources = []; g.primary_source = ''; }
    games[0].exec_path = ''; games[0].launch_missing = true;
    games[1].launch_missing = true; // A missing nonempty primary must not be replaced.
    games[2].folder_missing = true; games[2].launch_missing = false;
    games[3].sources = []; games[3].exec_path = ''; games[3].launch_missing = true;
    collections.push({ id: 'smart-hidden', name: 'Private', type: 'manual', game_ids: [games[3].id], tags: [], hidden: true });
  }
  const originalSearch = impl.SearchGameInfo;
  return {
    BeginSmartSearch: () => { state.active = !priv.locked; return ++state.token; },
    StopSmartSearch: token => { if (!token || token === state.token) state.active = false; },
    DetectLaunchFiles: ids => {
      state.detections.push(ids);
      let found = 0;
      for (const g of games) if (ids.includes(g.id) && !g.exec_path && !g.folder_missing) {
        g.exec_path = g.folder_path + '/Game.exe'; g.launch_missing = false; found++;
      }
      return found;
    },
    SearchGameInfo: async (query, sources, id) => {
      if (!enabled) return originalSearch(query, sources, id);
      state.searches.push({ id, query, sources, at: Date.now() });
      await pause(state.delay);
      if (id === 'g03') return { hits: [], failed: [{ source: sources[0], error: '[parse.request] fixture error' }] };
      return { hits: [{ source: sources[0], title: query, url: 'https://example.invalid/' + id, author: 'Fixture', score: .9, best: id === 'g01' }], failed: [] };
    },
    ApplySmartGameInfo: async (token, id, query, result) => {
      await pause(state.saveDelay);
      if (!state.active || token !== state.token || priv.locked || !result.hits?.[0]?.best || result.hits[0].title !== query) return false;
      const g = games.find(g => g.id === id);
      if (g.sources.length) return false;
      g.sources = [{ source: result.hits[0].source, url: result.hits[0].url, last_version: '' }];
      state.applies.push(id); return true;
    },
  };
};
