<script>
    // Корень приложения: состояние библиотеки, навигация и связка компонентов.
    // Разметка крупных частей — в components/, чистые функции — в lib/.
    import { onMount, onDestroy } from 'svelte';
    import {
        AddGamesFromDrop,
        GetGames,
        GetScanPaths,
        ScanAllFolders,
        SaveWindowSize,
        Launch,
        OpenFolder,
        RemoveGame,
        SearchGames,
        UpdateGame,
        CheckAllUpdates,
        IsConfigured,
        RelinkGame,
        SelectRelinkFolder,
        GetCollections,
        SaveCollections,
        GetSupportedSources,
        GetCustomLocales,
        GetAppVersion,
        CheckLauncherUpdate,
    } from '../wailsjs/go/main/App.js';
    import { OnFileDrop, OnFileDropOff, EventsOn } from '../wailsjs/runtime/runtime';
    import { t, tr, setLang, addLocales, availableLangs, initialLang, langStore } from './i18n.js';
    import { showToast, askConfirm, isConfirmOpen } from './lib/ui.js';
    import { mediaSrc, sortGames, lsGet, lsSet } from './lib/util.js';
    import { loadShelves, buildShelf, buildCol } from './lib/shelves.js';
    import { isEditableTarget, inputCtxItems } from './lib/inputMenu.js';

    import TitleBar from './components/TitleBar.svelte';
    import Sidebar from './components/Sidebar.svelte';
    import Hero from './components/Hero.svelte';
    import Shelf from './components/Shelf.svelte';
    import GameGrid from './components/GameGrid.svelte';
    import SortSelect from './components/SortSelect.svelte';
    import ShelfEditor from './components/ShelfEditor.svelte';
    import GameDetail from './components/GameDetail.svelte';
    import GameEditor from './components/GameEditor.svelte';
    import Lightbox from './components/Lightbox.svelte';
    import SetupWizard from './components/SetupWizard.svelte';
    import SettingsModal from './components/SettingsModal.svelte';
    import CollectionModal from './components/CollectionModal.svelte';
    import RelinkModal from './components/RelinkModal.svelte';
    import ConfirmDialog from './components/ConfirmDialog.svelte';
    import ContextMenu from './components/ContextMenu.svelte';
    import Toast from './components/Toast.svelte';

    // --- Состояние библиотеки и навигации ---
    let games = [];
    let scanning = false;
    let checkingUpdates = false;
    let selectedGame = null;
    let isEditing = false;
    let originalGameSnapshot = '';       // слепок игры для «есть ли несохранённые изменения»
    let searchQuery = '';
    let activeFilter = 'all';
    let activeTag = '';                  // выбранный тег (пусто — не фильтруем)
    let discreet = false;                // дискретный режим: блюр обложек (Ctrl+H)
    let isDragging = false;              // оверлей при перетаскивании папок из проводника
    let scanPaths = [];
    let supportedSources = [];
    let appVersion = '';
    let launcherUpdate = null;           // результат проверки обновления лаунчера

    // Тихая проверка обновления лаунчера — не чаще раза в сутки
    async function autoCheckLauncherUpdate() {
        const last = parseInt(lsGet('plauncher_upd_checked', '0')) || 0;
        if (appVersion === 'dev' || Date.now() - last < 24 * 3600 * 1000) return;
        try {
            launcherUpdate = await CheckLauncherUpdate();
            lsSet('plauncher_upd_checked', String(Date.now()));
            if (launcherUpdate.available) showToast(tr('upd.available', { v: launcherUpdate.latest }), 'success');
        } catch (e) { /* нет сети или релизов — молча */ }
    }

    let sortBy = lsGet('plauncher_sort', 'added_desc');
    $: lsSet('plauncher_sort', sortBy);

    // «Только оформленные» — скрывать игры без обложки
    let onlyDressed = lsGet('plauncher_only_dressed') === '1';
    $: lsSet('plauncher_only_dressed', onlyDressed ? '1' : '0');

    // Модалки
    let needsSetup = false;
    let showSettings = false;
    let showRelink = false;
    let collectionModal = null;          // null — закрыто; {collection: null|col}
    let lightboxIndex = null;            // null — закрыто
    let ctx = null;                      // {x, y, items} — контекстное меню

    // --- Язык интерфейса ---
    let langs = availableLangs();
    setLang(initialLang());
    async function loadCustomLocales() {
        try {
            const extra = await GetCustomLocales();
            if (extra && Object.keys(extra).length) { addLocales(extra); langs = availableLangs(); }
        } catch (e) {}
    }
    function changeLang(code) { setLang(code); langs = availableLangs(); }

    // --- Производные списки ---
    // Базовый список, на котором строится вся библиотека/полки/коллекции
    $: libGames = onlyDressed ? games.filter(g => g.cover_path) : games;
    $: filteredGames = sortGames(libGames.filter(g => {
        if (activeTag && !(g.tags || []).includes(activeTag)) return false;
        if (activeFilter === 'favorites') return g.favorite;
        if (activeFilter === 'updates') return g.update_available;
        return true;
    }), sortBy);
    $: allGamesSorted = sortGames(libGames, sortBy);
    $: updatesGames = libGames.filter(g => g.update_available);
    $: missingGames = games.filter(g => g.folder_missing);
    $: recentlyPlayed = libGames
        .filter(g => (g.last_launched_at || 0) > 0)
        .sort((a, b) => (b.last_launched_at || 0) - (a.last_launched_at || 0));
    $: recentlyAdded = [...libGames].sort((a, b) => (b.added_at || 0) - (a.added_at || 0));
    $: favoriteGames = libGames.filter(g => g.favorite);
    $: heroGame = recentlyPlayed[0] || recentlyAdded[0] || null;
    // Домашний вид: без поиска, фильтра и выбранного тега
    $: isHome = searchQuery.trim() === '' && activeFilter === 'all' && !activeTag;

    // Заголовок текущего вида ($langStore — зависимость от языка)
    $: viewTitle = ($langStore,
        activeTag ? tr('view.tag', { tag: activeTag })
        : searchQuery.trim() ? tr('view.search')
        : activeFilter === 'favorites' ? tr('shelf.favorites')
        : activeFilter === 'updates' ? tr('nav.updates')
        : tr('lib.title'));

    // Уникальные теги по всей библиотеке, по частоте
    $: allTags = (() => {
        const counts = {};
        for (const g of games) for (const tag of (g.tags || [])) counts[tag] = (counts[tag] || 0) + 1;
        return Object.keys(counts).sort((a, b) => counts[b] - counts[a]);
    })();

    // --- Коллекции (как в Steam: ручные + динамические по тегам) ---
    let collections = [];
    $: collectionsView = collections.map(c => buildCol(c, libGames));
    $: categorizedIds = new Set(collectionsView.flatMap(c => c.items.map(g => g.id)));
    $: uncategorized = sortGames(libGames.filter(g => !categorizedIds.has(g.id)), 'title_asc');
    $: sidebarGroups = ($langStore, [
        ...collectionsView.map(c => ({ key: c.id, name: c.name, items: sortGames(c.items, 'title_asc'), col: c })),
        ...(uncategorized.length ? [{ key: '__uncat', name: tr('nav.uncategorized'), items: uncategorized, col: null }] : []),
    ]);
    $: manualCollections = collections.filter(c => c.type === 'manual');

    async function loadCollections() {
        try { collections = await GetCollections() || []; } catch (e) { console.error(e); }
    }
    async function persistCollections() {
        try { await SaveCollections(collections); } catch (err) { showToast(tr('toast.col_fail', { err }), 'error'); }
    }
    function openCreateCollection() { collectionModal = { collection: null }; }
    function openEditCollection(c) { collectionModal = { collection: c }; }
    async function saveCollection({ name, type, tags }) {
        const editing = collectionModal && collectionModal.collection;
        if (editing) {
            collections = collections.map(c => c.id === editing.id
                ? { ...c, name, type, tags: type === 'dynamic' ? tags : (c.tags || []) }
                : c);
        } else {
            collections = [...collections, {
                id: 'c_' + Math.random().toString(36).slice(2, 9),
                name, type, game_ids: [], tags: type === 'dynamic' ? tags : [],
            }];
        }
        collectionModal = null;
        await persistCollections();
    }
    async function deleteCollection(c) {
        const ok = await askConfirm({ title: tr('dlg.delete_collection_title'), message: tr('dlg.delete_collection_msg', { name: c.name }), confirmText: tr('btn.delete'), danger: true });
        if (!ok) return;
        collections = collections.filter(x => x.id !== c.id);
        collectionModal = null;
        shelves = shelves.filter(s => !(s.type === 'collection' && s.collectionId === c.id));
        await persistCollections();
    }
    // Ручные коллекции: добавить/убрать игру
    async function toggleGameInCollection(colId, game) {
        collections = collections.map(c => {
            if (c.id !== colId) return c;
            const ids = new Set(c.game_ids || []);
            if (ids.has(game.id)) ids.delete(game.id); else ids.add(game.id);
            return { ...c, game_ids: [...ids] };
        });
        await persistCollections();
    }
    // Принудительно добавить игру (Drag&Drop; работает и с динамическими)
    async function addGameToCollection(col, game) {
        if ((col.game_ids || []).includes(game.id)) return;
        collections = collections.map(c => c.id === col.id ? { ...c, game_ids: [...(c.game_ids || []), game.id] } : c);
        await persistCollections();
        showToast(tr('toast.col_added', { title: game.title, name: col.name }), 'success');
    }

    // --- Полки главной (порядок/состав, как в Steam) ---
    let shelves = loadShelves();
    $: lsSet('plauncher_shelves', JSON.stringify(shelves));
    let layoutEditing = false;
    // Зависимости перечислены явно, чтобы Svelte пересчитывал при изменении данных
    $: shelfData = { recentlyPlayed, recentlyAdded, favoriteGames, allGamesSorted, games: libGames, collectionsView, _lang: $langStore };
    $: renderedShelves = shelves.map(s => buildShelf(s, shelfData));
    function shelfMenuItems(shelf) {
        return [
            { label: tr('ctx.configure_sections'), icon: '✎', action: () => layoutEditing = true },
            { label: tr('ctx.remove_section'), icon: '🗑', danger: true, action: () => shelves = shelves.filter(s => s.id !== shelf.id) },
        ];
    }

    // --- Ширина сайдбара (resizable, как в Steam) ---
    let sidebarWidth = (() => {
        const v = parseInt(lsGet('plauncher_sidebar_w'));
        return (v && v >= 200 && v <= 520) ? v : 256;
    })();
    let resizingSidebar = false;
    function startSidebarResize(e) {
        e.preventDefault();
        resizingSidebar = true;
        const onMove = (ev) => { sidebarWidth = Math.max(200, Math.min(520, ev.clientX)); };
        const onUp = () => {
            resizingSidebar = false;
            lsSet('plauncher_sidebar_w', String(sidebarWidth));
            window.removeEventListener('mousemove', onMove);
            window.removeEventListener('mouseup', onUp);
        };
        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
    }

    // --- Загрузка данных ---
    async function loadGames() {
        try {
            games = await GetGames() || [];
            // Открыта карточка (и не редактируем) — подхватываем свежие данные
            if (selectedGame && !isEditing) {
                const fresh = games.find(g => g.id === selectedGame.id);
                if (fresh) selectedGame = fresh;
            }
        } catch (err) {
            console.error('Ошибка загрузки игр:', err);
        }
    }
    // Полная перезагрузка библиотеки (смена папки данных, импорт, очистка)
    async function reloadLibrary(resetSelection) {
        if (resetSelection) { selectedGame = null; isEditing = false; }
        try { scanPaths = await GetScanPaths(); } catch (e) {}
        await loadCollections();
        await loadGames();
    }

    async function handleScan() {
        if (!scanPaths.length) { showToast(tr('toast.scan_first'), 'error'); return; }
        scanning = true;
        try {
            const count = await ScanAllFolders();
            await loadGames();
            showToast(count > 0 ? tr('toast.scan_added', { n: count }) : tr('toast.scan_none'), 'success');
        } catch (err) {
            showToast(tr('toast.error', { err }), 'error');
        } finally {
            scanning = false;
        }
    }

    async function handleCheckAllUpdates() {
        if (checkingUpdates) return;
        checkingUpdates = true;
        try {
            const n = await CheckAllUpdates();
            await loadGames();
            showToast(n > 0 ? tr('toast.updates_found_n', { n }) : tr('toast.update_none'), 'success');
        } catch (err) { showToast(tr('toast.update_fail', { err }), 'error'); }
        finally { checkingUpdates = false; }
    }

    async function handleSearch() {
        try {
            games = searchQuery.trim() === '' ? (await GetGames() || []) : (await SearchGames(searchQuery) || []);
        } catch (err) {
            console.error('Ошибка поиска:', err);
        }
    }

    // --- Действия с играми ---
    function selectGame(game) {
        selectedGame = game;
        isEditing = false;
        originalGameSnapshot = JSON.stringify(game);
    }
    function closeGame() {
        selectedGame = null;
        isEditing = false;
    }
    function applyFilter(filter) {
        activeFilter = filter;
        activeTag = '';
        closeGame();
    }
    function applyTag(tag) {
        activeTag = activeTag === tag ? '' : tag;
        activeFilter = 'all';
        searchQuery = '';
        closeGame();
    }

    async function toggleFavorite(game) {
        if (!game) return;
        game.favorite = !game.favorite;
        games = games; // game — ссылка внутри массива
        if (selectedGame && selectedGame.id === game.id) selectedGame.favorite = game.favorite;
        try { await UpdateGame(game); }
        catch (err) { showToast(tr('toast.fav_fail', { err }), 'error'); }
    }

    async function playGame(game) {
        if (!game) return;
        if (!game.exec_path || game.folder_missing) { selectGame(game); return; }
        try {
            await Launch(game.id, game.exec_path, game.folder_path);
            await loadGames(); // «последний запуск» → в начало
        } catch (err) {
            showToast(tr('toast.launch_fail', { err }), 'error');
        }
    }

    async function removeGame(game) {
        const ok = await askConfirm({ title: tr('dlg.remove_game_title'), message: tr('dlg.remove_game_msg', { title: game.title }), confirmText: tr('btn.delete'), danger: true });
        if (!ok) return;
        try {
            await RemoveGame(game.id);
            if (selectedGame && selectedGame.id === game.id) closeGame();
            await loadGames();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

    // Указать новое расположение одной игры (диалог откроется у ближайшей уцелевшей папки)
    async function relinkManually(game) {
        try {
            const p = await SelectRelinkFolder(game.folder_path);
            if (!p) return;
            await RelinkGame(game.id, p);
            await loadGames();
            showToast(tr('missing.relinked', { title: game.title }), 'success');
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

    // --- Редактирование ---
    function startEdit() {
        originalGameSnapshot = JSON.stringify(selectedGame);
        isEditing = true;
    }
    async function cancelEdit() {
        if (JSON.stringify(selectedGame) === originalGameSnapshot) { isEditing = false; return; }
        const ok = await askConfirm({ title: tr('dlg.discard_title'), message: tr('dlg.discard_msg'), confirmText: tr('btn.discard'), danger: true });
        if (ok) {
            selectedGame = JSON.parse(originalGameSnapshot);
            isEditing = false;
        }
    }
    async function saveEdit() {
        try {
            await UpdateGame(selectedGame);
            isEditing = false;
            await loadGames();
            showToast(tr('toast.saved'), 'success');
        } catch (err) {
            showToast(tr('toast.save_fail', { err }), 'error');
        }
    }

    // --- Контекстные меню ---
    function openCtx(e, items) {
        e.preventDefault(); e.stopPropagation();
        const W = 232;
        ctx = {
            x: Math.min(e.clientX, window.innerWidth - W - 8),
            y: Math.min(e.clientY, window.innerHeight - 340),
            items,
        };
    }
    function gameMenuItems(game) {
        const items = [];
        if (game.folder_missing) items.push({ label: tr('missing.pick_long'), icon: '⚠', action: () => relinkManually(game) });
        else if (game.exec_path) items.push({ label: tr('ctx.play'), icon: '▶', action: () => playGame(game) });
        else items.push({ label: tr('ctx.set_exe'), icon: '▶', action: () => selectGame(game) });
        items.push({ label: game.favorite ? tr('ctx.fav_remove') : tr('ctx.fav_add'), icon: game.favorite ? '🤍' : '❤️', action: () => toggleFavorite(game) });
        items.push({
            label: tr('ctx.add_to'), icon: '＋',
            submenu: collections.map(c => ({ label: c.name, checked: (c.game_ids || []).includes(game.id), action: () => toggleGameInCollection(c.id, game) }))
                .concat([{ label: tr('ctx.new_collection'), action: openCreateCollection }]),
        });
        items.push({ sep: true });
        items.push({ label: tr('ctx.open_folder'), icon: '📁', action: () => OpenFolder(game.folder_path) });
        items.push({ label: tr('ctx.edit'), icon: '✎', action: () => { selectGame(game); startEdit(); } });
        items.push({ label: tr('ctx.remove_game'), icon: '🗑', danger: true, action: () => removeGame(game) });
        return items;
    }
    const gameCtx = (g, e) => openCtx(e, gameMenuItems(g));
    // Глобальный contextmenu: на текстовом поле — меню буфера, иначе блокируем нативное
    function handleGlobalContext(e) {
        if (isEditableTarget(e.target)) { openCtx(e, inputCtxItems(e.target)); return; }
        e.preventDefault();
    }

    // --- Drag&Drop игр в коллекции ---
    let draggingGame = null;
    function onGameDragStart(game) { draggingGame = game; }
    async function onDropGame(col) {
        const g = draggingGame;
        draggingGame = null;
        if (g && col) await addGameToCollection(col, g);
    }

    // --- Drag&Drop папок из проводника ---
    // DOM-события dragover/dragleave нужны только для оверлея; реальные абсолютные пути
    // приходят из нативного OnFileDrop (WebView2 не отдаёт их в dataTransfer).
    function isFileDrag(e) {
        return e.dataTransfer && Array.from(e.dataTransfer.types || []).includes('Files');
    }
    function onDragOver(e) { if (isFileDrag(e)) { e.preventDefault(); isDragging = true; } }
    function onDragLeave(e) { if (isFileDrag(e)) { e.preventDefault(); isDragging = false; } }
    async function handleNativeDrop(paths) {
        isDragging = false;
        if (!paths || paths.length === 0) return;
        try {
            const added = await AddGamesFromDrop(paths);
            await loadGames();
            showToast(added > 0 ? tr('toast.scan_added', { n: added }) : tr('toast.scan_none'), 'success');
        } catch (err) {
            showToast(tr('toast.error', { err }), 'error');
        }
    }

    // --- Клавиатура и мышь ---
    // Диалог подтверждения и лайтбокс обрабатывают свои клавиши сами и вызывают
    // preventDefault — такие события здесь пропускаются.
    function handleKeydown(e) {
        // Дискретный режим (Boss key): Ctrl+H — мгновенно заблюрить/показать обложки
        if (e.ctrlKey && ['h', 'H', 'р', 'Р'].includes(e.key)) {
            e.preventDefault();
            discreet = !discreet;
            return;
        }
        if (e.defaultPrevented || isConfirmOpen() || lightboxIndex !== null) return;
        if (e.key !== 'Escape') return;
        // Escape закрывает верхний слой: меню → модалка → редактор → карточка игры
        if (ctx) ctx = null;
        else if (collectionModal) collectionModal = null;
        else if (showRelink) showRelink = false;
        else if (showSettings) showSettings = false;
        else if (isEditing) isEditing = false;
        else if (selectedGame) closeGame();
    }
    // Боковая кнопка мыши «Назад» — выйти из карточки игры в сетку
    function handleGlobalMouseDown(e) {
        if (e.button !== 3 && e.button !== 4) return;
        const handled = e.defaultPrevented; // лайтбокс уже обработал нажатие
        e.preventDefault();
        if (handled || lightboxIndex !== null) return;
        if (e.button === 3 && selectedGame && !isEditing) closeGame();
    }

    // --- Старт ---
    let resizeTimer = null;
    function onWindowResize() {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(() => { SaveWindowSize().catch(() => {}); }, 500);
    }

    onMount(async () => {
        OnFileDrop((x, y, paths) => handleNativeDrop(paths), false);
        // Бэкенд шлёт событие, когда обновилось время в игре (после выхода из игры)
        EventsOn('games-updated', () => loadGames());
        window.addEventListener('resize', onWindowResize);

        try { supportedSources = await GetSupportedSources() || []; } catch (e) {}
        try { appVersion = await GetAppVersion(); } catch (e) {}

        // Первый запуск: папка для данных ещё не выбрана — мастер настройки
        try {
            if (!(await IsConfigured())) { needsSetup = true; return; }
        } catch (err) {
            console.error('Ошибка проверки конфигурации:', err);
        }
        await reloadLibrary(false);
        await loadCustomLocales();
        autoCheckLauncherUpdate();
    });

    onDestroy(() => {
        OnFileDropOff();
        window.removeEventListener('resize', onWindowResize);
    });

    async function onSetupConfigured() {
        await loadCollections();
        await loadCustomLocales();
    }
    async function onSetupFinish(paths) {
        needsSetup = false;
        scanPaths = paths;
        if (scanPaths.length) await handleScan(); // первичное сканирование
        else await loadGames();
    }
</script>

<svelte:window on:keydown={handleKeydown} on:mousedown={handleGlobalMouseDown}/>
<div class="flex flex-col h-screen overflow-hidden aurora-bg text-slate-300 font-sans relative"
     on:dragover={onDragOver}
     on:dragleave={onDragLeave}
     on:contextmenu={handleGlobalContext}>

    <TitleBar/>

    <div class="flex flex-1 overflow-hidden min-h-0 relative">
        {#if isDragging}
            <div class="absolute inset-0 bg-indigo-900/80 backdrop-blur-sm z-50 flex flex-col items-center justify-center border-4 border-dashed border-indigo-400 m-4 rounded-2xl pointer-events-none transition-all">
                <div class="text-7xl mb-6">📥</div>
                <h2 class="text-4xl font-bold text-white mb-2 tracking-wider">{$t("drop.title")}</h2>
                <p class="text-indigo-200 text-lg">{$t("drop.sub")}</p>
            </div>
        {/if}

        <Sidebar width={sidebarWidth}
                 groups={sidebarGroups}
                 totalGames={games.length}
                 libCount={libGames.length}
                 favoritesCount={favoriteGames.length}
                 updatesCount={updatesGames.length}
                 missingCount={missingGames.length}
                 {isHome} {activeFilter}
                 selectedId={selectedGame ? selectedGame.id : ''}
                 {scanning} {checkingUpdates} {draggingGame}
                 settingsBadge={!!(launcherUpdate && launcherUpdate.available)}
                 bind:onlyDressed bind:discreet
                 onScan={handleScan}
                 onCheckAll={handleCheckAllUpdates}
                 onFilter={applyFilter}
                 onRelink={() => showRelink = true}
                 onSelect={selectGame}
                 onSettings={() => showSettings = true}
                 onCreateCollection={openCreateCollection}
                 onEditCollection={openEditCollection}
                 onDeleteCollection={deleteCollection}
                 onGameContext={gameCtx}
                 {onGameDragStart} {onDropGame} {openCtx}/>

        <!-- Отдельная колонка-разделитель: не перекрывает ни скроллбар сайдбара, ни контент -->
        <div on:mousedown={startSidebarResize} title={$t("app.resize")}
             class="w-1 shrink-0 self-stretch z-20 cursor-col-resize bg-white/5 hover:bg-indigo-400/50 transition-colors {resizingSidebar ? 'bg-indigo-400/60' : ''}"></div>

        <main class="flex-1 overflow-y-auto p-8 relative">
            {#if selectedGame}
                {#if selectedGame.cover_path && !isEditing}
                    <div class="absolute inset-0 z-0 overflow-hidden pointer-events-none">
                        <img src={mediaSrc(selectedGame.cover_path)} alt="bg" class="w-full h-full object-cover blur-2xl scale-110 opacity-20" />
                        <div class="absolute inset-0 bg-gradient-to-b from-[#0a0912]/70 via-[#0a0912]/90 to-[#0a0912]"></div>
                    </div>
                {/if}

                <div class="animate-fade-in relative z-10">
                    <button on:click={closeGame} class="mb-6 text-indigo-400 hover:text-indigo-300 transition-colors flex items-center gap-2 font-semibold">
                        {$t("detail.back")}
                    </button>

                    {#if isEditing}
                        <GameEditor bind:game={selectedGame} {supportedSources} onSave={saveEdit} onCancel={cancelEdit}/>
                    {:else}
                        <GameDetail game={selectedGame} {supportedSources} {manualCollections}
                                    onEdit={startEdit}
                                    onRemove={() => removeGame(selectedGame)}
                                    onToggleFav={toggleFavorite}
                                    onRelink={relinkManually}
                                    onRelinkAll={() => showRelink = true}
                                    onToggleCollection={toggleGameInCollection}
                                    onCreateCollection={openCreateCollection}
                                    onApplyTag={applyTag}
                                    onOpenImage={(i) => lightboxIndex = i}
                                    onReload={loadGames}/>
                    {/if}
                </div>

            {:else}
                <div class="flex justify-between items-center mb-7 animate-fade-in gap-4">
                    <div class="flex items-center gap-3 min-w-0">
                        <h2 class="text-3xl font-black text-white tracking-tight truncate">{viewTitle}</h2>
                        {#if !isHome}
                            <button on:click={() => { searchQuery = ''; applyFilter('all'); handleSearch(); }}
                                    class="shrink-0 flex items-center gap-1 text-xs text-slate-400 hover:text-white bg-white/5 hover:bg-white/10 px-2.5 py-1 rounded-full border border-white/10 transition-colors">
                                ✕ {$t("lib.reset")}
                            </button>
                        {/if}
                    </div>

                    <div class="relative w-80 shrink-0">
                        <input type="text" bind:value={searchQuery} on:input={handleSearch} placeholder={$t("search.placeholder")}
                               class="w-full glass text-white rounded-full pl-10 pr-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 transition-all placeholder:text-slate-500"/>
                        <div class="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none">🔍</div>
                    </div>
                </div>

                {#if isHome}
                    {#if layoutEditing}
                        <ShelfEditor bind:shelves {allTags} {collections}
                                     countFor={(s) => buildShelf(s, shelfData).items.length}
                                     onDone={() => layoutEditing = false}/>
                    {:else}
                        {#if heroGame}
                            <Hero game={heroGame} {discreet} onOpen={selectGame} onPlay={playGame} onToggleFav={toggleFavorite}/>
                        {/if}

                        {#if games.length}
                            <div class="flex justify-end mb-5 -mt-2">
                                <button on:click={() => layoutEditing = true}
                                        class="text-sm text-slate-400 hover:text-white bg-white/5 hover:bg-white/10 px-3 py-1.5 rounded-lg border border-white/10 transition-colors">
                                    ✎ {$t("lib.edit_sections")}
                                </button>
                            </div>
                        {/if}

                        {#each renderedShelves as shelf (shelf.id)}
                            {#if shelf.type === 'all'}
                                {#if shelf.items.length}
                                    <div class="flex items-center justify-between mb-4" on:contextmenu={(e) => openCtx(e, shelfMenuItems(shelf))}>
                                        <h3 class="text-lg font-bold text-white tracking-tight">{shelf.title}
                                            <span class="text-slate-500 text-sm font-medium ml-2">{shelf.items.length}</span>
                                        </h3>
                                        <SortSelect bind:value={sortBy}/>
                                    </div>
                                    <GameGrid class="mb-9" items={shelf.items} {discreet} onOpen={selectGame} onPlay={playGame}
                                              onToggleFav={toggleFavorite} onDrag={onGameDragStart} onContext={gameCtx}/>
                                {/if}
                            {:else}
                                <div on:contextmenu={(e) => openCtx(e, shelfMenuItems(shelf))}>
                                    <Shelf title={shelf.title} items={shelf.items} onOpen={selectGame} onPlay={playGame} onToggleFav={toggleFavorite} {discreet}
                                           onDrag={onGameDragStart} onContext={gameCtx}/>
                                </div>
                            {/if}
                        {/each}

                        {#if games.length === 0 && !scanning}
                            <div class="flex flex-col items-center justify-center py-24 text-slate-500 animate-fade-in">
                                <span class="text-6xl mb-5 opacity-30">🗂️</span>
                                <p class="text-xl text-slate-400 mb-1">{$t("lib.empty_title")}</p>
                                <p class="text-sm">{$t("lib.empty_sub")}</p>
                            </div>
                        {/if}
                    {/if}
                {:else}
                    <div class="flex items-center justify-end mb-4">
                        <SortSelect bind:value={sortBy}/>
                    </div>
                    <GameGrid items={filteredGames} {discreet} onOpen={selectGame} onPlay={playGame}
                              onToggleFav={toggleFavorite} onDrag={onGameDragStart} onContext={gameCtx}/>
                    {#if filteredGames.length === 0}
                        <div class="flex flex-col items-center justify-center py-24 text-slate-500 animate-fade-in">
                            <span class="text-6xl mb-5 opacity-30">🔍</span>
                            <p class="text-xl text-slate-400 mb-1">{$t("lib.not_found_title")}</p>
                            <p class="text-sm">{$t("lib.not_found_sub")}</p>
                        </div>
                    {/if}
                {/if}
            {/if}

            {#if lightboxIndex !== null && selectedGame && selectedGame.images && selectedGame.images.length}
                <Lightbox images={selectedGame.images} bind:index={lightboxIndex} onClose={() => lightboxIndex = null}/>
            {/if}

            {#if needsSetup}
                <SetupWizard {langs} onChangeLang={changeLang} onConfigured={onSetupConfigured} onFinish={onSetupFinish}/>
            {/if}

            {#if showSettings}
                <SettingsModal bind:scanPaths bind:launcherUpdate {langs} {appVersion}
                               onChangeLang={changeLang}
                               onClose={() => showSettings = false}
                               onScan={handleScan}
                               onReload={reloadLibrary}
                               onRelink={() => showRelink = true}/>
            {/if}

            {#if collectionModal}
                <CollectionModal collection={collectionModal.collection} {allTags}
                                 onSave={saveCollection} onDelete={deleteCollection} onClose={() => collectionModal = null}/>
            {/if}

            {#if showRelink}
                <RelinkModal onClose={() => showRelink = false} onChanged={loadGames} {discreet}/>
            {/if}

            <ConfirmDialog/>
            <Toast/>

            {#if ctx}
                <ContextMenu x={ctx.x} y={ctx.y} items={ctx.items} onClose={() => ctx = null}/>
            {/if}
        </main>
    </div>
</div>
