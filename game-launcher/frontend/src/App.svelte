<script>
    import {onMount} from 'svelte';
    // Импортируем методы Go, которые Wails сгенерировал для нас
    import {
        AddGamesFromDrop,
        AddSingleGameManual,
        CopyCoverToData,
        FindExecutables,
        GetGames,
        GetScanPath,
        Launch,
        OpenFolder,
        RemoveGame,
        ScanLocalFolder,
        SearchGames,
        SelectCoverImage,
        SelectFolder,
        SetScanPath,
        UpdateGame,
        UpdateGameMetadata,
        SelectScreenshots,
        CopyScreenshotToData,
        SelectExecutable
    } from '../wailsjs/go/main/App.js';


    let games = [];
    let scanning = false;
    let selectedGame = null;
    let parseUrl = ""; // Добавили переменную для URL
    let parsing = false; // Состояние загрузки
    let currentScanPath = "";
    let searchQuery = "";
    let isEditing = false; // Переменная-флаг для режима редактирования
    let isDragging = false; // Состояние для визуального эффекта Drag & Drop
    let activeFilter = 'all';

    $: filteredGames = games.filter(g => {
        if (activeFilter === 'no_cover') return !g.cover_path;
        if (activeFilter === 'no_desc') return !g.description;
        if (activeFilter === 'ready') return !!g.exec_path;
        return true; // 'all'
    });
    let toast = { show: false, message: "", type: "success" };

    // Применение фильтра и выход из карточки игры
    function applyFilter(filter) {
        activeFilter = filter;
        selectedGame = null; // Выходим из просмотра
        isEditing = false;   // Сбрасываем режим редактирования (на всякий случай)
        parseUrl = "";       // Очищаем инпут парсера
    }

    // --- ФУНКЦИЯ КОПИРОВАНИЯ ---
    async function copyToClipboard(text, labelName) {
        if (!text) return;
        try {
            await navigator.clipboard.writeText(text);
            showToast(`${labelName} скопировано в буфер обмена!`, "success");
        } catch (err) {
            console.error("Ошибка копирования: ", err);
            showToast("Не удалось скопировать текст", "error");
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
            showToast("Ошибка выбора файла", "error");
        }
    }

    function showToast(message, type = "success") {
        toast = { show: true, message, type };
        // Автоматически скрываем через 3 секунды
        setTimeout(() => {
            toast.show = false;
        }, 3000);
    }

    // --- ЛОГИКА РЕДАКТИРОВАНИЯ ---
    function toggleEditMode() {
        if (isEditing) {
            // Если мы УЖЕ в режиме редактирования и жмем на карандаш - спрашиваем
            if (confirm("Выйти из режима редактирования? Несохраненные изменения будут потеряны.")) {
                isEditing = false;
                // Возвращаем данные к тому состоянию, которое сейчас в базе данных
                selectedGame = games.find(g => g.id === selectedGame.id);
            }
        } else {
            isEditing = true;
        }
    }

    function cancelEdit() {
        if (confirm("Отменить изменения?")) {
            isEditing = false;
            // Сбрасываем локальные изменения
            selectedGame = games.find(g => g.id === selectedGame.id);
        }
    }

    async function handleUpdateMetadata() {
        if (!parseUrl || !selectedGame) return;
        parsing = true;
        try {
            await UpdateGameMetadata(selectedGame.id, parseUrl);
            showToast("Данные из интернета успешно загружены!", "success"); // <-- Красивое уведомление
            parseUrl = "";
            await loadGames();
            selectedGame = games.find(g => g.id === selectedGame.id);
        } catch (err) {
            showToast("Ошибка обновления: " + err, "error");
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
            alert("Ошибка: " + err);
        }
    }

    async function handleRemoveGame() {
        if (!selectedGame) return;
        // Спрашиваем подтверждение
        if (confirm(`Удалить игру "${selectedGame.title}" из лаунчера?\n(Файлы на диске удалены не будут)`)) {
            try {
                await RemoveGame(selectedGame.id);
                selectedGame = null; // Закрываем детальный вид
                await loadGames();   // Обновляем сетку
            } catch (err) {
                alert("Ошибка удаления: " + err);
            }
        }
    }

    onMount(async () => {
        // При запуске загружаем сохраненный путь
        try {
            currentScanPath = await GetScanPath();
        } catch (err) {
            console.error("Не удалось загрузить путь:", err);
        }
        await loadGames();
    });

    async function handleSelectFolder() {
        try {
            const path = await SelectFolder();
            if (path) {
                await SetScanPath(path);
                currentScanPath = path;
            }
        } catch (err) {
            console.error(err);
        }
    }

    async function loadGames() {
        try {
            games = await GetGames() || [];
        } catch (err) {
            console.error("Ошибка загрузки игр:", err);
        }
    }

    async function handleScan() {
        if (!currentScanPath) {
            alert("Сначала выберите папку для сканирования!");
            return;
        }

        scanning = true;
        try {
            // ИСПОЛЬЗУЕМ ВЫБРАННЫЙ ПУТЬ
            const count = await ScanLocalFolder(currentScanPath);
            alert(`Сканирование завершено. Добавлено/обновлено игр: ${count}`);
            await loadGames();
        } catch (err) {
            alert("Ошибка сканирования: " + err);
        } finally {
            scanning = false;
        }
    }

    function selectGame(game) {
        selectedGame = game;
        isEditing = false; // Выключаем редактор при смене игры
        parseUrl = "";
    }

    async function handleSaveChanges() {
        try {
            await UpdateGame(selectedGame);
            isEditing = false;
            await loadGames();
            showToast("Изменения успешно сохранены!", "success"); // <-- Красивое уведомление
        } catch (err) {
            showToast("Ошибка сохранения: " + err, "error");
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

    function onDragOver(e) {
        e.preventDefault();
        isDragging = true;
    }

    function onDragLeave(e) {
        e.preventDefault();
        isDragging = false;
    }

    async function onDrop(e) {
        e.preventDefault();
        isDragging = false;

        if (!e.dataTransfer.files || e.dataTransfer.files.length === 0) return;

        const paths = [];
        for (let i = 0; i < e.dataTransfer.files.length; i++) {
            const p = e.dataTransfer.files[i].path;
            if (!p) {
                alert("Ограничение безопасности Windows: браузер не передал абсолютный путь папки.\nПожалуйста, воспользуйтесь кнопкой «+ Игра».");
                return;
            }
            paths.push(p);
        }

        try {
            const added = await AddGamesFromDrop(paths);
            if (added > 0) {
                alert(`Успешно добавлено игр: ${added}`);
                await loadGames();
            } else {
                alert("Новых игр не найдено. Проверьте консоль терминала Wails.");
            }
        } catch (err) {
            alert("Ошибка при добавлении: " + err);
        }
    }

    // НОВАЯ ФУНКЦИЯ ДЛЯ РУЧНОГО ДОБАВЛЕНИЯ
    async function handleAddSingleGame() {
        try {
            await AddSingleGameManual();
            await loadGames();
        } catch (err) {
            alert(err);
        }
    }

    async function handleLaunch() {
        if (!selectedGame) return;

        if (!selectedGame.exec_path) {
            try {
                const exes = await FindExecutables(selectedGame.folder_path);
                if (exes && exes.length > 0) {
                    selectedGame.exec_path = exes[0];
                    // ДОБАВЛЯЕМ СОХРАНЕНИЕ В БАЗУ:
                    await UpdateGame(selectedGame);
                } else {
                    alert("Не удалось найти .exe файл в папке игры!");
                    return;
                }
            } catch (err) {
                console.error(err);
            }
        }

        try {
            await Launch(selectedGame.exec_path, selectedGame.folder_path);
        } catch (err) {
            alert("Ошибка запуска: " + err);
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
    function handleRemoveCover() {
        if (confirm("Удалить обложку?")) {
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
            showToast("Ошибка при добавлении обложки: " + err, "error");
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
            alert("Ошибка при добавлении скриншотов: " + err);
        }
    }

    // НОВАЯ ФУНКЦИЯ: Удаление скриншота из интерфейса
    function handleRemoveScreenshot(index) {
        if (confirm("Удалить этот скриншот?")) {
            selectedGame.images.splice(index, 1);
            selectedGame.images = [...selectedGame.images]; // Триггерим реактивность
        }
    }
</script>
<svelte:window on:keydown={handleKeydown}/>
<div
        class="flex h-screen overflow-hidden bg-slate-900 text-slate-300 font-sans relative"
        on:dragover={onDragOver}
        on:dragleave={onDragLeave}
        on:drop={onDrop}
>

    {#if isDragging}
        <div class="absolute inset-0 bg-indigo-900/80 backdrop-blur-sm z-50 flex flex-col items-center justify-center border-4 border-dashed border-indigo-400 m-4 rounded-2xl pointer-events-none transition-all">
            <div class="text-7xl mb-6">📥</div>
            <h2 class="text-4xl font-bold text-white mb-2 tracking-wider">Бросайте папки сюда</h2>
            <p class="text-indigo-200 text-lg">Лаунчер автоматически добавит их в библиотеку</p>
        </div>
    {/if}

    <aside class="w-64 bg-slate-800 p-4 flex flex-col shadow-xl z-10">
        <h1 class="text-2xl font-bold text-white mb-8 text-center tracking-wider">pLauncher 2.0</h1>

        <div class="mb-6 bg-slate-900/50 p-3 rounded border border-slate-700">
            <div class="text-xs text-slate-400 mb-1 uppercase tracking-wider font-bold">Папка для сканирования:</div>
            <div class="text-sm text-slate-300 break-words mb-2" title={currentScanPath}>
                {currentScanPath ? currentScanPath : 'Не выбрана'}
            </div>
            <button
                    on:click={handleSelectFolder}
                    class="w-full bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold py-1.5 px-3 rounded transition-colors">
                Изменить папку
            </button>
        </div>

        <button
                on:click={handleScan}
                disabled={scanning || !currentScanPath}
                class="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold py-2 px-4 rounded shadow transition-all mb-8 disabled:opacity-50">
            {scanning ? 'Сканирование...' : 'Сканировать папку'}
        </button>

        <div class="flex flex-col gap-2 mb-8">
            <h3 class="text-xs font-bold text-slate-500 uppercase tracking-wider mb-2 border-b border-slate-700 pb-2">
                Фильтры</h3>

            <button on:click={() => applyFilter('all')} class="text-left px-3 py-2 rounded transition-colors text-sm {activeFilter === 'all' ? 'bg-slate-700 text-white font-bold' : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'}">
                🌟 Все игры
            </button>
            <button on:click={() => applyFilter('no_cover')} class="text-left px-3 py-2 rounded transition-colors text-sm {activeFilter === 'no_cover' ? 'bg-slate-700 text-white font-bold' : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'}">
                🖼️ Без обложки
            </button>
            <button on:click={() => applyFilter('no_desc')} class="text-left px-3 py-2 rounded transition-colors text-sm {activeFilter === 'no_desc' ? 'bg-slate-700 text-white font-bold' : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'}">
                📝 Без описания
            </button>
            <button on:click={() => applyFilter('ready')} class="text-left px-3 py-2 rounded transition-colors text-sm {activeFilter === 'ready' ? 'bg-slate-700 text-white font-bold' : 'text-slate-400 hover:bg-slate-700/50 hover:text-slate-200'}">
                🚀 Готовы к запуску
            </button>
        </div>

        <div class="mt-auto text-sm text-slate-500 text-center">
            Всего игр: {games.length}
        </div>
    </aside>

    <main class="flex-1 overflow-y-auto p-8 relative">

        {#if selectedGame}
            <div class="animate-fade-in">
                <button on:click={() => selectedGame = null}
                        class="mb-4 text-indigo-400 hover:text-indigo-300 transition-colors">
                    &larr; Назад к библиотеке
                </button>

                <div class="flex gap-8">
                    <div class="w-1/3">
                        {#if selectedGame.cover_path}
                            <div class="aspect-[3/4] bg-slate-800 rounded-lg shadow-2xl border border-slate-700 overflow-hidden relative group">
                                <img src={`/${selectedGame.cover_path.replaceAll('\\', '/')}`} alt="cover"
                                     class="w-full h-full object-cover"/>

                                {#if isEditing}
                                    <div class="absolute inset-0 bg-black/60 flex flex-col gap-3 items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
                                        <button on:click|stopPropagation={handleSelectCover}
                                                class="bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-2 px-6 rounded shadow w-3/4 transition-colors">
                                            Сменить фото
                                        </button>

                                        <button on:click|stopPropagation={handleRemoveCover}
                                                class="bg-red-600/80 hover:bg-red-500 text-white font-bold py-2 px-6 rounded shadow w-3/4 transition-colors border border-red-500/50">
                                            Удалить
                                        </button>
                                    </div>
                                {/if}
                            </div>
                        {:else}
                            <div class="aspect-[3/4] bg-slate-800 rounded-lg shadow-2xl flex flex-col items-center justify-center text-slate-600 relative group">
                                <span class="text-6xl mb-4 opacity-30">🖼️</span>
                                <p class="font-semibold uppercase tracking-wider text-sm">Нет обложки</p>
                                {#if isEditing}
                                    <button on:click|stopPropagation={handleSelectCover}
                                            class="mt-4 border-2 border-indigo-600/50 hover:border-indigo-500 text-indigo-400 hover:text-indigo-300 font-bold py-2 px-6 rounded shadow transition-all">
                                        + Добавить
                                    </button>
                                {/if}
                            </div>
                        {/if}

                        <button
                                on:click={handleLaunch}
                                class="w-full mt-6 bg-green-600 hover:bg-green-500 text-white text-xl font-bold py-4 px-4 rounded shadow-[0_0_15px_rgba(34,197,94,0.4)] transition-all">
                            ИГРАТЬ
                        </button>

                        <div class="flex gap-2 mt-3">
                            <button
                                    on:click={handleOpenFolder}
                                    class="flex-1 bg-slate-700 hover:bg-slate-600 text-slate-200 font-semibold py-2 px-4 rounded transition-colors text-sm">
                                📁 Открыть папку
                            </button>

                            <button
                                    on:click={toggleEditMode}
                                    class="{isEditing ? 'bg-indigo-600 text-white shadow-inner' : 'bg-slate-700 hover:bg-slate-600 text-slate-200'} font-semibold py-2 px-4 rounded transition-colors text-sm"
                                    title="Редактировать">
                                ✏️
                            </button>

                            <button
                                    on:click={handleRemoveGame}
                                    class="bg-red-900/50 hover:bg-red-600 text-red-200 hover:text-white font-semibold py-2 px-4 rounded transition-colors text-sm"
                                    title="Удалить из лаунчера">
                                🗑️
                            </button>
                        </div>

                        {#if isEditing}
                            <div class="flex flex-col gap-2 mt-4 animate-fade-in border-t border-slate-700 pt-4">
                                <button on:click={handleSaveChanges} class="w-full bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-3 px-4 rounded shadow-[0_0_10px_rgba(16,185,129,0.3)] transition-colors flex justify-center items-center gap-2">
                                    <span>💾</span> Сохранить изменения
                                </button>
                                <button on:click={cancelEdit} class="w-full bg-slate-700 hover:bg-slate-600 text-slate-300 font-bold py-2 px-4 rounded transition-colors">
                                    Отмена
                                </button>
                            </div>
                        {/if}
                    </div>

                    <div class="w-2/3 relative">
                        {#if isEditing}
                            <input type="text" bind:value={selectedGame.title}
                                   class="w-full text-4xl font-bold bg-slate-800 text-white border border-slate-600 rounded px-3 py-2 mb-2 focus:border-indigo-500 focus:outline-none"
                                   placeholder="Название игры"/>

                            <div class="flex flex-col gap-2 mb-6 border-b border-slate-700 pb-4">
                                <div class="flex items-center gap-2">
                                    <span class="text-indigo-400">Версия:</span>
                                    <input type="text" bind:value={selectedGame.version}
                                           class="bg-slate-800 text-white border border-slate-600 rounded px-2 py-1 text-sm focus:border-indigo-500 focus:outline-none w-24"
                                           placeholder="1.0"/>
                                </div>
                                <div class="flex items-center gap-2 mt-2">
                                    <span class="text-indigo-400 whitespace-nowrap">.exe файл:</span>
                                    <input type="text" bind:value={selectedGame.exec_path} class="flex-1 bg-slate-800 text-white border border-slate-600 rounded px-2 py-1 text-sm focus:border-indigo-500 focus:outline-none" placeholder="C:\Games\Game\run.exe" />
                                    <button
                                            on:click={handleSelectExecutable}
                                            class="bg-slate-700 hover:bg-slate-600 text-slate-200 px-3 py-1 rounded border border-slate-600 transition-colors"
                                            title="Выбрать файл">
                                        📁
                                    </button>
                                </div>
                            </div>

                            <textarea bind:value={selectedGame.description} rows="6"
                                      class="w-full bg-slate-800 text-white border border-slate-600 rounded px-3 py-2 mb-4 text-lg leading-relaxed focus:border-indigo-500 focus:outline-none resize-y"
                                      placeholder="Описание игры..."></textarea>

                        {:else}
                            <h2
                                    class="text-4xl font-bold text-white mb-2 cursor-pointer hover:text-indigo-300 transition-colors group relative inline-block"
                                    on:click={() => copyToClipboard(selectedGame.title, 'Название')}
                                    title="Нажмите, чтобы скопировать"
                            >
                                {selectedGame.title}
                                <span class="opacity-0 group-hover:opacity-100 text-xl font-normal text-indigo-400 absolute -right-8 top-1 transition-opacity">📋</span>
                            </h2>

                            <p
                                    class="text-indigo-400 mb-6 border-b border-slate-700 pb-4 cursor-pointer hover:text-indigo-300 transition-colors w-max group relative"
                                    on:click={() => copyToClipboard(selectedGame.version, 'Версия')}
                                    title="Нажмите, чтобы скопировать"
                            >
                                Версия: {selectedGame.version || 'Неизвестно'}
                                <span class="opacity-0 group-hover:opacity-100 text-sm ml-2 transition-opacity">📋</span>
                            </p>

                            <div class="relative group mb-8">
                                <p
                                        class="text-lg leading-relaxed whitespace-pre-wrap cursor-pointer hover:bg-slate-800/80 p-3 -mx-3 rounded-lg transition-colors border border-transparent hover:border-slate-600"
                                        on:click={() => copyToClipboard(selectedGame.description, 'Описание')}
                                        title="Нажмите, чтобы скопировать"
                                >
                                    {selectedGame.description || 'Описание отсутствует.'}
                                </p>
                                <span class="opacity-0 group-hover:opacity-100 absolute top-3 right-0 text-slate-400 pointer-events-none bg-slate-900/80 px-2 py-1 rounded text-sm transition-opacity">
                            📋 Копировать
                          </span>
                            </div>
                        {/if}

                        {#if (selectedGame.images && selectedGame.images.length > 0) || isEditing}
                            <div class="mb-8">
                                <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">Скриншоты</h3>
                                <div class="flex gap-4 overflow-x-auto pb-4 custom-scrollbar">

                                    {#each selectedGame.images || [] as img, i}
                                        <div class="relative flex-shrink-0 w-48 aspect-video bg-slate-900 rounded border border-slate-700 overflow-hidden group hover:border-indigo-500 hover:shadow-[0_0_15px_rgba(99,102,241,0.3)] transition-all">
                                            <img
                                                    src={`/${img.replaceAll('\\', '/')}`}
                                                    alt="screenshot"
                                                    class="w-full h-full object-cover cursor-pointer"
                                                    on:click={() => { if (!isEditing) openLightbox(i) }}
                                            />

                                            {#if isEditing}
                                                <button
                                                        on:click|stopPropagation={() => handleRemoveScreenshot(i)}
                                                        class="absolute top-1 right-1 bg-red-600 hover:bg-red-500 text-white rounded-full w-7 h-7 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity text-sm font-bold shadow-lg"
                                                        title="Удалить скриншот"
                                                >
                                                    ✕
                                                </button>
                                            {/if}
                                        </div>
                                    {/each}

                                    {#if isEditing}
                                        <div
                                                class="flex-shrink-0 w-48 aspect-video bg-slate-800 rounded border-2 border-dashed border-slate-600 hover:border-indigo-500 flex flex-col items-center justify-center cursor-pointer transition-colors text-slate-400 hover:text-indigo-400"
                                                on:click={handleAddScreenshots}
                                        >
                                            <span class="text-4xl mb-1 font-light">+</span>
                                            <span class="text-xs font-bold uppercase tracking-wider">Добавить</span>
                                        </div>
                                    {/if}

                                </div>
                            </div>
                        {/if}

                        <div class="bg-slate-800 p-5 rounded-lg border border-slate-700 shadow-inner">
                            <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">Обновить
                                метаданные из интернета</h3>
                            <div class="flex gap-3">
                                <input
                                        type="text"
                                        bind:value={parseUrl}
                                        placeholder="Вставьте ссылку..."
                                        class="flex-1 bg-slate-900 border border-slate-600 text-white rounded px-4 py-2 focus:outline-none focus:border-indigo-500 transition-colors"
                                />
                                <button
                                        on:click={handleUpdateMetadata}
                                        disabled={parsing || !parseUrl}
                                        class="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold py-2 px-6 rounded shadow transition-all disabled:opacity-50">
                                    {parsing ? 'Скачивание...' : 'Обновить'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

        {:else}
            <div class="flex justify-between items-center mb-6 animate-fade-in">
                <div class="flex items-center gap-4">
                    <h2 class="text-2xl font-bold text-white tracking-wide">Моя библиотека</h2>
                    <button
                            on:click={handleAddSingleGame}
                            class="bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-1 px-3 rounded shadow transition-colors text-sm">
                        + Игра
                    </button>
                </div>

                <div class="relative w-72">
                    <input
                            type="text"
                            bind:value={searchQuery}
                            on:input={handleSearch}
                            placeholder="Поиск по названию..."
                            class="w-full bg-slate-800 border border-slate-600 text-white rounded-full px-4 py-2 pr-10 focus:outline-none focus:border-indigo-500 transition-colors shadow-inner"
                    />
                    <div class="absolute right-3 top-2.5 text-slate-400 pointer-events-none">
                        🔍
                    </div>
                </div>
            </div>

            <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-6">
                {#each filteredGames as game}
                    <div
                            class="group cursor-pointer bg-slate-800 rounded-lg overflow-hidden border border-slate-700 hover:border-indigo-500 transition-all hover:-translate-y-1 hover:shadow-[0_10px_20px_rgba(99,102,241,0.2)]"
                            on:click={() => selectGame(game)}>

                        <div class="aspect-[3/4] bg-slate-900 relative">
                            {#if game.cover_path}
                                <img src={`/${game.cover_path.replaceAll('\\', '/')}`} alt={game.title}
                                     class="w-full h-full object-cover opacity-80 group-hover:opacity-100 transition-opacity"/>
                            {:else}
                                <div class="w-full h-full flex items-center justify-center text-slate-600 text-sm p-4 text-center">
                                    {game.title}
                                </div>
                            {/if}
                        </div>

                        <div class="p-3 bg-slate-800">
                            <h3 class="font-bold text-white truncate">{game.title}</h3>
                            <p class="text-xs text-slate-400 truncate">{game.version || 'v?'}</p>
                        </div>
                    </div>
                {/each}

                {#if games.length === 0 && !scanning}
                    <div class="col-span-full flex flex-col items-center justify-center py-20 text-slate-500">
                        <p class="text-xl mb-4">Библиотека пуста</p>
                        <p>Нажмите «Сканировать папку», чтобы найти игры</p>
                    </div>
                {/if}
            </div>
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

                <img src={`/${lightboxImage.replaceAll('\\', '/')}`} alt="fullscreen"
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

        {#if toast.show}
            <div class="fixed bottom-8 right-8 p-4 rounded-lg shadow-2xl flex items-center gap-3 z-50 animate-fade-in transition-all {toast.type === 'error' ? 'bg-red-900 border border-red-500 text-red-100' : 'bg-slate-800 border-l-4 border-emerald-500 text-slate-200'}">
                {#if toast.type === 'success'}
                    <span class="text-emerald-400 text-xl">✓</span>
                {:else}
                    <span class="text-red-400 text-xl">⚠</span>
                {/if}
                <span class="font-semibold tracking-wide">{toast.message}</span>
            </div>
        {/if}

    </main>
</div>