<script>
    // Создание/редактирование коллекции: ручная (список игр) или динамическая (по тегам).
    import { t, tr } from '../i18n.js';
    import { showToast } from '../lib/ui.js';

    export let collection = null;                        // null — новая коллекция
    export let allTags = [];
    export let onSave = (data) => {};                    // {name, type, tags}
    export let onDelete = (col) => {};
    export let onClose = () => {};

    let name = collection ? collection.name : '';
    let type = collection ? collection.type : 'dynamic';
    let tags = collection ? [...(collection.tags || [])] : [];
    let hidden = collection ? !!collection.hidden : false;

    function toggleTag(tag) {
        tags = tags.includes(tag) ? tags.filter(x => x !== tag) : [...tags, tag];
    }
    function save() {
        const n = name.trim();
        if (!n) { showToast(tr('toast.col_name_required'), 'error'); return; }
        onSave({ name: n, type, tags, hidden });
    }
</script>

<div class="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={onClose}>
    <div class="w-[540px] max-w-[92vw] glass-strong rounded-2xl shadow-2xl p-7" on:click|stopPropagation>
        <div class="flex items-center justify-between mb-5">
            <h2 class="text-2xl font-black text-white">{collection ? $t('col.edit') : $t('col.new')}</h2>
            <button on:click={onClose} class="text-slate-400 hover:text-white text-3xl leading-none">&times;</button>
        </div>

        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("col.name")}</div>
        <input type="text" bind:value={name} placeholder={$t("col.name_ph")}
               on:keydown={(e) => { if (e.key === 'Enter') save(); }}
               class="w-full glass text-white rounded-lg px-4 py-2.5 mb-5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 placeholder:text-slate-500"/>

        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("col.type")}</div>
        <div class="grid grid-cols-2 gap-3 mb-5">
            <button on:click={() => type = 'manual'} class="text-left p-3 rounded-lg border transition-colors {type === 'manual' ? 'border-indigo-500 bg-indigo-600/15' : 'border-white/10 hover:bg-white/5'}">
                <div class="font-bold text-white text-sm mb-0.5">{$t("col.manual")}</div>
                <div class="text-xs text-slate-400">{$t("col.manual_hint")}</div>
            </button>
            <button on:click={() => type = 'dynamic'} class="text-left p-3 rounded-lg border transition-colors {type === 'dynamic' ? 'border-indigo-500 bg-indigo-600/15' : 'border-white/10 hover:bg-white/5'}">
                <div class="font-bold text-white text-sm mb-0.5">{$t("col.dynamic")}</div>
                <div class="text-xs text-slate-400">{$t("col.dynamic_hint")}</div>
            </button>
        </div>

        {#if type === 'dynamic'}
            <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("col.tags")}</div>
            {#if allTags.length}
                <div class="flex flex-wrap gap-1.5 mb-5 max-h-40 overflow-y-auto">
                    {#each allTags as tag}
                        <button on:click={() => toggleTag(tag)}
                                class="px-2.5 py-1 rounded-full text-xs transition-colors {tags.includes(tag) ? 'bg-indigo-500/30 text-white ring-1 ring-indigo-400/40' : 'bg-white/5 text-slate-300 hover:bg-white/10'}">
                            {tag}
                        </button>
                    {/each}
                </div>
            {:else}
                <p class="text-xs text-slate-500 mb-5">{$t("col.no_tags")}</p>
            {/if}
        {:else}
            <p class="text-xs text-slate-500 mb-5">{$t("col.manual_note")}</p>
        {/if}

        <label class="flex items-start gap-3 mb-5 p-3 rounded-lg border border-white/10 hover:bg-white/5 cursor-pointer">
            <input type="checkbox" bind:checked={hidden} class="mt-0.5 accent-indigo-500 w-4 h-4"/>
            <span>
                <span class="block text-sm font-bold text-white">🔒 {$t("col.hidden")}</span>
                <span class="block text-xs text-slate-400">{$t("col.hidden_hint")}</span>
            </span>
        </label>

        <div class="flex items-center gap-3">
            {#if collection}
                <button on:click={() => onDelete(collection)} class="text-red-400 hover:text-red-300 text-sm font-semibold mr-auto">{$t("btn.delete")}</button>
            {/if}
            <button on:click={onClose} class="ml-auto px-5 py-2.5 rounded-lg font-semibold text-slate-300 bg-white/5 hover:bg-white/10 border border-white/10 transition-colors">{$t("btn.cancel")}</button>
            <button on:click={save} class="px-5 py-2.5 rounded-lg font-bold text-white bg-indigo-600 hover:bg-indigo-500 transition-colors">{$t("btn.save")}</button>
        </div>
    </div>
</div>
