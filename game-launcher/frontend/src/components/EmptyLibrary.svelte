<script>
    // Главная без игр: большой круг «+» (добавить папку с играми и сразу просканировать),
    // ссылка «одна игра» и подсказка про перетаскивание. Если папки уже есть, но игр
    // не нашлось — другой текст и кнопка повторного скана.
    import { t } from '../i18n.js';

    export let scanPaths = [];
    export let scanning = false;
    export let onAddFolder = () => {};
    export let onAddSingle = () => {};
    export let onScan = () => {};

    $: hasFolders = scanPaths.length > 0;
</script>

<div class="flex flex-col items-center justify-center text-center py-20 animate-fade-in">
    <button on:click={onAddFolder} disabled={scanning} title={$t('lib.add_folder')} aria-label={$t('lib.add_folder')}
            class="group relative w-40 h-40 rounded-full border-2 border-dashed border-indigo-400/40 bg-indigo-500/5
                   hover:border-indigo-300/80 hover:bg-indigo-500/10 hover:scale-105 active:scale-100
                   disabled:hover:scale-100 disabled:cursor-default transition-all duration-200
                   flex items-center justify-center mb-7 shadow-[0_0_60px_-10px_rgba(99,102,241,0.35)]">
        {#if scanning}
            <svg class="w-14 h-14 text-indigo-300 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-2.64-6.36"/></svg>
        {:else}
            <svg class="w-16 h-16 text-indigo-300/80 group-hover:text-indigo-200 transition-colors" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
        {/if}
    </button>

    {#if scanning}
        <p class="text-xl font-bold text-slate-200 mb-2">{$t('menu.scanning')}</p>
    {:else if hasFolders}
        <p class="text-xl font-bold text-slate-200 mb-2">{$t('lib.none_found_title')}</p>
        <p class="text-sm text-slate-400 max-w-md mb-5">{$t('lib.none_found_sub', { n: scanPaths.length })}</p>
        <div class="flex items-center gap-3">
            <button on:click={onAddFolder} class="text-sm font-semibold text-white bg-indigo-600 hover:bg-indigo-500 px-4 py-2 rounded-lg transition-colors">{$t('lib.add_folder')}</button>
            <button on:click={onScan} class="text-sm text-slate-300 hover:text-white bg-white/5 hover:bg-white/10 border border-white/10 px-4 py-2 rounded-lg transition-colors">⟳ {$t('lib.rescan')}</button>
        </div>
    {:else}
        <p class="text-xl font-bold text-slate-200 mb-2">{$t('lib.empty_title')}</p>
        <p class="text-sm text-slate-400 max-w-md mb-5">{$t('lib.empty_sub')}</p>
        <button on:click={onAddFolder} class="text-sm font-semibold text-white bg-indigo-600 hover:bg-indigo-500 px-4 py-2 rounded-lg transition-colors">{$t('lib.add_folder')}</button>
    {/if}

    {#if !scanning}
        <button on:click={onAddSingle} class="mt-4 text-xs text-slate-500 hover:text-slate-300 underline-offset-4 hover:underline transition-colors">{$t('lib.add_single')}</button>
    {/if}
</div>
