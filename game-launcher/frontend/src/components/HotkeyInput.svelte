<script>
    // Поле записи сочетания клавиш: клик → нажать комбинацию. Отдаёт WinAPI-коды:
    // mods = Alt(1)|Ctrl(2)|Shift(4)|Win(8), vk = virtual-key code, label — подпись.
    import { t } from '../i18n.js';

    export let label = '';
    export let onChange = (hk) => {};       // {mods, vk, label}

    let capturing = false;
    let hint = '';

    const NAMED = {
        Space: [0x20, 'Space'], Backquote: [0xC0, '`'], Minus: [0xBD, '-'], Equal: [0xBB, '='],
        BracketLeft: [0xDB, '['], BracketRight: [0xDD, ']'], Backslash: [0xDC, '\\'], Semicolon: [0xBA, ';'],
        Quote: [0xDE, "'"], Comma: [0xBC, ','], Period: [0xBE, '.'], Slash: [0xBF, '/'],
        Insert: [0x2D, 'Insert'], Delete: [0x2E, 'Delete'], Home: [0x24, 'Home'], End: [0x23, 'End'],
        PageUp: [0x21, 'PgUp'], PageDown: [0x22, 'PgDn'], ArrowLeft: [0x25, '←'], ArrowUp: [0x26, '↑'],
        ArrowRight: [0x27, '→'], ArrowDown: [0x28, '↓'], Pause: [0x13, 'Pause'], ScrollLock: [0x91, 'ScrollLock'],
    };
    // Клавиши, которые можно назначить без модификаторов (их почти никто не использует)
    const SOLO_OK = new Set(['Pause', 'ScrollLock']);

    function keyFromCode(code) {
        let m;
        if ((m = /^Key([A-Z])$/.exec(code))) return [m[1].charCodeAt(0), m[1]];
        if ((m = /^Digit(\d)$/.exec(code))) return [0x30 + +m[1], m[1]];
        if ((m = /^Numpad(\d)$/.exec(code))) return [0x60 + +m[1], 'Num' + m[1]];
        if ((m = /^F(\d{1,2})$/.exec(code)) && +m[1] >= 1 && +m[1] <= 24) return [0x6F + +m[1], 'F' + m[1]];
        return NAMED[code] || null;
    }

    function onKey(e) {
        if (!capturing) return;
        e.preventDefault();
        e.stopPropagation();
        if (e.key === 'Escape') { capturing = false; hint = ''; return; }
        if (['Control', 'Alt', 'Shift', 'Meta'].includes(e.key)) return; // ждём основную клавишу
        const k = keyFromCode(e.code);
        if (!k) { hint = $t('hotkey.unsupported'); return; }
        const mods = (e.altKey ? 1 : 0) | (e.ctrlKey ? 2 : 0) | (e.shiftKey ? 4 : 0) | (e.metaKey ? 8 : 0);
        const isF = /^F\d+$/.test(k[1]);
        if (!mods && !isF && !SOLO_OK.has(e.code)) { hint = $t('hotkey.need_mod'); return; }
        const parts = [];
        if (mods & 2) parts.push('Ctrl');
        if (mods & 1) parts.push('Alt');
        if (mods & 4) parts.push('Shift');
        if (mods & 8) parts.push('Win');
        parts.push(k[1]);
        capturing = false; hint = '';
        onChange({ mods, vk: k[0], label: parts.join('+') });
    }
</script>

<div class="flex items-center gap-2">
    <button type="button" on:click={() => { capturing = true; hint = ''; }} on:keydown={onKey} on:blur={() => capturing = false}
            class="min-w-[160px] px-3 py-2 rounded-lg font-mono text-sm border transition-colors {capturing ? 'border-indigo-400 bg-indigo-500/15 text-indigo-200 animate-pulse' : 'border-white/10 bg-slate-900/60 text-white hover:bg-white/5'}">
        {capturing ? $t('hotkey.press') : (label || '—')}
    </button>
    {#if hint}<span class="text-xs text-amber-300">{hint}</span>{/if}
</div>
