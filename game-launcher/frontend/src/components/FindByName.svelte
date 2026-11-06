<script>
    import { onMount, onDestroy } from 'svelte';
    import { t, tr } from '../i18n.js';
    import { SuggestSearchName, SearchGameInfo } from '../../wailsjs/go/main/App.js';
    import { lsJSON, lsSet, mediaSrc } from '../lib/util.js';
    import { showToast } from '../lib/ui.js';
    import { coverBlur, blurCls } from '../lib/privacy.js';
    import Icon from './Icon.svelte';

    export let game;
    export let onPreview = async (hit, version) => {};
    const sites = ['Steam', 'F95zone', 'VNDB'];
    const stored = lsJSON('plauncher_find_sources', sites);
    let sources = Array.isArray(stored) ? sites.filter(s => stored.includes(s)) : sites;
    let query = '';
    let version = '';
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
        result = null;
        try {
            const found = await SearchGameInfo(query.trim(), sources, game.id);
            if (alive && request === ticket) result = { hits: found?.hits || [], failed: found?.failed || [] };
        } catch (err) {
            if (alive && request === ticket) showToast(tr('toast.error', { err }), 'error');
        } finally { if (alive && request === ticket) searching = false; }
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
    {#if version}<p class="text-xs text-slate-400 mt-2">{$t('find.folder_version', { version })}</p>{/if}
    <div class="flex flex-wrap items-center gap-2 mt-3" role="group" aria-label={$t('find.sources')}>
        {#each sites as site}
            <button type="button" data-find-source={site} aria-pressed={sources.includes(site)} on:click={() => toggle(site)} disabled={searching || !!choosing}
                    class="flex items-center gap-1.5 px-3 py-1 rounded-full border text-xs transition-colors disabled:opacity-60 {sources.includes(site) ? 'border-indigo-500/50 bg-indigo-900/50 text-indigo-100' : 'border-white/10 text-slate-400 hover:text-white'}">
                {#if sources.includes(site)}<Icon name="check" size={12}/>{/if}{site}
            </button>
        {/each}
    </div>
    {#if !sources.length}<p class="text-xs text-slate-400 mt-2">{$t('find.select_source')}</p>{/if}
    <p class="text-xs text-slate-500 mt-3 leading-relaxed">{$t('find.link_only')}</p>
    <div aria-live="polite" aria-busy={searching || !!choosing}>
        {#if searching}
            <p class="text-sm text-slate-400 mt-5 flex items-center gap-2"><Icon name="refresh" size={16} class="animate-spin"/>{$t('find.searching')}</p>
        {:else if result}
            <div class="mt-4 divide-y divide-white/5" data-find-results>
                {#each result.hits as hit (hit.url)}
                    <div class="group flex items-center gap-3 py-3">
                        <div class="w-16 h-20 shrink-0 rounded-lg overflow-hidden bg-slate-950 flex items-center justify-center text-slate-600">
                            {#if hit.thumb}<img src={mediaSrc(hit.thumb)} alt="" loading="lazy" decoding="async" on:error={e => e.target.style.visibility = 'hidden'} class="w-full h-full object-cover transition {blurCls($coverBlur, 'cover')}"/>
                            {:else}<Icon name="image" size={24}/>{/if}
                        </div>
                        <div class="min-w-0 flex-1">
                            {#if hit.best}<div data-find-best class="text-xs text-indigo-300 mb-1 flex items-center gap-1"><Icon name="check" size={13}/>{$t('find.best')}</div>{/if}
                            <div class="text-sm font-semibold text-white break-words">{hit.title}</div>
                            <div class="text-xs text-slate-400 mt-1 break-words">{[hit.source, hit.author, hit.version, hit.year].filter(Boolean).join(' · ')}</div>
                        </div>
                        <button data-hit-url={hit.url} on:click={() => choose(hit)} disabled={!!choosing}
                                class="shrink-0 max-w-[120px] px-3 py-2 rounded-lg text-xs font-semibold text-indigo-100 border border-indigo-500/40 hover:bg-indigo-900/60 disabled:opacity-50">
                            {choosing === hit.url ? $t('find.loading_preview') : $t('find.use')}
                        </button>
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
