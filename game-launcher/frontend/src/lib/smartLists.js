import { writable } from 'svelte/store';
import { SuggestSearchName, SearchGameInfo, BeginSmartSearch, StopSmartSearch, ApplySmartGameInfo } from '../../wailsjs/go/main/App.js';
import { lsJSON } from './util.js';
import { tr } from '../i18n.js';
import { showToast } from './ui.js';

export const noLaunch = g => !g.folder_missing && !!g.launch_missing;
export const noSource = g => !(g.sources || []).length;
export const searchResults = writable({}); // Memory only; cleared on privacy transitions.
export const smartSearch = writable(null);
export function searchSources() {
    const sites = ['F95zone', 'Steam', 'VNDB'];
    const stored = lsJSON('plauncher_find_sources', sites);
    return Array.isArray(stored) ? sites.filter(s => stored.includes(s)) : sites;
}
let run = null;
const publish = job => smartSearch.set({ active: !job.stopped, done: job.done, total: job.ids.length, applied: job.applied, manual: job.manual, errors: job.errors });
function summarize(job) {
    if (job.summarized) return;
    job.summarized = true;
    showToast(tr('smart.search_summary', { n: job.applied, m: job.manual, k: job.errors }), job.errors ? 'error' : 'success');
}
export function stopSmartSearch(privacy = false) {
    const job = run;
    if (privacy) searchResults.set({});
    if (!job || job.stopped) return;
    job.stopped = true;
    job.wake?.();
    if (job.token) StopSmartSearch(job.token).catch(() => {});
    publish(job);
    if (!privacy) summarize(job);
}

export function reconcileSearchVisibility(games) {
    if (!run || run.stopped) return;
    const visible = new Set(games.map(g => g.id));
    if (run.ids.some(id => !visible.has(id))) stopSmartSearch(true);
}

export async function findMissingSources(games, getVisible, reload) {
    if (run && !run.stopped) return;
    const sources = searchSources();
    if (!sources.length) { showToast(tr('find.select_source')); return; }
    const ids = games.filter(noSource).map(g => g.id);
    if (!ids.length) return;
    const job = { ids, token: 0, stopped: false, done: 0, applied: 0, manual: 0, errors: 0 };
    run = job; publish(job);
    const eligible = id => !job.stopped && getVisible().some(g => g.id === id && noSource(g));
    async function findOne(id) {
        const suggestion = await SuggestSearchName(id);
        if (!eligible(id)) return;
        const query = suggestion?.title || '';
        const result = query ? await SearchGameInfo(query, sources, id) : { hits: [], failed: [] };
        if (!eligible(id)) return;
        searchResults.update(cache => ({ ...cache, [id]: { query, version: suggestion?.version || '', sources, result } }));
        // Go owns the confidence rule and serializes saving with cancellation.
        const applied = await ApplySmartGameInfo(job.token, id, query, result);
        if (job.stopped) return;
        if (applied) {
            job.applied++;
            searchResults.update(cache => { const next = { ...cache }; delete next[id]; return next; });
            await reload();
        } else if (result?.failed?.length) job.errors++;
        else job.manual++;
    }
    try {
        job.token = await BeginSmartSearch();
        if (!job.token) { job.stopped = true; return; }
        for (const id of ids) {
            if (job.stopped) break;
            try { if (eligible(id)) await findOne(id); }
            catch (err) { if (!job.stopped) job.errors++; }
            if (job.stopped) break;
            job.done++; publish(job);
            if (job.done < ids.length) await new Promise(resolve => {
                const timer = setTimeout(resolve, 1000);
                job.wake = () => { clearTimeout(timer); resolve(); };
            });
        }
    } catch (err) { if (!job.stopped) { job.errors++; showToast(tr('toast.error', { err }), 'error'); } }
    finally {
        if (job.token) await StopSmartSearch(job.token).catch(() => {});
        if (!job.stopped) { job.stopped = true; summarize(job); }
        if (run === job) publish(job);
    }
}
