import { writable } from 'svelte/store';
import { tr } from '../i18n.js';

// Every request has an owner/generation. A changed visibility scope drops cached
// data synchronously; navigation, lock and destroy invalidate late responses.
export function statsRequest() {
    const state = writable({ key: '', data: null, loading: false, error: null });
    let generation = 0, current = { key: '', data: null }, alive = true;
    return {
        subscribe: state.subscribe,
        async load(key, fetchData, active = true) {
            const ticket = ++generation;
            if (current.key !== key) current = { key, data: null };
            state.set({ ...current, loading: active, error: null });
            if (!active) return false;
            try {
                const data = await fetchData();
                if (!alive || ticket !== generation) return false;
                current = { key, data };
                state.set({ ...current, loading: false, error: null });
                return true;
            } catch (error) {
                if (alive && ticket === generation) {
                    current = { key, data: null };
                    state.set({ ...current, loading: false, error });
                }
                return false;
            }
        },
        destroy() { alive = false; generation++; state.set({ key: '', data: null, loading: false, error: null }); },
    };
}

export function statsLocale(lang) {
    const locale = ({ pt: 'pt-BR', zh: 'zh-Hans' })[lang] || lang;
    try { new Intl.NumberFormat(locale); return locale; } catch { return 'en'; }
}
export const statsNumber = (n, lang) => new Intl.NumberFormat(statsLocale(lang)).format(n || 0);
const unit = (n, name, lang, digits = 0) => new Intl.NumberFormat(statsLocale(lang), {
    style: 'unit', unit: name, unitDisplay: 'short', maximumFractionDigits: digits,
}).format(n);
export const statsHours = (seconds, lang) => unit((seconds || 0) / 3600, 'hour', lang, 1);
export function statsSize(bytes, lang) {
    let value = Math.max(0, bytes || 0), index = 0;
    while (value >= 1024 && index < 4) { value /= 1024; index++; }
    return tr(['size.b', 'size.kb', 'size.mb', 'size.gb', 'size.tb'][index], {
        n: new Intl.NumberFormat(statsLocale(lang), { maximumFractionDigits: index ? 1 : 0 }).format(value),
    });
}
export function statsDuration(seconds, lang) {
    const s = Math.max(0, Math.floor(seconds || 0)), h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60;
    return [h ? unit(h, 'hour', lang) : '', m ? unit(m, 'minute', lang) : '', s % 60 || !s ? unit(s % 60, 'second', lang) : ''].filter(Boolean).join(' ');
}
export function statsDate(seconds, lang, zone, withTime = false) {
    return new Intl.DateTimeFormat(statsLocale(lang), {
        timeZone: zone, year: 'numeric', month: 'short', day: 'numeric',
        ...(withTime ? { hour: '2-digit', minute: '2-digit' } : {}),
    }).format(new Date(seconds * 1000));
}
export function statsMonth(seconds, lang, zone) {
    return new Intl.DateTimeFormat(statsLocale(lang), { timeZone: zone, month: 'short' }).format(new Date(seconds * 1000));
}
export function calendarState() {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
    const now = new Date();
    return { zone, key: `${zone}:${now.getFullYear()}-${now.getMonth()}-${now.getDate()}` };
}
