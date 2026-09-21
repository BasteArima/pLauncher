<script>
    // Настройка разделов главной: состав, порядок (drag), тип полки.
    import { t } from '../i18n.js';
    import { newShelfId } from '../lib/shelves.js';

    export let shelves = [];
    export let allTags = [];
    export let collections = [];
    export let countFor = (s) => 0;
    export let onDone = () => {};

    function add() { shelves = [...shelves, { id: newShelfId(), type: 'continue' }]; }
    function remove(id) { shelves = shelves.filter(s => s.id !== id); }
    function patch(id, changes) { shelves = shelves.map(s => s.id === id ? { ...s, ...changes } : s); }
    function setType(id, type) {
        const s = shelves.find(x => x.id === id);
        patch(id, {
            type,
            tag: type === 'tag' ? (s.tag || allTags[0] || '') : undefined,
            collectionId: type === 'collection' ? (s.collectionId || (collections[0] && collections[0].id) || '') : undefined,
        });
    }

    // drag-reorder
    let dragIndex = null;
    function onDrop(i) {
        if (dragIndex === null || dragIndex === i) { dragIndex = null; return; }
        const arr = [...shelves];
        const [moved] = arr.splice(dragIndex, 1);
        arr.splice(i, 0, moved);
        shelves = arr;
        dragIndex = null;
    }

    const selectCls = 'bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40';
</script>

<div class="animate-fade-in">
    <div class="flex items-center justify-between mb-4">
        <div>
            <h2 class="text-2xl font-black text-white">{$t("layout.title")}</h2>
            <p class="text-sm text-slate-400 mt-0.5">{$t("layout.hint")}</p>
        </div>
        <button on:click={onDone} class="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold px-6 py-2.5 rounded-xl transition-colors">{$t("btn.done")}</button>
    </div>
    <div class="space-y-2">
        {#each shelves as s, i (s.id)}
            <div draggable="true"
                 on:dragstart={() => dragIndex = i}
                 on:dragover|preventDefault
                 on:drop={() => onDrop(i)}
                 class="flex items-center gap-3 glass rounded-xl px-3 py-3 cursor-move transition-all {dragIndex === i ? 'ring-2 ring-indigo-400/60 opacity-60' : ''}">
                <span class="text-slate-500 text-xl select-none">⋮⋮</span>
                <select value={s.type} on:change={(e) => setType(s.id, e.target.value)} class={selectCls}>
                    <option value="continue">{$t("shelf.continue")}</option>
                    <option value="added">{$t("shelf.added")}</option>
                    <option value="favorites">{$t("shelf.favorites")}</option>
                    <option value="all">{$t("shelf.all")}</option>
                    <option value="collection">{$t("layout.opt.collection")}</option>
                    <option value="tag">{$t("layout.opt.tag")}</option>
                </select>
                {#if s.type === 'tag'}
                    <select value={s.tag} on:change={(e) => patch(s.id, { tag: e.target.value })} class={selectCls}>
                        {#each allTags as tag}<option value={tag}>{tag}</option>{/each}
                        {#if !allTags.length}<option value="">{$t("layout.no_tags")}</option>{/if}
                    </select>
                {/if}
                {#if s.type === 'collection'}
                    <select value={s.collectionId} on:change={(e) => patch(s.id, { collectionId: e.target.value })} class={selectCls}>
                        {#each collections as c}<option value={c.id}>{c.name}</option>{/each}
                        {#if !collections.length}<option value="">{$t("layout.no_collections")}</option>{/if}
                    </select>
                {/if}
                <span class="ml-auto text-xs text-slate-500">{$t("layout.games_n", { n: countFor(s) })}</span>
                <button on:click={() => remove(s.id)} class="text-slate-400 hover:text-red-400 transition-colors" title={$t("layout.remove")}>🗑️</button>
            </div>
        {/each}
    </div>
    <button on:click={add} class="mt-3 w-full border-2 border-dashed border-white/15 hover:border-indigo-500 text-slate-400 hover:text-indigo-300 py-2.5 rounded-xl transition-colors font-semibold">{$t("layout.add")}</button>
</div>
