<script>
    // Свой титлбар безрамочного окна (перетаскивание через --wails-draggable).
    // Слева — меню «Библиотека» (общие действия) и статус фоновых операций,
    // справа — быстрые кнопки приватности (видны на любом экране) и управление окном.
    import { t } from '../i18n.js';
    import { discreet } from '../lib/privacy.js';
    import { WindowMinimise, WindowToggleMaximise, Quit } from '../../wailsjs/runtime/runtime';

    export let menuOpen = false;
    export let scanning = false;
    export let checkingUpdates = false;
    export let hasPin = false;
    export let locked = false;             // экран PIN: только управление окном
    export let ready = true;               // false — идёт мастер первого запуска
    export let onMenu = (rect) => {};
    export let onLock = () => {};

    let menuBtn;
    const winBtn = 'h-full px-4 hover:bg-white/10 text-slate-400 hover:text-white transition-colors';
    const quickBtn = 'h-7 px-2 rounded-md flex items-center gap-1.5 text-xs transition-colors';
</script>

<div class="h-9 shrink-0 flex items-center justify-between glass-strong border-b border-white/5 select-none z-30" style="--wails-draggable:drag" on:dblclick={() => WindowToggleMaximise()}>
    <div class="flex items-center gap-1 pl-3 h-full min-w-0">
        <span class="flex items-center gap-2 pr-2 text-slate-400 text-xs font-semibold tracking-wide"><span class="text-indigo-400">◆</span> pLauncher</span>
        {#if ready && !locked}
        <button bind:this={menuBtn} on:click={() => onMenu(menuBtn.getBoundingClientRect())} on:dblclick|stopPropagation
                style="--wails-draggable:no-drag"
                class="{quickBtn} {menuOpen ? 'bg-white/10 text-white' : 'text-slate-300 hover:bg-white/10 hover:text-white'}">
            {$t('menu.library')}
            <svg class="w-3 h-3 transition-transform {menuOpen ? 'rotate-180' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>
        </button>
        {/if}
        {#if scanning || checkingUpdates}
            <span class="flex items-center gap-1.5 ml-2 text-xs text-slate-400 truncate">
                <svg class="w-3.5 h-3.5 animate-spin shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-2.64-6.36"/></svg>
                {scanning ? $t('menu.scanning') : $t('menu.checking')}
            </span>
        {/if}
    </div>
    <div class="flex items-center h-full" style="--wails-draggable:no-drag">
        {#if ready && !locked}
        <div class="flex items-center gap-1 pr-2">
            <button on:click={() => discreet.update(v => !v)} on:dblclick|stopPropagation
                    title={$discreet ? $t('app.discreet_show') : $t('app.discreet_hide')}
                    class="{quickBtn} {$discreet ? 'bg-indigo-500/25 text-indigo-200 ring-1 ring-inset ring-indigo-400/40' : 'text-slate-400 hover:bg-white/10 hover:text-white'}">
                {#if $discreet}
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.6 10.6a2 2 0 0 0 2.8 2.8"/><path d="M9.4 5.2A9.6 9.6 0 0 1 12 5c4.5 0 8.3 2.9 10 7a11 11 0 0 1-2.2 3.4M6.6 6.6A11 11 0 0 0 2 12c1.7 4.1 5.5 7 10 7a9.7 9.7 0 0 0 5.4-1.6"/><path d="M3 3l18 18"/></svg>
                    {$t('menu.covers_hidden')}
                {:else}
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12c1.7-4.1 5.5-7 10-7s8.3 2.9 10 7c-1.7 4.1-5.5 7-10 7s-8.3-2.9-10-7z"/><circle cx="12" cy="12" r="3"/></svg>
                {/if}
            </button>
            {#if hasPin}
                <button on:click={onLock} on:dblclick|stopPropagation title={$t('menu.lock') + ' · Ctrl+L'}
                        class="{quickBtn} text-slate-400 hover:bg-white/10 hover:text-white">
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>
                </button>
            {/if}
        </div>
        {/if}
        <button on:click={() => WindowMinimise()} class={winBtn} title={$t("titlebar.min")} aria-label={$t("titlebar.min")}>
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/></svg>
        </button>
        <button on:click={() => WindowToggleMaximise()} class={winBtn} title={$t("titlebar.max")} aria-label={$t("titlebar.max")}>
            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="1"/></svg>
        </button>
        <button on:click={() => Quit()} class="h-full px-4 hover:bg-red-600 text-slate-400 hover:text-white transition-colors" title={$t("titlebar.close")} aria-label={$t("titlebar.close")}>
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 6l12 12M18 6L6 18"/></svg>
        </button>
    </div>
</div>
