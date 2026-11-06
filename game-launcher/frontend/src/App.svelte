<script>
    // Корень приложения: состояние библиотеки, навигация и связка компонентов.
    // Разметка крупных частей — в components/, чистые функции — в lib/.
    import { onMount, onDestroy, tick } from 'svelte';
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
        GetLauncherStartInfo,
        GetPrivacy,
        Lock,
        NotifyWindowShown,
        AddSingleGameManual,
        SelectFolder,
        AddScanPath,
        CopyScreenshotToData,
        PreviewGameInfo,
        ApplyGameInfo,
    } from '../wailsjs/go/main/App.js';
    import { OnFileDrop, OnFileDropOff, EventsOn } from '../wailsjs/runtime/runtime';
    import { t, tr, setLang, addLocales, availableLangs, initialLang, langStore } from './i18n.js';
    import { showToast, askConfirm, isConfirmOpen } from './lib/ui.js';
    import { sortGames, lsGet, lsSet, lsJSON } from './lib/util.js';
    import { loadShelves, buildShelf, renderShelves, buildCol } from './lib/shelves.js';
    import { isEditableTarget, inputCtxItems } from './lib/inputMenu.js';
    import { discreet, blurMode } from './lib/privacy.js';
    import { showHero, homeView, ctxGameId, selection, selecting, clearSelection, selectAllVisible, focusNeighbor, stepCardSize, setCardSize, CARD_DEFAULT } from './lib/view.js';
    import { initUpdater, updateModalOpen } from './lib/update.js';
    import { bulkRemove, bulkIgnore, bulkFavorite, bulkDetectLaunch, bulkCheckUpdates, bulkReparse, primaryUrl } from './lib/bulk.js';
    import SmartBanner from './components/SmartBanner.svelte';

    import TitleBar from './components/TitleBar.svelte';
    import Sidebar from './components/Sidebar.svelte';
    import Hero from './components/Hero.svelte';
    import Shelf from './components/Shelf.svelte';
    import GameGrid from './components/GameGrid.svelte';
    import ViewControls from './components/ViewControls.svelte';
    import ViewOptions from './components/ViewOptions.svelte';
    import TagFilter from './components/TagFilter.svelte';
    import Icon from './components/Icon.svelte';
    import { STATUSES } from './lib/status.js';
    import SelectionBar from './components/SelectionBar.svelte';
    import HotkeysHelp from './components/HotkeysHelp.svelte';
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
    import PinDialog from './components/PinDialog.svelte';
    import EmptyLibrary from './components/EmptyLibrary.svelte';
    import UpdateModal from './components/UpdateModal.svelte';

    // --- Состояние библиотеки и навигации ---
    let games = [];
    let scanning = false;
    let checkingUpdates = false;
    let selectedGame = null;
    let gameInfoPreview = null; // { game, source, url }: только в памяти до сохранения
    let savingInfo = false;
    let infoRequest = 0;
    let previewScroll = 0;
    let previewLoading = false;
    let isEditing = false;
    $: editDirty = isEditing && !!selectedGame && JSON.stringify(selectedGame) !== originalGameSnapshot;
    let originalGameSnapshot = '';       // слепок игры для «есть ли несохранённые изменения»
    let searchQuery = '';
    let activeFilter = 'all';
    // Фильтр по тегам: нужные (все или любой из них) и исключённые
    const emptyTags = () => ({ inc: [], exc: [], mode: 'all' });
    let tagFilter = emptyTags();
    $: hasTags = tagFilter.inc.length + tagFilter.exc.length > 0;
    let activeCollection = '';           // открытая коллекция (id; '__uncat' — без категории)
    let isDragging = false;              // оверлей при перетаскивании папок из проводника
    let scanPaths = [];
    let supportedSources = [];
    let appVersion = '';

    // Итог прошлого обновления и состояние окна до перезапуска
    async function applyStartInfo() {
        try {
            const info = await GetLauncherStartInfo();
            if (info.discreet) discreet.set(true);
            if (info.failed_update) showToast(tr('upd.failed', { v: info.failed_update }), 'error');
            else if (info.updated_to) showToast(tr('upd.updated_to', { v: info.updated_to }), 'success');
        } catch (e) {}
    }

    let sortBy = lsGet('plauncher_sort', 'added_desc');
    $: lsSet('plauncher_sort', sortBy);

    // «Только оформленные» — скрывать игры без обложки

    // Модалки
    let needsSetup = false;
    let showSettings = false;
    let showRelink = false;
    let collectionModal = null;          // null — закрыто; {collection: null|col}
    let lightboxIndex = null;            // null — закрыто
    // Картинки страницы игры: обложка первой, затем скриншоты (тот же порядок, что в ленте)
    $: galleryGame = gameInfoPreview ? gameInfoPreview.game : selectedGame;
    $: gallery = galleryGame ? [...(galleryGame.cover_path ? [galleryGame.cover_path] : []), ...(galleryGame.images || [])] : [];
    let ctx = null;                      // {x, y, items} — контекстное меню

    // --- Приватность: PIN-блокировка, паника, скрытые коллекции ---
    let privacy = null;                  // GetPrivacy()
    let locked = false;                  // лаунчер заблокирован PIN-кодом
    let showHidden = false;              // скрытые коллекции показаны (до блокировки/паники)
    let pinPrompt = null;                // {onSuccess} — проверка PIN для показа скрытого
    let showHotkeys = false;             // справка по горячим клавишам (F1)
    let searchInput;                     // поле поиска (Ctrl+F)

    async function refreshPrivacy() {
        try {
            privacy = await GetPrivacy();
            blurMode.set(privacy.blur_mode || 'none');
        } catch (e) { console.error(e); }
    }
    // Закрыть всё, что может показывать содержимое библиотеки
    function closeOverlays() {
        clearSelection();
        showHotkeys = false;
        updateModalOpen.set(false);
        closeGame();
        showSettings = false;
        showRelink = false;
        collectionModal = null;
        lightboxIndex = null;
        ctx = null;
        pinPrompt = null;
    }
    function onLocked() {
        locked = true;
        showHidden = false;
        closeOverlays();
        games = [];
        collections = [];
    }
    async function onUnlocked() {
        locked = false;
        await refreshPrivacy();
        await reloadLibrary(false);
        await loadCustomLocales();
    }
    function onPanic() {
        showHidden = false;
        discreet.set(true);              // вернувшись, обложки будут размыты
        closeOverlays();
    }
    async function lockNow() {
        try { await Lock(); } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }
    // Показать/спрятать скрытые коллекции (Ctrl+Shift+H). При заданном PIN — спросить его.
    function toggleHidden() {
        if (showHidden) { showHidden = false; return; }
        if (privacy && privacy.has_pin) pinPrompt = { onSuccess: () => { pinPrompt = null; showHidden = true; } };
        else showHidden = true;
    }

    // Автоблокировка после бездействия
    let lastActivity = Date.now();
    function markActivity() { lastActivity = Date.now(); }
    const ACTIVITY_EVENTS = ['mousemove', 'mousedown', 'keydown', 'wheel'];
    let idleTimer = null;
    function onWindowFocus() { NotifyWindowShown().catch(() => {}); markActivity(); }
    function checkIdle() {
        if (locked || !privacy || !privacy.has_pin || !privacy.idle_lock_min) return;
        if (Date.now() - lastActivity > privacy.idle_lock_min * 60 * 1000) lockNow();
    }

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
    // Скрытые коллекции: их игры не видны нигде, пока не показаны (showHidden)
    $: hiddenCols = collections.filter(c => c.hidden);
    $: hiddenIds = new Set(hiddenCols.flatMap(c => buildCol(c, games).items.map(g => g.id)));
    $: visibleGames = showHidden || !hiddenIds.size ? games : games.filter(g => !hiddenIds.has(g.id));
    $: visibleCollections = showHidden ? collections : collections.filter(c => !c.hidden);
    // Открытая коллекция: её игры (динамические — по тегам), «Без категории» — не попавшие никуда
    $: collectionIds = !activeCollection ? null
        : activeCollection === '__uncat' ? new Set(uncategorized.map(g => g.id))
        : new Set(((collectionsView.find(c => c.id === activeCollection) || {}).items || []).map(g => g.id));
    $: openedCollection = activeCollection && activeCollection !== '__uncat' ? collectionsView.find(c => c.id === activeCollection) : null;
    function matchTags(gt, tf) {
        if (tf.exc.some(x => gt.includes(x))) return false;
        if (!tf.inc.length) return true;
        return tf.mode === 'any' ? tf.inc.some(x => gt.includes(x)) : tf.inc.every(x => gt.includes(x));
    }
    $: filteredGames = sortGames(visibleGames.filter(g => {
        if (collectionIds && !collectionIds.has(g.id)) return false;
        if (hasTags && !matchTags(g.tags || [], tagFilter)) return false;
        if (activeFilter === 'favorites') return g.favorite;
        if (activeFilter === 'updates') return g.update_available;
        if (activeFilter === 'nocover') return !g.cover_path;
        if (activeFilter === 'missing') return g.folder_missing;
        if (activeFilter.startsWith('status:')) return (g.status || '') === activeFilter.slice(7);
        return true;
    }), sortBy);
    $: allGamesSorted = sortGames(visibleGames, sortBy);
    $: updatesGames = visibleGames.filter(g => g.update_available);
    $: missingGames = visibleGames.filter(g => g.folder_missing);
    $: noCoverCount = visibleGames.filter(g => !g.cover_path).length;
    // Умные списки: плашка с объяснением и общим действием ($langStore — пересчёт при смене языка)
    $: noCoverWithLink = visibleGames.filter(g => !g.cover_path && primaryUrl(g));
    $: smartBanner = ($langStore, searchQuery.trim() || hasTags ? null
        : activeFilter === 'updates' ? {
            tone: 'update', title: tr('smart.updates_title', { n: updatesGames.length }), text: tr('smart.updates_text'),
            action: checkingUpdates ? tr('menu.checking') : tr('menu.check_updates'), busy: checkingUpdates, onAction: handleCheckAllUpdates }
        : activeFilter === 'nocover' ? {
            tone: 'info', title: tr('smart.nocover_title', { n: noCoverCount }),
            text: noCoverWithLink.length ? tr('smart.nocover_text_links', { n: noCoverWithLink.length }) : tr('smart.nocover_text'),
            action: noCoverWithLink.length ? tr('smart.nocover_action') : '', onAction: () => bulkReparse(noCoverWithLink, loadGames) }
        : activeFilter === 'missing' ? {
            tone: 'danger', title: tr('smart.missing_title', { n: missingGames.length }), text: tr('smart.missing_text'),
            action: tr('smart.missing_action'), onAction: () => showRelink = true }
        : null);
    // Убранные из «Продолжить»: id → время последнего запуска на момент скрытия.
    // Запустил игру снова — время запуска стало новее, и она возвращается сама.
    let continueHidden = lsJSON('plauncher_continue_hidden', {});
    const inContinue = (g, hidden) => (g.last_launched_at || 0) > 0 && !['finished', 'dropped'].includes(g.status)
        && !(hidden[g.id] && hidden[g.id] >= (g.last_launched_at || 0));
    function hideFromContinue(g) {
        continueHidden = { ...continueHidden, [g.id]: g.last_launched_at || 0 };
        lsSet('plauncher_continue_hidden', JSON.stringify(continueHidden));
        showToast(tr('continue.hidden', { title: g.title }), 'success');
    }
    $: recentlyPlayed = visibleGames
        .filter(g => inContinue(g, continueHidden))
        .sort((a, b) => (b.last_launched_at || 0) - (a.last_launched_at || 0));
    $: recentlyAdded = [...visibleGames].sort((a, b) => (b.added_at || 0) - (a.added_at || 0));
    $: favoriteGames = visibleGames.filter(g => g.favorite);
    $: heroGame = recentlyPlayed[0] || recentlyAdded[0] || null;
    // Домашний вид: без поиска, фильтра и выбранного тега
    $: isHome = searchQuery.trim() === '' && activeFilter === 'all' && !hasTags && !activeCollection;
    // Полки — только на «Все игры» без фильтров и если выбран этот вид
    $: showShelves = isHome && $homeView === 'shelves';
    // Счётчики для чипсов — в пределах открытой коллекции
    $: scopeGames = collectionIds ? visibleGames.filter(g => collectionIds.has(g.id)) : visibleGames;
    $: chips = ($langStore, [
        { id: 'all', label: tr('chip.all'), n: scopeGames.length },
        // Статусы — только те, что встречаются в текущем виде
        ...STATUSES.map(st => ({ id: 'status:' + st.id, label: tr('status.' + st.id), n: scopeGames.filter(g => g.status === st.id).length, dot: st.dot }))
            .filter(c => c.n > 0),
        { id: 'favorites', label: tr('nav.favorites'), n: scopeGames.filter(g => g.favorite).length },
        ...(scopeGames.some(g => g.update_available) ? [{ id: 'updates', label: tr('nav.updates'), n: scopeGames.filter(g => g.update_available).length }] : []),
    ]);

    // Заголовок текущего вида ($langStore — зависимость от языка)
    $: viewTitle = ($langStore,
        activeCollection === '__uncat' ? tr('nav.uncategorized')
        : openedCollection ? openedCollection.name
        : searchQuery.trim() ? tr('view.search')
        : hasTags ? (tagFilter.inc.length === 1 && !tagFilter.exc.length ? tr('view.tag', { tag: tagFilter.inc[0] }) : tr('view.tags'))
        : activeFilter === 'favorites' ? tr('shelf.favorites')
        : activeFilter === 'updates' ? tr('nav.updates')
        : activeFilter === 'nocover' ? tr('nav.no_cover')
        : activeFilter === 'missing' ? tr('missing.nav')
        : activeFilter.startsWith('status:') ? tr('status.' + activeFilter.slice(7))
        : tr('nav.all_games'));

    // Уникальные теги по всей библиотеке, по частоте
    $: allTags = (() => {
        const counts = {};
        for (const g of visibleGames) for (const tag of (g.tags || [])) counts[tag] = (counts[tag] || 0) + 1;
        return Object.keys(counts).sort((a, b) => counts[b] - counts[a]);
    })();

    // --- Коллекции (как в Steam: ручные + динамические по тегам) ---
    let collections = [];
    $: collectionsView = visibleCollections.map(c => buildCol(c, visibleGames));
    $: categorizedIds = new Set(collectionsView.flatMap(c => c.items.map(g => g.id)));
    $: uncategorized = sortGames(visibleGames.filter(g => !categorizedIds.has(g.id)), 'title_asc');
    $: sidebarGroups = ($langStore, [
        ...collectionsView.map(c => ({ key: c.id, name: c.name, items: sortGames(c.items, 'title_asc'), col: c })),
        ...(uncategorized.length ? [{ key: '__uncat', name: tr('nav.uncategorized'), items: uncategorized, col: null }] : []),
    ]);
    $: manualCollections = visibleCollections.filter(c => c.type === 'manual');

    async function loadCollections() {
        try { collections = await GetCollections() || []; } catch (e) { console.error(e); }
    }
    async function persistCollections() {
        try { await SaveCollections(collections); } catch (err) { showToast(tr('toast.col_fail', { err }), 'error'); }
    }
    function openCreateCollection() { collectionModal = { collection: null }; }
    function openEditCollection(c) { collectionModal = { collection: c }; }
    async function saveCollection({ name, type, tags, hidden }) {
        const editing = collectionModal && collectionModal.collection;
        if (editing) {
            collections = collections.map(c => c.id === editing.id
                ? { ...c, name, type, hidden, tags: type === 'dynamic' ? tags : (c.tags || []) }
                : c);
        } else {
            collections = [...collections, {
                id: 'c_' + Math.random().toString(36).slice(2, 9),
                name, type, hidden, game_ids: [], tags: type === 'dynamic' ? tags : [],
            }];
        }
        collectionModal = null;
        await persistCollections();
        if (hidden && !showHidden) showToast(tr('privacy.col_hidden_toast', { name }), 'success');
    }
    // Скрыть коллекцию / вернуть в библиотеку (из контекстного меню)
    async function toggleHiddenCol(col) {
        const hidden = !col.hidden;
        collections = collections.map(c => c.id === col.id ? { ...c, hidden } : c);
        await persistCollections();
        if (hidden && !showHidden) showToast(tr('privacy.col_hidden_toast', { name: col.name }), 'success');
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
    $: shelfData = { recentlyPlayed, recentlyAdded, favoriteGames, allGamesSorted, games: visibleGames, collectionsView, _lang: $langStore };
    // Баннер выключен — дубля нет, полку не трогаем
    $: renderedShelves = renderShelves(shelves, shelfData, $showHero ? heroGame : null);
    function shelfMenuItems(shelf) {
        return [
            { label: tr('ctx.configure_sections'), icon: 'layers', action: () => layoutEditing = true },
            { label: tr('ctx.remove_section'), icon: 'trash', danger: true, action: () => shelves = shelves.filter(s => s.id !== shelf.id) },
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
            pruneSelection();
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
        if (resetSelection) { clearInfoPreview(); selectedGame = null; isEditing = false; }
        try { scanPaths = await GetScanPaths(); } catch (e) {}
        await refreshPrivacy();          // у другой библиотеки (импорт/смена папки) свои настройки
        await loadCollections();
        await loadGames();
    }

    async function handleScan() {
        if (scanning) return;
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

    // Пустая библиотека: выбрать папку с играми, добавить в сканирование и сразу просканировать
    async function addGamesFolder() {
        try {
            const p = await SelectFolder();
            if (!p) return;
            await AddScanPath(p);
            scanPaths = await GetScanPaths();
            await handleScan();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

    async function addSingleGame() {
        try {
            await AddSingleGameManual();
            await loadGames();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

    // Кнопка «Сканировать» в боковой панели: без папок для сканирования — сразу выбор папки
    function scanOrAddFolder() {
        if (scanPaths.length) handleScan();
        else addGamesFolder();
    }

    async function handleSearch() {
        try {
            games = searchQuery.trim() === '' ? (await GetGames() || []) : (await SearchGames(searchQuery) || []);
        } catch (err) {
            console.error('Ошибка поиска:', err);
        }
    }

    // Убрать из выделения игры, которых больше нет (удалены/скрыты)
    function pruneSelection() {
        if (!$selection.size) return;
        const ids = new Set(games.map(g => g.id));
        const kept = [...$selection].filter(id => ids.has(id));
        if (kept.length !== $selection.size) selection.set(new Set(kept));
    }

    // --- Массовые действия с коллекциями ---
    async function bulkAddToCollection(colId, ids) {
        const col = collections.find(c => c.id === colId);
        if (!col) return;
        const set = new Set(col.game_ids || []);
        ids.forEach(id => set.add(id));
        collections = collections.map(c => c.id === colId ? { ...c, game_ids: [...set] } : c);
        await persistCollections();
        showToast(tr('bulk.added_to', { n: ids.length, name: col.name }), 'success');
    }
    async function bulkRemoveFromCollection(colId, ids) {
        const col = collections.find(c => c.id === colId);
        if (!col) return;
        const drop = new Set(ids);
        collections = collections.map(c => c.id === colId ? { ...c, game_ids: (c.game_ids || []).filter(id => !drop.has(id)) } : c);
        await persistCollections();
        showToast(tr('bulk.removed_from', { name: col.name }), 'success');
    }

    // --- Прокрутка основной колонки ---
    let mainEl;
    let showToTop = false;
    let libScroll = 0;                  // где были в библиотеке, когда открыли игру
    function onMainScroll() { showToTop = !!mainEl && mainEl.scrollTop > mainEl.clientHeight * 0.6; }
    function scrollTop(smooth) { if (mainEl) mainEl.scrollTo({ top: 0, behavior: smooth ? 'smooth' : 'auto' }); }
    // Сменили вид: новый список — сверху; тот же вид (повторный щелчок) — плавно наверх
    async function afterViewChange(same) { libScroll = 0; await tick(); scrollTop(same); }

    // Превью не изменяет запись библиотеки; применяем те же непустые поля, что и бэкенд.
    function clearInfoPreview() {
        infoRequest++;
        gameInfoPreview = null;
        previewLoading = false;
        savingInfo = false;
        lightboxIndex = null;
    }
    async function previewInfo(hit, folderVersion) {
        if (!selectedGame || locked) return;
        const original = selectedGame;
        const ticket = ++infoRequest;
        previewLoading = true;
        try {
            const metadata = await PreviewGameInfo(hit.url);
            if (ticket !== infoRequest || locked || selectedGame?.id !== original.id || !metadata) return;
            const merged = { ...original };
            for (const key of ['title', 'version', 'description', 'author', 'engine', 'cover_path']) {
                if (metadata[key]) merged[key] = metadata[key];
            }
            for (const key of ['images', 'languages']) if (metadata[key]?.length) merged[key] = metadata[key];
            merged.tags = [...(original.tags || [])];
            const tagKeys = new Set(merged.tags.map(tag => tag.toLowerCase()));
            for (const tag of metadata.tags || []) {
                if (!tagKeys.has(tag.toLowerCase())) { tagKeys.add(tag.toLowerCase()); merged.tags.push(tag); }
            }
            if (!merged.version) merged.version = folderVersion;
            merged.sources = [...(original.sources || []).filter(s => s.source !== hit.source), { source: hit.source, url: hit.url, last_version: metadata.version || '' }];
            merged.primary_source = original.primary_source || hit.source;
            previewScroll = mainEl?.scrollTop || 0;
            gameInfoPreview = { game: merged, source: hit.source, url: hit.url };
            await tick();
            scrollTop(false);
            mainEl?.querySelector('[data-preview-back]')?.focus({ preventScroll: true });
        } catch (err) {
            if (ticket === infoRequest && !locked) showToast(tr('toast.error', { err }), 'error');
        } finally { if (ticket === infoRequest) previewLoading = false; }
    }
    async function backFromPreview() {
        if (savingInfo) return;
        const url = gameInfoPreview?.url;
        clearInfoPreview();
        await tick();
        if (mainEl) {
            mainEl.scrollTop = previewScroll;
            [...mainEl.querySelectorAll('[data-hit-url]')].find(el => el.dataset.hitUrl === url)?.focus({ preventScroll: true });
        }
    }
    async function saveInfoPreview() {
        if (!gameInfoPreview || savingInfo || locked) return;
        const chosen = gameInfoPreview;
        const ticket = infoRequest;
        savingInfo = true;
        try {
            await ApplyGameInfo(chosen.game.id, chosen.url);
            if (ticket !== infoRequest || locked) return;
            const fresh = await GetGames();
            if (ticket !== infoRequest || locked) return;
            games = fresh || [];
            selectedGame = games.find(g => g.id === chosen.game.id) || selectedGame;
            clearInfoPreview();
            showToast(tr('find.saved'), 'success');
            await tick();
            scrollTop(false);
        } catch (err) {
            if (ticket === infoRequest && !locked) showToast(tr('toast.error', { err }), 'error');
        } finally { if (ticket === infoRequest) savingInfo = false; }
    }

    // --- Действия с играми ---
    async function selectGame(game) {
        clearInfoPreview();
        if (!selectedGame && mainEl) libScroll = mainEl.scrollTop;
        const other = !selectedGame || selectedGame.id !== game.id;
        selectedGame = game;
        isEditing = false;
        originalGameSnapshot = JSON.stringify(game);
        if (other) { await tick(); scrollTop(false); }
    }
    async function closeGame() {
        clearInfoPreview();
        const wasOpen = !!selectedGame;
        selectedGame = null;
        isEditing = false;
        if (wasOpen) { await tick(); if (mainEl) mainEl.scrollTop = libScroll; onMainScroll(); }
    }
    // Пункт боковой панели (все игры, избранное, умные списки): сбрасывает коллекцию и тег
    function applyFilter(filter) {
        const same = !selectedGame && activeFilter === filter && !activeCollection && !hasTags;
        clearSelection();
        afterViewChange(same);
        activeFilter = filter;
        tagFilter = emptyTags();
        activeCollection = '';
        closeGame();
    }
    // Открыть коллекцию сеткой (щелчок по её названию в панели)
    function openCollection(key) {
        const same = !selectedGame && activeCollection === key;
        clearSelection();
        afterViewChange(same);
        activeCollection = key;
        activeFilter = 'all';
        tagFilter = emptyTags();
        closeGame();
    }
    // Чипсы над сеткой: фильтр внутри текущей коллекции
    function applyChip(id) { clearSelection(); activeFilter = id; afterViewChange(false); }
    // Сброс всего: поиск, фильтр, тег, коллекция
    function resetView() {
        searchQuery = '';
        applyFilter('all');
        handleSearch();
    }
    // Ввод в поиске (верхняя панель): ищем по всей библиотеке, а не внутри открытой коллекции
    function onSearchInput() {
        if (selectedGame) closeGame();
        if (searchQuery.trim()) { activeCollection = ''; activeFilter = 'all'; tagFilter = emptyTags(); }
        handleSearch();
    }
    // «Теги ▾»: теги текущего вида (коллекции/фильтра) по частоте
    $: scopeTags = (() => {
        const counts = {};
        for (const g of scopeGames) for (const tag of (g.tags || [])) counts[tag] = (counts[tag] || 0) + 1;
        return Object.keys(counts).sort((a, b) => counts[b] - counts[a]).map(tag => ({ tag, n: counts[tag] }));
    })();
    function setTags(tf) { clearSelection(); tagFilter = tf; afterViewChange(false); }
    function applyTag(tag) {
        tagFilter = { inc: [tag], exc: [], mode: 'all' };
        activeFilter = 'all';
        activeCollection = '';
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

    // Игнорировать при сканировании: убрать из лаунчера, папку больше не добавлять
    async function ignoreGame(game) {
        if (await bulkIgnore([game.id], loadGames, game.title) && selectedGame && selectedGame.id === game.id) closeGame();
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
    // Контекстное меню открывается от точки щелчка (и для ⋯ тоже — как в Epic: меню ложится поверх
    // кнопки «Играть»). Не влезает вправо/вниз — ContextMenu открывает его зеркально, влево/вверх.
    function openCtx(e, items) {
        e.preventDefault(); e.stopPropagation();
        ctx = { x: e.clientX, y: e.clientY, items };
    }
    // ПКМ по одной из нескольких выделенных карточек — меню массовых действий
    function bulkMenuItems() {
        const ids = [...$selection];
        const picked = visibleGames.filter(g => $selection.has(g.id));
        const allFav = picked.length > 0 && picked.every(g => g.favorite);
        return [
            { label: tr('bulk.add_to'), icon: 'plus', submenu: visibleCollections.map(c => ({ label: c.name, action: () => bulkAddToCollection(c.id, ids) })) },
            { label: allFav ? tr('ctx.fav_remove') : tr('bulk.fav'), icon: allFav ? 'heart-off' : 'heart', tone: allFav ? '' : 'pink', action: () => bulkFavorite(ids, !allFav, loadGames) },
            { label: tr('bulk.detect'), icon: 'search', action: () => bulkDetectLaunch(ids, loadGames) },
            { label: tr('bulk.check'), icon: 'update', tone: 'amber', action: () => bulkCheckUpdates(picked, loadGames) },
            { label: tr('bulk.reparse'), icon: 'refresh', action: () => bulkReparse(picked, loadGames) },
            { sep: true },
            { label: tr('sel.clear'), icon: 'x', action: clearSelection },
            { label: tr('ignore.action'), icon: 'ban', action: () => bulkIgnore(ids, loadGames) },
            { label: tr('bulk.remove_n', { n: ids.length }), icon: 'trash', danger: true, action: () => bulkRemove(ids, loadGames) },
        ];
    }
    function gameMenuItems(game) {
        if ($selection.size > 1 && $selection.has(game.id)) return bulkMenuItems();
        const items = [];
        if (game.folder_missing) items.push({ label: tr('missing.pick_long'), icon: 'alert', tone: 'amber', action: () => relinkManually(game) });
        else if (game.exec_path) items.push({ label: tr('ctx.play'), icon: 'play', action: () => playGame(game) });
        else items.push({ label: tr('ctx.set_exe'), icon: 'play', action: () => selectGame(game) });
        if (inContinue(game, continueHidden)) items.push({ label: tr('continue.hide'), icon: 'x', action: () => hideFromContinue(game) });
        items.push({ label: game.favorite ? tr('ctx.fav_remove') : tr('ctx.fav_add'), icon: game.favorite ? 'heart-off' : 'heart', tone: game.favorite ? '' : 'pink', action: () => toggleFavorite(game) });
        items.push({
            label: tr('ctx.add_to'), icon: 'plus',
            submenu: visibleCollections.map(c => ({ label: c.name, checked: (c.game_ids || []).includes(game.id), action: () => toggleGameInCollection(c.id, game) }))
                .concat([{ label: tr('ctx.new_collection'), action: openCreateCollection }]),
        });
        items.push({ sep: true });
        items.push({ label: tr('ctx.open_folder'), icon: 'folder', action: () => OpenFolder(game.folder_path) });
        items.push({ label: tr('ctx.edit'), icon: 'edit', action: () => { selectGame(game); startEdit(); } });
        items.push({ label: tr('ignore.action'), icon: 'ban', action: () => ignoreGame(game) });
        items.push({ label: tr('ctx.remove_game'), icon: 'trash', danger: true, action: () => removeGame(game) });
        return items;
    }
    // «+ В коллекцию» на странице игры: ручные коллекции с галочками и «Новая коллекция»
    function collectionsMenuItems(game) {
        return [
            ...manualCollections.map(c => ({ label: c.name, checked: (c.game_ids || []).includes(game.id), action: () => toggleGameInCollection(c.id, game) })),
            ...(manualCollections.length ? [{ sep: true }] : []),
            { label: tr('ctx.new_collection'), icon: 'plus', action: openCreateCollection },
        ];
    }
    const gameCtx = (g, e) => { openCtx(e, gameMenuItems(g)); ctxGameId.set(g.id); };
    $: if (!ctx) ctxGameId.set('');
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
    // Перетаскивание, начатое внутри окна (карточка, миниатюра), — не файлы из проводника,
    // даже если браузер кладёт в него тип Files. Оверлей для него не показываем.
    let internalDrag = false;
    function isFileDrag(e) {
        return !internalDrag && e.dataTransfer && Array.from(e.dataTransfer.types || []).includes('Files');
    }
    function onDragOver(e) { if (isFileDrag(e)) { e.preventDefault(); isDragging = true; } }
    function onDragLeave(e) { if (isFileDrag(e)) { e.preventDefault(); isDragging = false; } }
    // Страховка от «залипшего» оверлея: отпустили где угодно, перетаскивание кончилось,
    // или мышь снова двигается без перетаскивания (во время drag событий mousemove нет).
    function endDrag() { internalDrag = false; isDragging = false; }
    function onWindowDrop(e) { if (isFileDrag(e)) e.preventDefault(); isDragging = false; }
    function onWindowMouseMove() { if (isDragging) isDragging = false; }
    const IMAGE_RE = /\.(png|jpe?g|webp|gif|avif|bmp)$/i;
    async function handleNativeDrop(paths) {
        isDragging = false;
        if (!paths || paths.length === 0) return;
        if (isEditing && selectedGame && paths.every(p => IMAGE_RE.test(p))) {
            try {
                const added = [];
                for (const p of paths) added.push(await CopyScreenshotToData(selectedGame.id, p));
                selectedGame.images = [...(selectedGame.images || []), ...added];
            } catch (err) { showToast(tr('toast.screens_fail', { err }), 'error'); }
            return;
        }
        try {
            const added = await AddGamesFromDrop(paths);
            await loadGames();
            showToast(added > 0 ? tr('toast.scan_added', { n: added }) : tr('toast.scan_none'), 'success');
        } catch (err) {
            showToast(tr('toast.error', { err }), 'error');
        }
    }

    // --- История переходов (как в браузере): вид библиотеки + открытая игра ---
    let navStack = [];
    let navPos = -1;
    let navApplying = false;
    $: navKey = JSON.stringify({ g: selectedGame ? selectedGame.id : '', f: activeFilter, c: activeCollection, t: tagFilter });
    $: recordNav(navKey);
    function recordNav(key) {
        if (navApplying || navStack[navPos] === key) return;
        navStack = [...navStack.slice(0, navPos + 1), key].slice(-60);
        navPos = navStack.length - 1;
    }
    async function goNav(dir) {
        if (gameInfoPreview) { if (dir < 0) backFromPreview(); return; }
        const i = navPos + dir;
        if (i < 0 || i >= navStack.length || isEditing) return;
        navApplying = true;
        navPos = i;
        const st = JSON.parse(navStack[i]);
        clearSelection();
        activeFilter = st.f; activeCollection = st.c; tagFilter = st.t || emptyTags();
        const g = st.g ? games.find(x => x.id === st.g) : null;
        if (g) selectGame(g); else closeGame();
        await tick();
        navApplying = false;
    }

    // --- Клавиатура и мышь ---
    // Диалог подтверждения и лайтбокс обрабатывают свои клавиши сами и вызывают
    // preventDefault — такие события здесь пропускаются.
    function handleKeydown(e) {
        if (e.defaultPrevented || locked || pinPrompt) return;
        const isH = ['h', 'H', 'р', 'Р'].includes(e.key) || e.code === 'KeyH';
        // Ctrl+Shift+H — показать/спрятать скрытые коллекции
        if (e.ctrlKey && e.shiftKey && isH) {
            e.preventDefault();
            toggleHidden();
            return;
        }
        // Дискретный режим (Boss key): Ctrl+H — мгновенно заблюрить/показать обложки
        if (e.ctrlKey && isH) {
            e.preventDefault();
            discreet.update(v => !v);
            return;
        }
        if (isConfirmOpen() || lightboxIndex !== null) return;
        const ctrl = e.ctrlKey || e.metaKey;
        const inInput = isEditableTarget(e.target) || (e.target && e.target.tagName === 'SELECT');
        const modalOpen = needsSetup || showSettings || showRelink || collectionModal || showHotkeys || ctx || $updateModalOpen;

        // F5/Ctrl+R перезагрузили бы страницу WebView (и сбросили состояние): F5 — скан
        if (e.key === 'F5' || (ctrl && e.code === 'KeyR')) {
            e.preventDefault();
            if (!modalOpen && e.key === 'F5') handleScan();
            return;
        }
        if (e.key === 'Escape') {
            // Escape закрывает верхний слой: меню → модалка → редактор → выделение → карточка игры
            if (ctx) ctx = null;
            else if ($updateModalOpen) updateModalOpen.set(false);
            else if (showHotkeys) showHotkeys = false;
            else if (collectionModal) collectionModal = null;
            else if (showRelink) showRelink = false;
            else if (showSettings) showSettings = false;
            else if (gameInfoPreview) { e.preventDefault(); backFromPreview(); }
            else if (previewLoading) { e.preventDefault(); clearInfoPreview(); }
            else if (isEditing) cancelEdit();
            else if ($selecting) clearSelection();
            else if (inInput) e.target.blur();
            else if (selectedGame) closeGame();
            return;
        }
        if (needsSetup) return;
        if (isEditing && ctrl && e.code === 'KeyS') { e.preventDefault(); saveEdit(); return; }
        if (e.altKey && (e.key === 'ArrowLeft' || e.key === 'ArrowRight') && !modalOpen) {
            e.preventDefault();
            goNav(e.key === 'ArrowLeft' ? -1 : 1);
            return;
        }
        if (e.key === 'F1') { e.preventDefault(); showHotkeys = !showHotkeys; return; }
        if (ctrl && e.key === ',') { e.preventDefault(); showSettings = true; return; }
        if (ctrl && e.code === 'KeyL') { e.preventDefault(); if (privacy && privacy.has_pin) lockNow(); return; }
        if (modalOpen) return;

        // Поиск: Ctrl+F или «/»
        if ((ctrl && e.code === 'KeyF') || (!inInput && e.key === '/')) {
            e.preventDefault();
            if (isEditing) return;
            closeGame();
            setTimeout(() => { if (searchInput) { searchInput.focus(); searchInput.select(); } }, 0);
            return;
        }
        // Размер карточек
        if (ctrl && (e.key === '=' || e.key === '+')) { e.preventDefault(); stepCardSize(1); return; }
        if (ctrl && e.key === '-') { e.preventDefault(); stepCardSize(-1); return; }
        if (ctrl && e.key === '0') { e.preventDefault(); setCardSize(CARD_DEFAULT); return; }
        if (inInput) return;

        // Страница игры: Ctrl+Enter — играть, Backspace — назад
        if (selectedGame) {
            if (gameInfoPreview) {
                if (ctrl && e.key === 'Enter') e.preventDefault();
                else if (e.key === 'Backspace') { e.preventDefault(); backFromPreview(); }
                return;
            }
            if (isEditing) return;
            if (ctrl && e.key === 'Enter') { e.preventDefault(); playGame(selectedGame); }
            else if (e.key === 'Backspace') { e.preventDefault(); closeGame(); }
            return;
        }
        // Библиотека: стрелки, выделение
        const dirs = { ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'up', ArrowDown: 'down' };
        if (dirs[e.key] && !e.altKey) { e.preventDefault(); focusNeighbor(dirs[e.key]); return; }
        if (ctrl && e.code === 'KeyA') { e.preventDefault(); selectAllVisible(); return; }
        if (e.key === 'Delete' && $selection.size) { e.preventDefault(); bulkRemove([...$selection], loadGames); return; }
    }
    // Ctrl+колесо над библиотекой — размер карточек
    function onMainWheel(e) {
        if (!(e.ctrlKey || e.metaKey) || selectedGame) return;
        e.preventDefault();
        stepCardSize(e.deltaY < 0 ? 1 : -1);
    }
    // Боковая кнопка мыши «Назад» — выйти из карточки игры в сетку
    function handleGlobalMouseDown(e) {
        if (e.button !== 3 && e.button !== 4) return;
        const handled = e.defaultPrevented; // лайтбокс уже обработал нажатие
        e.preventDefault();
        if (handled || lightboxIndex !== null || locked || pinPrompt || isConfirmOpen() || showSettings || showRelink || collectionModal || showHotkeys || ctx || $updateModalOpen) return;
        goNav(e.button === 3 ? -1 : 1);
    }
    // Chromium выполняет нативный переход на отпускании боковой кнопки.
    // Отменяем и его, чтобы возврат из превью не выгружал весь WebView.
    function preventBrowserMouseNav(e) {
        if (e.button === 3 || e.button === 4) e.preventDefault();
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
        EventsOn('games-updated', () => { if (!locked) loadGames(); });
        EventsOn('privacy-locked', onLocked);
        EventsOn('privacy-panic', onPanic);
        window.addEventListener('resize', onWindowResize);
        // Окно снова в фокусе (например, развернули после «свернуть по панике»)
        window.addEventListener('focus', onWindowFocus);
        for (const ev of ACTIVITY_EVENTS) window.addEventListener(ev, markActivity, { passive: true });
        idleTimer = setInterval(checkIdle, 20000);

        try { supportedSources = await GetSupportedSources() || []; } catch (e) {}
        try { appVersion = await GetAppVersion(); } catch (e) {}
        await applyStartInfo();
        // Обновление проверяем и на экране блокировки: плашка всё равно появится только после PIN
        initUpdater({ appVersion, busy: () => isEditing || gameInfoPreview || previewLoading ? 'editing' : '' });

        // Первый запуск: папка для данных ещё не выбрана — мастер настройки
        try {
            if (!(await IsConfigured())) { needsSetup = true; return; }
        } catch (err) {
            console.error('Ошибка проверки конфигурации:', err);
        }
        await refreshPrivacy();
        if (privacy && privacy.start_discreet) discreet.set(true);
        // С PIN-кодом лаунчер стартует заблокированным: библиотеку грузим после ввода
        if (privacy && privacy.locked) { locked = true; return; }
        await reloadLibrary(false);
        await loadCustomLocales();
    });

    onDestroy(() => {
        OnFileDropOff();
        window.removeEventListener('resize', onWindowResize);
        window.removeEventListener('focus', onWindowFocus);
        for (const ev of ACTIVITY_EVENTS) window.removeEventListener(ev, markActivity);
        clearInterval(idleTimer);
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

<svelte:window on:keydown={handleKeydown} on:mousedown={handleGlobalMouseDown}
               on:dragstart={() => internalDrag = true} on:dragend={endDrag} on:drop={onWindowDrop} on:mousemove={onWindowMouseMove} on:mouseup={preventBrowserMouseNav} on:auxclick={preventBrowserMouseNav}/>
<div class="flex flex-col h-screen overflow-hidden aurora-bg text-slate-300 font-sans relative"
     on:dragover={onDragOver}
     on:dragleave={onDragLeave}
     on:contextmenu={handleGlobalContext}>

    <TitleBar {scanning} {checkingUpdates} {locked} {showHidden}
              bind:searchQuery bind:searchInput onSearch={onSearchInput}
              ready={!needsSetup}
              hasPin={!!(privacy && privacy.has_pin)}
              hiddenCount={hiddenCols.length}
              onLock={lockNow}
              onToggleHidden={toggleHidden}
              onHotkeys={() => showHotkeys = true}
              onSettings={() => showSettings = true}/>

    <div class="flex flex-1 overflow-hidden min-h-0 relative">
        {#if isDragging}
            <div class="absolute inset-0 bg-indigo-900/80 backdrop-blur-sm z-50 flex flex-col items-center justify-center border-4 border-dashed border-indigo-400 m-4 rounded-2xl pointer-events-none transition-all">
                <Icon name="inbox" size={64} class="mb-6 text-indigo-200"/>
                <h2 class="text-4xl font-bold text-white mb-2 tracking-wider">{isEditing ? $t('drop.images_title') : $t("drop.title")}</h2>
                <p class="text-indigo-200 text-lg">{isEditing ? $t('drop.images_sub') : $t("drop.sub")}</p>
            </div>
        {/if}

        <Sidebar width={sidebarWidth}
                 groups={sidebarGroups}
                 totalGames={games.length}
                 libCount={visibleGames.length}
                 favoritesCount={favoriteGames.length}
                 updatesCount={updatesGames.length}
                 missingCount={missingGames.length}
                 {isHome} {activeFilter} {activeCollection}
                 onOpenCollection={openCollection}
                 selectedId={selectedGame ? selectedGame.id : ''}
                 {noCoverCount} {draggingGame}
                 {scanning} {checkingUpdates}
                 onFilter={applyFilter}
                 onSelect={selectGame}
                 onPlay={playGame}
                 onScan={scanOrAddFolder}
                 onAddGame={addSingleGame}
                 onCheckUpdates={handleCheckAllUpdates}
                 onCreateCollection={openCreateCollection}
                 onEditCollection={openEditCollection}
                 onDeleteCollection={deleteCollection}
                 onToggleHiddenCol={toggleHiddenCol}
                 onGameContext={gameCtx}
                 {onGameDragStart} {onDropGame} {openCtx}/>

        <!-- Отдельная колонка-разделитель: не перекрывает ни скроллбар сайдбара, ни контент -->
        <div on:mousedown={startSidebarResize} title={$t("app.resize")}
             class="w-1 shrink-0 self-stretch z-20 cursor-col-resize bg-white/5 hover:bg-indigo-400/50 transition-colors {resizingSidebar ? 'bg-indigo-400/60' : ''}"></div>

        <main bind:this={mainEl} on:scroll={onMainScroll} class="flex-1 overflow-y-auto px-8 py-6 relative {$selecting && !selectedGame ? 'pb-32' : ''}" on:wheel|nonpassive={onMainWheel}>
            {#if selectedGame}
                <div class="animate-fade-in relative z-10">
                    <button on:click={closeGame} title="Esc" class:hidden={isEditing || !!gameInfoPreview}
                            class="mb-5 h-9 px-3 rounded-lg border border-white/10 text-sm text-slate-300 hover:text-white hover:bg-white/5 transition-colors flex items-center gap-2">
                        {$t("detail.back")}
                    </button>

                    {#if isEditing}
                        <GameEditor bind:game={selectedGame} {supportedSources} dirty={editDirty} onSave={saveEdit} onCancel={cancelEdit}/>
                    {:else}
                        {#key selectedGame.id}
                        <GameDetail game={selectedGame} {supportedSources}
                                    preview={gameInfoPreview?.game || null} previewSource={gameInfoPreview?.source || ''} {savingInfo}
                                    onPreview={previewInfo} onPreviewBack={backFromPreview} onPreviewSave={saveInfoPreview}
                                    inCollections={collectionsView.filter(c => c.items.some(x => x.id === selectedGame.id)).map(c => ({ id: c.id, name: c.name, dynamic: c.type === 'dynamic' }))}
                                    onOpenCollection={openCollection}
                                    onCollectionsMenu={(e) => openCtx(e, collectionsMenuItems(selectedGame))}
                                    onEdit={startEdit}
                                    onMenu={(e) => openCtx(e, gameMenuItems(selectedGame))}
                                    onToggleFav={toggleFavorite}
                                    onRelink={relinkManually}
                                    onRelinkAll={() => showRelink = true}
                                    onApplyTag={applyTag}
                                    onOpenImage={(i) => lightboxIndex = i}
                                    onReload={loadGames}/>
                        {/key}
                    {/if}
                </div>

            {:else}
                <!-- Заголовок вида: название + число, справа сортировка -->
                <div class="flex items-center gap-3 mb-4 animate-fade-in flex-wrap">
                    <h2 class="text-2xl font-semibold text-white tracking-tight truncate min-w-0">{viewTitle}</h2>
                    {#if !showShelves}<span class="text-slate-500 text-sm">{filteredGames.length}</span>{/if}
                    {#if !isHome}
                        <button on:click={resetView}
                                class="shrink-0 flex items-center gap-1 text-xs text-slate-400 hover:text-white bg-white/5 hover:bg-white/10 px-2.5 py-1 rounded-full border border-white/10 transition-colors">
                            ✕ {$t("lib.reset")}
                        </button>
                    {/if}
                </div>

                {#if smartBanner}
                    <SmartBanner {...smartBanner}/>
                {/if}

                {#if games.length}
                    <!-- Чипсы-фильтры (внутри открытой коллекции), теги, инструменты вида -->
                    <div class="flex items-center gap-2 flex-wrap mb-6">
                        {#if !showShelves}
                            {#each chips as c (c.id)}
                                <button on:click={() => applyChip(c.id)}
                                        class="h-8 px-3.5 rounded-full text-[13px] border transition-colors {activeFilter === c.id ? 'border-indigo-500 bg-indigo-900 text-indigo-100' : 'border-white/10 text-slate-400 hover:text-white hover:border-white/20'}">
                                    {#if c.dot}<span class="inline-block w-1.5 h-1.5 rounded-full mr-1 align-middle {c.dot}"></span>{/if}{c.label} <span class="opacity-70 ml-0.5">{c.n}</span>
                                </button>
                            {/each}
                            <span class="w-px h-5 bg-white/10 mx-1"></span>
                            <TagFilter tags={scopeTags} value={tagFilter} onChange={setTags}/>
                        {/if}
                        <div class="flex-1"></div>
                        {#if !showShelves}<ViewControls bind:sortBy/>{/if}
                        <ViewOptions {isHome} onEditSections={() => layoutEditing = true}/>
                    </div>
                {/if}

                {#if showShelves}
                    {#if layoutEditing}
                        <ShelfEditor bind:shelves {allTags} collections={visibleCollections}
                                     countFor={(s) => buildShelf(s, shelfData).items.length}
                                     onDone={() => layoutEditing = false}/>
                    {:else}
                        {#if heroGame && $showHero}
                            <Hero game={heroGame} onOpen={selectGame} onPlay={playGame} onToggleFav={toggleFavorite}/>
                        {/if}
                        {#each renderedShelves as shelf (shelf.id)}
                            {#if shelf.type === 'all'}
                                {#if shelf.items.length}
                                    <div class="flex items-center justify-between mb-4" on:contextmenu={(e) => openCtx(e, shelfMenuItems(shelf))}>
                                        <h3 class="text-lg font-semibold text-white tracking-tight">{shelf.title}
                                            <span class="text-slate-500 text-sm font-medium ml-2">{shelf.items.length}</span>
                                        </h3>
                                        <ViewControls bind:sortBy/>
                                    </div>
                                    <GameGrid class="mb-9" items={shelf.items} onOpen={selectGame} onPlay={playGame}
                                              onToggleFav={toggleFavorite} onDrag={onGameDragStart} onContext={gameCtx}/>
                                {/if}
                            {:else}
                                <div on:contextmenu={(e) => openCtx(e, shelfMenuItems(shelf))}>
                                    <Shelf title={shelf.title} items={shelf.items} onOpen={selectGame} onPlay={playGame} onToggleFav={toggleFavorite}
                                           onDrag={onGameDragStart} onContext={gameCtx}/>
                                </div>
                            {/if}
                        {/each}
                    {/if}
                {:else}
                    <GameGrid items={filteredGames} onOpen={selectGame} onPlay={playGame}
                              onToggleFav={toggleFavorite} onDrag={onGameDragStart} onContext={gameCtx}/>
                    {#if filteredGames.length === 0 && games.length}
                        <div class="flex flex-col items-center justify-center py-24 text-slate-500 animate-fade-in">
                            <p class="text-xl text-slate-400 mb-1">{$t("lib.not_found_title")}</p>
                            <p class="text-sm">{$t("lib.not_found_sub")}</p>
                        </div>
                    {/if}
                {/if}

                {#if games.length === 0 && isHome}
                    <EmptyLibrary {scanPaths} {scanning} onAddFolder={addGamesFolder} onAddSingle={addSingleGame} onScan={handleScan}/>
                {/if}
            {/if}

            {#if lightboxIndex !== null && gallery.length}
                <Lightbox images={gallery} bind:index={lightboxIndex} onClose={() => lightboxIndex = null}/>
            {/if}

            {#if needsSetup}
                <SetupWizard {langs} onChangeLang={changeLang} onConfigured={onSetupConfigured} onFinish={onSetupFinish}/>
            {/if}

            {#if $updateModalOpen}
                <UpdateModal/>
            {/if}
            {#if showSettings}
                <SettingsModal bind:scanPaths bind:privacy {langs} {appVersion}
                               hiddenCount={hiddenCols.length} {showHidden}
                               onToggleHidden={toggleHidden}
                               onLockNow={lockNow}
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
                <RelinkModal onClose={() => showRelink = false} onChanged={loadGames}/>
            {/if}

            {#if pinPrompt}
                <PinDialog mode="verify" onSuccess={pinPrompt.onSuccess} onCancel={() => pinPrompt = null}/>
            {/if}
            {#if locked}
                <PinDialog mode="lock" onSuccess={onUnlocked}/>
            {/if}

            {#if $selecting && !selectedGame && !locked}
                <SelectionBar games={visibleGames} collections={visibleCollections}
                              onAddToCollection={bulkAddToCollection}
                              onRemoveFromCollection={bulkRemoveFromCollection}
                              reload={loadGames}
                              left={sidebarWidth + 4}/>
            {/if}
            {#if showHotkeys}
                <HotkeysHelp panicLabel={privacy && privacy.panic_enabled ? privacy.panic_label : ''} onClose={() => showHotkeys = false}/>
            {/if}

            {#if showToTop}
                <!-- Быстро наверх, когда библиотека прокручена далеко -->
                <button on:click={() => scrollTop(true)} title={$t('lib.to_top')} aria-label={$t('lib.to_top')}
                        class="fixed right-7 z-30 w-11 h-11 rounded-full flex items-center justify-center text-white bg-slate-800/90 hover:bg-slate-700 ring-1 ring-white/15 shadow-xl animate-fade-in transition-colors {$selecting && !selectedGame ? 'bottom-28' : 'bottom-7'}">
                    <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5m0 0l-6 6m6-6l6 6"/></svg>
                </button>
            {/if}

            <ConfirmDialog/>
            <Toast/>

            {#if ctx}{#key ctx}
                <ContextMenu x={ctx.x} y={ctx.y} items={ctx.items} width={ctx.width || 232} onClose={() => ctx = null}/>{/key}
            {/if}
        </main>
    </div>
</div>
