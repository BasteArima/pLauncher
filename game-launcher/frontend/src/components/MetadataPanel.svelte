<script>
    // Блок «Метаданные» на странице игры: ссылки по площадкам, проверка обновлений
    // и загрузка данных по ссылке через парсер.
    import { t, tr } from '../i18n.js';
    import { showToast } from '../lib/ui.js';
    import { clickOutside } from '../lib/util.js';
    import { OpenURL, CheckGameUpdates, CheckSourceUpdate } from '../../wailsjs/go/main/App.js';

    export let game;
    export let supportedSources = [];
    export let parsing = false;
    export let onParse = async (url) => {};             // распарсить ссылку в эту игру
    export let onReload = async () => {};

    let parseUrl = '';
    let showSources = false;
    let checking = false;

    // При переключении на другую игру очищаем инпут парсера
    let lastId = game && game.id;
    $: if (game && game.id !== lastId) { lastId = game.id; parseUrl = ''; }

    $: activeSources = (game.sources || []).filter(s => s.url);

    async function parse() {
        if (!parseUrl) return;
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

<div class="glass p-5 rounded-xl">
    <div class="flex items-center gap-2 mb-3 relative">
        <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider">{$t("meta.title")}</h3>
        <button on:click={() => showSources = !showSources} title={$t("meta.sources")}
                class="w-5 h-5 rounded-full flex items-center justify-center text-[11px] bg-white/5 hover:bg-white/10 text-slate-300 transition-colors">?</button>
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
    {#if activeSources.length}
        <div class="flex flex-wrap gap-2 mb-3">
            {#each activeSources as s}
                <span class="flex items-center rounded-full bg-white/5 ring-1 ring-white/10 overflow-hidden text-sm">
                    <button on:click={() => OpenURL(s.url)} title={s.url}
                            class="px-3 py-1 text-slate-200 hover:bg-white/10 transition-colors">
                        {#if s.source === game.primary_source}<span class="text-amber-400" title={$t("edit.primary_source")}>★</span> {/if}{s.source} ↗
                    </button>
                    <button on:click={() => parseUrl = s.url} title={$t("meta.use_for_parse")}
                            class="px-2 py-1 text-slate-400 hover:bg-white/10 hover:text-indigo-300 border-l border-white/10 transition-colors">↧</button>
                    <button on:click={() => checkSource(s.source)} disabled={checking} title={$t("meta.check_source")}
                            class="px-2 py-1 text-slate-400 hover:bg-white/10 hover:text-amber-300 border-l border-white/10 transition-colors disabled:opacity-50">↻</button>
                </span>
            {/each}
        </div>
        <button on:click={checkGame} disabled={checking}
                class="mb-3 text-sm font-semibold text-amber-300 hover:text-amber-200 disabled:opacity-50">
            {checking ? $t('meta.checking') : $t('meta.check_updates')}
        </button>
    {/if}
    <div class="flex gap-3">
        <input type="text" bind:value={parseUrl} placeholder={$t("meta.url_ph")}
               on:keydown={(e) => { if (e.key === 'Enter') parse(); }}
               class="flex-1 bg-slate-900/60 border border-white/10 text-white rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 transition-all"/>
        <button on:click={parse} disabled={parsing || !parseUrl} class="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold py-2.5 px-6 rounded-lg shadow-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed">
            {parsing ? $t('meta.downloading') : $t('meta.update')}
        </button>
    </div>
</div>
