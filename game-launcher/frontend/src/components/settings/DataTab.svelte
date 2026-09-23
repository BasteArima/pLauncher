<script>
    // Данные: папка данных лаунчера, резервная копия, опасная зона.
    import { onMount } from 'svelte';
    import { t, tr } from '../../i18n.js';
    import { showToast, askConfirm } from '../../lib/ui.js';
    import {
        GetDataDir, SelectDataFolder, ChangeDataDir, OpenDataDir, ClearData,
        ExportLibrary, SelectBackupFile, ImportLibrary, GetScanPaths,
    } from '../../../wailsjs/go/main/App.js';
    import Section from './Section.svelte';
    import { btn } from './styles.js';

    export let scanPaths = [];
    export let onReload = async (resetSelection) => {};
    export let onClose = () => {};

    let dataDir = '';
    let backupBusy = false;
    onMount(async () => { try { dataDir = await GetDataDir(); } catch (e) {} });

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
        } catch (err) { showToast(tr('toast.dir_change_fail', { err }), 'error'); }
    }
    async function openDataDir() {
        try { await OpenDataDir(); } catch (err) { showToast(tr('toast.open_folder_fail', { err }), 'error'); }
    }
    async function exportLibrary() {
        backupBusy = true;
        try {
            const p = await ExportLibrary();
            if (p) showToast(tr('toast.exported', { path: p }), 'success');
        } catch (err) { showToast(tr('toast.export_fail', { err }), 'error'); }
        finally { backupBusy = false; }
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
        } catch (err) { showToast(tr('toast.import_fail', { err }), 'error'); }
        finally { backupBusy = false; }
    }
    async function clearData() {
        const ok = await askConfirm({ title: tr('dlg.clear_title'), message: tr('dlg.clear_msg'), confirmText: tr('btn.clear'), danger: true });
        if (!ok) return;
        try {
            await ClearData();
            await onReload(true);
            onClose();
            showToast(tr('toast.cleared'), 'success');
        } catch (err) { showToast(tr('toast.clear_fail', { err }), 'error'); }
    }
</script>

<Section title={$t('settings.data_folder')}>
    <div class="flex gap-2 items-stretch mb-2">
        <div class="flex-1 bg-slate-900/60 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-300 font-mono break-all">{dataDir || '—'}</div>
        <button on:click={openDataDir} title={$t('settings.open_folder')} class="{btn} text-lg">📂</button>
    </div>
    <button on:click={changeDataDir} class="w-full {btn}">{$t('settings.change_folder')}</button>
</Section>

<Section title={$t('settings.backup')} hint={backupBusy ? $t('settings.backup_busy') : $t('settings.backup_hint')}>
    <div class="flex gap-2">
        <button on:click={exportLibrary} disabled={backupBusy} class="flex-1 {btn}">⤓ {$t('settings.export')}</button>
        <button on:click={importLibrary} disabled={backupBusy} class="flex-1 {btn}">⤒ {$t('settings.import')}</button>
    </div>
</Section>

<Section title={$t('settings.danger')} hint={$t('settings.clear_hint')}>
    <button on:click={clearData} class="w-full bg-red-900/40 hover:bg-red-800/60 text-red-300 hover:text-red-200 font-semibold py-2.5 rounded-lg border border-red-800/50 transition-colors">
        {$t('settings.clear')}
    </button>
</Section>
