// Общие классы кнопок/полей для вкладок настроек.
export const btn = 'bg-white/5 hover:bg-white/10 text-slate-200 text-sm font-semibold py-2 px-3 rounded-lg border border-white/10 transition-colors disabled:opacity-50';
export const input = 'bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-400/40';
export const choice = (active) => 'text-left p-3 rounded-lg border transition-colors ' +
    (active ? 'border-indigo-500 bg-indigo-600/15' : 'border-white/10 hover:bg-white/5');
