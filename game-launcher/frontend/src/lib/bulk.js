// Массовые действия над выделенными играми. Используются панелью выделения
// и контекстным меню (ПКМ по одной из выделенных карточек).
import { writable } from 'svelte/store';
import { tr } from '../i18n.js';
import { showToast, askConfirm } from './ui.js';
import { clearSelection } from './view.js';
import {
    RemoveGames, IgnoreGames, SetFavorites, DetectLaunchFiles, CheckGameUpdates, UpdateGameMetadata,
} from '../../wailsjs/go/main/App.js';

// Прогресс долгой операции: null или {label, done, total}
export const bulkProgress = writable(null);
let cancelled = false;
export function cancelBulk() { cancelled = true; }

// Ссылка на основную площадку игры (для перепарса/проверки обновлений)
function primaryUrl(g) {
    const list = (g.sources || []).filter(s => s.url);
    const p = list.find(s => s.source === g.primary_source) || list[0];
    return p ? p.url : '';
}

// Последовательно выполняет fn для каждой игры с прогрессом и возможностью отмены.
async function runEach(label, items, fn) {
    cancelled = false;
    let ok = 0, fail = 0;
    bulkProgress.set({ label, done: 0, total: items.length });
    for (let i = 0; i < items.length && !cancelled; i++) {
        try { await fn(items[i]); ok++; } catch (e) { fail++; console.error(e); }
        bulkProgress.set({ label, done: i + 1, total: items.length });
    }
    bulkProgress.set(null);
    return { ok, fail };
}

export async function bulkFavorite(ids, favorite, reload) {
    try {
        const n = await SetFavorites(ids, favorite);
        await reload();
        showToast(tr(favorite ? 'bulk.fav_added' : 'bulk.fav_removed', { n }), 'success');
    } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
}

export async function bulkDetectLaunch(ids, reload) {
    try {
        const n = await DetectLaunchFiles(ids);
        await reload();
        showToast(n ? tr('bulk.launch_found', { n }) : tr('bulk.launch_none'), 'success');
    } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
}

export async function bulkRemove(ids, reload) {
    const ok = await askConfirm({
        title: tr('bulk.remove_title', { n: ids.length }),
        message: tr('bulk.remove_msg'),
        confirmText: tr('btn.delete'),
        danger: true,
    });
    if (!ok) return false;
    try {
        const n = await RemoveGames(ids);
        clearSelection();
        await reload();
        showToast(tr('bulk.removed', { n }), 'success');
        return true;
    } catch (err) { showToast(tr('toast.error', { err }), 'error'); return false; }
}

// Игнор при сканировании: убрать из лаунчера и не добавлять папки снова.
// title — название, если игра одна (для текста подтверждения).
export async function bulkIgnore(ids, reload, title = '') {
    const ok = await askConfirm({
        title: title ? tr('ignore.title_one', { title }) : tr('ignore.title_n', { n: ids.length }),
        message: tr('ignore.msg'),
        confirmText: tr('ignore.confirm'),
        danger: true,
    });
    if (!ok) return false;
    try {
        const n = await IgnoreGames(ids);
        clearSelection();
        await reload();
        showToast(tr('ignore.done', { n }), 'success');
        return true;
    } catch (err) { showToast(tr('toast.error', { err }), 'error'); return false; }
}

export async function bulkCheckUpdates(games, reload) {
    const items = games.filter(primaryUrl);
    if (!items.length) { showToast(tr('bulk.no_sources'), 'error'); return; }
    const { ok, fail } = await runEach(tr('bulk.checking'), items, g => CheckGameUpdates(g.id));
    await reload();
    showToast(tr('bulk.done', { ok, fail }), fail ? 'error' : 'success');
}

export async function bulkReparse(games, reload) {
    const items = games.filter(primaryUrl);
    if (!items.length) { showToast(tr('bulk.no_sources'), 'error'); return; }
    const confirmed = await askConfirm({
        title: tr('bulk.reparse_title', { n: items.length }),
        message: tr('bulk.reparse_msg'),
        confirmText: tr('bulk.reparse'),
    });
    if (!confirmed) return;
    const { ok, fail } = await runEach(tr('bulk.reparsing'), items, g => UpdateGameMetadata(g.id, primaryUrl(g)));
    await reload();
    showToast(tr('bulk.done', { ok, fail }), fail ? 'error' : 'success');
}
