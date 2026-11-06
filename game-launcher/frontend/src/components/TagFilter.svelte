<script>
    // Фильтр по тегам над сеткой. Можно выбрать несколько тегов («все из них» или «любой из них»)
    // и исключить ненужные. Выбранные теги видны чипсами рядом с кнопкой: × снимает один,
    // «Сбросить» — все сразу. Окошко не закрывается после щелчка, чтобы отмечать подряд.
    import { tick } from 'svelte';
    import { t } from '../i18n.js';
    import { clickOutside } from '../lib/util.js';
    import Icon from './Icon.svelte';

    export let tags = [];                   // [{tag, n}] — по частоте, в пределах текущего вида
    export let value = { inc: [], exc: [], mode: 'all' };
    export let onChange = (v) => {};

    let open = false;
    let query = '';
    let input;
    $: q = query.trim().toLowerCase();
    // Выбранные теги показываем всегда, даже если в текущем виде их уже нет
    $: list = (() => {
        const known = new Set(tags.map(x => x.tag));
        const extra = [...value.inc, ...value.exc].filter(tag => !known.has(tag)).map(tag => ({ tag, n: 0 }));
        return [...extra, ...tags];
    })();
    $: shown = q ? list.filter(x => x.tag.toLowerCase().includes(q)) : list;
    $: count = value.inc.length + value.exc.length;
    const stateOf = (tag, v) => v.inc.includes(tag) ? 'inc' : v.exc.includes(tag) ? 'exc' : '';

    async function toggleOpen() {
        open = !open;
        query = '';
        if (open) { await tick(); if (input) input.focus(); }
    }
    const without = (arr, tag) => arr.filter(x => x !== tag);
    // Щелчок по строке: выбрать / снять; кнопка «−»: исключить / снять исключение
    function toggleInc(tag) {
        const inc = value.inc.includes(tag) ? without(value.inc, tag) : [...value.inc, tag];
        onChange({ ...value, inc, exc: without(value.exc, tag) });
    }
    function toggleExc(tag) {
        const exc = value.exc.includes(tag) ? without(value.exc, tag) : [...value.exc, tag];
        onChange({ ...value, exc, inc: without(value.inc, tag) });
    }
    function remove(tag) { onChange({ ...value, inc: without(value.inc, tag), exc: without(value.exc, tag) }); }
    function clearAll() { onChange({ inc: [], exc: [], mode: value.mode }); }
    function setMode(mode) { onChange({ ...value, mode }); }
    function onKey(e) {
        if (e.key === 'Escape') { e.preventDefault(); open = false; }
        else if (e.key === 'Enter' && shown.length) { e.preventDefault(); toggleInc(shown[0].tag); query = ''; }
    }
    const segBtn = (on) => 'flex-1 h-7 rounded-md text-xs transition-colors ' + (on ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-100');
</script>

<div class="flex items-center gap-1.5 flex-wrap">
    <div class="relative" use:clickOutside on:clickoutside={() => open = false}>
        <button on:click={toggleOpen} aria-expanded={open}
                class="h-8 px-3.5 rounded-full text-[13px] border transition-colors flex items-center gap-1.5 {count ? 'border-indigo-500 bg-indigo-900 text-indigo-100' : open ? 'border-white/20 text-white' : 'border-white/10 text-slate-400 hover:text-white hover:border-white/20'}">
            {$t('tags.label')}
            {#if count}<span class="min-w-[18px] h-[18px] px-1 rounded-full bg-indigo-500 text-white text-[11px] flex items-center justify-center">{count}</span>{/if}
            <svg class="w-3 h-3 opacity-70 transition-transform {open ? 'rotate-180' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>
        </button>

        {#if open}
            <div class="absolute left-0 top-10 z-40 w-72 rounded-xl glass-strong shadow-2xl p-2 animate-fade-in" on:keydown={onKey}>
                <input bind:this={input} bind:value={query} type="text" placeholder={$t('tags.search')} aria-label={$t('tags.search')}
                       class="w-full h-8 mb-2 bg-slate-950 border border-white/10 rounded-md px-2.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-400/50"/>
                {#if value.inc.length > 1}
                    <div class="flex gap-0.5 p-0.5 mb-2 rounded-lg bg-slate-950 border border-white/10" role="group" aria-label={$t('tags.mode')}>
                        <button on:click={() => setMode('all')} class={segBtn(value.mode !== 'any')} title={$t('tags.mode_all_hint')}>{$t('tags.mode_all')}</button>
                        <button on:click={() => setMode('any')} class={segBtn(value.mode === 'any')} title={$t('tags.mode_any_hint')}>{$t('tags.mode_any')}</button>
                    </div>
                {/if}
                <div class="max-h-64 overflow-y-auto overscroll-contain">
                    {#each shown as x (x.tag)}
                        {@const st = stateOf(x.tag, value)}
                        <div class="group flex items-center rounded-md transition-colors {st === 'inc' ? 'bg-indigo-900' : st === 'exc' ? 'bg-rose-950/60' : 'hover:bg-white/5'}">
                            <button on:click={() => toggleInc(x.tag)} role="checkbox" aria-checked={st === 'inc'}
                                    class="flex-1 min-w-0 h-8 flex items-center gap-2 pl-2.5 text-[13px] text-left {st === 'inc' ? 'text-indigo-100' : st === 'exc' ? 'text-rose-200 line-through decoration-rose-300/60' : 'text-slate-300 group-hover:text-white'}">
                                <span class="w-4 h-4 rounded border flex items-center justify-center shrink-0 {st === 'inc' ? 'bg-indigo-500 border-indigo-400 text-white' : st === 'exc' ? 'bg-rose-500/80 border-rose-400 text-white' : 'border-white/20'}">
                                    {#if st === 'inc'}<Icon name="check" size={11}/>{:else if st === 'exc'}<span class="text-[11px] leading-none">−</span>{/if}
                                </span>
                                <span class="flex-1 truncate">{x.tag}</span>
                                <span class="text-[11px] text-slate-500 no-underline">{x.n || ''}</span>
                            </button>
                            <button on:click={() => toggleExc(x.tag)} title={st === 'exc' ? $t('tags.unexclude') : $t('tags.exclude')} aria-label={$t('tags.exclude') + ': ' + x.tag}
                                    class="w-8 h-8 shrink-0 flex items-center justify-center rounded-md text-sm transition {st === 'exc' ? 'text-rose-300' : 'text-slate-500 opacity-0 group-hover:opacity-100 hover:text-rose-300'}">−</button>
                        </div>
                    {/each}
                    {#if shown.length === 0}
                        <p class="px-2.5 py-2 text-xs text-slate-500">{$t('lib.not_found_title')}</p>
                    {/if}
                </div>
                <div class="flex items-center justify-between gap-2 pt-2 mt-1 border-t border-white/10 text-[11px] text-slate-500">
                    <span>{$t('tags.help')}</span>
                    {#if count}<button on:click={clearAll} class="shrink-0 text-xs text-indigo-300 hover:text-indigo-100">{$t('tags.clear_n', { n: count })}</button>{/if}
                </div>
            </div>
        {/if}
    </div>

    <!-- Выбранные теги: видно, что отфильтровано, и снимается в один щелчок -->
    {#each value.inc as tag (tag)}
        <span class="h-7 pl-2.5 pr-1 rounded-full text-xs flex items-center gap-1 bg-indigo-900 text-indigo-100 border border-indigo-500/50">
            {tag}
            <button on:click={() => remove(tag)} aria-label={$t('tags.remove') + ': ' + tag} class="w-5 h-5 rounded-full flex items-center justify-center text-indigo-300 hover:text-white hover:bg-white/10"><Icon name="x" size={11}/></button>
        </span>
    {/each}
    {#each value.exc as tag (tag)}
        <span class="h-7 pl-2.5 pr-1 rounded-full text-xs flex items-center gap-1 bg-rose-950/60 text-rose-200 border border-rose-700/50" title={$t('tags.excluded')}>
            − {tag}
            <button on:click={() => remove(tag)} aria-label={$t('tags.remove') + ': ' + tag} class="w-5 h-5 rounded-full flex items-center justify-center text-rose-300 hover:text-white hover:bg-white/10"><Icon name="x" size={11}/></button>
        </span>
    {/each}
    {#if count > 1}
        <button on:click={clearAll} class="h-7 px-2 text-xs text-slate-400 hover:text-white">{$t('tags.clear_all')}</button>
    {/if}
</div>
