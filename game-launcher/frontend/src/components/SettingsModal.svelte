<script>
    // Окно настроек с вкладками. Содержимое вкладок — в components/settings/.
    import { t } from '../i18n.js';
    import { lsGet, lsSet } from '../lib/util.js';
    import GeneralTab from './settings/GeneralTab.svelte';
    import LibraryTab from './settings/LibraryTab.svelte';
    import PrivacyTab from './settings/PrivacyTab.svelte';
    import DataTab from './settings/DataTab.svelte';

    export let scanPaths = [];
    export let langs = [];
    export let appVersion = '';
    export let launcherUpdate = null;                    // bind
    export let privacy = null;                           // bind
    export let hiddenCount = 0;
    export let showHidden = false;
    export let onChangeLang = (code) => {};
    export let onClose = () => {};
    export let onScan = async () => {};
    export let onReload = async (resetSelection) => {};
    export let onRelink = () => {};
    export let onToggleHidden = () => {};
    export let onLockNow = () => {};

    const TABS = [
        { id: 'general', icon: '⚙️', label: 'settings.tab_general' },
        { id: 'library', icon: '📚', label: 'settings.tab_library' },
        { id: 'privacy', icon: '🔒', label: 'settings.tab_privacy' },
        { id: 'data', icon: '💾', label: 'settings.tab_data' },
    ];
    let tab = lsGet('plauncher_settings_tab', 'general');
    if (!TABS.some(x => x.id === tab)) tab = 'general';
    $: lsSet('plauncher_settings_tab', tab);
</script>

<div class="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={onClose}>
    <div class="w-[820px] max-w-[94vw] h-[620px] max-h-[88vh] flex glass-strong rounded-2xl shadow-2xl overflow-hidden" on:click|stopPropagation>
        <nav class="w-52 shrink-0 border-r border-white/10 p-4 flex flex-col gap-1 bg-black/10">
            <h2 class="text-xl font-black text-white px-2 mb-3">{$t('settings.title')}</h2>
            {#each TABS as x}
                <button on:click={() => tab = x.id}
                        class="w-full flex items-center gap-2.5 text-left px-3 py-2 rounded-lg text-sm transition-colors {tab === x.id ? 'bg-indigo-500/20 text-white font-semibold ring-1 ring-inset ring-indigo-400/30' : 'text-slate-300 hover:bg-white/5 hover:text-white'}">
                    <span class="w-5 text-center">{x.icon}</span> {$t(x.label)}
                    {#if x.id === 'general' && launcherUpdate && launcherUpdate.available}
                        <span class="ml-auto w-2 h-2 rounded-full bg-emerald-400"></span>
                    {/if}
                </button>
            {/each}
        </nav>

        <div class="flex-1 min-w-0 flex flex-col">
            <div class="flex items-center justify-between px-7 pt-6 pb-3">
                <h3 class="text-2xl font-black text-white">{$t(TABS.find(x => x.id === tab).label)}</h3>
                <button on:click={onClose} class="text-slate-400 hover:text-white text-3xl leading-none">&times;</button>
            </div>
            <div class="flex-1 overflow-y-auto px-7 pb-7">
                {#if tab === 'general'}
                    <GeneralTab {langs} {appVersion} bind:launcherUpdate {onChangeLang} panicLabel={privacy && privacy.panic_enabled ? privacy.panic_label : ''}/>
                {:else if tab === 'library'}
                    <LibraryTab bind:scanPaths {onScan} {onReload} onRelink={() => { onClose(); onRelink(); }}/>
                {:else if tab === 'privacy' && privacy}
                    <PrivacyTab bind:privacy {hiddenCount} {showHidden} {onToggleHidden} {onLockNow}/>
                {:else if tab === 'data'}
                    <DataTab bind:scanPaths {onReload} {onClose}/>
                {/if}
            </div>
        </div>
    </div>
</div>
