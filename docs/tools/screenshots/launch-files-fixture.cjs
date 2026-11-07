// Launch-file fixture; these methods never open files or native dialogs.
module.exports = function launchFilesFixture({ games, priv, params, saveGame }) {
  const state = window.__launchFilesMock = { delay: 0, error: false, empty: false, incomplete: false, selectedPath: '', selectDelay: 0, selectError: false, availabilityDelay: 0, availabilityError: false, candidates: null, lateReply: false, missing: [], calls: [] };
  const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
  const key = path => (path || '').replaceAll('\\', '/').toLowerCase();
  const labelKey = path => {
    const words = path.split(/[\\/]/).pop().replace(/\.[^.]*$/, '').replace(/([a-z])([A-Z])/g, '$1 $2').toLowerCase().match(/[a-z]+/g) || [];
    return words.some(w => ['walkthrough', 'guide'].includes(w)) ? 'launch.label_walkthrough' : words.some(w => ['config', 'settings'].includes(w)) ? 'launch.label_settings' : '';
  };
  const files = g => [...(g.exec_path ? [{ path: g.exec_path, label: g.exec_label || '', primary: true }] : []), ...(g.launch_files || []).map(f => ({ ...f, primary: false }))];
  const describe = (g, f) => ({ ...f, name: f.path.split(/[\\/]/).pop(), relative_path: f.path.slice(g.folder_path.length + 1), label_key: f.label ? '' : labelKey(f.path), availability: state.missing.some(p => key(p) === key(f.path)) ? 'missing' : 'available' });
  for (const g of games) { g.exec_label ||= ''; g.launch_files ||= []; }
  games[0].launch_files = [{ path: games[0].folder_path + '/config.exe', label: 'Configuration' }, { path: games[0].folder_path + '/docs/guide.pdf', label: '' }];
  if (params.get('shot') === 'launch') {
    games[1].launch_files = [{ path: games[1].folder_path + '/missing-guide.pdf', label: '' }];
    state.missing.push(games[1].launch_files[0].path, games[3].exec_path);
    games[2].launch_files = Array.from({ length: 8 }, (_, n) => ({ path: games[2].folder_path + '/guide-' + n + '.txt', label: '' }));
    games[4].exec_path = ''; games[4].launch_files = [];
    games[5].exec_path = ''; games[5].launch_files = [{ path: games[5].folder_path + '/guide.pdf', label: '' }];
    games[6].launch_files = Array.from({ length: 10 }, (_, n) => ({ path: games[6].folder_path + '/guide-' + n + '.txt', label: '' }));
  }
  const launch = (id, path) => {
    if (priv.locked) return null;
    const g = games.find(g => g.id === id), f = g && files(g).find(f => key(f.path) === key(path));
    if (!f) throw '[launch.unlisted] file no longer listed';
    if (describe(g, f).availability === 'missing') throw '[launch.missing ' + JSON.stringify({ path: f.path }) + '] file not found';
    state.calls.push({ id, path: f.path, primary: f.primary });
    if (f.primary) g.last_launched_at = Math.floor(Date.now() / 1000);
    return null;
  };
  return {
    GetLaunchFileLabel: path => priv.locked ? '' : labelKey(path),
    GetLaunchFileAvailability: async id => {
      const g = games.find(g => g.id === id), result = g ? files(g).map(f => describe(g, f)) : [];
      await pause(state.availabilityDelay);
      if (priv.locked && !state.lateReply) return null;
      if (state.availabilityError) throw '[app.library_read] mock availability error';
      return result;
    },
    SelectLaunchFile: async () => {
      const result = state.selectedPath;
      await pause(state.selectDelay);
      if (priv.locked && !state.lateReply) return '';
      if (state.selectError) throw '[launch.select] mock dialog failure';
      return result;
    },
    UpdateGame: g => {
      const old = games.find(x => x.id === g.id), list = files(g), seen = new Set();
      for (const f of list) {
        if (seen.has(key(f.path))) throw '[launch.duplicate ' + JSON.stringify({ path: f.path }) + '] duplicate file';
        seen.add(key(f.path));
      }
      if (g.launch_files.length > 8 && (!old || g.launch_files.length > old.launch_files.length || list.some(f => !files(old).some(o => key(o.path) === key(f.path))))) throw '[launch.limit {"n":8}] too many files';
      return saveGame(g);
    },
    LaunchGameFile: launch, Launch: launch,
    FindLaunchFileCandidates: async id => {
      if (priv.locked) return null;
      await pause(state.delay);
      if (priv.locked && !state.lateReply) return null;
      const g = games.find(g => g.id === id);
      if (state.error) throw '[launch.search ' + JSON.stringify({ path: g.folder_path }) + '] mock search failure';
      const candidates = state.empty ? [] : state.candidates || [...files(g), { path: g.exec_path.toUpperCase().replaceAll('/', '\\'), label: '' }, { path: g.folder_path + '/walkthrough.html', label: '' }, { path: g.folder_path + '/settings.exe', label: '' }].filter(f => f.path);
      return { files: candidates.map(f => describe(g, f)), incomplete: state.incomplete, reason: state.incomplete ? 'candidates' : '' };
    },
  };
};
