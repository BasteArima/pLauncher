<script>
    // Общие: язык интерфейса и обновление лаунчера.
    import { t, tr, langStore, localizeError } from '../../i18n.js';
    import { updateInfo, updateModalOpen, autoCheck, checkUpdate } from '../../lib/update.js';
    import { showToast } from '../../lib/ui.js';
    import { OpenLanguagesFolder } from '../../../wailsjs/go/main/App.js';
    import Section from './Section.svelte';
    import Icon from '../Icon.svelte';
    import HotkeyList from '../HotkeyList.svelte';
    import { showHero } from '../../lib/view.js';
    import { hotkeys, hotkeyOverrides, hkLabel, resetAllHotkeys } from '../../lib/hotkeys.js';
    import { askConfirm } from '../../lib/ui.js';
    import { btn, input } from './styles.js';

    export let langs = [];
    export let appVersion = '';
    export let onChangeLang = (code) => {};
    export let panicLabel = '';

    $: hkHint = $t('hk.edit_hint') + (hkLabel($hotkeys, 'help') ? ' ' + $t('hk.settings_hint', { key: hkLabel($hotkeys, 'help') }) : '');
    $: hkChanged = Object.keys($hotkeyOverrides).length > 0;
    async function resetAll() {
        if (await askConfirm({ title: tr('hk.reset_all'), message: tr('hk.reset_all_confirm'), confirmText: tr('hk.reset_all') })) resetAllHotkeys();
    }

    let updBusy = false;
    let updError = '';
    async function checkLauncher() {
        updBusy = true; updError = '';
        try {
            const info = await checkUpdate();
            if (!info.available) showToast(tr('upd.latest'), 'success');
        } catch (err) { updError = localizeError(err); }
        finally { updBusy = false; }
    }
</script>

<Section title={$t('settings.language')}>
    <div class="flex gap-2 items-stretch">
        <select value={$langStore} on:change={(e) => onChangeLang(e.target.value)} class="flex-1 {input}">
            {#each langs as l}<option value={l.code}>{l.name}</option>{/each}
        </select>
        <button on:click={() => OpenLanguagesFolder()} title={$t('settings.custom_lang')} class="{btn} inline-flex items-center"><Icon name="folder" size={16}/></button>
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
            {#if $updateInfo && $updateInfo.latest}<span class="text-slate-500"> · {$t('upd.latest_is', { v: $updateInfo.latest })}</span>{/if}
        </span>
        {#if $updateInfo && $updateInfo.available}
            <button on:click={() => updateModalOpen.set(true)} class="text-sm font-bold text-white bg-indigo-600 hover:bg-indigo-500 px-3 py-1.5 rounded-lg transition-colors">{$t('upd.open')}</button>
        {:else}
            <button on:click={checkLauncher} disabled={updBusy} class={btn}>{updBusy ? $t('meta.checking') : $t('upd.check')}</button>
        {/if}
    </div>
    <label class="flex items-center gap-2 text-sm text-slate-300 cursor-pointer" title={$t('upd.auto_check_hint')}>
        <input type="checkbox" bind:checked={$autoCheck} class="accent-indigo-500"/> {$t('upd.auto_check')}
    </label>
    {#if updError}<p class="text-xs text-rose-300 break-words mt-2">{updError}</p>{/if}
</Section>

<Section title={$t('hk.title')} hint={hkHint}>
    <HotkeyList {panicLabel} editable/>
    {#if hkChanged}
        <div class="mt-4 flex justify-end">
            <button on:click={resetAll} class="{btn} inline-flex items-center gap-2" data-hotkey-reset-all><Icon name="refresh" size={14}/>{$t('hk.reset_all')}</button>
        </div>
    {/if}
</Section>

<div class="pt-4 border-t border-white/10 flex items-center justify-between text-xs text-slate-500">
    <span>pLauncher <span class="text-slate-400">v{appVersion}</span></span>
    <span>© 2026 BasteArima</span>
</div>
