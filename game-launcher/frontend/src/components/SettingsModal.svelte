<script>
    // Окно настроек: язык, папки с играми, папка данных, обслуживание,
    // резервная копия и опасная зона.
    import { onMount } from 'svelte';
    import { t, tr, langStore } from '../i18n.js';
    import { showToast, askConfirm } from '../lib/ui.js';
    import {
        GetDataDir, SelectDataFolder, ChangeDataDir, OpenDataDir, ClearData,
        RemoveMissingGames, SelectFolder, AddScanPath, RemoveScanPath, GetScanPaths,
        AddSingleGameManual, OpenLanguagesFolder,
        ExportLibrary, SelectBackupFile, ImportLibrary,
        CheckLauncherUpdate, InstallLauncherUpdate, OpenLauncherReleasePage,
    } from '../../wailsjs/go/main/App.js';

    export let scanPaths = [];
    export let langs = [];
    export let appVersion = '';
    export let launcherUpdate = null;                    // результат CheckLauncherUpdate (bind)
    export let onChangeLang = (code) => {};
    export let onClose = () => {};
    export let onScan = async () => {};                 // пересканировать папки
    export let onReload = async (resetSelection) => {};  // перечитать библиотеку
    export let onRelink = () => {};                      // открыть окно перепривязки

    let dataDir = '';
    let backupBusy = false;

    onMount(async () => {
        try { dataDir = await GetDataDir(); } catch (e) {}
    });

    // --- Папки с играми ---
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

    // --- Папка данных ---
    async function changeDataDir() {
        try {
            const p = await SelectDataFolder();
            if (!p) return;
            const ok = await askConfirm({ title: tr('dlg.change_dir_title'), message: tr('dlg.change_dir_msg', { path: p }), confirmText: tr('btn.move') });
            if (!ok) return;
            await ChangeDataDir(p);
            dataDir = await GetDataDir();
            await onReload(true);
            showToast(tr('toast.dir_changed'), 'success');
        } catch (err) {
            showToast(tr('toast.dir_change_fail', { err }), 'error');
        }
    }
    async function openDataDir() {
        try { await OpenDataDir(); } catch (err) { showToast(tr('toast.open_folder_fail', { err }), 'error'); }
    }

    // --- Обслуживание ---
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
        } catch (err) {
            showToast(tr('toast.remove_missing_fail', { err }), 'error');
        }
    }

    // --- Резервная копия ---
    async function exportLibrary() {
        backupBusy = true;
        try {
            const p = await ExportLibrary();
            if (p) showToast(tr('toast.exported', { path: p }), 'success');
        } catch (err) {
            showToast(tr('toast.export_fail', { err }), 'error');
        } finally { backupBusy = false; }
    }
    async function importLibrary() {
        let p = '';
        try { p = await SelectBackupFile(); } catch (err) { return; }
        if (!p) return;
        const ok = await askConfirm({ title: tr('dlg.import_title'), message: tr('dlg.import_msg', { path: p }), confirmText: tr('btn.import'), danger: true });
        if (!ok) return;
        backupBusy = true;
        try {
            const n = await ImportLibrary(p);
            scanPaths = await GetScanPaths();
            await onReload(true);
            showToast(tr('toast.imported', { n }), 'success');
        } catch (err) {
            showToast(tr('toast.import_fail', { err }), 'error');
        } finally { backupBusy = false; }
    }

    // --- Обновление лаунчера ---
    let updBusy = false;
    let updError = '';
    async function checkLauncher() {
        updBusy = true; updError = '';
        try {
            launcherUpdate = await CheckLauncherUpdate();
            if (!launcherUpdate.available) showToast(tr('upd.latest'), 'success');
        } catch (err) { updError = String(err); }
        finally { updBusy = false; }
    }
    async function installLauncher() {
        const ok = await askConfirm({ title: tr('upd.install_title', { v: launcherUpdate.latest }), message: tr('upd.install_msg'), confirmText: tr('upd.install') });
        if (!ok) return;
        updBusy = true; updError = '';
        try { await InstallLauncherUpdate(); }         // при успехе лаунчер перезапустится
        catch (err) { updError = String(err); updBusy = false; }
    }

    // --- Опасная зона ---
    async function clearData() {
        const ok = await askConfirm({ title: tr('dlg.clear_title'), message: tr('dlg.clear_msg'), confirmText: tr('btn.clear'), danger: true });
        if (!ok) return;
        try {
            await ClearData();
            await onReload(true);
            onClose();
            showToast(tr('toast.cleared'), 'success');
        } catch (err) {
            showToast(tr('toast.clear_fail', { err }), 'error');
        }
    }

    const sectionCls = 'text-xs font-bold text-slate-400 uppercase tracking-wider mb-2 border-t border-white/10 pt-5';
    const btnCls = 'bg-white/5 hover:bg-white/10 text-slate-200 text-sm font-semibold py-2 px-3 rounded-lg border border-white/10 transition-colors disabled:opacity-50';
</script>

<div class="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={onClose}>
    <div class="w-[560px] max-w-[90vw] max-h-[85vh] overflow-y-auto glass-strong rounded-2xl shadow-2xl p-7" on:click|stopPropagation>
        <div class="flex items-center justify-between mb-5">
            <h2 class="text-2xl font-black text-white">{$t("settings.title")}</h2>
            <button on:click={onClose} class="text-slate-400 hover:text-white text-3xl leading-none">&times;</button>
        </div>

        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("settings.language")}</div>
        <div class="flex gap-2 items-stretch mb-6">
            <select value={$langStore} on:change={(e) => onChangeLang(e.target.value)}
                    class="flex-1 bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                {#each langs as l}<option value={l.code}>{l.name}</option>{/each}
            </select>
            <button on:click={() => OpenLanguagesFolder()} title={$t("settings.custom_lang")}
                    class="shrink-0 flex items-center justify-center bg-white/10 hover:bg-white/20 text-slate-200 px-4 rounded-lg border border-white/10 transition-colors text-lg">📂</button>
        </div>

        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("settings.games_folders")}</div>
        <div class="flex flex-col gap-2 mb-2">
            {#each scanPaths as p}
                <div class="flex items-center gap-2 bg-white/5 border border-white/10 rounded-lg px-3 py-2">
                    <span class="flex-1 text-sm text-slate-300 font-mono break-all">{p}</span>
                    <button on:click={() => removeScanFolder(p)} class="text-slate-500 hover:text-red-400 shrink-0" title={$t("btn.remove")}>✕</button>
                </div>
            {/each}
            {#if scanPaths.length === 0}
                <p class="text-xs text-slate-500">{$t("settings.no_folders")}</p>
            {/if}
        </div>
        <div class="flex gap-2 mb-6">
            <button on:click={addScanFolder} class="flex-1 {btnCls}">{$t("settings.add_folder")}</button>
            <button on:click={addSingleGame} class="flex-1 {btnCls}">{$t("settings.single_game")}</button>
        </div>

        <div class={sectionCls}>{$t("settings.data_folder")}</div>
        <div class="flex gap-2 items-stretch mb-2">
            <div class="flex-1 bg-slate-900/60 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-300 font-mono break-all">{dataDir || '—'}</div>
            <button on:click={openDataDir} title={$t("settings.open_folder")} class="shrink-0 bg-white/10 hover:bg-white/20 text-slate-200 px-4 rounded-lg border border-white/10 transition-colors text-xl">📂</button>
        </div>
        <button on:click={changeDataDir} class="w-full bg-white/5 hover:bg-white/10 text-slate-200 font-semibold py-2.5 rounded-lg border border-white/10 transition-colors mb-6">
            {$t("settings.change_folder")}
        </button>

        <div class={sectionCls}>{$t("upd.title")}</div>
        <div class="flex items-center gap-2 mb-2">
            <span class="text-sm text-slate-300 flex-1">
                {$t("upd.current", { v: appVersion })}
                {#if launcherUpdate && launcherUpdate.latest}<span class="text-slate-500"> · {$t("upd.latest_is", { v: launcherUpdate.latest })}</span>{/if}
            </span>
            <button on:click={checkLauncher} disabled={updBusy} class={btnCls}>{updBusy ? $t("meta.checking") : $t("upd.check")}</button>
        </div>
        {#if launcherUpdate && launcherUpdate.available}
            <div class="rounded-lg bg-emerald-500/10 ring-1 ring-emerald-400/30 p-3 mb-2">
                <div class="flex items-center gap-2 mb-1">
                    <span class="font-bold text-emerald-200 flex-1">{$t("upd.available", { v: launcherUpdate.latest })}</span>
                    <button on:click={() => OpenLauncherReleasePage()} class="text-xs text-slate-300 hover:text-white">{$t("upd.page")} ↗</button>
                    <button on:click={installLauncher} disabled={updBusy} class="text-sm font-bold text-white bg-emerald-600 hover:bg-emerald-500 px-3 py-1.5 rounded-lg transition-colors disabled:opacity-50">{$t("upd.install")}</button>
                </div>
                {#if launcherUpdate.notes}
                    <p class="text-xs text-slate-400 whitespace-pre-wrap max-h-32 overflow-y-auto select-text">{launcherUpdate.notes}</p>
                {/if}
            </div>
        {/if}
        {#if updError}
            <p class="text-xs text-rose-300 mb-2 break-words">{updError}</p>
        {/if}
        <div class="mb-6"></div>

        <div class={sectionCls}>{$t("settings.backup")}</div>
        <div class="flex gap-2">
            <button on:click={exportLibrary} disabled={backupBusy} class="flex-1 {btnCls}">⤓ {$t("settings.export")}</button>
            <button on:click={importLibrary} disabled={backupBusy} class="flex-1 {btnCls}">⤒ {$t("settings.import")}</button>
        </div>
        <p class="text-xs text-slate-500 mt-2 mb-6">{backupBusy ? $t("settings.backup_busy") : $t("settings.backup_hint")}</p>

        <div class={sectionCls}>{$t("settings.maintenance")}</div>
        <button on:click={() => { onClose(); onRelink(); }} class="w-full bg-indigo-900/30 hover:bg-indigo-800/50 text-indigo-200 hover:text-white font-semibold py-2.5 rounded-lg border border-indigo-700/40 transition-colors">
            {$t("settings.relink")}
        </button>
        <p class="text-xs text-slate-500 mt-2 mb-3">{$t("settings.relink_hint")}</p>
        <button on:click={removeMissing} class="w-full bg-amber-900/30 hover:bg-amber-800/50 text-amber-300 hover:text-amber-200 font-semibold py-2.5 rounded-lg border border-amber-800/40 transition-colors">
            {$t("settings.remove_missing")}
        </button>
        <p class="text-xs text-slate-500 mt-2 mb-4">{$t("settings.remove_missing_hint")}</p>

        <div class={sectionCls}>{$t("settings.danger")}</div>
        <button on:click={clearData} class="w-full bg-red-900/40 hover:bg-red-800/60 text-red-300 hover:text-red-200 font-semibold py-2.5 rounded-lg border border-red-800/50 transition-colors">
            {$t("settings.clear")}
        </button>
        <p class="text-xs text-slate-500 mt-2">{$t("settings.clear_hint")}</p>

        <div class="mt-6 pt-4 border-t border-white/10 flex items-center justify-between text-xs text-slate-500">
            <span>pLauncher <span class="text-slate-400">v{appVersion}</span></span>
            <span>© 2026 Midaser</span>
        </div>
    </div>
</div>
