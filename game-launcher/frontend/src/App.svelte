<script>
    import {onMount} from 'svelte';
    // Импортируем методы Go, которые Wails сгенерировал для нас
    import {
        AddGamesFromDrop,
        AddSingleGameManual,
        CopyCoverToData,
        FindExecutables,
        GetGames,
        GetScanPaths,
        AddScanPath,
        RemoveScanPath,
        ScanAllFolders,
        SaveWindowSize,
        Launch,
        OpenFolder,
        RemoveGame,
        SearchGames,
        SelectCoverImage,
        SelectFolder,
        UpdateGame,
        UpdateGameMetadata,
        SelectScreenshots,
        CopyScreenshotToData,
        SelectExecutable,
        IsConfigured,
        GetDataDir,
        GetDefaultDataDir,
        GetPortableDataDir,
        GetDocumentsDataDir,
        ConfigureDataDir,
        ChangeDataDir,
        SelectDataFolder,
        OpenDataDir,
        ClearData,
        GetCollections,
        SaveCollections,
        GetSupportedSources,
        GetCustomLocales,
        OpenLanguagesFolder
    } from '../wailsjs/go/main/App.js';
    import { t, tr, setLang, addLocales, availableLangs, initialLang, langStore } from './i18n.js';
    import { OnFileDrop, OnFileDropOff, EventsOn, WindowMinimise, WindowToggleMaximise, Quit } from '../wailsjs/runtime/runtime';
    import { onDestroy } from 'svelte';
    import Shelf from './Shelf.svelte';
    import GameCard from './GameCard.svelte';

    // Возвращает URL обложки/скриншота (файлы отдаются бэкендом по /media/<путь>)
    function mediaSrc(p) {
        return p ? `/media/${p.replaceAll('\\', '/')}` : '';
    }


    let games = [];
    let scanning = false;
    let selectedGame = null;
    let parseUrl = ""; // Добавили переменную для URL
    let parsing = false; // Состояние загрузки
    let scanPaths = [];
    let searchQuery = "";
    let isEditing = false; // Переменная-флаг для режима редактирования
    let isDragging = false; // Состояние для визуального эффекта Drag & Drop
    let activeFilter = 'all';
    let originalGameSnapshot = ""; // Храним слепок игры для сравнения

    let activeTag = "";        // выбранный тег для фильтрации (пусто = не фильтруем)
    let discreet = false;      // дискретный режим: блюр всех обложек
    let sortBy = (typeof localStorage !== 'undefined' && localStorage.getItem('plauncher_sort')) || 'added_desc';
    $: if (typeof localStorage !== 'undefined') localStorage.setItem('plauncher_sort', sortBy);

    const APP_VERSION = '1.0.0';
    let supportedSources = [];
    let showSources = false;

    // Язык интерфейса
    let langs = availableLangs();
    setLang(initialLang());
    async function loadCustomLocales() {
        try {
            const extra = await GetCustomLocales();
            if (extra && Object.keys(extra).length) { addLocales(extra); langs = availableLangs(); }
        } catch (e) {}
    }
    function changeLang(code) { setLang(code); langs = availableLangs(); }

    // Svelte-action: закрыть элемент по клику вне его
    function clickOutside(node) {
        const handler = (e) => { if (!node.contains(e.target)) node.dispatchEvent(new CustomEvent('clickoutside')); };
        document.addEventListener('click', handler, true);
        return { destroy() { document.removeEventListener('click', handler, true); } };
    }

    // «Только оформленные» — скрывать игры без обложки (необработанные парсером)
    let onlyDressed = typeof localStorage !== 'undefined' && localStorage.getItem('plauncher_only_dressed') === '1';
    $: if (typeof localStorage !== 'undefined') localStorage.setItem('plauncher_only_dressed', onlyDressed ? '1' : '0');
    // Базовый список, на котором строится вся библиотека/полки/коллекции
    $: libGames = onlyDressed ? games.filter(g => g.cover_path) : games;

    // Сортировка списка игр по выбранному ключу (sortBy передаём аргументом для реактивности)
    function sortGames(arr, key) {
        const a = [...arr];
        switch (key) {
            case 'title_asc':     a.sort((x, y) => (x.title || '').localeCompare(y.title || '')); break;
            case 'played_desc':   a.sort((x, y) => (y.last_launched_at || 0) - (x.last_launched_at || 0)); break;
            case 'playtime_desc': a.sort((x, y) => (y.time_played || 0) - (x.time_played || 0)); break;
            case 'added_asc':     a.sort((x, y) => (x.added_at || 0) - (y.added_at || 0)); break;
            default:              a.sort((x, y) => (y.added_at || 0) - (x.added_at || 0)); // added_desc
        }
        return a;
    }

    function fmtPlaytime(min) {
        if (!min || min <= 0) return '';
        const h = Math.floor(min / 60), m = min % 60;
        return h ? `${h} ч ${m} мин` : `${m} мин`;
    }

    $: filteredGames = sortGames(libGames.filter(g => {
        if (activeTag && !(g.tags || []).includes(activeTag)) return false;
        if (activeFilter === 'favorites') return g.favorite;
        if (activeFilter === 'ready') return g.exec_path !== '';
        if (activeFilter === 'no_cover') return g.cover_path === '';
        if (activeFilter === 'no_desc') return !g.description;
        return true;
    }), sortBy);
    $: allGamesSorted = sortGames(libGames, sortBy);

    // --- Ширина сайдбара (resizable, как в Steam) ---
    let sidebarWidth = (() => {
        const v = parseInt(localStorage.getItem('plauncher_sidebar_w'));
        return (v && v >= 200 && v <= 520) ? v : 256;
    })();
    let resizingSidebar = false;
    function startSidebarResize(e) {
        e.preventDefault();
        resizingSidebar = true;
        const onMove = (ev) => {
            sidebarWidth = Math.max(200, Math.min(520, ev.clientX));
            localStorage.setItem('plauncher_sidebar_w', String(sidebarWidth));
        };
        const onUp = () => {
            resizingSidebar = false;
            localStorage.setItem('plauncher_sidebar_w', String(sidebarWidth));
            window.removeEventListener('mousemove', onMove);
            window.removeEventListener('mouseup', onUp);
        };
        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
    }

    // --- Настраиваемые полки главной (порядок/состав, как в Steam) ---
    function newShelfId() { return 's_' + Math.random().toString(36).slice(2, 9); }
    function defaultShelves() {
        return [
            { id: newShelfId(), type: 'favorites' },
            { id: newShelfId(), type: 'continue' },
            { id: newShelfId(), type: 'added' },
            { id: newShelfId(), type: 'all' },
        ];
    }
    let shelves = (() => {
        try {
            const raw = JSON.parse(localStorage.getItem('plauncher_shelves') || 'null');
            if (Array.isArray(raw) && raw.length) return raw.map(s => ({ id: s.id || newShelfId(), type: s.type, tag: s.tag }));
        } catch (e) {}
        return defaultShelves();
    })();
    $: if (typeof localStorage !== 'undefined') localStorage.setItem('plauncher_shelves', JSON.stringify(shelves));
    let layoutEditing = false;

    function shelfTitle(s) {
        if (s.type === 'tag') return s.tag || tr('layout.opt.tag');
        if (s.type === 'continue' || s.type === 'added' || s.type === 'favorites' || s.type === 'all') return tr('shelf.' + s.type);
        return tr('shelf.section');
    }
    function buildShelf(s, d) {
        let items, title = shelfTitle(s);
        switch (s.type) {
            case 'continue':  items = d.recentlyPlayed; break;
            case 'added':     items = d.recentlyAdded; break;
            case 'favorites': items = d.favoriteGames; break;
            case 'all':       items = d.allGamesSorted; break;
            case 'tag':       items = d.games.filter(g => (g.tags || []).includes(s.tag)); break;
            case 'collection': {
                const col = (d.collectionsView || []).find(c => c.id === s.collectionId);
                items = col ? col.items : [];
                title = col ? col.name : 'Коллекция';
                break;
            }
            default:          items = [];
        }
        return { ...s, title, items };
    }
    // Зависимости перечислены явно, чтобы Svelte пересчитывал при изменении данных
    $: shelfData = { recentlyPlayed, recentlyAdded, favoriteGames, allGamesSorted, games: libGames, collectionsView, _lang: $langStore };
    $: renderedShelves = shelves.map(s => buildShelf(s, shelfData));

    function addShelf() { shelves = [...shelves, { id: newShelfId(), type: 'continue' }]; }
    function removeShelf(id) { shelves = shelves.filter(s => s.id !== id); }
    function setShelfType(id, type) {
        shelves = shelves.map(s => s.id === id ? {
            ...s, type,
            tag: type === 'tag' ? (s.tag || allTags[0] || '') : undefined,
            collectionId: type === 'collection' ? (s.collectionId || (collections[0] && collections[0].id) || '') : undefined,
        } : s);
    }
    function setShelfTag(id, tag) { shelves = shelves.map(s => s.id === id ? { ...s, tag } : s); }
    function setShelfCollection(id, collectionId) { shelves = shelves.map(s => s.id === id ? { ...s, collectionId } : s); }
    function shelfCount(s) { return buildShelf(s, shelfData).items.length; }

    // drag-reorder полок в режиме настройки
    let dragIndex = null;
    function onRowDragStart(i) { dragIndex = i; }
    function onRowDragOver(e) { e.preventDefault(); }
    function onRowDrop(i) {
        if (dragIndex === null || dragIndex === i) { dragIndex = null; return; }
        const arr = [...shelves];
        const [moved] = arr.splice(dragIndex, 1);
        arr.splice(i, 0, moved);
        shelves = arr;
        dragIndex = null;
    }

    // --- ДОМАШНЯЯ СТРАНИЦА (hero + полки в духе Steam) ---
    $: recentlyPlayed = libGames
        .filter(g => (g.last_launched_at || 0) > 0)
        .sort((a, b) => (b.last_launched_at || 0) - (a.last_launched_at || 0));
    $: recentlyAdded = [...libGames].sort((a, b) => (b.added_at || 0) - (a.added_at || 0));
    $: favoriteGames = libGames.filter(g => g.favorite);
    $: heroGame = recentlyPlayed[0] || recentlyAdded[0] || null;
    // Домашний вид: без поиска, фильтра и выбранного тега
    $: isHome = searchQuery.trim() === "" && activeFilter === 'all' && !activeTag;

    // Заголовок текущего вида библиотеки (первый аргумент — зависимость от языка)
    $: viewTitle = ($langStore,
        activeTag ? tr('view.tag', { tag: activeTag })
        : searchQuery.trim() ? tr('view.search')
        : activeFilter === 'favorites' ? tr('shelf.favorites')
        : tr('lib.title'));

    // Уникальные теги по всей библиотеке, отсортированные по частоте
    $: allTags = (() => {
        const counts = {};
        for (const g of games) for (const t of (g.tags || [])) counts[t] = (counts[t] || 0) + 1;
        return Object.keys(counts).sort((a, b) => counts[b] - counts[a]);
    })();

    // --- КОЛЛЕКЦИИ (как в Steam: ручные + динамические по тегам) ---
    let collections = [];
    async function loadCollections() {
        try { collections = await GetCollections() || []; } catch (e) { console.error(e); }
    }
    async function persistCollections() {
        try { await SaveCollections(collections); } catch (err) { showToast(tr("toast.col_fail", {err}), "error"); }
    }

    // Вычисление игр коллекции (allGames передаём аргументом для реактивности)
    function buildCol(c, allGames) {
        const ids = new Set(c.game_ids || []); // вручную закинутые игры (для обоих типов)
        let items;
        if (c.type === 'dynamic') {
            const tags = c.tags || [];
            // динамическая = совпавшие по тегам ∪ вручную добавленные
            items = allGames.filter(g => ids.has(g.id) || (tags.length && (g.tags || []).some(t => tags.includes(t))));
        } else {
            items = allGames.filter(g => ids.has(g.id));
        }
        return { ...c, items };
    }
    $: collectionsView = collections.map(c => buildCol(c, libGames));
    $: categorizedIds = (() => { const s = new Set(); for (const c of collectionsView) for (const g of c.items) s.add(g.id); return s; })();
    $: uncategorized = sortGames(libGames.filter(g => !categorizedIds.has(g.id)), 'title_asc');
    $: sidebarGroups = [
        ...collectionsView.map(c => ({ key: c.id, name: c.name, items: sortGames(c.items, 'title_asc'), col: c })),
        ...(uncategorized.length ? [{ key: '__uncat', name: 'Без категории', items: uncategorized, col: null }] : []),
    ];

    // CRUD коллекций
    let showCollectionModal = false;
    let editingColId = null;
    let colName = "";
    let colType = "dynamic";
    let colTags = [];

    function openCreateCollection() {
        editingColId = null; colName = ""; colType = "dynamic"; colTags = [];
        showCollectionModal = true;
    }
    function openEditCollection(c) {
        editingColId = c.id; colName = c.name; colType = c.type; colTags = [...(c.tags || [])];
        showCollectionModal = true;
    }
    function toggleColTag(t) {
        colTags = colTags.includes(t) ? colTags.filter(x => x !== t) : [...colTags, t];
    }
    async function saveCollection() {
        const name = colName.trim();
        if (!name) { showToast(tr("toast.col_name_required"), "error"); return; }
        if (editingColId) {
            collections = collections.map(c => c.id === editingColId
                ? { ...c, name, type: colType, tags: colType === 'dynamic' ? colTags : (c.tags || []) }
                : c);
        } else {
            collections = [...collections, {
                id: 'c_' + Math.random().toString(36).slice(2, 9),
                name, type: colType,
                game_ids: [], tags: colType === 'dynamic' ? colTags : [],
            }];
        }
        showCollectionModal = false;
        await persistCollections();
    }
    async function deleteCollection(c) {
        const ok = await askConfirm({ title: tr('dlg.delete_collection_title'), message: tr('dlg.delete_collection_msg', { name: c.name }), confirmText: tr('btn.delete'), danger: true });
        if (!ok) return;
        collections = collections.filter(x => x.id !== c.id);
        showCollectionModal = false;
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
    $: manualCollections = collections.filter(c => c.type === 'manual');

    // Принудительно добавить игру в коллекцию (для Drag&Drop; работает и с динамическими)
    async function addGameToCollection(colId, game) {
        let changed = false;
        collections = collections.map(c => {
            if (c.id !== colId) return c;
            const ids = new Set(c.game_ids || []);
            if (!ids.has(game.id)) { ids.add(game.id); changed = true; }
            return { ...c, game_ids: [...ids] };
        });
        if (changed) {
            await persistCollections();
            const col = collections.find(c => c.id === colId);
            showToast(tr('toast.col_added', { title: game.title, name: col ? col.name : '…' }), "success");
        }
    }

    function coverStyle(g) {
        return `object-fit:${(g && g.cover_fit) || 'cover'};object-position:${(g && g.cover_pos) || '50% 20%'}`;
    }
    function coverPosXY(g) {
        const p = ((g && g.cover_pos) || '50% 20%').split(' ');
        return { x: parseInt(p[0]) || 50, y: parseInt(p[1]) || 20 };
    }
    function setCoverFit(f) { if (selectedGame) { selectedGame.cover_fit = f; selectedGame = selectedGame; } }
    function setCoverPosX(v) { if (selectedGame) { selectedGame.cover_pos = `${v}% ${coverPosXY(selectedGame).y}%`; selectedGame = selectedGame; } }
    function setCoverPosY(v) { if (selectedGame) { selectedGame.cover_pos = `${coverPosXY(selectedGame).x}% ${v}%`; selectedGame = selectedGame; } }

    // --- Drag&Drop игр в коллекции ---
    let draggingGame = null;
    let dragOverColId = "";
    function onGameDragStart(game) { draggingGame = game; }
    function onColDragOver(e, key) { if (draggingGame && key !== '__uncat') { e.preventDefault(); dragOverColId = key; } }
    function onColDragLeave(key) { if (dragOverColId === key) dragOverColId = ""; }
    async function onColDrop(e, col) {
        e.preventDefault();
        dragOverColId = "";
        const g = draggingGame;
        draggingGame = null;
        if (g && col) await addGameToCollection(col.id, g);
    }

    // --- Контекстное меню (как в Steam) ---
    let ctx = { show: false, x: 0, y: 0, items: [] };
    function openCtx(e, items) {
        e.preventDefault(); e.stopPropagation();
        const W = 232;
        ctx = {
            show: true,
            x: Math.min(e.clientX, window.innerWidth - W - 8),
            y: Math.min(e.clientY, window.innerHeight - 340),
            items,
        };
    }
    function closeCtx() { ctx = { ...ctx, show: false }; }

    async function removeGameDirect(game) {
        const ok = await askConfirm({ title: tr('dlg.remove_game_title'), message: tr('dlg.remove_game_msg', { title: game.title }), confirmText: tr('btn.delete'), danger: true });
        if (!ok) return;
        try {
            await RemoveGame(game.id);
            if (selectedGame && selectedGame.id === game.id) selectedGame = null;
            await loadGames();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    function gameMenuItems(game) {
        const items = [];
        if (game.exec_path) items.push({ label: tr('ctx.play'), icon: '▶', action: () => playGame(game) });
        else items.push({ label: tr('ctx.set_exe'), icon: '▶', action: () => selectGame(game) });
        items.push({ label: game.favorite ? tr('ctx.fav_remove') : tr('ctx.fav_add'), icon: game.favorite ? '🤍' : '❤️', action: () => toggleFavorite(game) });
        items.push({
            label: tr('ctx.add_to'), icon: '＋',
            submenu: collections.map(c => ({ label: c.name, checked: (c.game_ids || []).includes(game.id), action: () => toggleGameInCollection(c.id, game) }))
                .concat([{ label: tr('ctx.new_collection'), action: openCreateCollection }]),
        });
        items.push({ sep: true });
        items.push({ label: tr('ctx.open_folder'), icon: '📁', action: () => OpenFolder(game.folder_path) });
        items.push({ label: tr('ctx.edit'), icon: '✎', action: () => { selectGame(game); isEditing = true; originalGameSnapshot = JSON.stringify(game); } });
        items.push({ label: tr('ctx.remove_game'), icon: '🗑', danger: true, action: () => removeGameDirect(game) });
        return items;
    }
    function expandAllGroups() { collapsedGroups = {}; if (typeof localStorage !== 'undefined') localStorage.setItem('plauncher_collapsed', '{}'); }
    function collapseAllGroups() {
        const m = {}; for (const g of sidebarGroups) m[g.key] = true;
        collapsedGroups = m; if (typeof localStorage !== 'undefined') localStorage.setItem('plauncher_collapsed', JSON.stringify(m));
    }
    function collectionMenuItems(col) {
        const items = [{ label: tr('ctx.rename'), icon: '✎', action: () => openEditCollection(col) }];
        if (col.type === 'dynamic') items.push({ label: tr('ctx.change_filters'), icon: '⚡', action: () => openEditCollection(col) });
        items.push({ label: collapsedGroups[col.id] ? tr('ctx.expand') : tr('ctx.collapse'), action: () => toggleGroup(col.id) });
        items.push({ sep: true });
        items.push({ label: tr('ctx.expand_all'), action: expandAllGroups });
        items.push({ label: tr('ctx.collapse_all'), action: collapseAllGroups });
        items.push({ sep: true });
        items.push({ label: tr('ctx.delete_collection'), icon: '🗑', danger: true, action: () => deleteCollection(col) });
        return items;
    }
    function shelfMenuItems(shelf) {
        return [
            { label: tr('ctx.configure_sections'), icon: '✎', action: () => layoutEditing = true },
            { label: tr('ctx.remove_section'), icon: '🗑', danger: true, action: () => removeShelf(shelf.id) },
        ];
    }

    let collapsedGroups = (() => {
        try { return JSON.parse(localStorage.getItem('plauncher_collapsed') || '{}'); } catch (e) { return {}; }
    })();
    function toggleGroup(name) {
        collapsedGroups[name] = !collapsedGroups[name];
        collapsedGroups = collapsedGroups;
        if (typeof localStorage !== 'undefined') localStorage.setItem('plauncher_collapsed', JSON.stringify(collapsedGroups));
    }

    function applyTag(tag) {
        activeTag = activeTag === tag ? "" : tag;
        activeFilter = 'all';
        selectedGame = null;
        searchQuery = "";
    }

    let newTag = "";
    function addTag() {
        const t = newTag.trim();
        if (!t || !selectedGame) return;
        if (!selectedGame.tags) selectedGame.tags = [];
        if (!selectedGame.tags.includes(t)) selectedGame.tags = [...selectedGame.tags, t];
        newTag = "";
    }
    function removeTag(t) {
        if (!selectedGame) return;
        selectedGame.tags = (selectedGame.tags || []).filter(x => x !== t);
    }

    async function toggleFavorite(game) {
        if (!game) return;
        game.favorite = !game.favorite;
        games = games; // триггерим реактивность (game — ссылка внутри массива)
        if (selectedGame && selectedGame.id === game.id) selectedGame.favorite = game.favorite;
        try {
            await UpdateGame(game);
        } catch (err) {
            showToast(tr("toast.fav_fail", {err}), "error");
        }
    }

    async function playGame(game) {
        if (!game) return;
        if (!game.exec_path) { selectGame(game); return; }
        try {
            await Launch(game.id, game.exec_path, game.folder_path);
            await loadGames(); // обновляем порядок (последний запуск -> в начало)
        } catch (err) {
            showToast(tr("toast.launch_fail", {err}), "error");
        }
    }

    let toast = { show: false, message: "", type: "success" };

    // --- КАСТОМНЫЙ ДИАЛОГ ПОДТВЕРЖДЕНИЯ (вместо нативного confirm) ---
    let confirmDialog = { show: false, title: "", message: "", confirmText: "OK", danger: false, _resolve: null };

    // askConfirm возвращает Promise<boolean>: true — нажали подтверждение, false — отмена.
    function askConfirm({ title = "Подтверждение", message = "", confirmText = "OK", danger = false } = {}) {
        return new Promise((resolve) => {
            confirmDialog = { show: true, title, message, confirmText, danger, _resolve: resolve };
        });
    }

    function closeConfirm(result) {
        const resolve = confirmDialog._resolve;
        confirmDialog = { ...confirmDialog, show: false, _resolve: null };
        if (resolve) resolve(result);
    }

    // --- ПАПКА ДАННЫХ ЛАУНЧЕРА ---
    let needsSetup = false;          // показывать окно первого запуска
    let setupStep = 1;               // 1 = папка данных, 2 = первая папка с играми
    let setupBusy = false;
    let setupChoice = 'default';     // default | portable | documents | custom
    let setupPaths = { default: '', portable: '', documents: '' };
    let setupCustom = '';
    let showSettings = false;        // окно настроек
    let currentDataDir = '';

    function chosenSetupPath() {
        if (setupChoice === 'custom') return setupCustom;
        return setupPaths[setupChoice] || '';
    }

    async function pickSetupCustom() {
        try {
            const p = await SelectDataFolder();
            if (p) { setupCustom = p; setupChoice = 'custom'; }
        } catch (err) { console.error(err); }
    }

    // Шаг 1: создаём папку данных и переходим к выбору папки с играми
    async function confirmSetup() {
        const path = chosenSetupPath();
        if (!path) { return; }
        setupBusy = true;
        try {
            await ConfigureDataDir(path);
            currentDataDir = await GetDataDir();
            await loadCollections();
            await loadCustomLocales();
            setupStep = 2; // дальше — добавить первую папку с играми
        } catch (err) {
            showToast(tr("toast.setup_fail", {err}), "error");
        } finally {
            setupBusy = false;
        }
    }

    // Шаг 2: добавить папку с играми (можно несколько) и завершить
    async function setupAddFolder() {
        try {
            const p = await SelectFolder();
            if (!p) return;
            await AddScanPath(p);
            scanPaths = await GetScanPaths();
        } catch (err) { showToast(tr("toast.error", { err }), "error"); }
    }
    async function finishSetup() {
        needsSetup = false;
        setupStep = 1;
        if (scanPaths.length) { await handleScan(); } // первичное сканирование добавленных папок
        else await loadGames();
    }

    async function openSettings() {
        try { currentDataDir = await GetDataDir(); } catch (e) {}
        showSettings = true;
    }

    async function handleChangeDataDir() {
        try {
            const p = await SelectDataFolder();
            if (!p) return;
            const ok = await askConfirm({
                title: tr('dlg.change_dir_title'),
                message: tr('dlg.change_dir_msg', { path: p }),
                confirmText: tr('btn.move')
            });
            if (!ok) return;
            await ChangeDataDir(p);
            currentDataDir = await GetDataDir();
            selectedGame = null;
            await loadGames();
            showToast(tr('toast.dir_changed'), "success");
        } catch (err) {
            showToast(tr('toast.dir_change_fail', { err }), "error");
        }
    }

    async function handleOpenDataDir() {
        try { await OpenDataDir(); } catch (err) { showToast(tr('toast.open_folder_fail', { err }), "error"); }
    }

    async function handleClearData() {
        const ok = await askConfirm({
            title: tr('dlg.clear_title'),
            message: tr('dlg.clear_msg'),
            confirmText: tr('btn.clear'),
            danger: true
        });
        if (!ok) return;
        try {
            await ClearData();
            selectedGame = null;
            await loadGames();
            showSettings = false;
            showToast(tr('toast.cleared'), "success");
        } catch (err) {
            showToast(tr('toast.clear_fail', { err }), "error");
        }
    }

    // Классы навигационной кнопки сайдбара
    function navCls(active) {
        const base = "w-full flex items-center gap-2.5 text-left px-3 py-2 rounded-lg transition-colors text-sm ";
        return base + (active
            ? "bg-indigo-500/20 text-white font-semibold ring-1 ring-inset ring-indigo-400/30"
            : "text-slate-300 hover:bg-white/5 hover:text-white");
    }

    // Применение фильтра и выход из карточки игры
    function applyFilter(filter) {
        activeFilter = filter;
        activeTag = "";      // Фильтр сбрасывает выбранный тег
        selectedGame = null; // Выходим из просмотра
        isEditing = false;   // Сбрасываем режим редактирования (на всякий случай)
        parseUrl = "";       // Очищаем инпут парсера
    }

    // --- ФУНКЦИЯ КОПИРОВАНИЯ ---
    async function copyToClipboard(text, labelName) {
        if (!text) return;
        try {
            await navigator.clipboard.writeText(text);
            showToast(tr("toast.copied", { label: labelName }), "success");
        } catch (err) {
            console.error("Ошибка копирования: ", err);
            showToast(tr("toast.copy_fail"), "error");
        }
    }

    // Измени свою функцию toggleEdit (или кнопку отмены) на это:
    async function toggleEdit() {
        if (isEditing) {
            if (JSON.stringify(selectedGame) === originalGameSnapshot) {
                isEditing = false; // Ничего не изменилось, просто закрываем
            } else {
                const ok = await askConfirm({
                    title: tr("dlg.discard_title"),
                    message: tr("dlg.discard_msg"),
                    confirmText: tr("btn.discard"),
                    danger: true
                });
                if (ok) {
                    selectedGame = JSON.parse(originalGameSnapshot);
                    isEditing = false;
                }
            }
        } else {
            originalGameSnapshot = JSON.stringify(selectedGame);
            isEditing = true;
        }
    }

    // НОВАЯ ФУНКЦИЯ: Выбор экзешника
    async function handleSelectExecutable() {
        try {
            // Передаем папку игры, чтобы диалог открылся прямо в ней
            const path = await SelectExecutable(selectedGame.folder_path);
            if (path) {
                selectedGame.exec_path = path;
            }
        } catch (err) {
            console.error(err);
            showToast(tr("toast.error", {err}), "error");
        }
    }

    function showToast(message, type = "success") {
        toast = { show: true, message, type };
        // Автоматически скрываем через 3 секунды
        setTimeout(() => {
            toast.show = false;
        }, 3000);
    }

    async function handleSmartPlay() {
        if (!selectedGame.exec_path) {
            const path = await SelectExecutable(selectedGame.folder_path);
            if (path) {
                selectedGame.exec_path = path;
                await UpdateGame(selectedGame);
                originalGameSnapshot = JSON.stringify(selectedGame); // Обновляем слепок
                showToast(tr("toast.saved"), "success");
            }
        } else {
            // Вызываем наш обновленный метод Launch (теперь с 3 аргументами!)
            Launch(selectedGame.id, selectedGame.exec_path, selectedGame.folder_path);
        }
    }

    // --- ЛОГИКА РЕДАКТИРОВАНИЯ ---
    async function handleUpdateMetadata() {
        if (!parseUrl || !selectedGame) return;
        parsing = true;
        try {
            await UpdateGameMetadata(selectedGame.id, parseUrl);
            showToast(tr("toast.meta_loaded"), "success"); // <-- Красивое уведомление
            parseUrl = "";
            await loadGames();
            selectedGame = games.find(g => g.id === selectedGame.id);
        } catch (err) {
            showToast(tr("toast.meta_fail", {err}), "error");
        } finally {
            parsing = false;
        }
    }

    async function handleSearch() {
        try {
            if (searchQuery.trim() === "") {
                await loadGames(); // Если пусто - грузим всё
            } else {
                games = await SearchGames(searchQuery) || []; // Иначе ищем через БД
            }
        } catch (err) {
            console.error("Ошибка поиска:", err);
        }
    }

    async function handleOpenFolder() {
        if (!selectedGame) return;
        try {
            await OpenFolder(selectedGame.folder_path);
        } catch (err) {
            showToast(tr("toast.open_folder_fail", { err }), "error");
        }
    }

    async function handleRemoveGame() {
        if (!selectedGame) return;
        const ok = await askConfirm({
            title: tr("dlg.remove_game_title"),
            message: tr("dlg.remove_game_msg", { title: selectedGame.title }),
            confirmText: tr("btn.delete"),
            danger: true
        });
        if (!ok) return;
        try {
            await RemoveGame(selectedGame.id);
            selectedGame = null; // Закрываем детальный вид
            await loadGames();   // Обновляем сетку
        } catch (err) {
            showToast("Ошибка удаления: " + err, "error");
        }
    }

    onMount(async () => {
        // Регистрируем нативный обработчик перетаскивания файлов/папок.
        // useDropTarget=false — drop ловится по всему окну.
        OnFileDrop((x, y, paths) => handleNativeDrop(paths), false);

        // Бэкенд шлёт это событие, когда обновилось время в игре (после выхода из игры)
        EventsOn('games-updated', () => loadGames());

        // Сохраняем размер окна при изменении (надёжнее, чем только при выходе)
        window.addEventListener('resize', onWindowResize);

        try { supportedSources = await GetSupportedSources() || []; } catch (e) {}

        // Первый запуск: если папка для данных ещё не выбрана — показываем окно выбора
        try {
            const ok = await IsConfigured();
            if (!ok) {
                setupPaths.default = await GetDefaultDataDir();
                setupPaths.portable = await GetPortableDataDir();
                setupPaths.documents = await GetDocumentsDataDir();
                needsSetup = true;
                return; // библиотеку грузим только после настройки
            }
            currentDataDir = await GetDataDir();
        } catch (err) {
            console.error("Ошибка проверки конфигурации:", err);
        }

        try {
            scanPaths = await GetScanPaths();
        } catch (err) {
            console.error("Не удалось загрузить папки:", err);
        }
        await loadGames();
        await loadCollections();
        await loadCustomLocales();
    });

    onDestroy(() => {
        OnFileDropOff();
        window.removeEventListener('resize', onWindowResize);
    });

    // Сохранение размера окна (надёжно, по ресайзу с задержкой)
    let resizeTimer = null;
    function onWindowResize() {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(() => { SaveWindowSize().catch(() => {}); }, 500);
    }

    // Управление папками с играми (в настройках)
    async function addScanFolder() {
        try {
            const p = await SelectFolder();
            if (!p) return;
            await AddScanPath(p);
            scanPaths = await GetScanPaths();
            showToast(tr("toast.folder_added_scan"), "success");
            await handleScan();
        } catch (err) { showToast(tr("toast.error", { err }), "error"); }
    }
    async function removeScanFolder(path) {
        const ok = await askConfirm({ title: tr("dlg.remove_folder_title"), message: tr("dlg.remove_folder_msg", { path }), confirmText: tr("btn.remove") });
        if (!ok) return;
        try {
            await RemoveScanPath(path);
            scanPaths = await GetScanPaths();
        } catch (err) { showToast(tr("toast.error", { err }), "error"); }
    }

    async function loadGames() {
        try {
            games = await GetGames() || [];
            // Если открыта детальная карточка (и мы не редактируем) — подхватываем свежие данные
            if (selectedGame && !isEditing) {
                const fresh = games.find(g => g.id === selectedGame.id);
                if (fresh) selectedGame = fresh;
            }
        } catch (err) {
            console.error("Ошибка загрузки игр:", err);
        }
    }

    async function handleScan() {
        if (!scanPaths.length) {
            showToast(tr("toast.scan_first"), "error");
            return;
        }
        scanning = true;
        try {
            const count = await ScanAllFolders();
            await loadGames();
            if (count > 0) {
                showToast(tr("toast.scan_added", {n: count}), "success");
            } else {
                showToast(tr("toast.scan_none"), "success");
            }
        } catch (err) {
            showToast(tr("toast.error", {err}), "error");
        } finally {
            scanning = false;
        }
    }

    // Обработка боковых кнопок мыши
    function handleGlobalMouseDown(e) {
        // e.button === 3 это "Назад", e.button === 4 это "Вперед"
        if (e.button === 3 || e.button === 4) {
            e.preventDefault(); // Блокируем стандартное поведение браузера

            if (e.button === 3) { // Назад
                if (lightboxImage) {
                    closeLightbox();
                } else if (selectedGame && !isEditing) {
                    applyFilter(activeFilter); // Возвращаемся в сетку
                }
            } else if (e.button === 4) { // Вперед
                if (lightboxImage) {
                    nextLightboxImage();
                }
            }
        }
    }

    function selectGame(game) {
        selectedGame = game;
        isEditing = false; // Выключаем редактор при смене игры
        parseUrl = "";
        originalGameSnapshot = JSON.stringify(game);
    }

    async function handleSaveChanges() {
        try {
            await UpdateGame(selectedGame);
            isEditing = false;
            await loadGames();
            showToast("Изменения успешно сохранены!", "success"); // <-- Красивое уведомление
        } catch (err) {
            showToast(tr("toast.save_fail", {err}), "error");
        }
    }

    async function handleSelectCover() {
        try {
            const sourcePath = await SelectCoverImage();
            if (sourcePath) {
                // Копируем картинку в папку data лаунчера и получаем новый локальный путь
                const newLocalPath = await CopyCoverToData(selectedGame.id, sourcePath);
                selectedGame.cover_path = newLocalPath;
            }
        } catch (err) {
            console.error(err);
        }
    }

    // Визуальный оверлей: DOM-события dragover/dragleave всё ещё приходят в WebView2.
    // Оверлей «бросайте папки» — только для внешних файлов, не для внутреннего drag игр
    function isFileDrag(e) {
        return e.dataTransfer && Array.from(e.dataTransfer.types || []).includes('Files');
    }
    function onDragOver(e) {
        if (!isFileDrag(e)) return;
        e.preventDefault();
        isDragging = true;
    }

    function onDragLeave(e) {
        if (!isFileDrag(e)) return;
        e.preventDefault();
        isDragging = false;
    }

    // Реальные абсолютные пути приходят из нативного OnFileDrop (см. onMount),
    // а не из e.dataTransfer (WebView2 его не отдаёт).
    async function handleNativeDrop(paths) {
        isDragging = false;
        if (!paths || paths.length === 0) return;

        try {
            const added = await AddGamesFromDrop(paths);
            await loadGames();
            if (added > 0) {
                showToast(tr("toast.scan_added", {n: added}), "success");
            } else {
                showToast(tr("toast.scan_none"), "success");
            }
        } catch (err) {
            showToast(tr("toast.error", {err}), "error");
        }
    }

    // НОВАЯ ФУНКЦИЯ ДЛЯ РУЧНОГО ДОБАВЛЕНИЯ
    async function handleAddSingleGame() {
        try {
            await AddSingleGameManual();
            await loadGames();
        } catch (err) {
            showToast(tr("toast.error", {err}), "error");
        }
    }

    // --- ЛОГИКА ГАЛЕРЕИ (LIGHTBOX) ---
    let lightboxImage = null;
    let lightboxIndex = 0;

    function openLightbox(index) {
        if (!selectedGame.images || selectedGame.images.length === 0) return;
        lightboxIndex = index;
        lightboxImage = selectedGame.images[lightboxIndex];
    }

    function closeLightbox() {
        lightboxImage = null;
    }

    function nextLightboxImage(e) {
        if (e) e.stopPropagation();
        if (lightboxIndex < selectedGame.images.length - 1) {
            lightboxIndex++;
        } else {
            lightboxIndex = 0; // Зацикливаем в начало
        }
        lightboxImage = selectedGame.images[lightboxIndex];
    }

    function prevLightboxImage(e) {
        if (e) e.stopPropagation();
        if (lightboxIndex > 0) {
            lightboxIndex--;
        } else {
            lightboxIndex = selectedGame.images.length - 1; // Зацикливаем в конец
        }
        lightboxImage = selectedGame.images[lightboxIndex];
    }

    // Обработка клавиш (Esc, Стрелки)
    function handleKeydown(e) {
        // Дискретный режим (Boss key): Ctrl+H — мгновенно заблюрить/показать обложки
        if (e.ctrlKey && (e.key === 'h' || e.key === 'H' || e.key === 'р' || e.key === 'Р')) {
            e.preventDefault();
            discreet = !discreet;
            return;
        }
        // Диалог подтверждения перехватывает клавиши первым
        if (confirmDialog.show) {
            if (e.key === 'Escape') { e.preventDefault(); closeConfirm(false); }
            if (e.key === 'Enter')  { e.preventDefault(); closeConfirm(true); }
            return;
        }
        if (lightboxImage) {
            if (e.key === 'Escape') closeLightbox();
            if (e.key === 'ArrowRight') nextLightboxImage();
            if (e.key === 'ArrowLeft') prevLightboxImage();
        } else if (selectedGame) {
            // Если открыта игра и мы не в режиме редактирования - выходим в библиотеку
            if (e.key === 'Escape') {
                if (isEditing) {
                    isEditing = false; // Esc отменяет редактирование
                } else {
                    selectedGame = null; // Esc закрывает игру
                    parseUrl = "";
                }
            }
        }
    }

    // --- УПРАВЛЕНИЕ ОБЛОЖКОЙ ---
    async function handleRemoveCover() {
        const ok = await askConfirm({ title: tr("dlg.remove_cover_title"), confirmText: tr("btn.delete"), danger: true });
        if (ok) {
            selectedGame.cover_path = "";
        }
    }

    async function handleAddCover() {
        try {
            const path = await SelectCoverImage();
            if (path) {
                const newLocalPath = await CopyCoverToData(selectedGame.id, path);
                selectedGame.cover_path = newLocalPath;
            }
        } catch (err) {
            console.error(err);
            showToast(tr("toast.cover_fail", {err}), "error");
        }
    }

    // НОВАЯ ФУНКЦИЯ: Добавление скриншотов
    async function handleAddScreenshots() {
        try {
            const paths = await SelectScreenshots();
            if (!paths || paths.length === 0) return;

            // Если массив изображений еще не существует, создаем его
            if (!selectedGame.images) {
                selectedGame.images = [];
            }

            // Копируем каждый выбранный файл и добавляем в игру
            for (const sourcePath of paths) {
                const newLocalPath = await CopyScreenshotToData(selectedGame.id, sourcePath);
                selectedGame.images.push(newLocalPath);
            }

            // Переназначаем массив, чтобы Svelte увидел изменения и перерисовал интерфейс
            selectedGame.images = [...selectedGame.images];
        } catch (err) {
            console.error(err);
            showToast(tr("toast.screens_fail", {err}), "error");
        }
    }

    // НОВАЯ ФУНКЦИЯ: Удаление скриншота из интерфейса
    async function handleRemoveScreenshot(index) {
        const ok = await askConfirm({ title: tr("dlg.remove_screenshot_title"), confirmText: tr("btn.delete"), danger: true });
        if (ok) {
            selectedGame.images.splice(index, 1);
            selectedGame.images = [...selectedGame.images]; // Триггерим реактивность
        }
    }
</script>
<svelte:window on:keydown={handleKeydown} on:mousedown={handleGlobalMouseDown}/>
<div
        class="flex flex-col h-screen overflow-hidden aurora-bg text-slate-300 font-sans relative"
        on:dragover={onDragOver}
        on:dragleave={onDragLeave}
        on:contextmenu={(e) => e.preventDefault()}
>

    <div class="h-9 shrink-0 flex items-center justify-between glass-strong border-b border-white/5 select-none z-30" style="--wails-draggable:drag" on:dblclick={() => WindowToggleMaximise()}>
        <div class="flex items-center gap-2 px-3 text-slate-400 text-xs font-semibold tracking-wide">
            <span class="text-indigo-400">◆</span> pLauncher
        </div>
        <div class="flex items-center h-full" style="--wails-draggable:no-drag">
            <button on:click={() => WindowMinimise()} class="h-full px-4 hover:bg-white/10 text-slate-400 hover:text-white transition-colors" title={$t("titlebar.min")} aria-label={$t("titlebar.min")}>
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/></svg>
            </button>
            <button on:click={() => WindowToggleMaximise()} class="h-full px-4 hover:bg-white/10 text-slate-400 hover:text-white transition-colors" title={$t("titlebar.max")} aria-label={$t("titlebar.max")}>
                <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="1"/></svg>
            </button>
            <button on:click={() => Quit()} class="h-full px-4 hover:bg-red-600 text-slate-400 hover:text-white transition-colors" title={$t("titlebar.close")} aria-label={$t("titlebar.close")}>
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 6l12 12M18 6L6 18"/></svg>
            </button>
        </div>
    </div>

    <div class="flex flex-1 overflow-hidden min-h-0 relative">

    {#if isDragging}
        <div class="absolute inset-0 bg-indigo-900/80 backdrop-blur-sm z-50 flex flex-col items-center justify-center border-4 border-dashed border-indigo-400 m-4 rounded-2xl pointer-events-none transition-all">
            <div class="text-7xl mb-6">📥</div>
            <h2 class="text-4xl font-bold text-white mb-2 tracking-wider">{$t("drop.title")}</h2>
            <p class="text-indigo-200 text-lg">{$t("drop.sub")}</p>
        </div>
    {/if}

    <aside class="glass-strong flex flex-col z-10 relative shrink-0" style="width:{sidebarWidth}px">
        <div on:mousedown={startSidebarResize} title={$t("app.resize")}
             class="absolute top-0 right-0 w-1.5 h-full cursor-col-resize hover:bg-indigo-400/40 transition-colors z-20 {resizingSidebar ? 'bg-indigo-400/50' : ''}"></div>
        <div class="flex items-center justify-between flex-wrap gap-y-2 px-4 pt-4 pb-4">
            <h1 class="text-2xl font-black tracking-wide">
                <span class="bg-gradient-to-r from-indigo-400 to-fuchsia-400 bg-clip-text text-transparent">pLauncher</span>
            </h1>
            <div class="flex items-center gap-1.5 ml-auto">
                <button on:click={handleScan}
                        disabled={scanning}
                        title={$t("app.scan")}
                        class="w-9 h-9 rounded-lg flex items-center justify-center transition-colors text-slate-300 bg-white/5 hover:bg-white/10 disabled:opacity-60">
                    <svg class="w-5 h-5 pointer-events-none {scanning ? 'animate-spin' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M21 12a9 9 0 1 1-2.64-6.36"/><path d="M21 3v6h-6"/>
                    </svg>
                </button>
                <button on:click={() => onlyDressed = !onlyDressed}
                        title={$t("app.only_dressed")}
                        class="w-9 h-9 rounded-lg flex items-center justify-center text-base transition-colors {onlyDressed ? 'bg-indigo-500/25 ring-1 ring-indigo-400/40 text-indigo-200' : 'bg-white/5 hover:bg-white/10 text-slate-300'}">
                    ✨
                </button>
                <button on:click={() => discreet = !discreet}
                        title={discreet ? $t('app.discreet_show') : $t('app.discreet_hide')}
                        class="w-9 h-9 rounded-lg flex items-center justify-center text-lg transition-colors {discreet ? 'bg-indigo-500/25 ring-1 ring-indigo-400/40' : 'bg-white/5 hover:bg-white/10'}">
                    {discreet ? '🙈' : '👁️'}
                </button>
            </div>
        </div>

        <nav class="flex-1 overflow-y-auto px-2 pb-2">
            <div class="space-y-0.5 mb-3 px-1">
                <button on:click={() => applyFilter('all')} class={navCls(isHome)}>🏠 {$t('nav.home')}</button>
                <button on:click={() => applyFilter('favorites')} class={navCls(activeFilter === 'favorites')}>
                    <span>❤️</span> {$t('nav.favorites')}
                    {#if favoriteGames.length}<span class="ml-auto text-xs text-slate-400">{favoriteGames.length}</span>{/if}
                </button>
            </div>

            <div class="flex items-center justify-between px-3 mt-1 mb-1">
                <span class="text-[10px] font-bold text-slate-500 uppercase tracking-widest">{$t("nav.collections")}</span>
                <button on:click={openCreateCollection} title={$t("nav.create_collection")}
                        class="w-5 h-5 rounded flex items-center justify-center text-slate-400 hover:text-white hover:bg-white/10 transition-colors text-base leading-none">＋</button>
            </div>

            {#each sidebarGroups as group (group.key)}
                <div>
                    <div class="w-full flex items-center gap-1.5 px-2 py-1.5 rounded-md transition-colors group {dragOverColId === group.key ? 'bg-indigo-500/25 ring-1 ring-indigo-400/50' : 'text-slate-300 hover:bg-white/5'}"
                         on:contextmenu={group.col ? (e) => openCtx(e, collectionMenuItems(group.col)) : undefined}
                         on:dragover={group.col ? (e) => onColDragOver(e, group.key) : undefined}
                         on:dragleave={group.col ? () => onColDragLeave(group.key) : undefined}
                         on:drop={group.col ? (e) => onColDrop(e, group.col) : undefined}>
                        <button on:click={() => toggleGroup(group.key)} class="flex items-center gap-1.5 min-w-0 flex-1 text-left hover:text-white">
                            <span class="text-[10px] text-slate-500 transition-transform duration-200 {collapsedGroups[group.key] ? '-rotate-90' : ''}">▼</span>
                            <span class="text-[11px] font-bold uppercase tracking-wider truncate">{group.name}</span>
                            {#if group.col && group.col.type === 'dynamic'}<span class="text-indigo-400 text-[10px]" title={$t("nav.dynamic")}>⚡</span>{/if}
                        </button>
                        <span class="text-[11px] text-slate-500">{group.items.length}</span>
                        {#if group.col}
                            <button on:click|stopPropagation={() => openEditCollection(group.col)} title={$t("nav.edit_collection")}
                                    class="text-slate-500 hover:text-white opacity-0 group-hover:opacity-100 transition-opacity text-xs">✎</button>
                        {/if}
                    </div>
                    {#if !collapsedGroups[group.key]}
                        <div class="ml-2 border-l border-white/10 pl-1.5 mb-1">
                            {#each group.items as g (g.id)}
                                <button on:click={() => selectGame(g)}
                                        draggable="true"
                                        on:dragstart={(e) => { onGameDragStart(g); try { e.dataTransfer.setData('text/plain', g.id); } catch (_) {} }}
                                        on:contextmenu={(e) => openCtx(e, gameMenuItems(g))}
                                        class="w-full flex items-center gap-2.5 px-2 py-1 rounded-md text-sm text-left transition-colors {selectedGame && selectedGame.id === g.id ? 'bg-indigo-500/20 text-white' : 'text-slate-400 hover:bg-white/5 hover:text-slate-100'}">
                                    <span class="w-7 h-7 rounded-md overflow-hidden bg-slate-800 ring-1 ring-white/10 shrink-0 flex items-center justify-center">
                                        {#if g.cover_path}
                                            <img src={mediaSrc(g.cover_path)} alt="" on:error={(e) => e.target.style.display = 'none'} class="w-full h-full object-cover {discreet ? 'blur-md' : ''}"/>
                                        {:else}
                                            <span class="text-[11px] text-slate-500 uppercase">{(g.title || '?').slice(0, 1)}</span>
                                        {/if}
                                    </span>
                                    <span class="truncate">{g.title}</span>
                                </button>
                            {/each}
                            {#if group.items.length === 0}
                                <p class="text-xs text-slate-600 px-2 py-1">{$t("nav.empty")}</p>
                            {/if}
                        </div>
                    {/if}
                </div>
            {/each}

            {#if games.length === 0}
                <p class="text-xs text-slate-600 px-3 py-4 text-center">{$t("nav.lib_empty")}</p>
            {/if}
        </nav>

        <div class="px-3 py-2 border-t border-white/10 flex items-center justify-between">
            <span class="text-xs text-slate-500 tracking-wide">{$t("nav.games", { n: libGames.length })}</span>
            <button on:click={openSettings} title={$t("nav.settings")}
                    class="w-8 h-8 rounded-lg flex items-center justify-center text-slate-400 hover:text-white hover:bg-white/10 transition-colors">⚙️</button>
        </div>
    </aside>

    <main class="flex-1 overflow-y-auto p-8 relative">

        {#if selectedGame}
            {#if selectedGame.cover_path && !isEditing}
                <div class="absolute inset-0 z-0 overflow-hidden pointer-events-none">
                    <img src={mediaSrc(selectedGame.cover_path)} alt="bg" class="w-full h-full object-cover blur-2xl scale-110 opacity-20" />
                    <div class="absolute inset-0 bg-gradient-to-b from-[#0a0912]/70 via-[#0a0912]/90 to-[#0a0912]"></div>
                </div>
            {/if}

            <div class="animate-fade-in relative z-10">
                <button on:click={() => selectedGame = null}
                        class="mb-6 text-indigo-400 hover:text-indigo-300 transition-colors flex items-center gap-2 font-semibold">
                    {$t("detail.back")}
                </button>

                {#if isEditing}
                    <div class="flex flex-col lg:flex-row gap-8">
                        <div class="w-full lg:w-[260px] shrink-0">
                            {#if selectedGame.cover_path}
                                <div class="aspect-[3/4] w-full rounded-xl ring-1 ring-white/10 overflow-hidden relative group mb-4">
                                    <img src={mediaSrc(selectedGame.cover_path)} alt="cover" style={coverStyle(selectedGame)} on:error={(e) => e.target.style.display = 'none'} class="w-full h-full"/>
                                    <div class="absolute inset-0 bg-black/70 flex flex-col gap-3 items-center justify-center opacity-0 group-hover:opacity-100 transition-all backdrop-blur-sm">
                                        <button on:click|stopPropagation={handleSelectCover} class="bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-2 px-6 rounded-lg w-3/4 transition-colors">{$t("cover.change")}</button>
                                        <button on:click|stopPropagation={handleRemoveCover} class="bg-red-600 hover:bg-red-500 text-white font-bold py-2 px-6 rounded-lg w-3/4 transition-colors">{$t("cover.remove")}</button>
                                    </div>
                                </div>
                            {:else}
                                <div class="aspect-[3/4] w-full rounded-xl ring-1 ring-white/10 bg-white/5 flex flex-col items-center justify-center text-slate-600 mb-4">
                                    <span class="text-6xl mb-4 opacity-30">🖼️</span>
                                    <button on:click|stopPropagation={handleSelectCover} class="border-2 border-indigo-600/50 hover:border-indigo-500 text-indigo-400 hover:text-indigo-300 font-bold py-2 px-6 rounded-lg transition-all">{$t("cover.add")}</button>
                                </div>
                            {/if}

                            {#if selectedGame.cover_path}
                                <div class="glass rounded-xl p-3 mb-4">
                                    <div class="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2">{$t("cover.view")}</div>
                                    <div class="flex gap-1.5 mb-3">
                                        {#each [['cover','cover.fill'],['contain','cover.fit'],['fill','cover.stretch']] as opt}
                                            <button on:click={() => setCoverFit(opt[0])}
                                                    class="flex-1 px-1 py-1.5 rounded-lg text-xs transition-colors {(selectedGame.cover_fit || 'cover') === opt[0] ? 'bg-indigo-500/30 text-white ring-1 ring-indigo-400/40' : 'bg-white/5 text-slate-300 hover:bg-white/10'}">{$t(opt[1])}</button>
                                        {/each}
                                    </div>
                                    {#if (selectedGame.cover_fit || 'cover') !== 'fill'}
                                        <div class="flex items-center gap-2 mb-1.5">
                                            <span class="text-xs text-slate-500 w-5 text-center">↔</span>
                                            <input type="range" min="0" max="100" step="1" value={coverPosXY(selectedGame).x} on:input={(e) => setCoverPosX(e.target.value)} class="flex-1 accent-indigo-500"/>
                                        </div>
                                        <div class="flex items-center gap-2">
                                            <span class="text-xs text-slate-500 w-5 text-center">↕</span>
                                            <input type="range" min="0" max="100" step="1" value={coverPosXY(selectedGame).y} on:input={(e) => setCoverPosY(e.target.value)} class="flex-1 accent-indigo-500"/>
                                        </div>
                                    {/if}
                                </div>
                            {/if}

                            <div class="flex flex-col gap-2">
                                <button on:click={handleSaveChanges} class="w-full bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-2.5 rounded-xl transition-colors">{$t("btn.save")}</button>
                                <button on:click={toggleEdit} class="w-full bg-white/5 hover:bg-white/10 text-slate-300 font-bold py-2.5 rounded-xl transition-colors border border-white/10">{$t("btn.cancel")}</button>
                            </div>
                        </div>

                        <div class="flex-1 min-w-0">
                            <input type="text" bind:value={selectedGame.title} class="w-full text-3xl font-black glass text-white rounded-xl px-4 py-3 mb-4 focus:outline-none focus:ring-2 focus:ring-indigo-400/40" placeholder={$t("edit.title_ph")}/>

                            <div class="flex flex-col gap-3 mb-6 glass rounded-xl p-4">
                                <div class="flex items-center gap-3">
                                    <span class="text-slate-400 font-semibold uppercase tracking-wider text-xs w-20">{$t("edit.version_label")}</span>
                                    <input type="text" bind:value={selectedGame.version} class="bg-slate-900/50 text-white border border-slate-600 rounded-md px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none w-40" placeholder="1.0"/>
                                </div>
                                <div class="flex items-center gap-3">
                                    <span class="text-slate-400 font-semibold uppercase tracking-wider text-xs w-20">{$t("edit.exe_label")}</span>
                                    <input type="text" bind:value={selectedGame.exec_path} class="flex-1 bg-slate-900/50 text-white border border-slate-600 rounded-md px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none font-mono text-xs" placeholder="C:\Games\Game\run.exe" />
                                    <button on:click={handleSelectExecutable} class="bg-white/10 hover:bg-white/20 text-slate-200 px-3 py-1.5 rounded-md border border-white/10 transition-colors" title={$t("edit.choose_file")}>📁</button>
                                </div>
                                <div class="flex items-start gap-3">
                                    <span class="text-slate-400 font-semibold uppercase tracking-wider text-xs w-20 pt-2">{$t("edit.tags_label")}</span>
                                    <div class="flex-1">
                                        <div class="flex flex-wrap gap-2 mb-2">
                                            {#each selectedGame.tags || [] as tag}
                                                <span class="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs bg-indigo-500/20 text-indigo-200 ring-1 ring-indigo-400/20">
                                                    {tag}
                                                    <button on:click={() => removeTag(tag)} class="text-indigo-300 hover:text-white" title={$t("btn.remove")}>✕</button>
                                                </span>
                                            {/each}
                                        </div>
                                        <input type="text" bind:value={newTag} on:keydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addTag(); } }}
                                               class="w-full bg-slate-900/50 text-white border border-slate-600 rounded-md px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none" placeholder={$t("edit.add_tag_ph")}/>
                                    </div>
                                </div>
                            </div>

                            <textarea bind:value={selectedGame.description} rows="6" class="w-full glass text-slate-300 rounded-xl px-4 py-4 mb-6 text-lg leading-relaxed focus:outline-none focus:ring-2 focus:ring-indigo-400/40 resize-y" placeholder={$t("edit.desc_ph")}></textarea>

                            <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">Скриншоты</h3>
                            <div class="flex gap-4 overflow-x-auto pb-3">
                                {#each selectedGame.images || [] as img, i}
                                    <div class="relative flex-shrink-0 w-64 aspect-video bg-slate-900 rounded-lg ring-1 ring-white/10 overflow-hidden group">
                                        <img src={mediaSrc(img)} alt="screenshot" on:error={(e) => e.target.style.display = 'none'} class="w-full h-full object-cover"/>
                                        <button on:click|stopPropagation={() => handleRemoveScreenshot(i)} class="absolute top-2 right-2 bg-red-600/90 hover:bg-red-500 text-white rounded-full w-8 h-8 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity text-sm font-bold shadow-lg" title={$t("edit.remove_screenshot")}>✕</button>
                                    </div>
                                {/each}
                                <div class="flex-shrink-0 w-64 aspect-video glass rounded-lg border-2 border-dashed border-white/15 hover:border-indigo-500 flex flex-col items-center justify-center cursor-pointer transition-colors text-slate-400 hover:text-indigo-400" on:click={handleAddScreenshots}>
                                    <span class="text-4xl mb-1 font-light">+</span>
                                    <span class="text-xs font-bold uppercase tracking-wider">{$t("edit.add")}</span>
                                </div>
                            </div>
                        </div>
                    </div>

                {:else}
                    <div class="relative rounded-2xl overflow-hidden ring-1 ring-white/10 mb-8 min-h-[340px] flex">
                        {#if selectedGame.cover_path}
                            <img src={mediaSrc(selectedGame.cover_path)} alt="" class="absolute inset-0 w-full h-full object-cover object-center scale-110 blur-lg opacity-40"/>
                        {/if}
                        <div class="absolute inset-0 bg-gradient-to-r from-[#0a0912] via-[#0a0912]/80 to-[#0a0912]/40"></div>
                        <div class="absolute inset-0 bg-gradient-to-t from-[#0a0912] to-transparent"></div>

                        <div class="relative flex gap-8 p-8 w-full">
                            {#if selectedGame.cover_path}
                                <img src={mediaSrc(selectedGame.cover_path)} alt={selectedGame.title}
                                     style={coverStyle(selectedGame)} on:error={(e) => e.target.style.display = 'none'}
                                     class="w-[210px] aspect-[3/4] rounded-xl ring-1 ring-white/15 shadow-2xl shrink-0 hidden md:block"/>
                            {/if}
                            <div class="flex-1 min-w-0 flex flex-col">
                                <h1 class="text-4xl xl:text-5xl font-black text-white drop-shadow-xl mb-3 break-words cursor-pointer hover:text-indigo-200 transition-colors" on:click={() => copyToClipboard(selectedGame.title, $t('label.title'))} title={$t("detail.copy_title")}>
                                    {selectedGame.title}
                                </h1>
                                <div class="flex items-center flex-wrap gap-2.5 text-slate-300 font-medium mb-3">
                                    <span class="glass px-3 py-1 rounded-full cursor-pointer hover:bg-white/10 transition-colors" on:click={() => copyToClipboard(selectedGame.version, $t('label.version'))}>
                                        {$t("detail.version")}: <span class="text-white">{selectedGame.version || $t("detail.unknown")}</span>
                                    </span>
                                    {#if selectedGame.languages && selectedGame.languages.length}
                                        <span class="glass px-3 py-1 rounded-full">{selectedGame.languages.join(', ')}</span>
                                    {/if}
                                    {#if selectedGame.time_played > 0}
                                        <span class="glass px-3 py-1 rounded-full">🕒 {fmtPlaytime(selectedGame.time_played)}</span>
                                    {/if}
                                </div>
                                {#if selectedGame.tags && selectedGame.tags.length}
                                    <div class="flex flex-wrap gap-2 mb-5">
                                        {#each selectedGame.tags as tag}
                                            <button on:click={() => applyTag(tag)}
                                                    class="px-3 py-1 rounded-full text-sm bg-indigo-500/15 text-indigo-200 ring-1 ring-indigo-400/20 hover:bg-indigo-500/30 transition-colors">{tag}</button>
                                        {/each}
                                    </div>
                                {/if}
                                <div class="flex items-center flex-wrap gap-2.5 mt-auto">
                                    <button on:click={handleSmartPlay}
                                            class="flex items-center justify-center gap-2.5 font-black py-3.5 px-10 rounded-xl text-lg tracking-wide transition-all hover:-translate-y-0.5 {selectedGame.exec_path ? 'bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-400 hover:to-green-500 text-white shadow-lg shadow-emerald-900/40' : 'bg-gradient-to-r from-orange-500 to-red-500 hover:from-orange-400 hover:to-red-400 text-white'}">
                                        <svg class="w-5 h-5 fill-current" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                                        {selectedGame.exec_path ? $t('btn.play') : $t('btn.set_exe')}
                                    </button>
                                    <button on:click={handleOpenFolder} class="glass hover:bg-white/10 text-slate-200 font-semibold py-3.5 px-5 rounded-xl transition-colors" title={$t("ctx.open_folder")}>📁 {$t("btn.folder")}</button>
                                    <button on:click={() => toggleFavorite(selectedGame)} class="{selectedGame.favorite ? 'bg-pink-500/80 text-white' : 'glass hover:bg-white/10 text-slate-200'} font-semibold py-3.5 px-5 rounded-xl transition-colors" title={selectedGame.favorite ? $t('detail.fav_in') : $t('detail.fav_add')}>
                                        {selectedGame.favorite ? '❤️' : '🤍'}
                                    </button>
                                    <button on:click={toggleEdit} class="glass hover:bg-white/10 text-slate-200 font-semibold py-3.5 px-5 rounded-xl transition-colors" title={$t("detail.edit")}>✏️</button>
                                    <button on:click={handleRemoveGame} class="glass hover:bg-red-900/50 text-slate-400 hover:text-red-400 font-semibold py-3.5 px-5 rounded-xl transition-colors" title={$t("detail.remove_from_launcher")}>🗑️</button>
                                </div>
                            </div>
                        </div>
                    </div>

                    {#if selectedGame.description}
                        <div class="mb-8">
                            <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">{$t("detail.description")}</h3>
                            <p class="text-lg text-slate-300 leading-relaxed whitespace-pre-wrap cursor-pointer hover:bg-white/5 p-4 -mx-4 rounded-xl transition-all" on:click={() => copyToClipboard(selectedGame.description, $t('label.description'))}>
                                {selectedGame.description}
                            </p>
                        </div>
                    {/if}

                    {#if selectedGame.images && selectedGame.images.length > 0}
                        <div class="mb-8">
                            <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">Скриншоты</h3>
                            <div class="flex gap-4 overflow-x-auto pb-3">
                                {#each selectedGame.images as img, i}
                                    <div class="relative flex-shrink-0 w-72 aspect-video bg-slate-900 rounded-lg ring-1 ring-white/10 overflow-hidden group hover:ring-indigo-400/50 transition-all">
                                        <img src={mediaSrc(img)} alt="screenshot" on:error={(e) => e.target.style.display = 'none'} class="w-full h-full object-cover cursor-pointer group-hover:scale-105 transition-transform duration-500" on:click={() => openLightbox(i)}/>
                                    </div>
                                {/each}
                            </div>
                        </div>
                    {/if}

                    <div class="mb-8">
                        <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">{$t("detail.collections")}</h3>
                        <div class="flex flex-wrap gap-2 items-center">
                            {#each manualCollections as c}
                                {@const inside = (c.game_ids || []).includes(selectedGame.id)}
                                <button on:click={() => toggleGameInCollection(c.id, selectedGame)}
                                        class="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm transition-colors {inside ? 'bg-indigo-500/25 text-white ring-1 ring-indigo-400/40' : 'bg-white/5 text-slate-300 hover:bg-white/10'}">
                                    <span class="text-xs">{inside ? '✓' : '＋'}</span> {c.name}
                                </button>
                            {/each}
                            <button on:click={openCreateCollection} class="px-3 py-1.5 rounded-full text-sm border border-dashed border-white/20 text-slate-400 hover:text-indigo-300 hover:border-indigo-500 transition-colors">
                                {$t("detail.new_collection")}
                            </button>
                        </div>
                        {#if manualCollections.length === 0}
                            <p class="text-xs text-slate-500 mt-2">{$t("detail.collections_hint")}</p>
                        {/if}
                    </div>

                    <div class="glass p-5 rounded-xl">
                        <div class="flex items-center gap-2 mb-3 relative">
                            <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider">{$t("meta.title")}</h3>
                            <button on:click={() => showSources = !showSources} title={$t("meta.sources")}
                                    class="w-5 h-5 rounded-full flex items-center justify-center text-[11px] bg-white/5 hover:bg-white/10 text-slate-300 transition-colors">?</button>
                            {#if showSources}
                                <div class="absolute left-0 top-7 z-20 w-64 glass-strong rounded-xl shadow-2xl p-3 animate-fade-in" use:clickOutside on:clickoutside={() => showSources = false}>
                                    <div class="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2">{$t("meta.sources")}</div>
                                    <div class="flex flex-col gap-1.5">
                                        {#each supportedSources as s}
                                            <div class="flex items-center justify-between gap-2 text-sm">
                                                <span class="text-slate-200">{s.name}</span>
                                                <span class="text-[11px] text-slate-500 font-mono truncate">{s.domain}</span>
                                            </div>
                                        {/each}
                                    </div>
                                    <p class="text-[11px] text-slate-500 mt-2 pt-2 border-t border-white/10">{$t("meta.sources_hint")}</p>
                                </div>
                            {/if}
                        </div>
                        <div class="flex gap-3">
                            <input type="text" bind:value={parseUrl} placeholder={$t("meta.url_ph")} class="flex-1 bg-slate-900/60 border border-white/10 text-white rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 transition-all"/>
                            <button on:click={handleUpdateMetadata} disabled={parsing || !parseUrl} class="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold py-2.5 px-6 rounded-lg shadow-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed">
                                {parsing ? $t('meta.downloading') : $t('meta.update')}
                            </button>
                        </div>
                    </div>
                {/if}
            </div>

        {:else}
            <div class="flex justify-between items-center mb-7 animate-fade-in gap-4">
                <div class="flex items-center gap-3 min-w-0">
                    <h2 class="text-3xl font-black text-white tracking-tight truncate">{viewTitle}</h2>
                    {#if !isHome}
                        <button on:click={() => { applyFilter('all'); searchQuery = ''; }}
                                class="shrink-0 flex items-center gap-1 text-xs text-slate-400 hover:text-white bg-white/5 hover:bg-white/10 px-2.5 py-1 rounded-full border border-white/10 transition-colors">
                            ✕ {$t("lib.reset")}
                        </button>
                    {/if}
                </div>

                <div class="relative w-80 shrink-0">
                    <input
                            type="text"
                            bind:value={searchQuery}
                            on:input={handleSearch}
                            placeholder={$t("search.placeholder")}
                            class="w-full glass text-white rounded-full pl-10 pr-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 transition-all placeholder:text-slate-500"
                    />
                    <div class="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none">🔍</div>
                </div>
            </div>

            {#if isHome}
                {#if layoutEditing}
                    <div class="animate-fade-in">
                        <div class="flex items-center justify-between mb-4">
                            <div>
                                <h2 class="text-2xl font-black text-white">{$t("layout.title")}</h2>
                                <p class="text-sm text-slate-400 mt-0.5">{$t("layout.hint")}</p>
                            </div>
                            <button on:click={() => layoutEditing = false} class="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold px-6 py-2.5 rounded-xl transition-colors">{$t("btn.done")}</button>
                        </div>
                        <div class="space-y-2">
                            {#each shelves as s, i (s.id)}
                                <div draggable="true"
                                     on:dragstart={() => onRowDragStart(i)}
                                     on:dragover={onRowDragOver}
                                     on:drop={() => onRowDrop(i)}
                                     class="flex items-center gap-3 glass rounded-xl px-3 py-3 cursor-move transition-all {dragIndex === i ? 'ring-2 ring-indigo-400/60 opacity-60' : ''}">
                                    <span class="text-slate-500 text-xl select-none">⋮⋮</span>
                                    <select value={s.type} on:change={(e) => setShelfType(s.id, e.target.value)}
                                            class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                                        <option value="continue">{$t("shelf.continue")}</option>
                                        <option value="added">{$t("shelf.added")}</option>
                                        <option value="favorites">{$t("shelf.favorites")}</option>
                                        <option value="all">{$t("shelf.all")}</option>
                                        <option value="collection">{$t("layout.opt.collection")}</option>
                                        <option value="tag">{$t("layout.opt.tag")}</option>
                                    </select>
                                    {#if s.type === 'tag'}
                                        <select value={s.tag} on:change={(e) => setShelfTag(s.id, e.target.value)}
                                                class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                                            {#each allTags as t}<option value={t}>{t}</option>{/each}
                                            {#if !allTags.length}<option value="">{$t("layout.no_tags")}</option>{/if}
                                        </select>
                                    {/if}
                                    {#if s.type === 'collection'}
                                        <select value={s.collectionId} on:change={(e) => setShelfCollection(s.id, e.target.value)}
                                                class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                                            {#each collections as c}<option value={c.id}>{c.name}</option>{/each}
                                            {#if !collections.length}<option value="">{$t("layout.no_collections")}</option>{/if}
                                        </select>
                                    {/if}
                                    <span class="ml-auto text-xs text-slate-500">{$t("layout.games_n", { n: shelfCount(s) })}</span>
                                    <button on:click={() => removeShelf(s.id)} class="text-slate-400 hover:text-red-400 transition-colors" title={$t("layout.remove")}>🗑️</button>
                                </div>
                            {/each}
                        </div>
                        <button on:click={addShelf} class="mt-3 w-full border-2 border-dashed border-white/15 hover:border-indigo-500 text-slate-400 hover:text-indigo-300 py-2.5 rounded-xl transition-colors font-semibold">{$t("layout.add")}</button>
                    </div>
                {:else}
                {#if heroGame}
                    <div class="relative h-[176px] rounded-2xl overflow-hidden ring-1 ring-white/10 mb-7 cursor-pointer animate-rise-in group"
                         on:click={() => selectGame(heroGame)}>
                        {#if heroGame.cover_path}
                            <img src={mediaSrc(heroGame.cover_path)} alt="" class="absolute inset-0 w-full h-full object-cover object-center scale-110 opacity-50 transition-transform duration-700 group-hover:scale-105 {discreet ? 'blur-3xl' : 'blur-sm'}"/>
                        {/if}
                        <div class="absolute inset-0 bg-gradient-to-r from-[#0a0912] via-[#0a0912]/75 to-transparent"></div>
                        <div class="absolute inset-0 bg-gradient-to-t from-[#0a0912] via-transparent to-transparent"></div>

                        <button on:click|stopPropagation={() => toggleFavorite(heroGame)}
                                title={heroGame.favorite ? $t('ctx.fav_remove') : $t('detail.fav_add')}
                                class="absolute top-3 right-3 w-8 h-8 rounded-full flex items-center justify-center backdrop-blur-sm transition-all {heroGame.favorite ? 'bg-pink-500/90 text-white' : 'bg-black/40 text-white/80 hover:bg-black/60'}">
                            <svg class="w-4 h-4" viewBox="0 0 24 24" fill={heroGame.favorite ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="2"><path d="M12 21s-7.5-4.6-10-9.3C.5 8.5 2 5 5.5 5 7.7 5 9 6.5 12 9c3-2.5 4.3-4 6.5-4C22 5 23.5 8.5 22 11.7 19.5 16.4 12 21 12 21z"/></svg>
                        </button>

                        <div class="relative h-full flex items-center gap-5 px-6 py-4">
                            {#if heroGame.cover_path}
                                <img src={mediaSrc(heroGame.cover_path)} alt={heroGame.title}
                                     style={coverStyle(heroGame)} on:error={(e) => e.target.style.display = 'none'}
                                     class="hidden sm:block h-full aspect-[3/4] rounded-lg ring-1 ring-white/15 shadow-2xl shrink-0 {discreet ? 'blur-2xl' : ''}"/>
                            {/if}
                            <div class="min-w-0">
                                <div class="text-[10px] uppercase tracking-widest text-indigo-300 mb-1 font-semibold">
                                    {heroGame.last_launched_at ? $t('hero.continue') : $t('hero.recent')}
                                </div>
                                <h2 class="text-2xl font-black text-white drop-shadow-lg mb-1.5 line-clamp-1">{heroGame.title}</h2>
                                <div class="flex items-center gap-2 text-xs text-slate-300 mb-3">
                                    <span>{heroGame.version || '—'}</span>
                                    {#if heroGame.languages && heroGame.languages.length}
                                        <span class="text-slate-600">•</span>
                                        <span class="truncate">{heroGame.languages.join(', ')}</span>
                                    {/if}
                                    {#if heroGame.time_played > 0}
                                        <span class="text-slate-600">•</span>
                                        <span>🕒 {fmtPlaytime(heroGame.time_played)}</span>
                                    {/if}
                                </div>
                                <div class="flex items-center gap-2.5">
                                    <button on:click|stopPropagation={() => playGame(heroGame)}
                                            class="flex items-center gap-2 bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-400 hover:to-green-500 text-white font-bold py-2 px-6 rounded-lg shadow-lg shadow-emerald-900/40 transition-all hover:-translate-y-0.5">
                                        <svg class="w-4 h-4 fill-current" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                                        {heroGame.exec_path ? $t('btn.play') : $t('btn.set_exe')}
                                    </button>
                                    <button on:click|stopPropagation={() => selectGame(heroGame)}
                                            class="glass hover:bg-white/10 text-white font-semibold py-2 px-5 rounded-lg transition-colors text-sm">
                                        {$t("btn.details")}
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
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
                                <div class="flex items-center gap-2 text-sm text-slate-400">
                                    <span class="text-xs uppercase tracking-wider">{$t("sort.label")}</span>
                                    <select bind:value={sortBy} class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                                        <option value="added_desc">{$t("sort.added_desc")}</option>
                                        <option value="added_asc">{$t("sort.added_asc")}</option>
                                        <option value="played_desc">{$t("sort.played")}</option>
                                        <option value="playtime_desc">{$t("sort.playtime")}</option>
                                        <option value="title_asc">{$t("sort.title")}</option>
                                    </select>
                                </div>
                            </div>
                            <div class="grid gap-5 grid-cols-[repeat(auto-fill,minmax(170px,1fr))] mb-9">
                                {#each shelf.items as game, i (game.id)}
                                    <div class="animate-rise-in" style="animation-delay:{Math.min(i, 14) * 30}ms">
                                        <GameCard {game} onOpen={() => selectGame(game)} onPlay={playGame} onToggleFav={toggleFavorite} {discreet}
                                                  onDrag={onGameDragStart} onContext={(g, e) => openCtx(e, gameMenuItems(g))} />
                                    </div>
                                {/each}
                            </div>
                        {/if}
                    {:else}
                        <div on:contextmenu={(e) => openCtx(e, shelfMenuItems(shelf))}>
                            <Shelf title={shelf.title} items={shelf.items} onOpen={selectGame} onPlay={playGame} onToggleFav={toggleFavorite} {discreet}
                                   onDrag={onGameDragStart} onContext={(g, e) => openCtx(e, gameMenuItems(g))} />
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
                    <div class="flex items-center gap-2 text-sm text-slate-400">
                        <span class="text-xs uppercase tracking-wider">{$t("sort.label")}</span>
                        <select bind:value={sortBy} class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                            <option value="added_desc">{$t("sort.added_desc")}</option>
                            <option value="added_asc">{$t("sort.added_asc")}</option>
                            <option value="played_desc">{$t("sort.played")}</option>
                            <option value="playtime_desc">{$t("sort.playtime")}</option>
                            <option value="title_asc">{$t("sort.title")}</option>
                        </select>
                    </div>
                </div>
                <div class="grid gap-5 grid-cols-[repeat(auto-fill,minmax(170px,1fr))]">
                    {#each filteredGames as game, i (game.id)}
                        <div class="animate-rise-in" style="animation-delay:{Math.min(i, 14) * 30}ms">
                            <GameCard {game} onOpen={() => selectGame(game)} onPlay={playGame} onToggleFav={toggleFavorite} {discreet}
                                      onDrag={onGameDragStart} onContext={(g, e) => openCtx(e, gameMenuItems(g))} />
                        </div>
                    {/each}
                </div>
                {#if filteredGames.length === 0}
                    <div class="flex flex-col items-center justify-center py-24 text-slate-500 animate-fade-in">
                        <span class="text-6xl mb-5 opacity-30">🔍</span>
                        <p class="text-xl text-slate-400 mb-1">{$t("lib.not_found_title")}</p>
                        <p class="text-sm">{$t("lib.not_found_sub")}</p>
                    </div>
                {/if}
            {/if}
        {/if}

        {#if lightboxImage}
            <div
                    class="absolute inset-0 bg-slate-900/95 backdrop-blur-md z-50 flex items-center justify-center p-10 animate-fade-in"
                    on:click={closeLightbox}
            >
                <button class="absolute top-6 right-8 text-slate-400 hover:text-white text-5xl font-light transition-colors z-50"
                        on:click={closeLightbox}>&times;
                </button>

                <div class="absolute left-0 top-0 bottom-0 w-1/4 flex items-center justify-start pl-8 group cursor-pointer"
                     on:click={prevLightboxImage}>
                    <div class="text-white/30 group-hover:text-white text-7xl transition-colors drop-shadow-2xl">
                        &#10094;
                    </div>
                </div>

                <img src={mediaSrc(lightboxImage)} alt="fullscreen"
                     class="max-w-[80vw] max-h-[80vh] object-contain rounded-lg shadow-2xl border border-slate-700"
                     on:click|stopPropagation/>

                <div class="absolute right-0 top-0 bottom-0 w-1/4 flex items-center justify-end pr-8 group cursor-pointer"
                     on:click={nextLightboxImage}>
                    <div class="text-white/30 group-hover:text-white text-7xl transition-colors drop-shadow-2xl">
                        &#10095;
                    </div>
                </div>

                <div class="absolute bottom-6 text-slate-400 font-semibold tracking-widest bg-slate-900/80 px-4 py-1 rounded-full">
                    {lightboxIndex + 1} / {selectedGame.images.length}
                </div>
            </div>
        {/if}

        {#if needsSetup}
            <div class="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in">
                <div class="w-[560px] max-w-[90vw] glass-strong rounded-2xl shadow-2xl p-7">
                    {#if setupStep === 1}
                        <div class="flex justify-end mb-2">
                            <select value={$langStore} on:change={(e) => changeLang(e.target.value)}
                                    class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                                {#each langs as l}<option value={l.code}>{l.name}</option>{/each}
                            </select>
                        </div>
                        <h2 class="text-2xl font-black text-white mb-1">{$t("setup.welcome")}</h2>
                        <p class="text-slate-400 text-sm mb-5">{$t("setup.step1")}</p>

                        <div class="flex flex-col gap-2 mb-5">
                            <button on:click={() => setupChoice = 'default'} class="text-left p-3 rounded-lg border transition-colors {setupChoice === 'default' ? 'border-indigo-500 bg-indigo-600/15' : 'border-slate-600/50 hover:bg-slate-700/40'}">
                                <div class="font-bold text-white text-sm">{$t("setup.sys")} <span class="text-indigo-400">{$t("setup.recommended")}</span></div>
                                <div class="text-xs text-slate-400 break-all mt-0.5 font-mono">{setupPaths.default}</div>
                            </button>
                            <button on:click={() => setupChoice = 'documents'} class="text-left p-3 rounded-lg border transition-colors {setupChoice === 'documents' ? 'border-indigo-500 bg-indigo-600/15' : 'border-slate-600/50 hover:bg-slate-700/40'}">
                                <div class="font-bold text-white text-sm">{$t("setup.docs")}</div>
                                <div class="text-xs text-slate-400 break-all mt-0.5 font-mono">{setupPaths.documents}</div>
                            </button>
                            <button on:click={() => setupChoice = 'portable'} class="text-left p-3 rounded-lg border transition-colors {setupChoice === 'portable' ? 'border-indigo-500 bg-indigo-600/15' : 'border-slate-600/50 hover:bg-slate-700/40'}">
                                <div class="font-bold text-white text-sm">{$t("setup.portable")}</div>
                                <div class="text-xs text-slate-400 break-all mt-0.5 font-mono">{setupPaths.portable}</div>
                            </button>
                            <button on:click={pickSetupCustom} class="text-left p-3 rounded-lg border transition-colors {setupChoice === 'custom' ? 'border-indigo-500 bg-indigo-600/15' : 'border-slate-600/50 hover:bg-slate-700/40'}">
                                <div class="font-bold text-white text-sm">{$t("setup.custom")}</div>
                                <div class="text-xs text-slate-400 break-all mt-0.5 font-mono">{setupCustom || $t('setup.custom_hint')}</div>
                            </button>
                        </div>

                        <button on:click={confirmSetup} disabled={setupBusy || !chosenSetupPath()} class="w-full bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-3 rounded-lg shadow transition-all disabled:opacity-50 disabled:cursor-not-allowed">
                            {setupBusy ? $t('setup.creating') : $t('btn.continue')}
                        </button>
                    {:else}
                        <h2 class="text-2xl font-black text-white mb-1">{$t("setup.games_title")}</h2>
                        <p class="text-slate-400 text-sm mb-5">{$t("setup.step2")}</p>

                        <div class="flex flex-col gap-2 mb-3 max-h-52 overflow-y-auto">
                            {#each scanPaths as p}
                                <div class="flex items-center gap-2 bg-white/5 border border-white/10 rounded-lg px-3 py-2">
                                    <span class="flex-1 text-sm text-slate-300 font-mono break-all">{p}</span>
                                    <button on:click={async () => { await RemoveScanPath(p); scanPaths = await GetScanPaths(); }} class="text-slate-500 hover:text-red-400 shrink-0" title={$t("btn.remove")}>✕</button>
                                </div>
                            {/each}
                            {#if scanPaths.length === 0}
                                <p class="text-xs text-slate-500 py-2">{$t("setup.none")}</p>
                            {/if}
                        </div>

                        <button on:click={setupAddFolder} class="w-full border-2 border-dashed border-white/15 hover:border-indigo-500 text-slate-300 hover:text-indigo-300 py-2.5 rounded-xl transition-colors font-semibold mb-5">
                            {$t("setup.add_folder")}
                        </button>

                        <button on:click={finishSetup} class="w-full bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-3 rounded-lg shadow transition-all">
                            {scanPaths.length ? $t('setup.done_scan') : $t('setup.skip')}
                        </button>
                    {/if}
                </div>
            </div>
        {/if}

        {#if showSettings}
            <div class="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={() => showSettings = false}>
                <div class="w-[560px] max-w-[90vw] max-h-[85vh] overflow-y-auto glass-strong rounded-2xl shadow-2xl p-7" on:click|stopPropagation>
                    <div class="flex items-center justify-between mb-5">
                        <h2 class="text-2xl font-black text-white">{$t("settings.title")}</h2>
                        <button on:click={() => showSettings = false} class="text-slate-400 hover:text-white text-3xl leading-none">&times;</button>
                    </div>

                    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("settings.language")}</div>
                    <div class="flex gap-2 items-stretch mb-6">
                        <select value={$langStore} on:change={(e) => changeLang(e.target.value)}
                                class="flex-1 bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
                            {#each langs as l}<option value={l.code}>{l.name}</option>{/each}
                        </select>
                        <button on:click={() => OpenLanguagesFolder()} title={$t("settings.custom_lang")}
                                class="shrink-0 flex items-center justify-center bg-white/10 hover:bg-white/20 text-slate-200 px-4 rounded-lg border border-white/10 transition-colors text-lg">📂</button>
                    </div>

                    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("settings.games_folders")}</div>
                    <div class="flex flex-col gap-2 mb-2">
                        {#each scanPaths as p}
                            <div class="flex items-center gap-2 bg-white/5 border border-white/10 rounded-lg px-3 py-2">
                                <span class="flex-1 text-sm text-slate-300 font-mono break-all">{p}</span>
                                <button on:click={() => removeScanFolder(p)} class="text-slate-500 hover:text-red-400 shrink-0" title={$t("btn.remove")}>✕</button>
                            </div>
                        {/each}
                        {#if scanPaths.length === 0}
                            <p class="text-xs text-slate-500">{$t("settings.no_folders")}</p>
                        {/if}
                    </div>
                    <div class="flex gap-2 mb-6">
                        <button on:click={addScanFolder} class="flex-1 bg-white/5 hover:bg-white/10 text-slate-200 text-sm font-semibold py-2 px-3 rounded-lg border border-white/10 transition-colors">
                            {$t("settings.add_folder")}
                        </button>
                        <button on:click={handleAddSingleGame} class="flex-1 bg-white/5 hover:bg-white/10 text-slate-200 text-sm font-semibold py-2 px-3 rounded-lg border border-white/10 transition-colors">
                            {$t("settings.single_game")}
                        </button>
                    </div>

                    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2 border-t border-white/10 pt-5">{$t("settings.data_folder")}</div>
                    <div class="flex gap-2 items-stretch mb-2">
                        <div class="flex-1 bg-slate-900/60 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-300 font-mono break-all">{currentDataDir || '—'}</div>
                        <button on:click={handleOpenDataDir} title={$t("settings.open_folder")} class="shrink-0 bg-white/10 hover:bg-white/20 text-slate-200 px-4 rounded-lg border border-white/10 transition-colors text-xl">📂</button>
                    </div>
                    <button on:click={handleChangeDataDir} class="w-full bg-white/5 hover:bg-white/10 text-slate-200 font-semibold py-2.5 rounded-lg border border-white/10 transition-colors mb-6">
                        {$t("settings.change_folder")}
                    </button>

                    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2 border-t border-white/10 pt-5">{$t("settings.danger")}</div>
                    <button on:click={handleClearData} class="w-full bg-red-900/40 hover:bg-red-800/60 text-red-300 hover:text-red-200 font-semibold py-2.5 rounded-lg border border-red-800/50 transition-colors">
                        {$t("settings.clear")}
                    </button>
                    <p class="text-xs text-slate-500 mt-2">{$t("settings.clear_hint")}</p>

                    <div class="mt-6 pt-4 border-t border-white/10 flex items-center justify-between text-xs text-slate-500">
                        <span>pLauncher <span class="text-slate-400">v{APP_VERSION}</span></span>
                        <span>© 2026 Midaser</span>
                    </div>
                </div>
            </div>
        {/if}

        {#if showCollectionModal}
            <div class="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={() => showCollectionModal = false}>
                <div class="w-[540px] max-w-[92vw] glass-strong rounded-2xl shadow-2xl p-7" on:click|stopPropagation>
                    <div class="flex items-center justify-between mb-5">
                        <h2 class="text-2xl font-black text-white">{editingColId ? $t('col.edit') : $t('col.new')}</h2>
                        <button on:click={() => showCollectionModal = false} class="text-slate-400 hover:text-white text-3xl leading-none">&times;</button>
                    </div>

                    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("col.name")}</div>
                    <input type="text" bind:value={colName} placeholder={$t("col.name_ph")}
                           class="w-full glass text-white rounded-lg px-4 py-2.5 mb-5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40 placeholder:text-slate-500"/>

                    <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("col.type")}</div>
                    <div class="grid grid-cols-2 gap-3 mb-5">
                        <button on:click={() => colType = 'manual'} class="text-left p-3 rounded-lg border transition-colors {colType === 'manual' ? 'border-indigo-500 bg-indigo-600/15' : 'border-white/10 hover:bg-white/5'}">
                            <div class="font-bold text-white text-sm mb-0.5">{$t("col.manual")}</div>
                            <div class="text-xs text-slate-400">{$t("col.manual_hint")}</div>
                        </button>
                        <button on:click={() => colType = 'dynamic'} class="text-left p-3 rounded-lg border transition-colors {colType === 'dynamic' ? 'border-indigo-500 bg-indigo-600/15' : 'border-white/10 hover:bg-white/5'}">
                            <div class="font-bold text-white text-sm mb-0.5">{$t("col.dynamic")}</div>
                            <div class="text-xs text-slate-400">{$t("col.dynamic_hint")}</div>
                        </button>
                    </div>

                    {#if colType === 'dynamic'}
                        <div class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{$t("col.tags")}</div>
                        {#if allTags.length}
                            <div class="flex flex-wrap gap-1.5 mb-5 max-h-40 overflow-y-auto">
                                {#each allTags as t}
                                    <button on:click={() => toggleColTag(t)}
                                            class="px-2.5 py-1 rounded-full text-xs transition-colors {colTags.includes(t) ? 'bg-indigo-500/30 text-white ring-1 ring-indigo-400/40' : 'bg-white/5 text-slate-300 hover:bg-white/10'}">
                                        {t}
                                    </button>
                                {/each}
                            </div>
                        {:else}
                            <p class="text-xs text-slate-500 mb-5">{$t("col.no_tags")}</p>
                        {/if}
                    {:else}
                        <p class="text-xs text-slate-500 mb-5">{$t("col.manual_note")}</p>
                    {/if}

                    <div class="flex items-center gap-3">
                        {#if editingColId}
                            <button on:click={() => deleteCollection(collections.find(c => c.id === editingColId))} class="text-red-400 hover:text-red-300 text-sm font-semibold mr-auto">{$t("btn.delete")}</button>
                        {/if}
                        <button on:click={() => showCollectionModal = false} class="ml-auto px-5 py-2.5 rounded-lg font-semibold text-slate-300 bg-white/5 hover:bg-white/10 border border-white/10 transition-colors">{$t("btn.cancel")}</button>
                        <button on:click={saveCollection} class="px-5 py-2.5 rounded-lg font-bold text-white bg-indigo-600 hover:bg-indigo-500 transition-colors">{$t("btn.save")}</button>
                    </div>
                </div>
            </div>
        {/if}

        {#if confirmDialog.show}
            <div class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={() => closeConfirm(false)}>
                <div class="w-[440px] max-w-[90vw] glass-strong rounded-2xl shadow-2xl p-6" on:click|stopPropagation>
                    <h3 class="text-xl font-black text-white mb-2">{confirmDialog.title}</h3>
                    {#if confirmDialog.message}
                        <p class="text-slate-400 text-sm whitespace-pre-line mb-6">{confirmDialog.message}</p>
                    {:else}
                        <div class="mb-6"></div>
                    {/if}
                    <div class="flex gap-3 justify-end">
                        <button on:click={() => closeConfirm(false)} class="px-5 py-2.5 rounded-lg font-semibold text-slate-300 bg-slate-700/60 hover:bg-slate-600 border border-slate-600/50 transition-colors">
                            {$t("btn.cancel")}
                        </button>
                        <button on:click={() => closeConfirm(true)} class="px-5 py-2.5 rounded-lg font-bold text-white transition-colors {confirmDialog.danger ? 'bg-red-600 hover:bg-red-500' : 'bg-indigo-600 hover:bg-indigo-500'}">
                            {confirmDialog.confirmText}
                        </button>
                    </div>
                </div>
            </div>
        {/if}

        {#if toast.show}
            <div class="fixed bottom-8 right-8 p-4 rounded-lg shadow-2xl flex items-center gap-3 z-[90] animate-fade-in transition-all {toast.type === 'error' ? 'bg-red-900 border border-red-500 text-red-100' : 'bg-slate-800 border-l-4 border-emerald-500 text-slate-200'}">
                {#if toast.type === 'success'}
                    <span class="text-emerald-400 text-xl">✓</span>
                {:else}
                    <span class="text-red-400 text-xl">⚠</span>
                {/if}
                <span class="font-semibold tracking-wide">{toast.message}</span>
            </div>
        {/if}

        {#if ctx.show}
            <div class="fixed inset-0 z-[99]" on:click={closeCtx} on:contextmenu|preventDefault={closeCtx}></div>
            <div class="fixed z-[100] w-[232px] glass-strong rounded-xl shadow-2xl py-1.5 text-sm animate-fade-in" style="left:{ctx.x}px; top:{ctx.y}px">
                {#each ctx.items as it}
                    {#if it.sep}
                        <div class="h-px bg-white/10 my-1.5 mx-2"></div>
                    {:else if it.submenu}
                        <div class="relative group/sub">
                            <button class="w-full flex items-center gap-2.5 px-3 py-1.5 text-left text-slate-200 hover:bg-white/10 transition-colors">
                                {#if it.icon}<span class="w-4 text-center text-xs">{it.icon}</span>{/if}
                                <span class="flex-1">{it.label}</span>
                                <span class="text-slate-500">›</span>
                            </button>
                            <div class="absolute left-full top-0 -ml-1 w-[210px] glass-strong rounded-xl shadow-2xl py-1.5 hidden group-hover/sub:block max-h-72 overflow-y-auto">
                                {#each it.submenu as sub}
                                    <button on:click={() => { sub.action(); closeCtx(); }}
                                            class="w-full flex items-center gap-2 px-3 py-1.5 text-left text-slate-200 hover:bg-white/10 transition-colors">
                                        <span class="w-4 text-center text-xs text-emerald-400">{sub.checked ? '✓' : ''}</span>
                                        <span class="flex-1 truncate">{sub.label}</span>
                                    </button>
                                {/each}
                            </div>
                        </div>
                    {:else}
                        <button on:click={() => { it.action(); closeCtx(); }}
                                class="w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors {it.danger ? 'text-rose-300 hover:bg-rose-500/20' : 'text-slate-200 hover:bg-white/10'}">
                            {#if it.icon}<span class="w-4 text-center text-xs">{it.icon}</span>{/if}
                            <span class="flex-1">{it.label}</span>
                        </button>
                    {/if}
                {/each}
            </div>
        {/if}

    </main>
    </div>
</div>