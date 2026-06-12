import { writable, derived } from 'svelte/store';
import en from './locales/en.json';
import ru from './locales/ru.json';
import es from './locales/es.json';

// Встроенные языки. Пользовательские добавляются из data/languages/*.json через addLocales().
let dicts = { en, ru, es };
let current = 'en';

const tick = writable(0);          // бампается при смене языка/добавлении словарей
export const langStore = writable('en'); // текущий код языка (для биндингов)

function bump() { tick.update((n) => n + 1); }

// tr(key, params) — синхронный перевод (для использования в скрипте)
export function tr(key, params) {
    const d = dicts[current] || dicts.en || {};
    let s = d[key];
    if (s == null) s = (dicts.en && dicts.en[key]);
    if (s == null) s = key;
    if (params) {
        for (const k in params) s = s.split('{' + k + '}').join(params[k]);
    }
    return s;
}

// $t — реактивная версия для разметки: {$t('key')} / {$t('key', {n: 5})}
export const t = derived(tick, () => (key, params) => tr(key, params));

export function setLang(code) {
    if (dicts[code]) current = code;
    langStore.set(current);
    try { localStorage.setItem('plauncher_lang', current); } catch (e) {}
    bump();
}

// Сливаем пользовательские словари ПОКЛЮЧЕВО: файл переопределяет только те строки,
// что в нём есть. Так неполный en.json не сломает интерфейс, а встроенные языки можно
// частично править (положив, напр., en.json лишь с нужными ключами).
export function addLocales(extra) {
    if (!extra) return;
    for (const code in extra) {
        dicts[code] = { ...(dicts[code] || {}), ...extra[code] };
    }
    bump();
}

export function availableLangs() {
    return Object.keys(dicts).map((code) => ({ code, name: (dicts[code] && dicts[code].__name) || code }));
}

// Подбор языка при первом запуске: сохранённый -> системный -> английский
export function initialLang() {
    try {
        const saved = localStorage.getItem('plauncher_lang');
        if (saved && dicts[saved]) return saved;
    } catch (e) {}
    const sys = (navigator.language || 'en').slice(0, 2).toLowerCase();
    return dicts[sys] ? sys : 'en';
}
