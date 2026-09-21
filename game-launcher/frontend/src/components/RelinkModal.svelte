<script>
    // Окно перепривязки перемещённых игр: список игр без папки на диске,
    // автоматически найденные новые места и ручной выбор папки.
    import { onMount } from 'svelte';
    import { t, tr } from '../i18n.js';
    import { showToast as notify } from '../lib/ui.js';
    import { mediaSrc } from '../lib/util.js';
    import { FindMissingGames, RelinkGame, RelinkAllFound, SelectRelinkFolder, RemoveGame } from '../../wailsjs/go/main/App.js';

    export let onClose = () => {};
    export let onChanged = async () => {};             // перезагрузить библиотеку
    export let discreet = false;

    let items = [];
    let loading = true;
    let busy = false;
    let confirmDeleteId = '';                          // двухкликовое подтверждение удаления

    $: foundCount = items.filter(m => m.candidate).length;

    async function refresh() {
        loading = true;
        try { items = await FindMissingGames() || []; }
        catch (err) { notify(tr('toast.error', { err }), 'error'); }
        finally { loading = false; }
    }

    async function relink(m, path) {
        busy = true;
        try {
            await RelinkGame(m.id, path);
            items = items.filter(x => x.id !== m.id);
            notify(tr('missing.relinked', { title: m.title }), 'success');
            await onChanged();
        } catch (err) { notify(tr('toast.error', { err }), 'error'); }
        finally { busy = false; }
    }

    async function pickFolder(m) {
        try {
            const p = await SelectRelinkFolder(m.old_path);
            if (p) await relink(m, p);
        } catch (err) { notify(tr('toast.error', { err }), 'error'); }
    }

    async function relinkAll() {
        busy = true;
        try {
            const n = await RelinkAllFound();
            notify(tr('missing.relinked_n', { n }), 'success');
            await onChanged();
            await refresh();
        } catch (err) { notify(tr('toast.error', { err }), 'error'); }
        finally { busy = false; }
    }

    async function remove(m) {
        if (confirmDeleteId !== m.id) { confirmDeleteId = m.id; return; }
        confirmDeleteId = '';
        busy = true;
        try {
            await RemoveGame(m.id);
            items = items.filter(x => x.id !== m.id);
            await onChanged();
        } catch (err) { notify(tr('toast.error', { err }), 'error'); }
        finally { busy = false; }
    }

    onMount(refresh);
</script>

<div class="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={onClose}>
    <div class="w-[720px] max-w-[94vw] max-h-[85vh] flex flex-col glass-strong rounded-2xl shadow-2xl p-7" on:click|stopPropagation>
        <div class="flex items-center justify-between mb-1">
            <h2 class="text-2xl font-black text-white">{$t('missing.title')}</h2>
            <button on:click={onClose} class="text-slate-400 hover:text-white text-3xl leading-none">&times;</button>
        </div>
        <p class="text-sm text-slate-400 mb-5">{$t('missing.hint')}</p>

        {#if loading}
            <p class="text-slate-500 text-sm py-10 text-center animate-pulse">{$t('missing.searching')}</p>
        {:else if items.length === 0}
            <div class="py-10 text-center">
                <div class="text-5xl mb-3 opacity-40">✓</div>
                <p class="text-slate-400">{$t('missing.none')}</p>
            </div>
        {:else}
            <div class="flex items-center gap-3 mb-3">
                <span class="text-xs text-slate-500">{$t('missing.summary', { n: items.length, found: foundCount })}</span>
                <button on:click={refresh} disabled={busy} class="ml-auto text-xs text-slate-400 hover:text-white bg-white/5 hover:bg-white/10 px-3 py-1.5 rounded-lg border border-white/10 transition-colors disabled:opacity-50">↻ {$t('missing.rescan')}</button>
                {#if foundCount}
                    <button on:click={relinkAll} disabled={busy}
                            class="text-sm font-bold text-white bg-indigo-600 hover:bg-indigo-500 px-4 py-1.5 rounded-lg transition-colors disabled:opacity-50">
                        {$t('missing.relink_all', { n: foundCount })}
                    </button>
                {/if}
            </div>

            <div class="flex-1 overflow-y-auto -mx-2 px-2 space-y-2">
                {#each items as m (m.id)}
                    <div class="glass rounded-xl p-3 flex gap-3">
                        <div class="w-12 h-16 rounded-md overflow-hidden bg-slate-800 ring-1 ring-white/10 shrink-0 flex items-center justify-center">
                            {#if m.cover_path}
                                <img src={mediaSrc(m.cover_path)} alt="" class="w-full h-full object-cover {discreet ? 'blur-md' : ''}" on:error={(e) => e.target.style.display = 'none'}/>
                            {:else}
                                <span class="text-slate-500 uppercase">{(m.title || '?').slice(0, 1)}</span>
                            {/if}
                        </div>
                        <div class="flex-1 min-w-0">
                            <div class="font-bold text-white truncate">{m.title}</div>
                            <div class="text-[11px] font-mono text-slate-500 line-through truncate" title={m.old_path}>{m.old_path}</div>
                            {#if m.candidate}
                                <div class="text-[11px] font-mono text-emerald-300 truncate" title={m.candidate}>→ {m.candidate}</div>
                                {#if m.duplicate_id}
                                    <div class="text-[11px] text-amber-300/90 mt-0.5">{tr('missing.will_merge', { title: m.duplicate_of })}</div>
                                {/if}
                            {:else}
                                <div class="text-[11px] text-slate-400 italic">{$t('missing.not_found')}</div>
                            {/if}
                        </div>
                        <div class="flex flex-col gap-1.5 shrink-0 justify-center">
                            {#if m.candidate}
                                <button on:click={() => relink(m, m.candidate)} disabled={busy}
                                        class="text-xs font-bold text-white bg-emerald-600 hover:bg-emerald-500 px-3 py-1.5 rounded-lg transition-colors disabled:opacity-50">{$t('missing.relink')}</button>
                            {/if}
                            <button on:click={() => pickFolder(m)} disabled={busy}
                                    class="text-xs font-semibold text-slate-200 bg-white/5 hover:bg-white/10 px-3 py-1.5 rounded-lg border border-white/10 transition-colors disabled:opacity-50">📁 {$t('missing.pick')}</button>
                            <button on:click={() => remove(m)} disabled={busy}
                                    class="text-xs font-semibold px-3 py-1.5 rounded-lg transition-colors disabled:opacity-50 {confirmDeleteId === m.id ? 'bg-red-600 hover:bg-red-500 text-white' : 'text-rose-300 hover:bg-rose-500/20'}">
                                {confirmDeleteId === m.id ? $t('missing.confirm_remove') : $t('btn.remove')}
                            </button>
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
</div>
