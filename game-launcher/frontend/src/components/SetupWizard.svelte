<script>
    // Окно первого запуска: 1) папка данных лаунчера, 2) папки с играми.
    import { onMount } from 'svelte';
    import { t, tr, langStore } from '../i18n.js';
    import { showToast } from '../lib/ui.js';
    import {
        GetDefaultDataDir, GetPortableDataDir, GetDocumentsDataDir,
        ConfigureDataDir, SelectDataFolder, SelectFolder,
        AddScanPath, RemoveScanPath, GetScanPaths,
    } from '../../wailsjs/go/main/App.js';

    export let langs = [];
    export let onChangeLang = (code) => {};
    export let onConfigured = async () => {};           // папка данных создана, БД открыта
    export let onFinish = async (scanPaths) => {};      // мастер завершён

    let step = 1;
    let busy = false;
    let choice = 'default';                              // default | portable | documents | custom
    let paths = { default: '', portable: '', documents: '' };
    let custom = '';
    let scanPaths = [];

    $: chosen = choice === 'custom' ? custom : (paths[choice] || '');

    onMount(async () => {
        try {
            paths = {
                default: await GetDefaultDataDir(),
                portable: await GetPortableDataDir(),
                documents: await GetDocumentsDataDir(),
            };
        } catch (e) { console.error(e); }
    });

    async function pickCustom() {
        try {
            const p = await SelectDataFolder();
            if (p) { custom = p; choice = 'custom'; }
        } catch (err) { console.error(err); }
    }

    // Шаг 1: создаём папку данных и переходим к выбору папок с играми
    async function confirmDataDir() {
        if (!chosen) return;
        busy = true;
        try {
            await ConfigureDataDir(chosen);
            await onConfigured();
            step = 2;
        } catch (err) {
            showToast(tr('toast.setup_fail', { err }), 'error');
        } finally {
            busy = false;
        }
    }

    // Шаг 2: папки с играми (можно несколько)
    async function addFolder() {
        try {
            const p = await SelectFolder();
            if (!p) return;
            await AddScanPath(p);
            scanPaths = await GetScanPaths();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    async function removeFolder(p) {
        await RemoveScanPath(p);
        scanPaths = await GetScanPaths();
    }
</script>

<div class="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in">
    <div class="w-[560px] max-w-[90vw] glass-strong rounded-2xl shadow-2xl p-7">
        {#if step === 1}
            <div class="flex justify-end mb-2">
                <select value={$langStore} on:change={(e) => onChangeLang(e.target.value)}
                        class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                    {#each langs as l}<option value={l.code}>{l.name}</option>{/each}
                </select>
            </div>
            <h2 class="text-2xl font-black text-white mb-1">{$t("setup.welcome")}</h2>
            <p class="text-slate-400 text-sm mb-5">{$t("setup.step1")}</p>

            <div class="flex flex-col gap-2 mb-5">
                {#each [['default', 'setup.sys'], ['documents', 'setup.docs'], ['portable', 'setup.portable']].filter(([k]) => paths[k] || k !== 'portable') as [key, label]}
                    <button on:click={() => choice = key} class="text-left p-3 rounded-lg border transition-colors {choice === key ? 'border-indigo-500 bg-indigo-600/15' : 'border-slate-600/50 hover:bg-slate-700/40'}">
                        <div class="font-bold text-white text-sm">{$t(label)} {#if key === 'default'}<span class="text-indigo-400">{$t("setup.recommended")}</span>{/if}</div>
                        <div class="text-xs text-slate-400 break-all mt-0.5 font-mono">{paths[key]}</div>
                    </button>
                {/each}
                <button on:click={pickCustom} class="text-left p-3 rounded-lg border transition-colors {choice === 'custom' ? 'border-indigo-500 bg-indigo-600/15' : 'border-slate-600/50 hover:bg-slate-700/40'}">
                    <div class="font-bold text-white text-sm">{$t("setup.custom")}</div>
                    <div class="text-xs text-slate-400 break-all mt-0.5 font-mono">{custom || $t('setup.custom_hint')}</div>
                </button>
            </div>

            <button on:click={confirmDataDir} disabled={busy || !chosen} class="w-full bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-3 rounded-lg shadow transition-all disabled:opacity-50 disabled:cursor-not-allowed">
                {busy ? $t('setup.creating') : $t('btn.continue')}
            </button>
        {:else}
            <h2 class="text-2xl font-black text-white mb-1">{$t("setup.games_title")}</h2>
            <p class="text-slate-400 text-sm mb-5">{$t("setup.step2")}</p>

            <div class="flex flex-col gap-2 mb-3 max-h-52 overflow-y-auto">
                {#each scanPaths as p}
                    <div class="flex items-center gap-2 bg-white/5 border border-white/10 rounded-lg px-3 py-2">
                        <span class="flex-1 text-sm text-slate-300 font-mono break-all">{p}</span>
                        <button on:click={() => removeFolder(p)} class="text-slate-500 hover:text-red-400 shrink-0" title={$t("btn.remove")}>✕</button>
                    </div>
                {/each}
                {#if scanPaths.length === 0}
                    <p class="text-xs text-slate-500 py-2">{$t("setup.none")}</p>
                {/if}
            </div>

            <button on:click={addFolder} class="w-full border-2 border-dashed border-white/15 hover:border-indigo-500 text-slate-300 hover:text-indigo-300 py-2.5 rounded-xl transition-colors font-semibold mb-5">
                {$t("setup.add_folder")}
            </button>

            <button on:click={() => onFinish(scanPaths)} class="w-full bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-3 rounded-lg shadow transition-all">
                {scanPaths.length ? $t('setup.done_scan') : $t('setup.skip')}
            </button>
        {/if}
    </div>
</div>
