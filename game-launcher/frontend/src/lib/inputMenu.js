// ПКМ по текстовым полям: вырезать/копировать/вставить/выделить всё.
// Нативное контекстное меню WebView отключено, поэтому собираем своё.
import { tr } from '../i18n.js';

export function isEditableTarget(el) {
    if (!el || el.disabled || el.readOnly) return false;
    if (el.tagName === 'TEXTAREA') return true;
    if (el.tagName !== 'INPUT') return false;
    return /^(text|search|url|email|tel|password|number|)$/i.test(el.type || '');
}

function replaceSelection(el, text) {
    const start = el.selectionStart ?? el.value.length;
    const end = el.selectionEnd ?? el.value.length;
    el.value = el.value.slice(0, start) + text + el.value.slice(end);
    const pos = start + text.length;
    el.focus();
    el.setSelectionRange(pos, pos);
    // Уведомляем Svelte о смене значения (двусторонний bind слушает input)
    el.dispatchEvent(new Event('input', { bubbles: true }));
}

async function copy(el) {
    const sel = el.value.substring(el.selectionStart, el.selectionEnd);
    try { await navigator.clipboard.writeText(sel); } catch (e) {}
}
async function cut(el) {
    await copy(el);
    replaceSelection(el, '');
}
async function paste(el) {
    let text = '';
    try { text = await navigator.clipboard.readText(); } catch (e) { return; }
    if (text) replaceSelection(el, text);
}

export function inputCtxItems(el) {
    const hasSel = el.selectionStart != null && el.selectionStart !== el.selectionEnd;
    const items = [];
    if (hasSel) {
        items.push({ label: tr('ctx.cut'), icon: '✂', action: () => cut(el) });
        items.push({ label: tr('ctx.copy'), icon: '⧉', action: () => copy(el) });
    }
    items.push({ label: tr('ctx.paste'), icon: '📋', action: () => paste(el) });
    items.push({ sep: true });
    items.push({ label: tr('ctx.select_all'), action: () => { el.focus(); el.select(); } });
    return items;
}
