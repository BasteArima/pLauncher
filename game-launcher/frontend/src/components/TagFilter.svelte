<script>
    // Фильтр по тегу над сеткой: кнопка «Теги ▾» и компактное окошко с поиском и прокручиваемым списком.
    import { tick } from 'svelte';
    import { t } from '../i18n.js';
    import { clickOutside } from '../lib/util.js';

    export let tags = [];                   // [{tag, n}] — по частоте
    export let active = '';                 // выбранный тег
    export let onPick = (tag) => {};        // '' — сбросить

    let open = false;
    let query = '';
    let input;
    $: q = query.trim().toLowerCase();
    $: shown = q ? tags.filter(x => x.tag.toLowerCase().includes(q)) : tags;

    async function toggle() {
        open = !open;
        query = '';
        if (open) { await tick(); if (input) input.focus(); }
    }
    function pick(tag) { onPick(tag); open = false; }
    function onKey(e) {
        if (e.key === 'Escape') { e.preventDefault(); open = false; }
        else if (e.key === 'Enter' && shown.length) { e.preventDefault(); pick(shown[0].tag); }
    }
</script>

<div class="relative" use:clickOutside on:clickoutside={() => open = false}>
    <button on:click={toggle} aria-expanded={open}
            class="h-8 px-3.5 rounded-full text-[13px] border transition-colors flex items-center gap-1.5 {active ? 'border-indigo-500 bg-indigo-900 text-indigo-100' : open ? 'border-white/20 text-white' : 'border-white/10 text-slate-400 hover:text-white hover:border-white/20'}">
        {active || $t('tags.label')}
        <svg class="w-3 h-3 opacity-70 transition-transform {open ? 'rotate-180' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>
    </button>

    {#if open}
        <div class="absolute left-0 top-10 z-40 w-64 rounded-xl glass-strong shadow-2xl p-2 animate-fade-in" on:keydown={onKey}>
            <input bind:this={input} bind:value={query} type="text" placeholder={$t('tags.search')} aria-label={$t('tags.search')}
                   class="w-full h-8 mb-1.5 bg-slate-950 border border-white/10 rounded-md px-2.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-400/50"/>
            <div class="max-h-64 overflow-y-auto overscroll-contain">
                {#if active}
                    <button on:click={() => pick('')} class="w-full h-8 flex items-center gap-2 px-2.5 rounded-md text-[13px] text-slate-300 hover:bg-white/5 hover:text-white">
                        ✕ {$t('tags.clear')}
                    </button>
                    <div class="h-px bg-white/10 my-1 mx-1"></div>
                {/if}
                {#each shown as x (x.tag)}
                    <button on:click={() => pick(active === x.tag ? '' : x.tag)}
                            class="w-full h-8 flex items-center gap-2 px-2.5 rounded-md text-[13px] text-left transition-colors {active === x.tag ? 'bg-indigo-900 text-indigo-100' : 'text-slate-300 hover:bg-white/5 hover:text-white'}">
                        <span class="w-3.5 text-indigo-300 text-xs">{active === x.tag ? '✓' : ''}</span>
                        <span class="flex-1 truncate">{x.tag}</span>
                        <span class="text-[11px] text-slate-500">{x.n}</span>
                    </button>
                {/each}
                {#if shown.length === 0}
                    <p class="px-2.5 py-2 text-xs text-slate-500">{$t('lib.not_found_title')}</p>
                {/if}
            </div>
        </div>
    {/if}
</div>
