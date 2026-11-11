<script>
    // Строка переназначаемой горячей клавиши в настройках: клик по сочетанию → нажать новое.
    // Esc — отменить запись. Клавиши перехватываются на самой кнопке (stopPropagation),
    // поэтому глобальный обработчик App их не видит.
    import { t } from '../i18n.js';
    import { HOTKEY_ACTIONS, hotkeys, hotkeyOverrides, hkLabel, comboFromEvent, checkCombo,
             setHotkey, clearHotkey, resetHotkey } from '../lib/hotkeys.js';
    import Icon from './Icon.svelte';

    export let id;

    let capturing = false;
    let hint = '';

    $: label = hkLabel($hotkeys, id);
    $: changed = !!$hotkeyOverrides[id];

    function start() { capturing = true; hint = ''; }
    function stop() { capturing = false; }

    function onKey(e) {
        if (!capturing) {
            // Enter/Space на кнопке запускают запись как клик — остальное не трогаем
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.shiftKey) { capturing = false; hint = ''; return; }
        const combo = comboFromEvent(e);
        if (combo === '') return;                       // ждём основную клавишу
        if (combo === null) { hint = $t('hotkey.unsupported'); return; }
        const res = checkCombo(combo, id);
        if (res.error === 'hk.conflict') { hint = $t('hk.conflict', { action: $t(HOTKEY_ACTIONS[res.other].label) }); return; }
        if (res.error) { hint = $t(res.error); return; }
        setHotkey(id, combo);
        capturing = false; hint = '';
    }

    const iconBtn = 'w-6 h-6 shrink-0 rounded flex items-center justify-center text-slate-500 hover:text-white hover:bg-white/10 transition-colors';
</script>

<div class="group/hk text-sm" data-hotkey-row={id}>
    <div class="flex items-center gap-1.5 min-h-[26px]">
        <span class="text-slate-300 flex-1 min-w-0">{$t(HOTKEY_ACTIONS[id].label)}</span>
        {#if changed}
            <button type="button" on:click={() => { resetHotkey(id); hint = ''; }} title={$t('hk.reset')} aria-label={$t('hk.reset')} class={iconBtn} data-hotkey-reset>
                <Icon name="refresh" size={13}/>
            </button>
        {/if}
        {#if label && !capturing}
            <button type="button" on:click={() => { clearHotkey(id); hint = ''; }} title={$t('hk.clear')} aria-label={$t('hk.clear')}
                    class="{iconBtn} opacity-0 group-hover/hk:opacity-100 focus-visible:opacity-100" data-hotkey-clear>
                <Icon name="x" size={13}/>
            </button>
        {/if}
        <button type="button" on:click={start} on:keydown={onKey} on:blur={stop} data-hotkey={id}
                title={$t('hk.click_to_change')}
                class="shrink-0 min-w-[72px] px-2 py-0.5 rounded font-mono text-xs ring-1 transition-colors
                       {capturing ? 'ring-indigo-400 bg-indigo-500/15 text-indigo-200 animate-pulse' : changed ? 'ring-indigo-400/40 bg-indigo-500/10 text-indigo-100 hover:bg-indigo-500/20' : 'ring-white/10 bg-white/5 text-slate-200 hover:bg-white/10'}">
            {capturing ? $t('hotkey.press') : (label || $t('hk.unassigned'))}
        </button>
    </div>
    {#if hint}<p class="text-xs text-amber-300 text-right mt-0.5" data-hotkey-hint>{hint}</p>{/if}
</div>
