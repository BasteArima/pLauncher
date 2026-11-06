<script>
    import { onMount, onDestroy } from 'svelte';
    import { t, tr, langStore } from '../i18n.js';
    import { SuggestSearchName, SearchGameInfo, OpenURL } from '../../wailsjs/go/main/App.js';
    import { lsJSON, lsSet, mediaSrc } from '../lib/util.js';
    import { showToast } from '../lib/ui.js';
    import { coverBlur, blurCls } from '../lib/privacy.js';
    import Icon from './Icon.svelte';

    export let game;
    export let onPreview = async (hit, version) => {};
    const sites = ['F95zone', 'Steam', 'VNDB'];
    const stored = lsJSON('plauncher_find_sources', sites);
    let sources = Array.isArray(stored) ? sites.filter(s => stored.includes(s)) : sites;
    let query = '';
    let version = '';
    let searchedQuery = '';
    const normVersion = value => (value || '').toLowerCase().trim().replace(/^v/, '');
    const normTitle = value => (value || '').normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().replace(/[^\p{L}\p{N}]+/gu, ' ').trim().replace(/^(the|a) /, '');
    function reason(hit) {
        if (version && normVersion(version) === normVersion(hit.version)) return 'version';
        if (normTitle(searchedQuery) === normTitle(hit.title)) return 'title';
        return hit.score < 0.5 ? 'different' : '';
    }
    function updated(value, lang) {
        if (!value) return '';
        const date = new Date(typeof value === 'number' ? value * 1000 : value);
        return Number.isNaN(date.getTime()) ? '' : tr('find.updated', { date: new Intl.DateTimeFormat(lang, { month: 'short', day: 'numeric' }).format(date) });
    }
    let touched = false;
    let searching = false;
    let choosing = '';
    let result = null;
    let alive = true;
    let request = 0;
    $: lsSet('plauncher_find_sources', JSON.stringify(sources));

    onMount(async () => {
        try {
            const suggestion = await SuggestSearchName(game.id);
            if (!alive) return;
            if (!touched) query = suggestion?.title || game.title || '';
            version = suggestion?.version || '';
        } catch (err) {
            if (alive) { if (!touched) query = game.title || ''; showToast(tr('toast.error', { err }), 'error'); }
        }
    });
    onDestroy(() => { alive = false; request++; });

    function toggle(source) {
        sources = sources.includes(source) ? sources.filter(s => s !== source) : [...sources, source];
    }
    async function search() {
        if (!query.trim() || !sources.length || searching || choosing) return;
        const ticket = ++request;
        searching = true;
        searchedQuery = query.trim();
        result = null;
        try {
            const found = await SearchGameInfo(query.trim(), sources, game.id);
            if (alive && request === ticket) result = { hits: found?.hits || [], failed: found?.failed || [] };
        } catch (err) {
            if (alive && request === ticket) showToast(tr('toast.error', { err }), 'error');
        } finally { if (alive && request === ticket) searching = false; }
    }
    async function openSource(url) {
        try { await OpenURL(url); }
        catch (err) { if (alive) showToast(tr('toast.error', { err }), 'error'); }
    }
    async function choose(hit) {
        if (choosing) return;
        choosing = hit.url;
        try { await onPreview(hit, version); }
        finally { if (alive) choosing = ''; }
    }
</script>

<div data-find-by-name>
    <form on:submit|preventDefault={search} class="flex gap-2">
        <input data-find-query aria-label={$t('find.name')} type="text" bind:value={query} on:input={() => touched = true}
               placeholder={$t('find.name')} disabled={searching || !!choosing}
               class="min-w-0 flex-1 bg-slate-950 border border-white/10 text-white rounded-lg px-3 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 disabled:opacity-60"/>
        <button data-find-search type="submit" disabled={!query.trim() || !sources.length || searching || !!choosing}
                class="flex items-center gap-2 px-4 py-2.5 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-sm font-semibold text-white disabled:opacity-50 disabled:cursor-not-allowed">
            <Icon name={searching ? 'refresh' : 'search'} size={16} class={searching ? 'animate-spin' : ''}/>
            {searching ? $t('find.searching') : $t('find.search')}
        </button>
    </form>
    <div class="flex flex-wrap items-center gap-2 mt-3" role="group" aria-label={$t('find.sources')}>
        <span class="text-xs text-slate-400">{$t('find.search_on')}</span>
        {#each sites as site}
            <button type="button" data-find-source={site} aria-pressed={sources.includes(site)} on:click={() => toggle(site)} disabled={searching || !!choosing}
                    class="flex items-center gap-1.5 px-3 py-1 rounded-full border text-xs transition-colors disabled:opacity-60 {sources.includes(site) ? 'border-transparent bg-indigo-900 text-indigo-100' : 'border-white/10 text-slate-400 hover:text-white'}">
                {site}
            </button>
        {/each}
        <span class="text-xs text-slate-500">{$t('find.link_only')}</span>
    </div>
    {#if !sources.length}<p class="text-xs text-slate-400 mt-2">{$t('find.select_source')}</p>{/if}
    <p class="text-xs text-slate-400 mt-4" data-find-summary>{version ? $t('find.folder_version', { version }) : $t('find.folder_name')}{#if result} · {$t('find.matches', { count: result.hits.length })}{/if}</p>
    <div aria-live="polite" aria-busy={searching || !!choosing}>
        {#if searching}
            <p class="text-sm text-slate-400 mt-5 flex items-center gap-2"><Icon name="refresh" size={16} class="animate-spin"/>{$t('find.searching')}</p>
        {:else if result}
            <div class="mt-2 flex flex-col gap-2" data-find-results>
                {#each result.hits as hit (hit.url)}
                    <div data-find-card class="group flex flex-wrap sm:flex-nowrap items-center gap-4 p-3 border rounded-xl {hit.best ? 'border-indigo-500/30 bg-slate-800/50' : 'border-white/10'}">
                        <div class="w-[136px] aspect-video shrink-0 rounded-lg overflow-hidden bg-slate-950 flex items-center justify-center text-slate-600">
                            {#if hit.thumb}<img src={mediaSrc(hit.thumb)} alt="" loading="lazy" decoding="async" on:error={e => e.target.style.visibility = 'hidden'} class="w-full h-full object-cover transition {blurCls($coverBlur, 'cover')}"/>
                            {:else}<Icon name="image" size={24}/>{/if}
                        </div>
                        <div class="min-w-0 flex-1">

                            <div class="text-sm font-semibold text-white break-words">{hit.title}</div>
                            <div class="text-xs text-slate-400 mt-1 break-words">{[hit.source, hit.version, hit.author, hit.engine, updated(hit.updated_at, $langStore)].filter(Boolean).join(' · ')}</div>
                            {#if reason(hit)}<p data-find-reason class="text-xs mt-1 {reason(hit) === 'different' ? 'text-slate-400' : 'text-emerald-400'}">{$t('find.reason_' + reason(hit), { version })}</p>{/if}
                        </div>
                        <div class="flex items-center gap-2 shrink-0">
                            <button data-find-best={hit.best || undefined} data-hit-url={hit.url} on:click={() => choose(hit)} disabled={!!choosing}
                                    class="max-w-[120px] px-3 py-2 rounded-lg text-xs font-semibold border {hit.best ? 'bg-white text-slate-950 border-white hover:bg-slate-200' : 'text-slate-200 border-white/10 hover:bg-white/5'} disabled:opacity-50">
                                {choosing === hit.url ? $t('find.loading_preview') : $t('find.use')}
                            </button>
                            <button type="button" data-hit-source-url={hit.url} on:click={() => openSource(hit.url)}
                                    title={`${$t('detail.update_open')} — ${hit.source}`} aria-label={`${$t('detail.update_open')} — ${hit.source}`}
                                    class="w-9 h-9 flex items-center justify-center rounded-lg text-slate-400 border border-white/10 hover:bg-white/5 hover:text-white transition-colors">
                                <Icon name="external" size={16}/>
                            </button>
                        </div>
                    </div>
                {:else}
                    <p data-find-empty class="text-sm text-slate-400 py-4">{$t('find.empty')}</p>
                {/each}
            </div>
            {#if result.failed.length}
                <p data-find-failed class="text-xs text-slate-500 mt-3 flex items-start gap-2"><Icon name="alert" size={14} class="shrink-0"/>{$t('find.failed', { sites: result.failed.map(f => f.source).join(', ') })}</p>
            {/if}
        {/if}
    </div>
</div>
