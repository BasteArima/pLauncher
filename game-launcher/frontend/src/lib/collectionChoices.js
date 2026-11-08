import { toast } from './ui.js';
import { writable, get } from 'svelte/store';
import { GetManualGameChoices } from '../../wailsjs/go/main/App.js';

export const collectionChoices = writable({});
export const manualGames = writable({});
export const manualViewRequest = writable(0);
let epoch = 0;
export const openManualGames = () => manualViewRequest.update(n => n + 1);
export const collectionError = err => /\[parse\.collection_(choice_required|game_missing|changed)(?:\s|\])/.test(String(err));
export function isPornolab(raw) {
    try { return /^(www\.)?porno?lab\.net$/i.test(new URL(raw).hostname); } catch { return false; }
}
export function releaseURL(raw) {
    const url = new URL(raw); url.hash = ''; return url.href;
}
export function rememberManual(id) { manualGames.update(items => ({ ...items, [id]: true })); }
export function forgetManual(id) {
    for (const store of [manualGames, collectionChoices]) store.update(items => {
        if (!(id in items)) return items;
        const next = { ...items }; delete next[id]; return next;
    });
}
export function clearManualChoices() { epoch++; manualGames.set({}); collectionChoices.set({}); toast.update(t => t.action ? { ...t, show: false, action: null } : t); }
export function reconcileManualVisibility(games) {
    const visible = new Set(games.map(g => g.id));
    for (const id of Object.keys(get(manualGames))) if (!visible.has(id)) forgetManual(id);
}
export async function loadManualChoices(ids, valid = () => true) {
    const current = epoch;
    const entries = await GetManualGameChoices();
    if (current !== epoch || !valid()) return;
    collectionChoices.update(items => {
        const next = { ...items };
        for (const entry of entries || []) if (ids.includes(entry.game_id)) next[entry.game_id] = entry;
        return next;
    });
}
