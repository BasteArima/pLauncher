<script>
    import { t, tr } from '../i18n.js';
    import { OpenURL } from '../../wailsjs/go/main/App.js';
    import { mediaSrc } from '../lib/util.js';
    import { coverBlur, blurCls } from '../lib/privacy.js';
    import { showToast } from '../lib/ui.js';
    import Icon from './Icon.svelte';
    export let candidates = [];
    export let choosing = 0;
    export let onChoose = async () => {};
    async function open(url) {
        try { await OpenURL(url); } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
</script>

<div data-collection-results class="mt-4" aria-live="polite" aria-busy={!!choosing}>
    <p class="text-xs text-slate-400 mb-2">{$t('collection.choose', { n: candidates.length })}</p>
    <div class="flex flex-col gap-2">
        {#each candidates as hit (hit.index)}
            <div data-collection-card={hit.index} class="group collection-row flex items-center gap-4 p-3 border rounded-xl {hit.best ? 'border-indigo-500/30 bg-slate-800/50' : 'border-white/10'}">
                <div class="collection-thumb w-[136px] aspect-video shrink-0 rounded-lg overflow-hidden bg-slate-950 flex items-center justify-center text-slate-600">
                    {#if hit.thumb}<img src={mediaSrc(hit.thumb)} alt="" loading="lazy" decoding="async" on:error={e => e.target.style.visibility = 'hidden'} class="w-full h-full object-cover transition {blurCls($coverBlur, 'cover')}"/>
                    {:else}<Icon name="image" size={24}/>{/if}
                </div>
                <div class="min-w-0 flex-1">
                    <div data-collection-title class="text-sm font-semibold text-white break-words">{hit.fallback_title ? $t('collection.game_number', { n: hit.index }) : hit.title}</div>
                    <div class="text-xs text-slate-400 mt-1 break-words">{[hit.version, hit.year, hit.fallback_title ? '' : hit.author].filter(Boolean).join(' · ')}</div>
                    {#if hit.fallback_title && hit.description}<p class="text-xs text-slate-400 mt-1 line-clamp-2">{hit.description}</p>{/if}
                    {#if hit.best}<p data-collection-reason class="text-xs text-emerald-400 mt-1">{$t('collection.folder_match')}</p>{/if}
                </div>
                <div class="collection-actions flex items-center gap-2 shrink-0">
                    <button data-collection-use={hit.index} data-hit-url={hit.url} data-collection-best={hit.best || undefined} on:click={() => onChoose(hit)} disabled={!!choosing}
                            class="max-w-[120px] px-3 py-2 rounded-lg text-xs font-semibold border {hit.best ? 'bg-white text-slate-950 border-white hover:bg-slate-200' : 'text-slate-200 border-white/10 hover:bg-white/5'} disabled:opacity-50">
                        {choosing === hit.index ? $t('find.loading_preview') : $t('collection.use')}
                    </button>
                    <button data-collection-open={hit.index} on:click={() => open(hit.url)} title={$t('detail.update_open')} aria-label={$t('detail.update_open')}
                            class="w-9 h-9 flex items-center justify-center rounded-lg text-slate-400 border border-white/10 hover:bg-white/5 hover:text-white"><Icon name="external" size={16}/></button>
                </div>
            </div>
        {:else}<p class="text-sm text-slate-400 py-4">{$t('collection.empty')}</p>{/each}
    </div>
</div>

<style>
    @container (max-width: 560px) {
        .collection-row { flex-wrap: wrap; gap: 12px; }
        .collection-actions { width: 100%; justify-content: flex-end; }
        .collection-thumb { width: 120px; }
    }
</style>
