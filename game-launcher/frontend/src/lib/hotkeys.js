// Горячие клавиши: переназначаемые действия + список для справки (F1) и вкладки настроек.
// Обработчики — в App.svelte (handleKeydown) и GameCard; они спрашивают hotkeyIs(e, 'id').
// Сочетание хранится строкой вида «Ctrl+Shift+H»: модификаторы в порядке Ctrl, Alt, Shift и
// клавиша по физическому коду (e.code), поэтому раскладка (русская и т.п.) не мешает.
import { writable, derived } from 'svelte/store';
import { lsJSON, lsSet } from './util.js';

// keys — сочетания по умолчанию (первое показывается в подсказках);
// extra — скрытые синонимы, действуют, пока действие не переназначено.
export const HOTKEY_ACTIONS = {
    search:      { label: 'hk.search',       keys: ['Ctrl+F'] },
    play:        { label: 'hk.play',         keys: ['Ctrl+Enter'] },
    nav_back:    { label: 'hk.nav_back',     keys: ['Alt+←'] },
    nav_forward: { label: 'hk.nav_forward',  keys: ['Alt+→'] },
    scan:        { label: 'hk.scan',         keys: ['F5'] },
    settings:    { label: 'hk.settings',     keys: ['Ctrl+,'] },
    help:        { label: 'hk.help',         keys: ['F1'] },
    save:        { label: 'hk.edit_save',    keys: ['Ctrl+S'] },
    select_all:  { label: 'hk.sel_all',      keys: ['Ctrl+A'] },
    delete:      { label: 'hk.sel_delete',   keys: ['Delete'] },
    zoom_in:     { label: 'hk.zoom_in',      keys: ['Ctrl+='], extra: ['Ctrl+Shift+=', 'Ctrl+Num+'] },
    zoom_out:    { label: 'hk.zoom_out',     keys: ['Ctrl+-'], extra: ['Ctrl+Num-'] },
    zoom_reset:  { label: 'hk.zoom_reset',   keys: ['Ctrl+0'], extra: ['Ctrl+Num0'] },
    discreet:    { label: 'hk.discreet',     keys: ['Ctrl+H'] },
    hidden:      { label: 'hk.hidden',       keys: ['Ctrl+Shift+H'] },
    lock:        { label: 'hk.lock',         keys: ['Ctrl+L'] },
};

// Строки справки: { id } — переназначаемое действие, { keys, label } — фиксированное.
export const HOTKEY_GROUPS = [
    { title: 'hk.g_nav', rows: [
        { id: 'search' },
        { keys: '← ↑ → ↓', label: 'hk.arrows' },
        { keys: 'Enter', label: 'hk.open' },
        { id: 'play' },
        { keys: 'Esc / Backspace', label: 'hk.back' },
        { id: 'nav_back' },
        { id: 'nav_forward' },
        { keys: 'hk.mouse_buttons', label: 'hk.history' },
        { id: 'scan' },
        { id: 'settings' },
        { id: 'help' },
    ] },
    { title: 'hk.g_edit', rows: [
        { id: 'save' },
        { keys: 'Esc', label: 'hk.edit_cancel' },
        { keys: 'Esc / Backspace', label: 'hk.preview_back' },
    ] },
    { title: 'hk.g_sel', rows: [
        { keys: 'Ctrl+Click', label: 'hk.sel_toggle' },
        { keys: 'Shift+Click', label: 'hk.sel_range' },
        { keys: 'Space', label: 'hk.sel_focused' },
        { id: 'select_all' },
        { id: 'delete' },
        { keys: 'Esc', label: 'hk.sel_clear' },
    ] },
    { title: 'hk.g_view', rows: [
        { id: 'zoom_in' },
        { id: 'zoom_out' },
        { id: 'zoom_reset' },
        { keys: 'Ctrl+Wheel', label: 'hk.zoom_wheel' },
    ] },
    { title: 'hk.g_privacy', rows: [
        { id: 'discreet' },
        { id: 'hidden' },
        { id: 'lock' },
    ] },
];

// --- Имя клавиши по физическому коду ---
const NAMED = {
    Minus: '-', Equal: '=', Comma: ',', Period: '.', Slash: '/', Backslash: '\\', Semicolon: ';',
    Quote: "'", BracketLeft: '[', BracketRight: ']', Backquote: '`', IntlBackslash: '\\',
    Enter: 'Enter', NumpadEnter: 'Enter', Space: 'Space', Tab: 'Tab', Backspace: 'Backspace', Escape: 'Esc',
    Delete: 'Delete', Insert: 'Insert', Home: 'Home', End: 'End', PageUp: 'PgUp', PageDown: 'PgDn',
    ArrowLeft: '←', ArrowUp: '↑', ArrowRight: '→', ArrowDown: '↓', Pause: 'Pause',
    NumpadAdd: 'Num+', NumpadSubtract: 'Num-', NumpadMultiply: 'Num*', NumpadDivide: 'Num/', NumpadDecimal: 'Num.',
};
const MODIFIERS = new Set(['Control', 'Alt', 'Shift', 'Meta', 'AltGraph', 'OS']);

export function keyName(code) {
    let m;
    if ((m = /^Key([A-Z])$/.exec(code))) return m[1];
    if ((m = /^Digit(\d)$/.exec(code))) return m[1];
    if ((m = /^Numpad(\d)$/.exec(code))) return 'Num' + m[1];
    if ((m = /^F(\d{1,2})$/.exec(code)) && +m[1] >= 1 && +m[1] <= 24) return 'F' + m[1];
    return NAMED[code] || null;
}

// Сочетание из события: '' — нажат только модификатор, null — клавиша не поддерживается.
export function comboFromEvent(e) {
    if (MODIFIERS.has(e.key)) return '';
    let key = keyName(e.code);
    if (!key && e.key === 'Enter') key = 'Enter';
    if (!key) return null;
    const parts = [];
    if (e.ctrlKey || e.metaKey) parts.push('Ctrl');
    if (e.altKey) parts.push('Alt');
    if (e.shiftKey) parts.push('Shift');
    parts.push(key);
    return parts.join('+');
}

// --- Назначения: в localStorage хранятся только изменённые действия (id → [сочетание] или []) ---
const KEY = 'plauncher_hotkeys';
function loadOverrides() {
    const raw = lsJSON(KEY, {});
    const out = {};
    if (raw && typeof raw === 'object') {
        for (const [id, v] of Object.entries(raw)) {
            if (HOTKEY_ACTIONS[id] && Array.isArray(v)) out[id] = v.filter(c => typeof c === 'string' && c).slice(0, 1);
        }
    }
    return out;
}
export const hotkeyOverrides = writable(loadOverrides());
hotkeyOverrides.subscribe(v => lsSet(KEY, JSON.stringify(v)));

// Действующие сочетания: id → массив (первое — основное)
export const hotkeys = derived(hotkeyOverrides, (o) => {
    const map = {};
    for (const [id, a] of Object.entries(HOTKEY_ACTIONS)) map[id] = o[id] ? o[id] : [...a.keys, ...(a.extra || [])];
    return map;
});
let current = {};
hotkeys.subscribe(v => current = v);

// Нажатие (событие или готовая строка сочетания) — это действие id?
export function hotkeyIs(e, id) {
    const combo = typeof e === 'string' ? e : comboFromEvent(e);
    return !!combo && (current[id] || []).includes(combo);
}

// Основное сочетание для подписей; map передаётся явно ($hotkeys) ради реактивности Svelte.
export const hkLabel = (map, id) => ((map && map[id]) || [])[0] || '';
// «Текст · Ctrl+X» или просто текст, если сочетание снято
export const withHotkey = (text, map, id) => (hkLabel(map, id) ? `${text} · ${hkLabel(map, id)}` : text);
export const isDefaultHotkey = (overrides, id) => !overrides[id];

// --- Проверка нового сочетания ---
// Без модификатора можно назначить только F-клавиши и пару служебных; Shift+буква — это ввод текста.
const SOLO_OK = new Set(['Delete', 'Insert', 'Pause']);
// Сочетания, которые нужны для работы с текстом или перезагрузили бы WebView
const RESERVED = new Set(['Ctrl+C', 'Ctrl+V', 'Ctrl+X', 'Ctrl+Z', 'Ctrl+Y', 'Ctrl+Shift+Z', 'Ctrl+R', 'Alt+F4', 'Esc', 'Enter', 'Space', 'Tab', 'Backspace']);

// → { ok: true } или { error: 'hk.need_mod' | 'hk.reserved' | 'hk.conflict', other }
export function checkCombo(combo, id) {
    const parts = combo.split('+');
    // «Num+» оканчивается на плюс: собираем клавишу обратно
    const key = combo.endsWith('+') && combo !== '+' ? parts.slice(-2).join('+') : parts[parts.length - 1];
    const hasMod = /(^|\+)(Ctrl|Alt)\+/.test(combo);
    if (RESERVED.has(combo)) return { error: 'hk.reserved' };
    if (!hasMod && !/^F\d+$/.test(key) && !SOLO_OK.has(key)) return { error: 'hk.need_mod' };
    for (const [other, combos] of Object.entries(current)) {
        if (other !== id && combos.includes(combo)) return { error: 'hk.conflict', other };
    }
    return { ok: true };
}

export function setHotkey(id, combo) { hotkeyOverrides.update(o => ({ ...o, [id]: [combo] })); }
export function clearHotkey(id) { hotkeyOverrides.update(o => ({ ...o, [id]: [] })); }
export function resetHotkey(id) { hotkeyOverrides.update(o => { const n = { ...o }; delete n[id]; return n; }); }
export function resetAllHotkeys() { hotkeyOverrides.set({}); }
