<script>
    // Инструменты вида: размер карточек и режим выделения. На главной — в строке с
    // «Изменить разделы» (над всеми разделами, при любом их порядке), в фильтре/поиске —
    // рядом с сортировкой.
    import { t } from '../i18n.js';
    import { cardSize, setCardSize, CARD_MIN, CARD_MAX, CARD_DEFAULT, selecting, toggleSelectMode } from '../lib/view.js';
</script>

<div class="flex items-center gap-3 shrink-0">
    <label class="flex items-center gap-2 text-slate-400" title={$t('view.card_size_hint')}>
        <span class="text-base leading-none">▦</span>
        <input type="range" min={CARD_MIN} max={CARD_MAX} step="5" value={$cardSize}
               on:input={(e) => setCardSize(e.target.value)}
               on:dblclick={() => setCardSize(CARD_DEFAULT)}
               class="w-24 accent-indigo-500"/>
    </label>
    <button on:click={toggleSelectMode} title={$t('sel.mode_hint')}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm border transition-colors {$selecting ? 'border-indigo-400/50 bg-indigo-500/25 text-indigo-100 hover:bg-indigo-500/35' : 'border-white/10 bg-white/5 hover:bg-white/10 text-slate-300'}">
        {#if $selecting}
            <!-- В режиме выделения — выход из него -->
            <span class="leading-none">✕</span> {$t('btn.cancel')}
        {:else}
            <!-- Иконка «квадрат с галочкой» -->
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="3.5" y="3.5" width="17" height="17" rx="4"/>
                <path d="M8 12.5l2.8 2.8L16.5 9"/>
            </svg>
            {$t('sel.select')}
        {/if}
    </button>
</div>
