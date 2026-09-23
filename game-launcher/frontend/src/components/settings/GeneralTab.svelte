<script>
    // Общие: язык интерфейса и обновление лаунчера.
    import { t, tr, langStore } from '../../i18n.js';
    import { showToast, askConfirm } from '../../lib/ui.js';
    import { OpenLanguagesFolder, CheckLauncherUpdate, InstallLauncherUpdate, OpenLauncherReleasePage } from '../../../wailsjs/go/main/App.js';
    import Section from './Section.svelte';
    import HotkeyList from '../HotkeyList.svelte';
    import { showHero } from '../../lib/view.js';
    import { btn, input } from './styles.js';

    export let langs = [];
    export let appVersion = '';
    export let launcherUpdate = null;
    export let onChangeLang = (code) => {};
    export let panicLabel = '';

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
</script>

<Section title={$t('settings.language')}>
    <div class="flex gap-2 items-stretch">
        <select value={$langStore} on:change={(e) => onChangeLang(e.target.value)} class="flex-1 {input}">
            {#each langs as l}<option value={l.code}>{l.name}</option>{/each}
        </select>
        <button on:click={() => OpenLanguagesFolder()} title={$t('settings.custom_lang')} class="{btn} text-lg">📂</button>
    </div>
</Section>

<Section title={$t('settings.home')}>
    <label class="flex items-start gap-3 cursor-pointer">
        <input type="checkbox" bind:checked={$showHero} class="mt-0.5 accent-indigo-500 w-4 h-4"/>
        <span>
            <span class="block text-sm text-slate-200">{$t('settings.show_hero')}</span>
            <span class="block text-xs text-slate-500">{$t('settings.show_hero_hint')}</span>
        </span>
    </label>
</Section>

<Section title={$t('upd.title')}>
    <div class="flex items-center gap-2 mb-2">
        <span class="text-sm text-slate-300 flex-1">
            {$t('upd.current', { v: appVersion })}
            {#if launcherUpdate && launcherUpdate.latest}<span class="text-slate-500"> · {$t('upd.latest_is', { v: launcherUpdate.latest })}</span>{/if}
        </span>
        <button on:click={checkLauncher} disabled={updBusy} class={btn}>{updBusy ? $t('meta.checking') : $t('upd.check')}</button>
    </div>
    {#if launcherUpdate && launcherUpdate.available}
        <div class="rounded-lg bg-emerald-500/10 ring-1 ring-emerald-400/30 p-3 mb-2">
            <div class="flex items-center gap-2 mb-1">
                <span class="font-bold text-emerald-200 flex-1">{$t('upd.available', { v: launcherUpdate.latest })}</span>
                <button on:click={() => OpenLauncherReleasePage()} class="text-xs text-slate-300 hover:text-white">{$t('upd.page')} ↗</button>
                {#if launcherUpdate.can_install}
                    <button on:click={installLauncher} disabled={updBusy} class="text-sm font-bold text-white bg-emerald-600 hover:bg-emerald-500 px-3 py-1.5 rounded-lg transition-colors disabled:opacity-50">{$t('upd.install')}</button>
                {:else}
                    <!-- Для этой ОС автоустановки нет (macOS) — скачать вручную со страницы релиза -->
                    <button on:click={() => OpenLauncherReleasePage()} class="text-sm font-bold text-white bg-emerald-600 hover:bg-emerald-500 px-3 py-1.5 rounded-lg transition-colors">{$t('upd.download')} ↗</button>
                {/if}
            </div>
            {#if launcherUpdate.notes}
                <p class="text-xs text-slate-400 whitespace-pre-wrap max-h-32 overflow-y-auto select-text">{launcherUpdate.notes}</p>
            {/if}
        </div>
    {/if}
    {#if updError}<p class="text-xs text-rose-300 break-words">{updError}</p>{/if}
</Section>

<Section title={$t('hk.title')} hint={$t('hk.settings_hint')}>
    <HotkeyList {panicLabel}/>
</Section>

<div class="pt-4 border-t border-white/10 flex items-center justify-between text-xs text-slate-500">
    <span>pLauncher <span class="text-slate-400">v{appVersion}</span></span>
    <span>© 2026 Midaser</span>
</div>
