<script>
    // Левая панель: навигация (главная/фильтры), дерево коллекций с поиском.
    import { t, tr } from '../i18n.js';
    import { mediaSrc, lsJSON, lsSet } from '../lib/util.js';
    import { coverBlur, blurCls } from '../lib/privacy.js';
    import { selection, selecting, toggleSelected } from '../lib/view.js';

    export let width = 256;
    export let groups = [];                 // [{key, name, items, col}] — коллекции + «Без категории»
    export let totalGames = 0;              // для «библиотека пуста»
    export let libCount = 0;
    export let favoritesCount = 0;
    export let updatesCount = 0;
    export let missingCount = 0;
    export let noCoverCount = 0;           // игры без обложки (ещё не спарсены)
    export let isHome = true;
    export let activeFilter = 'all';
    export let selectedId = '';
    export let draggingGame = null;         // игра, которую тащат (из сетки или из сайдбара)
    export let settingsBadge = false;       // точка на ⚙ — доступно обновление лаунчера

    export let onFilter = (f) => {};
    export let onRelink = () => {};
    export let onSelect = (g) => {};
    export let onSettings = () => {};
    export let onCreateCollection = () => {};
    export let onEditCollection = (col) => {};
    export let onDeleteCollection = (col) => {};
    export let onToggleHiddenCol = (col) => {};  // пометить коллекцию скрытой / вернуть
    export let onGameContext = (g, e) => {};
    export let onGameDragStart = (g) => {};
    export let onDropGame = (col) => {};
    export let openCtx = (e, items) => {};

    // --- Свёрнутые группы (запоминаются) ---
    let collapsed = lsJSON('plauncher_collapsed', {});
    function saveCollapsed() { lsSet('plauncher_collapsed', JSON.stringify(collapsed)); }
    function toggleGroup(key) { collapsed = { ...collapsed, [key]: !collapsed[key] }; saveCollapsed(); }
    function expandAll() { collapsed = {}; saveCollapsed(); }
    function collapseAll() { collapsed = Object.fromEntries(groups.map(g => [g.key, true])); saveCollapsed(); }

    function collectionMenu(col) {
        return [
            { label: tr('ctx.rename'), icon: '✎', action: () => onEditCollection(col) },
            ...(col.type === 'dynamic' ? [{ label: tr('ctx.change_filters'), icon: '⚡', action: () => onEditCollection(col) }] : []),
            { label: collapsed[col.id] ? tr('ctx.expand') : tr('ctx.collapse'), action: () => toggleGroup(col.id) },
            { sep: true },
            { label: tr('ctx.expand_all'), action: expandAll },
            { label: tr('ctx.collapse_all'), action: collapseAll },
            { sep: true },
            { label: col.hidden ? tr('ctx.unhide_collection') : tr('ctx.hide_collection'), icon: '🔒', action: () => onToggleHiddenCol(col) },
            { sep: true },
            { label: tr('ctx.delete_collection'), icon: '🗑', danger: true, action: () => onDeleteCollection(col) },
        ];
    }

    // --- Поиск по сайдбару: фильтрует игры в группах, прячет пустые группы ---
    let query = '';
    $: q = query.trim().toLowerCase();
    $: view = q
        ? groups.map(gr => ({ ...gr, items: gr.items.filter(g => (g.title || '').toLowerCase().includes(q)) })).filter(gr => gr.items.length)
        : groups;

    // --- Drag&Drop игр на заголовок коллекции ---
    let dragOverKey = '';
    function onDragOver(e, key) { if (draggingGame) { e.preventDefault(); dragOverKey = key; } }
    function onDragLeave(key) { if (dragOverKey === key) dragOverKey = ''; }
    function onDrop(e, col) { e.preventDefault(); dragOverKey = ''; onDropGame(col); }

    function navCls(active) {
        return 'w-full flex items-center gap-2.5 text-left px-3 py-2 rounded-lg transition-colors text-sm ' + (active
            ? 'bg-indigo-500/20 text-white font-semibold ring-1 ring-inset ring-indigo-400/30'
            : 'text-slate-300 hover:bg-white/5 hover:text-white');
    }
</script>

<aside class="glass-strong flex flex-col z-10 relative shrink-0" style="width:{width}px">
    <div class="px-4 pt-4 pb-4">
        <h1 class="text-2xl font-black tracking-wide">
            <span class="bg-gradient-to-r from-indigo-400 to-fuchsia-400 bg-clip-text text-transparent">pLauncher</span>
        </h1>
    </div>

    <nav class="flex-1 overflow-y-auto px-2 pb-2">
        <div class="space-y-0.5 mb-3 px-1">
            <button on:click={() => onFilter('all')} class={navCls(isHome)}>🏠 {$t('nav.home')}</button>
            <button on:click={() => onFilter('favorites')} class={navCls(activeFilter === 'favorites')}>
                <span>❤️</span> {$t('nav.favorites')}
                {#if favoritesCount}<span class="ml-auto text-xs text-slate-400">{favoritesCount}</span>{/if}
            </button>
            {#if updatesCount}
                <button on:click={() => onFilter('updates')} class={navCls(activeFilter === 'updates')}>
                    <span class="text-amber-300">⬆</span> {$t('nav.updates')}
                    <span class="ml-auto text-xs font-bold text-amber-300">{updatesCount}</span>
                </button>
            {/if}
            {#if noCoverCount}
                <button on:click={() => onFilter('nocover')} class={navCls(activeFilter === 'nocover')} title={$t('nav.no_cover_hint')}>
                    <span class="text-slate-400">▢</span> {$t('nav.no_cover')}
                    <span class="ml-auto text-xs text-slate-400">{noCoverCount}</span>
                </button>
            {/if}
            {#if missingCount}
                <button on:click={onRelink} class={navCls(false)} title={$t('missing.nav_hint')}>
                    <span class="text-amber-400">⚠</span> {$t('missing.nav')}
                    <span class="ml-auto text-xs font-bold text-amber-400">{missingCount}</span>
                </button>
            {/if}
        </div>

        <div class="flex items-center justify-between px-3 mt-1 mb-1">
            <span class="text-[10px] font-bold text-slate-500 uppercase tracking-widest">{$t("nav.collections")}</span>
            <button on:click={onCreateCollection} title={$t("nav.create_collection")}
                    class="w-5 h-5 rounded flex items-center justify-center text-slate-400 hover:text-white hover:bg-white/10 transition-colors text-base leading-none">＋</button>
        </div>

        <div class="px-1 mb-1.5 relative">
            <input type="text" bind:value={query} placeholder={$t('search.sidebar')}
                   class="w-full bg-slate-900/60 border border-white/10 rounded-md pl-2.5 pr-7 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-400/50 transition-colors" />
            {#if query}
                <button on:click={() => query = ''} title={$t('btn.clear')}
                        class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 hover:text-white text-sm leading-none">×</button>
            {/if}
        </div>

        {#each view as group (group.key)}
            <div>
                <div class="w-full flex items-center gap-1.5 px-2 py-1.5 rounded-md transition-colors group {dragOverKey === group.key ? 'bg-indigo-500/25 ring-1 ring-indigo-400/50' : 'text-slate-300 hover:bg-white/5'}"
                     on:contextmenu={group.col ? (e) => openCtx(e, collectionMenu(group.col)) : undefined}
                     on:dragover={group.col ? (e) => onDragOver(e, group.key) : undefined}
                     on:dragleave={group.col ? () => onDragLeave(group.key) : undefined}
                     on:drop={group.col ? (e) => onDrop(e, group.col) : undefined}>
                    <button on:click={() => toggleGroup(group.key)} class="flex items-center gap-1.5 min-w-0 flex-1 text-left hover:text-white">
                        <span class="text-[10px] text-slate-500 transition-transform duration-200 {collapsed[group.key] ? '-rotate-90' : ''}">▼</span>
                        <span class="text-[11px] font-bold uppercase tracking-wider truncate">{group.name}</span>
                        {#if group.col && group.col.hidden}<span class="text-amber-300 text-[10px]" title={$t('privacy.hidden_col')}>🔒</span>{/if}
                        {#if group.col && group.col.type === 'dynamic'}<span class="text-indigo-400 text-[10px]" title={$t("nav.dynamic")}>⚡</span>{/if}
                    </button>
                    <span class="text-[11px] text-slate-500">{group.items.length}</span>
                    {#if group.col}
                        <button on:click|stopPropagation={() => onEditCollection(group.col)} title={$t("nav.edit_collection")}
                                class="text-slate-500 hover:text-white opacity-0 group-hover:opacity-100 transition-opacity text-xs">✎</button>
                    {/if}
                </div>
                {#if !collapsed[group.key] || q}
                    <div class="ml-2 border-l border-white/10 pl-1.5 mb-1">
                        {#each group.items as g (g.id)}
                            <button on:click={(e) => (e.ctrlKey || e.metaKey || $selecting) ? toggleSelected(g.id) : onSelect(g)}
                                    draggable="true"
                                    on:dragstart={(e) => { onGameDragStart(g); try { e.dataTransfer.setData('text/plain', g.id); } catch (_) {} }}
                                    on:contextmenu={(e) => onGameContext(g, e)}
                                    class="group w-full flex items-center gap-2.5 px-2 py-1 rounded-md text-sm text-left transition-colors {$selection.has(g.id) ? 'bg-indigo-500/25 text-white ring-1 ring-inset ring-indigo-400/50' : selectedId === g.id ? 'bg-indigo-500/20 text-white' : 'text-slate-400 hover:bg-white/5 hover:text-slate-100'}">
                                <span class="w-7 h-7 rounded-md overflow-hidden bg-slate-800 ring-1 ring-white/10 shrink-0 flex items-center justify-center">
                                    {#if g.cover_path}
                                        <img src={mediaSrc(g.cover_path)} alt="" on:error={(e) => e.target.style.display = 'none'} class="w-full h-full object-cover transition {blurCls($coverBlur, 'icon')}"/>
                                    {:else}
                                        <span class="text-[11px] text-slate-500 uppercase">{(g.title || '?').slice(0, 1)}</span>
                                    {/if}
                                </span>
                                <span class="truncate {g.folder_missing ? 'opacity-60' : ''}">{g.title}</span>
                                {#if g.folder_missing}<span class="ml-auto text-amber-400 text-xs shrink-0" title={$t('missing.card_hint')}>⚠</span>{/if}
                            </button>
                        {/each}
                        {#if group.items.length === 0}
                            <p class="text-xs text-slate-600 px-2 py-1">{$t("nav.empty")}</p>
                        {/if}
                    </div>
                {/if}
            </div>
        {/each}

        {#if totalGames === 0}
            <p class="text-xs text-slate-600 px-3 py-4 text-center">{$t("nav.lib_empty")}</p>
        {:else if q && view.length === 0}
            <p class="text-xs text-slate-600 px-3 py-4 text-center">{$t("lib.not_found_title")}</p>
        {/if}
    </nav>

    <div class="px-3 py-2 border-t border-white/10 flex items-center justify-between">
        <span class="text-xs text-slate-500 tracking-wide">{$t("nav.games", { n: libCount })}</span>
        <button on:click={onSettings} title={$t("nav.settings")}
                class="relative w-8 h-8 rounded-lg flex items-center justify-center text-slate-400 hover:text-white hover:bg-white/10 transition-colors">
            ⚙️
            {#if settingsBadge}<span class="absolute top-1 right-1 w-2 h-2 rounded-full bg-emerald-400 ring-2 ring-slate-900"></span>{/if}
        </button>
    </div>
</aside>
