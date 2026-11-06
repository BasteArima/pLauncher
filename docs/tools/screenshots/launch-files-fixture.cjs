// Backend-only fixture; these methods never open files or native dialogs.
module.exports = function launchFilesFixture({ games, priv, params }) {
  const state = window.__launchFilesMock = { delay: 0, error: false, incomplete: false, selectedPath: '', missing: [], calls: [] };
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
    GetLaunchFileAvailability: id => priv.locked ? null : files(games.find(g => g.id === id)).map(f => describe(games.find(g => g.id === id), f)),
    SelectLaunchFile: () => priv.locked ? '' : state.selectedPath,
    LaunchGameFile: launch, Launch: launch,
    FindLaunchFileCandidates: async id => {
      if (priv.locked) return null;
      await new Promise(resolve => setTimeout(resolve, state.delay));
      if (priv.locked) return null;
      const g = games.find(g => g.id === id);
      if (state.error) throw '[launch.search ' + JSON.stringify({ path: g.folder_path }) + '] mock search failure';
      const candidates = [...files(g), { path: g.folder_path + '/walkthrough.html', label: '' }];
      return { files: candidates.map(f => describe(g, f)), incomplete: state.incomplete, reason: state.incomplete ? 'candidates' : '' };
    },
  };
};
