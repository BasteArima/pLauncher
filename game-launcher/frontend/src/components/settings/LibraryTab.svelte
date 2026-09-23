<script>
    // Библиотека: папки с играми и обслуживание (перемещённые/удалённые игры).
    import { onMount } from 'svelte';
    import { t, tr } from '../../i18n.js';
    import { showToast, askConfirm } from '../../lib/ui.js';
    import { SelectFolder, AddScanPath, RemoveScanPath, GetScanPaths, AddSingleGameManual, RemoveMissingGames, DetectLaunchFiles, RefreshSizes,
        GetIgnoredPaths, AddIgnoredPath, RemoveIgnoredPath } from '../../../wailsjs/go/main/App.js';
    import Section from './Section.svelte';
    import { btn } from './styles.js';

    export let scanPaths = [];
    export let onScan = async () => {};
    export let onReload = async (resetSelection) => {};
    export let onRelink = () => {};

    async function addScanFolder() {
        try {
            const p = await SelectFolder();
            if (!p) return;
            await AddScanPath(p);
            scanPaths = await GetScanPaths();
            showToast(tr('toast.folder_added_scan'), 'success');
            await onScan();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    async function removeScanFolder(path) {
        const ok = await askConfirm({ title: tr('dlg.remove_folder_title'), message: tr('dlg.remove_folder_msg', { path }), confirmText: tr('btn.remove') });
        if (!ok) return;
        try {
            await RemoveScanPath(path);
            scanPaths = await GetScanPaths();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    async function addSingleGame() {
        try {
            await AddSingleGameManual();
            await onReload(false);
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    // --- Игнорируемые папки ---
    let ignored = [];
    onMount(loadIgnored);
    async function loadIgnored() {
        try { ignored = await GetIgnoredPaths() || []; } catch (e) {}
    }
    async function addIgnored() {
        try {
            const p = await SelectFolder();
            if (!p) return;
            const removed = await AddIgnoredPath(p);
            await loadIgnored();
            if (removed) await onReload(false);
            showToast(removed ? tr('ignore.added_removed', { n: removed }) : tr('ignore.added'), 'success');
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    async function removeIgnored(p) {
        try {
            await RemoveIgnoredPath(p);
            await loadIgnored();
            showToast(tr('ignore.unignored'), 'success');
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

    async function detectLaunch() {
        try {
            const n = await DetectLaunchFiles([]);
            await onReload(false);
            showToast(n ? tr('bulk.launch_found', { n }) : tr('bulk.launch_none'), 'success');
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    async function refreshSizes() {
        try { await RefreshSizes(); showToast(tr('settings.sizes_started'), 'success'); }
        catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    async function removeMissing() {
        const ok = await askConfirm({ title: tr('dlg.remove_missing_title'), message: tr('dlg.remove_missing_msg'), confirmText: tr('btn.remove'), danger: true });
        if (!ok) return;
        try {
            const n = await RemoveMissingGames();
            if (n > 0) {
                await onReload(true);
                showToast(tr('toast.removed_missing_n', { n }), 'success');
            } else {
                showToast(tr('toast.removed_missing_none'), 'success');
            }
        } catch (err) { showToast(tr('toast.remove_missing_fail', { err }), 'error'); }
    }
</script>

<Section title={$t('settings.games_folders')}>
    <div class="flex flex-col gap-2 mb-2">
        {#each scanPaths as p}
            <div class="flex items-center gap-2 bg-white/5 border border-white/10 rounded-lg px-3 py-2">
                <span class="flex-1 text-sm text-slate-300 font-mono break-all">{p}</span>
                <button on:click={() => removeScanFolder(p)} class="text-slate-500 hover:text-red-400 shrink-0" title={$t('btn.remove')}>✕</button>
            </div>
        {/each}
        {#if scanPaths.length === 0}
            <p class="text-xs text-slate-500">{$t('settings.no_folders')}</p>
        {/if}
    </div>
    <div class="flex gap-2">
        <button on:click={addScanFolder} class="flex-1 {btn}">{$t('settings.add_folder')}</button>
        <button on:click={addSingleGame} class="flex-1 {btn}">{$t('settings.single_game')}</button>
    </div>
</Section>

<Section title={$t('ignore.section')} hint={$t('ignore.section_hint')}>
    <div class="flex flex-col gap-2 mb-2 max-h-56 overflow-y-auto">
        {#each ignored as p}
            <div class="flex items-center gap-2 bg-white/5 border border-white/10 rounded-lg px-3 py-2">
                <span class="text-amber-300/80 shrink-0">🚫</span>
                <span class="flex-1 text-sm text-slate-300 font-mono break-all">{p}</span>
                <button on:click={() => removeIgnored(p)} class="shrink-0 text-xs text-slate-400 hover:text-white bg-white/5 hover:bg-white/10 px-2 py-1 rounded-md border border-white/10" title={$t('ignore.unignore_hint')}>{$t('ignore.unignore')}</button>
            </div>
        {/each}
        {#if ignored.length === 0}
            <p class="text-xs text-slate-500">{$t('ignore.empty')}</p>
        {/if}
    </div>
    <button on:click={addIgnored} class="w-full {btn}">{$t('ignore.add')}</button>
</Section>

<Section title={$t('settings.maintenance')}>
    <div class="grid grid-cols-2 gap-2 mb-1">
        <button on:click={detectLaunch} class={btn}>🔎 {$t('settings.detect_launch')}</button>
        <button on:click={refreshSizes} class={btn}>💾 {$t('settings.refresh_sizes')}</button>
    </div>
    <p class="text-xs text-slate-500 mb-4">{$t('settings.detect_launch_hint')}</p>
    <button on:click={onRelink} class="w-full bg-indigo-900/30 hover:bg-indigo-800/50 text-indigo-200 hover:text-white font-semibold py-2.5 rounded-lg border border-indigo-700/40 transition-colors">
        {$t('settings.relink')}
    </button>
    <p class="text-xs text-slate-500 mt-2 mb-4">{$t('settings.relink_hint')}</p>
    <button on:click={removeMissing} class="w-full bg-amber-900/30 hover:bg-amber-800/50 text-amber-300 hover:text-amber-200 font-semibold py-2.5 rounded-lg border border-amber-800/40 transition-colors">
        {$t('settings.remove_missing')}
    </button>
    <p class="text-xs text-slate-500 mt-2">{$t('settings.remove_missing_hint')}</p>
</Section>
