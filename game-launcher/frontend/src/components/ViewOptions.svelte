<script>
    // Настройки вида сетки — по требованию: одна кнопка-иконка, остальное во всплывающем окне
    // (сетка/полки, форма и размер карточек, режим выделения, разделы полок).
    import { t } from '../i18n.js';
    import { clickOutside } from '../lib/util.js';
    import Icon from './Icon.svelte';
    import { cardSize, setCardSize, CARD_MIN, CARD_MAX, CARD_DEFAULT, selecting, toggleSelectMode, cardShape, homeView } from '../lib/view.js';

    export let isHome = false;              // «Все игры»: можно переключить на полки
    export let onEditSections = () => {};

    let open = false;
    const seg = 'flex items-center gap-0.5 p-0.5 rounded-lg bg-slate-950 border border-white/10';
    const segBtn = (on) => 'flex-1 h-7 px-2.5 rounded-md text-xs transition-colors ' + (on ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-100');
    const label = 'text-[11px] font-semibold text-slate-500 uppercase tracking-wider mb-1.5';
</script>

<div class="relative" use:clickOutside on:clickoutside={() => open = false}>
    <button on:click={() => open = !open} title={$t('view.options')} aria-label={$t('view.options')} aria-expanded={open}
            class="h-8 w-8 rounded-lg flex items-center justify-center border transition-colors {open || $selecting ? 'border-indigo-500 bg-indigo-900 text-indigo-100' : 'border-white/10 text-slate-400 hover:text-white hover:border-white/20'}">
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0"/><circle cx="16" cy="6" r="2"/><circle cx="10" cy="12" r="2"/><circle cx="18" cy="18" r="2"/></svg>
    </button>

    {#if open}
        <div class="absolute right-0 top-10 z-40 w-64 rounded-xl glass-strong shadow-2xl p-4 flex flex-col gap-4 animate-fade-in">
            {#if isHome}
                <div>
                    <div class={label}>{$t('view.layout')}</div>
                    <div class={seg} role="group">
                        <button on:click={() => homeView.set('grid')} class={segBtn($homeView === 'grid')}>{$t('view.grid')}</button>
                        <button on:click={() => homeView.set('shelves')} class={segBtn($homeView === 'shelves')}>{$t('view.shelves')}</button>
                    </div>
                    {#if $homeView === 'shelves'}
                        <button on:click={() => { open = false; onEditSections(); }} class="mt-2 inline-flex items-center gap-1.5 text-xs text-indigo-300 hover:text-indigo-100"><Icon name="layers" size={13}/>{$t('lib.edit_sections')}</button>
                    {/if}
                </div>
            {/if}
            <div>
                <div class={label}>{$t('view.cards')}</div>
                <div class={seg} role="group">
                    <button on:click={() => cardShape.set('wide')} class={segBtn($cardShape === 'wide')}>{$t('view.wide')}</button>
                    <button on:click={() => cardShape.set('tall')} class={segBtn($cardShape === 'tall')}>{$t('view.tall')}</button>
                </div>
                <input type="range" min={CARD_MIN} max={CARD_MAX} step="5" value={$cardSize}
                       on:input={(e) => setCardSize(e.target.value)} on:dblclick={() => setCardSize(CARD_DEFAULT)}
                       title={$t('view.card_size_hint')} aria-label={$t('view.card_size_hint')}
                       class="w-full mt-3 accent-indigo-500"/>
            </div>
            <button on:click={() => { open = false; toggleSelectMode(); }}
                    class="h-8 rounded-lg text-xs border transition-colors {$selecting ? 'border-indigo-500 bg-indigo-900 text-indigo-100' : 'border-white/10 text-slate-300 hover:text-white hover:bg-white/5'}">
                {$selecting ? $t('btn.cancel') : $t('sel.select')}
            </button>
        </div>
    {/if}
</div>
