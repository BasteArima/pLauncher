<script>
    // Блок «Метаданные» на странице игры: ссылки по площадкам, проверка обновлений
    // и загрузка данных по ссылке через парсер.
    import { t, tr } from '../i18n.js';
    import { showToast } from '../lib/ui.js';
    import Icon from './Icon.svelte';
    import FindByName from './FindByName.svelte';
    import { searchResults } from '../lib/smartLists.js';
    import { clickOutside } from '../lib/util.js';
    import { OpenURL, CheckGameUpdates, CheckSourceUpdate } from '../../wailsjs/go/main/App.js';

    export let game;
    export let supportedSources = [];
    export let parsing = false;
    export let onParse = async (url) => {};             // распарсить ссылку в эту игру
    export let onReload = async () => {};
    export let prominent = false;                       // игра без информации: крупный блок «Найти информацию»

    export let onPreview = async (hit, version) => {};
    export let onDismiss = () => {};
    let tab = prominent || $searchResults[game?.id] ? 'name' : 'link';
    let parseUrl = '';
    let showSources = false;
    let checking = false;

    // При переключении на другую игру очищаем инпут парсера
    let lastId = game && game.id;
    $: if (game && game.id !== lastId) { lastId = game.id; parseUrl = ''; tab = prominent || $searchResults[game.id] ? 'name' : 'link'; }

    $: activeSources = (game.sources || []).filter(s => s.url);

    async function parse() {
        if (!parseUrl.trim() || parsing) return;
        if (await onParse(parseUrl)) parseUrl = '';
    }

    async function runCheck(fn) {
        if (checking) return;
        checking = true;
        try {
            const g = await fn();
            await onReload();
            showToast(g && g.update_available ? tr('toast.update_found', { v: g.update_version }) : tr('toast.update_none'), 'success');
        } catch (err) { showToast(tr('toast.update_fail', { err }), 'error'); }
        finally { checking = false; }
    }
    const checkGame = () => runCheck(() => CheckGameUpdates(game.id));
    const checkSource = (platform) => runCheck(() => CheckSourceUpdate(game.id, platform));
</script>

<div class="metadata-panel rounded-2xl border p-5 {prominent ? 'bg-slate-900/60 border-white/10 p-6 mb-9' : 'bg-slate-900/60 border-white/5'}">
    <div class="metadata-header mb-5">
        {#if prominent}
            <h3 class="metadata-title text-xl font-bold text-white">{$t('meta.find_title')}</h3>
        {:else}
            <div class="metadata-heading flex items-center gap-2 relative">
                <h3 class="metadata-label text-xs font-bold text-slate-400 uppercase tracking-wider">{$t("meta.title")}</h3>
                <button on:click={() => showSources = !showSources} title={$t("meta.sources")}
                        class="shrink-0 w-5 h-5 rounded-full flex items-center justify-center text-[11px] bg-white/5 hover:bg-white/10 text-slate-300 transition-colors">?</button>
                {#if showSources}
                    <div class="absolute left-0 top-7 z-20 w-64 glass-strong rounded-xl shadow-2xl p-3 animate-fade-in" use:clickOutside on:clickoutside={() => showSources = false}>
                        <div class="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2">{$t("meta.sources")}</div>
                        <div class="flex flex-col gap-1.5">
                            {#each supportedSources as s}
                                <div class="flex items-center justify-between gap-2 text-sm">
                                    <span class="text-slate-200">{s.name}</span>
                                    <span class="text-[11px] text-slate-500 font-mono truncate">{s.domain}</span>
                                </div>
                            {/each}
                        </div>
                        <p class="text-[11px] text-slate-500 mt-2 pt-2 border-t border-white/10">{$t("meta.sources_hint")}</p>
                    </div>
                {/if}
            </div>
        {/if}
        <div class="metadata-tabs flex gap-1 p-1 rounded-xl bg-slate-950" role="group" aria-label={$t('meta.find_title')}>
            <button data-find-tab="name" aria-pressed={tab === 'name'} on:click={() => tab = 'name'}
                    class="px-3 py-2 text-sm rounded-lg transition {tab === 'name' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-white'}">{$t('find.tab_name')}</button>
            <button data-find-tab="link" aria-pressed={tab === 'link'} on:click={() => tab = 'link'}
                    class="px-3 py-2 text-sm rounded-lg transition {tab === 'link' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-white'}">{$t('find.tab_link')}</button>
        </div>
    </div>
    {#if activeSources.length}
        <div class="flex flex-wrap gap-2 mb-3">
            {#each activeSources as s}
                <span class="flex items-center rounded-full bg-white/5 ring-1 ring-white/10 overflow-hidden text-sm">
                    <button on:click={() => OpenURL(s.url)} title={s.url}
                            class="px-3 py-1 text-slate-200 hover:bg-white/10 transition-colors">
                        {#if s.source === game.primary_source}<span class="text-amber-400" title={$t("edit.primary_source")}>★</span> {/if}{s.source} ↗
                    </button>
                    <button on:click={() => { parseUrl = s.url; tab = 'link'; }} title={$t("meta.use_for_parse")}
                            class="px-2 py-1 text-slate-400 hover:bg-white/10 hover:text-indigo-300 border-l border-white/10 transition-colors"><Icon name="download" size={13}/></button>
                    <button on:click={() => checkSource(s.source)} disabled={checking} title={$t("meta.check_source")}
                            class="px-2 py-1 text-slate-400 hover:bg-white/10 hover:text-amber-300 border-l border-white/10 transition-colors disabled:opacity-50"><Icon name="refresh" size={13}/></button>
                </span>
            {/each}
        </div>
        <button on:click={checkGame} disabled={checking}
                class="mb-3 text-sm font-semibold text-amber-300 hover:text-amber-200 disabled:opacity-50">
            {checking ? $t('meta.checking') : $t('meta.check_updates')}
        </button>
    {/if}
    <div class:hidden={tab !== 'name'}><FindByName {game} {onPreview}/></div>
    <div class:hidden={tab !== 'link'}>
    <div class="flex gap-3">
        <input type="text" bind:value={parseUrl} placeholder={$t("meta.url_ph")}
               on:keydown={(e) => { if (e.key === 'Enter') parse(); }}
               class="flex-1 min-w-0 bg-slate-950 border border-white/10 text-white rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 transition-all"/>
        <button on:click={parse} disabled={parsing || !parseUrl} class="bg-indigo-500 hover:bg-indigo-400 text-white font-semibold py-2.5 px-6 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
            {parsing ? $t('meta.downloading') : prominent ? $t('meta.get_info') : $t('meta.update')}
        </button>
    </div>
    </div>
    {#if prominent}
        <button data-find-dismiss on:click={onDismiss} class="mt-4 text-xs text-slate-400 hover:text-white underline underline-offset-4">{$t('find.dismiss')}</button>
    {/if}
</div>

<style>
    .metadata-panel { container-type: inline-size; }
    .metadata-header { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px; }
    .metadata-heading { min-width: 0; }
    .metadata-tabs button { white-space: nowrap; }
    /* Card width changes independently of the window when the detail columns wrap. */
    @container (max-width: 600px) {
        .metadata-header { gap: 8px; }
        .metadata-title { font-size: 16px; line-height: 24px; }
        .metadata-label { font-size: 11px; letter-spacing: 0; }
        .metadata-tabs { gap: 2px; padding: 3px; }
        .metadata-tabs button { font-size: 12px; padding-left: 8px; padding-right: 8px; }
    }
</style>
