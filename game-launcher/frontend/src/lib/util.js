// Общие хелперы UI (без состояния).
import { tr } from '../i18n.js';

// URL обложки/скриншота (файлы отдаются бэкендом по /media/<путь>)
export function mediaSrc(p) {
    return p ? `/media/${p.replaceAll('\\', '/')}` : '';
}

// CSS object-fit/object-position обложки игры
export function coverStyle(g) {
    return `object-fit:${(g && g.cover_fit) || 'cover'};object-position:${(g && g.cover_pos) || '50% 20%'}`;
}

export function coverPosXY(g) {
    const p = ((g && g.cover_pos) || '50% 20%').split(' ');
    return { x: parseInt(p[0]) || 50, y: parseInt(p[1]) || 20 };
}

export function fmtPlaytime(min) {
    if (!min || min <= 0) return '';
    const h = Math.floor(min / 60), m = min % 60;
    return h ? tr('time.hm', { h, m }) : tr('time.m', { m });
}

// Сортировка списка игр по ключу (ключ передаётся аргументом — так Svelte видит зависимость)
export function sortGames(arr, key) {
    const a = [...arr];
    switch (key) {
        case 'title_asc':     a.sort((x, y) => (x.title || '').localeCompare(y.title || '')); break;
        case 'played_desc':   a.sort((x, y) => (y.last_launched_at || 0) - (x.last_launched_at || 0)); break;
        case 'playtime_desc': a.sort((x, y) => (y.time_played || 0) - (x.time_played || 0)); break;
        case 'added_asc':     a.sort((x, y) => (x.added_at || 0) - (y.added_at || 0)); break;
        default:              a.sort((x, y) => (y.added_at || 0) - (x.added_at || 0)); // added_desc
    }
    return a;
}

// Svelte-action: событие clickoutside при клике вне элемента
export function clickOutside(node) {
    const handler = (e) => { if (!node.contains(e.target)) node.dispatchEvent(new CustomEvent('clickoutside')); };
    document.addEventListener('click', handler, true);
    return { destroy() { document.removeEventListener('click', handler, true); } };
}

// Безопасная работа с localStorage (в превью/приватном режиме может бросать)
export function lsGet(key, fallback = null) {
    try { const v = localStorage.getItem(key); return v == null ? fallback : v; } catch (e) { return fallback; }
}
export function lsSet(key, value) {
    try { localStorage.setItem(key, value); } catch (e) {}
}
export function lsJSON(key, fallback) {
    try { const v = JSON.parse(localStorage.getItem(key) || 'null'); return v == null ? fallback : v; } catch (e) { return fallback; }
}

// Скопировать текст в буфер с тостом
export async function copyText(text, label, notify) {
    if (!text) return;
    try {
        await navigator.clipboard.writeText(text);
        notify(tr('toast.copied', { label }), 'success');
    } catch (err) {
        notify(tr('toast.copy_fail'), 'error');
    }
}
