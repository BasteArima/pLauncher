// Состояние вида библиотеки: размер карточек и выделение игр (массовые действия).
import { writable, derived, get } from 'svelte/store';
import { lsGet, lsSet } from './util.js';

// --- Размер карточек (минимальная ширина в px) ---
export const CARD_MIN = 120;
export const CARD_MAX = 280;
export const CARD_DEFAULT = 170;

function clampCard(v) {
    v = Math.round(+v || CARD_DEFAULT);
    return Math.max(CARD_MIN, Math.min(CARD_MAX, v));
}
export const cardSize = writable(clampCard(lsGet('plauncher_card_size', CARD_DEFAULT)));
cardSize.subscribe(v => lsSet('plauncher_card_size', String(v)));

export function setCardSize(v) { cardSize.set(clampCard(v)); }
export function stepCardSize(dir) { setCardSize(get(cardSize) + dir * 15); }

// --- Баннер «Продолжить» на главной (можно выключить в настройках) ---
export const showHero = writable(lsGet('plauncher_show_hero', '1') !== '0');
showHero.subscribe(v => lsSet('plauncher_show_hero', v ? '1' : '0'));

// --- Выделение игр ---
// Хранится Set id; «якорь» — последняя кликнутая карточка (для Shift+клик).
export const selection = writable(new Set());
// Режим выделения включён кнопкой «Выбрать» (ещё ничего не выделено) или самим выделением
// (Ctrl+клик). В режиме клик по карточке выделяет её, а не открывает игру.
export const selectMode = writable(false);
export const selecting = derived([selection, selectMode], ([s, m]) => m || s.size > 0);
let anchorId = '';

// Снять выделение и выйти из режима
export function clearSelection() { selection.set(new Set()); selectMode.set(false); anchorId = ''; }
export function toggleSelectMode() { if (get(selecting)) clearSelection(); else selectMode.set(true); }

export function toggleSelected(id) {
    selection.update(s => {
        const n = new Set(s);
        if (n.has(id)) n.delete(id); else n.add(id);
        return n;
    });
    anchorId = id;
}

export function selectMany(ids, add = true) {
    selection.update(s => {
        const n = add ? new Set(s) : new Set();
        for (const id of ids) n.add(id);
        return n;
    });
}

// Карточки игр в основной области, в порядке документа (для Shift-диапазона,
// «выделить всё» и навигации стрелками).
export function cardElements() {
    return Array.from(document.querySelectorAll('main [data-game-id]'));
}

// Shift+клик: выделить все карточки между якорем и кликнутой (по порядку на экране).
export function selectRange(targetEl) {
    const els = cardElements();
    const to = els.indexOf(targetEl);
    const from = anchorId ? els.findIndex(e => e.dataset.gameId === anchorId) : -1;
    if (to < 0 || from < 0) { toggleSelected(targetEl.dataset.gameId); return; }
    const [a, b] = from < to ? [from, to] : [to, from];
    selectMany(els.slice(a, b + 1).map(e => e.dataset.gameId));
    anchorId = targetEl.dataset.gameId;
}

export function selectAllVisible() {
    selectMany(cardElements().map(e => e.dataset.gameId));
}

// Навигация стрелками: ближайшая карточка в нужном направлении (по геометрии —
// работает и в сетке, и между горизонтальными полками).
export function focusNeighbor(dir) {
    const els = cardElements();
    if (!els.length) return;
    const cur = document.activeElement && document.activeElement.closest && document.activeElement.closest('[data-game-id]');
    if (!cur || !els.includes(cur)) { els[0].focus(); return; }
    const r = cur.getBoundingClientRect();
    const cx = r.left + r.width / 2, cy = r.top + r.height / 2;
    let best = null, bestScore = Infinity;
    for (const el of els) {
        if (el === cur) continue;
        const q = el.getBoundingClientRect();
        const dx = q.left + q.width / 2 - cx, dy = q.top + q.height / 2 - cy;
        const main = dir === 'left' ? -dx : dir === 'right' ? dx : dir === 'up' ? -dy : dy;
        const side = dir === 'left' || dir === 'right' ? Math.abs(dy) : Math.abs(dx);
        if (main <= 4) continue;
        const score = main + side * 3;
        if (score < bestScore) { bestScore = score; best = el; }
    }
    if (best) {
        best.focus({ preventScroll: true });
        best.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'smooth' });
    }
}
