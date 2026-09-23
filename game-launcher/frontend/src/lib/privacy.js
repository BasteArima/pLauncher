// Размытие обложек: режим из настроек + дискретный режим (Ctrl+H / 👁),
// который временно размывает всё. Компоненты берут класс через blurCls($coverBlur, kind).
import { writable, derived } from 'svelte/store';
import { tr } from '../i18n.js';

export const blurMode = writable('none');   // none | hover | always — из настроек приватности
export const discreet = writable(false);    // дискретный режим (перекрывает blurMode)

export const coverBlur = derived([blurMode, discreet], ([m, d]) => (d ? 'always' : m));

// Классы перечислены целиком — иначе Tailwind их не сгенерирует.
// hover-варианты раскрываются при наведении на ближайший .group (или на сам элемент для 'self').
const CLS = {
    card: { always: 'blur-2xl scale-110', hover: 'blur-2xl scale-110 group-hover:blur-0 group-hover:scale-105' },
    cover: { always: 'blur-2xl', hover: 'blur-2xl group-hover:blur-0' },
    icon: { always: 'blur-md', hover: 'blur-md group-hover:blur-0' },
    self: { always: 'blur-2xl', hover: 'blur-2xl hover:blur-0' },
    bg:   { always: 'blur-3xl', hover: 'blur-3xl' },
};

export function blurCls(mode, kind = 'card') {
    return (CLS[kind] && CLS[kind][mode]) || '';
}

// Ошибки PIN приходят с бэкенда на английском — переводим известные.
export function pinError(err) {
    const msg = String(err);
    if (/wrong PIN/i.test(msg)) return tr('lock.wrong');
    let m = /wait (\d+) s/.exec(msg);
    if (m) return tr('lock.wait', { s: m[1] });
    m = /at least (\d+)/.exec(msg);
    if (m) return tr('privacy.pin_short', { n: m[1] });
    return msg.replace(/^Error:\s*/, '');
}
