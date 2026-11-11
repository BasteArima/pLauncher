<script>
    // Свой титлбар безрамочного окна (перетаскивание через --wails-draggable).
    // Слева — название и статус фоновых операций, справа — быстрые кнопки
    // (приватность, горячие клавиши, настройки; видны на любом экране) и управление окном.
    // Сканирование и добавление игр — в боковой панели.
    import { t } from '../i18n.js';
    import { discreet } from '../lib/privacy.js';
    import { hotkeys, hkLabel, withHotkey } from '../lib/hotkeys.js';
    import { updateInfo, download, updateModalOpen } from '../lib/update.js';
    import { WindowMinimise, WindowToggleMaximise, Quit } from '../../wailsjs/runtime/runtime';

    export let scanning = false;
    export let checkingUpdates = false;
    export let hasPin = false;
    export let hiddenCount = 0;            // скрытых коллекций (кнопка показа — только если они есть)
    export let showHidden = false;
    export let locked = false;             // экран PIN: только управление окном
    export let ready = true;               // false — идёт мастер первого запуска
    export let onLock = () => {};
    export let onToggleHidden = () => {};
    export let onHotkeys = () => {};
    export let onSettings = () => {};
    export let searchQuery = '';           // bind
    export let searchInput = null;         // bind (Ctrl+F)
    export let onSearch = () => {};

    const winBtn = 'h-full px-4 hover:bg-white/10 text-slate-400 hover:text-white transition-colors';
    const quickBtn = 'h-8 px-2.5 rounded-lg flex items-center gap-1.5 text-xs transition-colors';

    // Плашка обновления лаунчера: есть новая версия → идёт загрузка → готово к перезапуску
    $: dlState = $download.state || 'idle';
    $: showUpdate = dlState === 'downloading' || dlState === 'ready' || ($updateInfo && $updateInfo.available);
    $: dlPercent = $download.total > 0 ? Math.min(100, Math.round($download.done / $download.total * 100)) : 0;
</script>

<div class="h-12 shrink-0 flex items-center justify-between glass-strong border-b border-white/5 select-none z-30" style="--wails-draggable:drag" on:dblclick={() => WindowToggleMaximise()}>
    <div class="flex items-center h-full min-w-0 flex-1">
        <span class="flex items-center gap-2.5 pl-4 text-[15px] font-semibold tracking-wide shrink-0">
            <span class="w-6 h-6 rounded-md bg-indigo-500 text-white text-xs flex items-center justify-center">p</span><span class="text-slate-100">pLauncher</span>
        </span>
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
            <!-- Поиск по библиотеке: справа, перед иконками (как в макете) -->
            <label class="relative flex items-center w-72 mr-3 h-9 rounded-lg bg-slate-900 border border-white/10 focus-within:border-indigo-400/60 transition-colors"
                   style="--wails-draggable:no-drag" on:dblclick|stopPropagation>
                <svg class="absolute left-3 w-4 h-4 text-slate-500 pointer-events-none" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="M20 20l-4-4"/></svg>
                <input type="text" bind:this={searchInput} bind:value={searchQuery} on:input={onSearch}
                       placeholder={$t('search.placeholder')} aria-label={$t('search.placeholder')}
                       class="w-full h-full bg-transparent pl-9 pr-16 text-sm text-slate-100 placeholder:text-slate-500 focus:outline-none"/>
                {#if searchQuery}
                    <button on:click={() => { searchQuery = ''; onSearch(); }} aria-label={$t('btn.clear')}
                            class="absolute right-2 w-6 h-6 rounded text-slate-500 hover:text-white">×</button>
                {:else if hkLabel($hotkeys, 'search')}
                    <span class="absolute right-2.5 text-[11px] text-slate-500 border border-white/10 rounded px-1.5 pointer-events-none">{hkLabel($hotkeys, 'search').replace(/\+(?=.)/g, ' ')}</span>
                {/if}
            </label>
            {#if showUpdate}
                <button on:click={() => updateModalOpen.set(true)} on:dblclick|stopPropagation
                        class="h-6 px-2.5 rounded-full flex items-center gap-1.5 text-xs font-semibold ring-1 ring-inset transition-colors mr-1 {dlState === 'ready' ? 'bg-emerald-500/15 text-emerald-200 ring-emerald-400/40 hover:bg-emerald-500/25' : 'bg-amber-500/15 text-amber-200 ring-amber-400/40 hover:bg-amber-500/25'}">
                    {#if dlState === 'ready'}
                        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-2.64-6.36"/><path d="M21 3v6h-6"/></svg>
                        {$t('upd.pill_restart')}
                    {:else if dlState === 'downloading'}
                        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 4v11m0 0l-4-4m4 4l4-4M5 20h14"/></svg>
                        {dlPercent}%
                    {:else}
                        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5m0 0l-6 6m6-6l6 6"/></svg>
                        {$t('upd.pill', { v: $updateInfo.latest })}
                    {/if}
                </button>
            {/if}
            <button on:click={() => discreet.update(v => !v)} on:dblclick|stopPropagation
                    title={$discreet ? withHotkey($t('app.discreet_show'), $hotkeys, 'discreet') : withHotkey($t('app.discreet_hide'), $hotkeys, 'discreet')}
                    class="{quickBtn} {$discreet ? 'bg-indigo-500/25 text-indigo-200 ring-1 ring-inset ring-indigo-400/40' : 'text-slate-400 hover:bg-white/10 hover:text-white'}">
                {#if $discreet}
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.6 10.6a2 2 0 0 0 2.8 2.8"/><path d="M9.4 5.2A9.6 9.6 0 0 1 12 5c4.5 0 8.3 2.9 10 7a11 11 0 0 1-2.2 3.4M6.6 6.6A11 11 0 0 0 2 12c1.7 4.1 5.5 7 10 7a9.7 9.7 0 0 0 5.4-1.6"/><path d="M3 3l18 18"/></svg>
                    {$t('menu.covers_hidden')}
                {:else}
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12c1.7-4.1 5.5-7 10-7s8.3 2.9 10 7c-1.7 4.1-5.5 7-10 7s-8.3-2.9-10-7z"/><circle cx="12" cy="12" r="3"/></svg>
                {/if}
            </button>
            {#if hiddenCount}
                <button on:click={onToggleHidden} on:dblclick|stopPropagation
                        title={withHotkey(showHidden ? $t('menu.hide_hidden') : $t('menu.show_hidden'), $hotkeys, 'hidden')}
                        aria-label={showHidden ? $t('menu.hide_hidden') : $t('menu.show_hidden')}
                        class="{quickBtn} {showHidden ? 'bg-amber-500/20 text-amber-200 ring-1 ring-inset ring-amber-400/40' : 'text-slate-400 hover:bg-white/10 hover:text-white'}">
                    <!-- Папка с замком: скрытые коллекции -->
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 19H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v1"/><rect x="14" y="15" width="8" height="6" rx="1"/><path d="M16 15v-2a2 2 0 0 1 4 0v2"/></svg>
                </button>
            {/if}
            {#if hasPin}
                <button on:click={onLock} on:dblclick|stopPropagation title={withHotkey($t('menu.lock'), $hotkeys, 'lock')} aria-label={$t('menu.lock')}
                        class="{quickBtn} text-slate-400 hover:bg-white/10 hover:text-white">
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>
                </button>
            {/if}
            <button on:click={onHotkeys} on:dblclick|stopPropagation title={withHotkey($t('menu.hotkeys'), $hotkeys, 'help')} aria-label={$t('menu.hotkeys')}
                    class="{quickBtn} text-slate-400 hover:bg-white/10 hover:text-white">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="6" width="20" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M8 14h8"/></svg>
            </button>
            <button on:click={onSettings} on:dblclick|stopPropagation title={withHotkey($t('nav.settings'), $hotkeys, 'settings')}
                    aria-label={$t('nav.settings')} class="{quickBtn} text-slate-400 hover:bg-white/10 hover:text-white">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></svg>
            </button>
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
