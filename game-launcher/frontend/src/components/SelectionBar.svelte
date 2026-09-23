<script>
    // Нижняя панель массовых действий (видна, когда выделена хотя бы одна игра).
    import { t } from '../i18n.js';
    import { clickOutside } from '../lib/util.js';
    import { selection, clearSelection, selectAllVisible } from '../lib/view.js';
    import { bulkProgress, cancelBulk, bulkFavorite, bulkDetectLaunch, bulkRemove, bulkIgnore, bulkCheckUpdates, bulkReparse } from '../lib/bulk.js';

    export let games = [];                           // видимые игры (для объектов выделения)
    export let collections = [];                     // видимые коллекции
    export let onAddToCollection = (colId, ids) => {};
    export let onRemoveFromCollection = (colId, ids) => {};
    export let reload = async () => {};
    export let left = 0;                             // ширина сайдбара — центрируем над библиотекой

    let menu = '';                                   // '' | add | remove

    $: ids = [...$selection];
    $: picked = games.filter(g => $selection.has(g.id));
    $: allFav = picked.length > 0 && picked.every(g => g.favorite);
    $: removable = collections.filter(c => c.type === 'manual' && (c.game_ids || []).some(id => $selection.has(id)));
    $: busy = !!$bulkProgress;

    $: none = ids.length === 0;
    const btn = 'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors disabled:opacity-40 disabled:pointer-events-none';
</script>

<!-- Центрируем flex-ом: transform занят анимацией появления -->
<div class="fixed bottom-6 right-0 z-[55] flex justify-center pointer-events-none px-4" style="left:{left}px">
    <div class="pointer-events-auto animate-rise-in glass-strong rounded-2xl shadow-2xl ring-1 ring-indigo-400/30 px-3 py-2 flex items-center gap-1.5 flex-wrap justify-center max-w-full">
        {#if busy}
            <span class="text-sm text-slate-200 px-2">
                {$bulkProgress.label} {$bulkProgress.done}/{$bulkProgress.total}
            </span>
            <div class="w-40 h-1.5 rounded-full bg-white/10 overflow-hidden">
                <div class="h-full bg-indigo-400 transition-all" style="width:{($bulkProgress.done / Math.max(1, $bulkProgress.total)) * 100}%"></div>
            </div>
            <button on:click={cancelBulk} class="{btn} text-slate-300 hover:bg-white/10">{$t('btn.cancel')}</button>
        {:else}
            <span class="text-sm font-bold text-white px-2">{ids.length ? $t('sel.count', { n: ids.length }) : $t('sel.pick_hint')}</span>
            <button on:click={selectAllVisible} class="{btn} text-slate-300 hover:bg-white/10" title="Ctrl+A">{$t('sel.select_all')}</button>
            <span class="w-px h-6 bg-white/10 mx-1"></span>

            <div class="relative">
                <button on:click={() => menu = menu === 'add' ? '' : 'add'} disabled={none || !collections.length} class="{btn} text-slate-200 hover:bg-white/10">＋ {$t('bulk.add_to')} ▾</button>
                {#if menu === 'add'}
                    <div class="absolute bottom-full mb-2 left-0 w-56 glass-strong rounded-xl shadow-2xl py-1.5 max-h-64 overflow-y-auto" use:clickOutside on:clickoutside={() => menu = ''}>
                        {#each collections as c}
                            <button on:click={() => { menu = ''; onAddToCollection(c.id, ids); }} class="w-full text-left px-3 py-1.5 text-sm text-slate-200 hover:bg-white/10 truncate">{c.name}</button>
                        {/each}
                    </div>
                {/if}
            </div>
            {#if removable.length}
                <div class="relative">
                    <button on:click={() => menu = menu === 'remove' ? '' : 'remove'} class="{btn} text-slate-200 hover:bg-white/10">－ {$t('bulk.remove_from')} ▾</button>
                    {#if menu === 'remove'}
                        <div class="absolute bottom-full mb-2 left-0 w-56 glass-strong rounded-xl shadow-2xl py-1.5 max-h-64 overflow-y-auto" use:clickOutside on:clickoutside={() => menu = ''}>
                            {#each removable as c}
                                <button on:click={() => { menu = ''; onRemoveFromCollection(c.id, ids); }} class="w-full text-left px-3 py-1.5 text-sm text-slate-200 hover:bg-white/10 truncate">{c.name}</button>
                            {/each}
                        </div>
                    {/if}
                </div>
            {/if}

            <button on:click={() => bulkFavorite(ids, !allFav, reload)} disabled={none} class="{btn} text-slate-200 hover:bg-white/10">
                {allFav ? '🤍 ' + $t('ctx.fav_remove') : '❤️ ' + $t('bulk.fav')}
            </button>
            <button on:click={() => bulkDetectLaunch(ids, reload)} disabled={none} class="{btn} text-slate-200 hover:bg-white/10" title={$t('bulk.detect_hint')}>🔎 {$t('bulk.detect')}</button>
            <button on:click={() => bulkCheckUpdates(picked, reload)} disabled={none} class="{btn} text-slate-200 hover:bg-white/10">⬆ {$t('bulk.check')}</button>
            <button on:click={() => bulkReparse(picked, reload)} disabled={none} class="{btn} text-slate-200 hover:bg-white/10" title={$t('bulk.reparse_hint')}>↻ {$t('bulk.reparse')}</button>
            <button on:click={() => bulkIgnore(ids, reload)} disabled={none} class="{btn} text-amber-200 hover:bg-amber-500/15" title={$t('ignore.hint')}>🚫 {$t('ignore.action')}</button>
            <button on:click={() => bulkRemove(ids, reload)} disabled={none} class="{btn} text-rose-300 hover:bg-rose-500/20" title="Delete">🗑 {$t('btn.delete')}</button>
            <span class="w-px h-6 bg-white/10 mx-1"></span>
            <button on:click={clearSelection} class="{btn} text-slate-400 hover:text-white hover:bg-white/10" title="Esc">✕</button>
        {/if}
    </div>
</div>
