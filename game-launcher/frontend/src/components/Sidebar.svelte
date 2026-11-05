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
    export let activeCollection = '';       // открытая коллекция (подсветка строки)
    export let onOpenCollection = (key) => {};
    export let selectedId = '';
    export let draggingGame = null;         // игра, которую тащат (из сетки или из сайдбара)
    export let scanning = false;
    export let checkingUpdates = false;

    export let onFilter = (f) => {};
    export let onSelect = (g) => {};
    export let onPlay = (g) => {};
    export let onScan = () => {};
    export let onAddGame = () => {};
    export let onCheckUpdates = () => {};
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
        return 'w-full h-8 flex items-center gap-2.5 text-left px-2.5 rounded-md transition-colors text-[13px] ' + (active
            ? 'bg-slate-800 text-white font-medium'
            : 'text-slate-300 hover:bg-white/5 hover:text-white');
    }
    const canPlay = (g) => g.exec_path && !g.folder_missing;
    const toolBtn = 'w-10 h-10 shrink-0 rounded-xl flex items-center justify-center text-slate-300 bg-white/5 border border-white/10 hover:bg-white/10 hover:text-white transition-colors disabled:opacity-50';
    const sectionCls = 'px-2.5 mb-1.5 text-[11px] font-semibold text-slate-500 uppercase tracking-wider';
    $: hasSmart = updatesCount || noCoverCount || missingCount;
    // «Все игры» подсвечены, пока не открыта коллекция или умный список (чипсы «Избранное» и т. п. — внутри них)
    $: allActive = !activeCollection && !['updates', 'nocover', 'missing'].includes(activeFilter);
</script>

<aside class="glass-strong flex flex-col z-10 relative shrink-0" style="width:{width}px">
    <!-- Частые действия: всегда под рукой, а не в меню -->
    <div class="px-3 pt-3 pb-3 flex gap-2">
        <button on:click={onScan} disabled={scanning} title={$t('nav.scan_hint')}
                class="flex-1 min-w-0 h-10 rounded-xl flex items-center justify-center gap-2 text-sm font-semibold text-indigo-100 bg-indigo-500/25 border border-indigo-400/40 hover:bg-indigo-500/35 transition-colors disabled:cursor-default">
            <svg class="w-4 h-4 shrink-0 {scanning ? 'animate-spin' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-2.64-6.36"/><path d="M21 3v6h-6"/></svg>
            <span class="truncate">{scanning ? $t('menu.scanning') : $t('menu.scan')}</span>
        </button>
        <button on:click={onAddGame} title={$t('menu.add_game')} aria-label={$t('menu.add_game')} class={toolBtn}>
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
        </button>
        <button on:click={onCheckUpdates} disabled={checkingUpdates} title={checkingUpdates ? $t('menu.checking') : $t('menu.check_updates')}
                aria-label={$t('menu.check_updates')} class={toolBtn}>
            <svg class="w-4 h-4 {checkingUpdates ? 'animate-pulse' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5m0 0l-6 6m6-6l6 6"/></svg>
        </button>
    </div>

    <nav class="flex-1 overflow-y-auto px-2 pb-2">


        <!-- Коллекции: щелчок по названию открывает сетку, стрелка раскрывает список игр -->
        <div class="mb-4 px-1">
            <div class="flex items-center justify-between pr-1">
                <span class={sectionCls}>{$t("nav.collections")}</span>
                <button on:click={onCreateCollection} title={$t("nav.create_collection")} aria-label={$t("nav.create_collection")}
                        class="w-6 h-6 -mt-1.5 rounded flex items-center justify-center text-slate-400 hover:text-white hover:bg-white/10 transition-colors">
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
                </button>
            </div>

            {#if groups.length > 6 || query}
                <div class="mb-1.5 relative">
                    <input type="text" bind:value={query} placeholder={$t('search.sidebar')}
                           class="w-full h-8 bg-slate-950 border border-white/10 rounded-md pl-2.5 pr-7 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-400/50 transition-colors" />
                    {#if query}
                        <button on:click={() => query = ''} title={$t('btn.clear')}
                                class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 hover:text-white text-sm leading-none">×</button>
                    {/if}
                </div>
            {/if}

            <!-- Системная коллекция «Все игры» -->
            <button on:click={() => onFilter('all')}
                    class="w-full h-8 flex items-center gap-1.5 pr-2.5 rounded-md text-[13px] transition-colors {allActive ? 'bg-slate-800 text-white font-medium' : 'text-slate-300 hover:bg-white/5 hover:text-white'}">
                <span class="w-6 flex items-center justify-center text-slate-500">
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"><rect x="3.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.5"/></svg>
                </span>
                {$t('nav.all_games')}<span class="ml-auto text-xs text-slate-500 font-normal">{libCount}</span>
            </button>

            {#each view as group (group.key)}
                {@const open = !collapsed[group.key] || q}
                <div>
                    <div class="w-full h-8 flex items-center rounded-md transition-colors group {dragOverKey === group.key ? 'bg-indigo-900 ring-1 ring-indigo-400/60' : activeCollection === group.key ? 'bg-slate-800 text-white' : 'text-slate-300 hover:bg-white/5'}"
                         on:contextmenu={group.col ? (e) => openCtx(e, collectionMenu(group.col)) : undefined}
                         on:dragover={group.col ? (e) => onDragOver(e, group.key) : undefined}
                         on:dragleave={group.col ? () => onDragLeave(group.key) : undefined}
                         on:drop={group.col ? (e) => onDrop(e, group.col) : undefined}>
                        <button on:click={() => toggleGroup(group.key)} aria-label={open ? $t('ctx.collapse') : $t('ctx.expand')}
                                class="w-6 h-8 shrink-0 flex items-center justify-center text-slate-500 hover:text-white">
                            <svg class="w-3 h-3 transition-transform duration-150 {open ? 'rotate-90' : ''}" viewBox="0 0 24 24" fill="currentColor"><path d="M9 6l6 6-6 6z"/></svg>
                        </button>
                        <button on:click={() => onOpenCollection(group.key)} class="flex items-center gap-1.5 min-w-0 flex-1 h-full text-left text-[13px] hover:text-white">
                            <span class="truncate {activeCollection === group.key ? 'font-medium' : ''}">{group.name}</span>
                            {#if group.col && group.col.hidden}<span class="text-amber-300 text-[10px]" title={$t('privacy.hidden_col')}>🔒</span>{/if}
                            {#if group.col && group.col.type === 'dynamic'}<span class="text-indigo-400 text-[10px]" title={$t("nav.dynamic")}>⚡</span>{/if}
                        </button>
                        {#if group.col}
                            <button on:click|stopPropagation={() => onEditCollection(group.col)} title={$t("nav.edit_collection")} aria-label={$t("nav.edit_collection")}
                                    class="w-6 h-6 rounded text-slate-500 hover:text-white opacity-0 group-hover:opacity-100 transition-opacity text-xs">✎</button>
                        {/if}
                        <span class="text-xs text-slate-500 pl-1 pr-2.5">{group.items.length}</span>
                    </div>
                    {#if open}
                        <div class="ml-3 border-l border-white/10 pl-1.5 mb-1">
                            {#each group.items as g (g.id)}
                                <div draggable="true"
                                     on:dragstart={(e) => { onGameDragStart(g); try { e.dataTransfer.setData('text/plain', g.id); } catch (_) {} }}
                                     on:contextmenu={(e) => onGameContext(g, e)}
                                     class="group w-full flex items-center rounded-md transition-colors {$selection.has(g.id) ? 'bg-indigo-900 text-white ring-1 ring-inset ring-indigo-400/50' : selectedId === g.id ? 'bg-slate-800 text-white' : 'text-slate-400 hover:bg-white/5 hover:text-slate-100'}">
                                    <button on:click={(e) => (e.ctrlKey || e.metaKey || $selecting) ? toggleSelected(g.id) : onSelect(g)}
                                            class="flex-1 min-w-0 flex items-center gap-2.5 px-2 h-8 text-[13px] text-left">
                                        <span class="w-5 h-5 rounded overflow-hidden bg-slate-800 shrink-0 flex items-center justify-center">
                                            {#if g.cover_path}
                                                <img src={mediaSrc(g.cover_path)} alt="" on:error={(e) => e.target.style.display = 'none'} class="w-full h-full object-cover transition {blurCls($coverBlur, 'icon')}"/>
                                            {:else}
                                                <span class="text-[10px] text-slate-500 uppercase">{(g.title || '?').slice(0, 1)}</span>
                                            {/if}
                                        </span>
                                        <span class="truncate {g.folder_missing ? 'opacity-60' : ''}">{g.title}</span>
                                        {#if g.folder_missing}<span class="ml-auto text-amber-400 text-xs shrink-0" title={$t('missing.card_hint')}>⚠</span>{/if}
                                    </button>
                                    {#if canPlay(g) && !$selecting}
                                        <!-- Запуск прямо из дерева, без захода на страницу игры -->
                                        <button on:click={() => onPlay(g)} title={$t('btn.play')} aria-label={$t('btn.play') + ' · ' + g.title}
                                                class="mr-1 h-6 px-2 shrink-0 rounded-md flex items-center gap-1 text-[11px] font-bold text-white bg-indigo-500 hover:bg-indigo-400 opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity">
                                            <svg class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="currentColor"><path d="M7 4l13 8-13 8z"/></svg>
                                            {$t('btn.play')}
                                        </button>
                                    {/if}
                                </div>
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
        </div>

        <!-- Умные списки: появляются, только когда в них что-то есть -->
        {#if hasSmart}
            <div class="mb-2 px-1">
                <div class={sectionCls}>{$t('nav.smart')}</div>
                {#if updatesCount}
                    <button on:click={() => onFilter('updates')} class={navCls(activeFilter === 'updates' && !activeCollection)}>
                        <svg class="w-4 h-4 shrink-0 text-amber-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5m0 0l-6 6m6-6l6 6"/></svg> {$t('nav.updates')}
                        <span class="ml-auto text-xs font-semibold text-amber-300">{updatesCount}</span>
                    </button>
                {/if}
                {#if noCoverCount}
                    <button on:click={() => onFilter('nocover')} class={navCls(activeFilter === 'nocover' && !activeCollection)} title={$t('nav.no_cover_hint')}>
                        <svg class="w-4 h-4 shrink-0 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 15l5-5 4 4 3-3 6 6"/></svg> {$t('nav.no_cover')}
                        <span class="ml-auto text-xs text-slate-500">{noCoverCount}</span>
                    </button>
                {/if}
                {#if missingCount}
                    <button on:click={() => onFilter('missing')} class={navCls(activeFilter === 'missing' && !activeCollection)} title={$t('missing.nav_hint')}>
                        <svg class="w-4 h-4 shrink-0 text-rose-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 10v3m0 3h.01"/></svg> {$t('missing.nav')}
                        <span class="ml-auto text-xs font-semibold text-rose-300">{missingCount}</span>
                    </button>
                {/if}
            </div>
        {/if}
    </nav>
</aside>
