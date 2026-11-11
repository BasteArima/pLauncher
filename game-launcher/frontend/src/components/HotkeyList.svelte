<script>
    // Таблица горячих клавиш (справка F1 и вкладка «Общие» в настройках).
    // editable — в настройках: клик по сочетанию переназначает его.
    import { t } from '../i18n.js';
    import { HOTKEY_GROUPS, HOTKEY_ACTIONS, hotkeys, hkLabel } from '../lib/hotkeys.js';
    import HotkeyBinding from './HotkeyBinding.svelte';
    export let panicLabel = '';              // сочетание кнопки паники, если включена
    export let editable = false;

    const kbd = 'shrink-0 font-mono text-xs text-slate-200 bg-white/5 ring-1 ring-white/10 rounded px-1.5 py-0.5';
</script>

<div class="grid sm:grid-cols-2 gap-x-8 gap-y-5">
    {#each HOTKEY_GROUPS as group}
        <div class="min-w-0">
            <div class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-2">{$t(group.title)}</div>
            <div class="flex flex-col gap-1.5">
                {#each group.rows as row}
                    {#if row.id && editable}
                        <HotkeyBinding id={row.id}/>
                    {:else}
                        <div class="flex items-center justify-between gap-3 text-sm min-h-[24px]">
                            <span class="text-slate-300 min-w-0">{$t(row.id ? HOTKEY_ACTIONS[row.id].label : row.label)}</span>
                            {#if row.id}
                                <kbd class="{kbd} {hkLabel($hotkeys, row.id) ? '' : 'text-slate-500'}" data-hotkey={row.id}>{hkLabel($hotkeys, row.id) || $t('hk.unassigned')}</kbd>
                            {:else}
                                <kbd class="{kbd} {editable ? 'opacity-50' : ''}">{row.keys.startsWith('hk.') ? $t(row.keys) : row.keys}</kbd>
                            {/if}
                        </div>
                    {/if}
                {/each}
                {#if group.title === 'hk.g_privacy' && panicLabel}
                    <div class="flex items-center justify-between gap-3 text-sm min-h-[24px]">
                        <span class="text-slate-300">{$t('hk.panic')}</span>
                        <kbd class={kbd}>{panicLabel}</kbd>
                    </div>
                {/if}
            </div>
        </div>
    {/each}
</div>
