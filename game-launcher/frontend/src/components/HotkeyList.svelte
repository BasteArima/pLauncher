<script>
    // Таблица горячих клавиш (справка F1 и вкладка «Общие» в настройках).
    import { t } from '../i18n.js';
    import { HOTKEY_GROUPS } from '../lib/hotkeys.js';
    export let panicLabel = '';              // сочетание кнопки паники, если включена
</script>

<div class="grid sm:grid-cols-2 gap-x-8 gap-y-5">
    {#each HOTKEY_GROUPS as group}
        <div>
            <div class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-2">{$t(group.title)}</div>
            <div class="flex flex-col gap-1.5">
                {#each group.keys as [keys, label]}
                    <div class="flex items-center justify-between gap-3 text-sm">
                        <span class="text-slate-300">{$t(label)}</span>
                        <kbd class="shrink-0 font-mono text-xs text-slate-200 bg-white/5 ring-1 ring-white/10 rounded px-1.5 py-0.5">{keys}</kbd>
                    </div>
                {/each}
                {#if group.title === 'hk.g_privacy' && panicLabel}
                    <div class="flex items-center justify-between gap-3 text-sm">
                        <span class="text-slate-300">{$t('hk.panic')}</span>
                        <kbd class="shrink-0 font-mono text-xs text-slate-200 bg-white/5 ring-1 ring-white/10 rounded px-1.5 py-0.5">{panicLabel}</kbd>
                    </div>
                {/if}
            </div>
        </div>
    {/each}
</div>
