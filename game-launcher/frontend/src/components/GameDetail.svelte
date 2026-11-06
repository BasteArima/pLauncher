<script>
    // Страница игры (режим просмотра) по макету: две независимые колонки, без пустот между блоками.
    // Слева — картинки (крупно + лента, стрелки ‹ ›), «Об игре» с тегами, заметки, блок ссылок.
    // Справа — карточка: «Играть», ⋯, ♡, время/последний запуск/размер, статус, оценка, обновление,
    // площадки; ниже коллекции и подробности (версия, движок, языки, папка, файл запуска).
    import { t, tr, langStore } from '../i18n.js';
    import { showToast } from '../lib/ui.js';
    import { mediaSrc, fmtPlaytime, fmtSize, copyText, lsJSON, lsSet } from '../lib/util.js';
    import { Launch, OpenFolder, OpenURL, SelectExecutable, UpdateGame, UpdateGameMetadata, RecalcGameSize } from '../../wailsjs/go/main/App.js';
    import Icon from './Icon.svelte';
    import MetadataPanel from './MetadataPanel.svelte';
    import { coverBlur, blurCls } from '../lib/privacy.js';
    import { STATUSES } from '../lib/status.js';

    export let game;
    export let preview = null;
    export let previewSource = '';
    export let savingInfo = false;
    export let onPreview = async (hit, version) => {};
    export let onPreviewBack = () => {};
    export let onPreviewSave = async () => {};
    $: displayGame = preview || game;
    const dismissedIDs = lsJSON('plauncher_noinfo_dismissed', []);
    let dismissed = Array.isArray(dismissedIDs) && dismissedIDs.includes(game.id);
    function dismissInfo() {
        const ids = lsJSON('plauncher_noinfo_dismissed', []);
        lsSet('plauncher_noinfo_dismissed', JSON.stringify([...new Set([...(Array.isArray(ids) ? ids : []), game.id])]));
        dismissed = true;
    }
    export let supportedSources = [];
    export let inCollections = [];                       // [{id, name, dynamic}] — коллекции, где есть игра
    export let onOpenCollection = (id) => {};
    export let onCollectionsMenu = (e) => {};            // «+»: меню с галочками ручных коллекций
    export let onEdit = () => {};
    export let onToggleFav = (g) => {};
    export let onMenu = (e) => {};                       // ⋯: то же меню, что по правому щелчку
    export let onRelink = (g) => {};                     // указать новую папку вручную
    export let onRelinkAll = () => {};                   // окно перепривязки
    export let onApplyTag = (tag) => {};
    export let onOpenImage = (index) => {};
    export let onReload = async () => {};

    let parsing = false;
    const copy = (text, label) => copyText(text, label, showToast);

    // --- Медиа: обложка + скриншоты в одной ленте; выбранное показывается крупно ---
    $: media = [
        ...(displayGame.cover_path ? [{ src: displayGame.cover_path, cover: true }] : []),
        ...(displayGame.images || []).map((src, i) => ({ src, shot: i })),
    ];
    let mediaIndex = 0;
    let shownId = '';
    let shownPreview = null;
    $: if (preview !== shownPreview) { shownPreview = preview; mediaIndex = 0; editingNotes = false; }
    $: if (game.id !== shownId) { shownId = game.id; mediaIndex = 0; editingNotes = false; revealed = false; }
    $: if (mediaIndex >= media.length) mediaIndex = 0;
    $: current = media[mediaIndex];
    // Листать картинки прямо на странице, как в Steam (по кругу)
    function stepMedia(dir) { if (media.length > 1) mediaIndex = (mediaIndex + dir + media.length) % media.length; }
    // Крупная картинка — в просмотр (индекс в общей ленте: обложка + скриншоты)
    function openCurrent() { if (current) onOpenImage(mediaIndex); }

    let sizing = false;
    async function recalcSize() {
        if (sizing || game.folder_missing) return;
        sizing = true;
        try { game.size_bytes = await RecalcGameSize(game.id); }
        catch (err) { showToast(tr('toast.error', { err }), 'error'); }
        finally { sizing = false; }
    }

    async function openFolder() {
        try { await OpenFolder(game.folder_path); }
        catch (err) { showToast(tr('toast.open_folder_fail', { err }), 'error'); }
    }

    // Главная кнопка: указать папку (если пропала) → выбрать exe (если нет) → запуск
    async function smartPlay() {
        if (game.folder_missing) { await onRelink(game); return; }
        if (!game.exec_path) { await pickExe(); return; }
        try {
            await Launch(game.id, game.exec_path, game.folder_path);
            await onReload(); // «последний запуск» → порядок в библиотеке
        } catch (err) { showToast(tr('toast.launch_fail', { err }), 'error'); }
    }
    async function pickExe() {
        try {
            const path = await SelectExecutable(game.folder_path);
            if (!path) return;
            game.exec_path = path;
            await save();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

    // Сохранить личные поля (статус, оценка, заметки, файл запуска)
    async function save() {
        if (preview) return;
        try { await UpdateGame(game); await onReload(); }
        catch (err) { showToast(tr('toast.save_fail', { err }), 'error'); }
    }
    // Повторный щелчок по выбранному статусу/звезде — снять
    function setStatus(id) { game.status = game.status === id ? '' : id; save(); }
    function setRating(n) { game.rating = game.rating === n ? 0 : n; save(); }

    // --- Заметки: ||текст|| — спойлер, скрыт, пока не нажать «Показать спойлеры» ---
    let editingNotes = false;
    let notesDraft = '';
    let revealed = false;
    $: noteParts = (game.notes || '').split(/\|\|([\s\S]+?)\|\|/).map((text, i) => ({ text, spoiler: i % 2 === 1 }));
    $: hasSpoilers = noteParts.some(p => p.spoiler);
    function editNotes() { notesDraft = game.notes || ''; editingNotes = true; }
    async function saveNotes() { game.notes = notesDraft.trim(); editingNotes = false; await save(); }

    // Загрузка метаданных по ссылке. Возвращает true при успехе.
    async function parse(url) {
        if (!url) return false;
        parsing = true;
        try {
            await UpdateGameMetadata(game.id, url);
            showToast(tr('toast.meta_loaded'), 'success');
            await onReload();
            return true;
        } catch (err) {
            showToast(tr('toast.meta_fail', { err }), 'error');
            return false;
        } finally {
            parsing = false;
        }
    }

    // Обновление: где вышла новая версия, открыть её страницу; «Я обновил игру» = подтянуть
    // свежие данные с этой площадки (версия, описание, картинки) и снять отметку обновления.
    $: updateSrc = (game.sources || []).find(s => s.source === game.update_source && s.url);
    function openUpdatePage() { if (updateSrc) OpenURL(updateSrc.url); }
    function markUpdated() { if (updateSrc) parse(updateSrc.url); }

    $: links = (displayGame.sources || []).filter(s => s.url);
    $: lastPlayed = ($langStore, game.last_launched_at ? fmtDay(game.last_launched_at * 1000, $langStore) : tr('detail.never'));
    $: addedOn = ($langStore, game.added_at ? fmtDay(game.added_at * 1000, $langStore) : '');
    // Дата без года, если это текущий год: «5 нояб.» вместо «5 нояб. 2026 г.»
    function fmtDay(ms, lang) {
        const d = new Date(ms);
        const opts = d.getFullYear() === new Date().getFullYear() ? { day: 'numeric', month: 'short' } : { day: 'numeric', month: 'short', year: 'numeric' };
        try { return d.toLocaleDateString(lang, opts); } catch (e) { return d.toLocaleDateString(undefined, opts); }
    }
    // Игра только с папкой (ещё ни разу не парсилась): блок «Найти информацию» сразу под картинками
    $: noInfo = !game.cover_path && !game.description && !(game.sources || []).some(s => s.url);
    $: launchName = (game.exec_path || '').split(/[\\/]/).pop();

    const card = 'rounded-2xl bg-slate-900/60 border border-white/5';
    const sectionTitle = 'text-xs font-semibold text-slate-500 uppercase tracking-wider mb-3';
    const iconBtn = 'h-12 w-12 shrink-0 rounded-xl flex items-center justify-center border border-white/10 bg-slate-900 text-slate-300 hover:bg-slate-800 hover:text-white transition-colors';
    const detailLabel = 'text-slate-500';
</script>

{#if !preview && displayGame.folder_missing}
    <div class="mb-6 rounded-xl bg-rose-950/50 border border-rose-800/50 p-4 flex items-center gap-4 flex-wrap animate-fade-in">
        <div class="flex-1 min-w-[240px]">
            <div class="font-semibold text-rose-100">{$t('missing.banner_title')}</div>
            <div class="text-xs text-rose-100/70 font-mono truncate" title={displayGame.folder_path}>{displayGame.folder_path}</div>
        </div>
        <button on:click={() => onRelink(game)}
                class="shrink-0 h-10 px-4 rounded-lg text-sm font-semibold bg-rose-300 hover:bg-rose-200 text-rose-950 transition-colors">{$t('missing.pick')}</button>
        <button on:click={onRelinkAll}
                class="shrink-0 h-10 px-4 rounded-lg text-sm text-rose-100 border border-rose-700/60 hover:bg-white/5 transition-colors">{$t('missing.auto')}</button>
    </div>
{/if}

{#if preview}
    <div data-info-preview class="sticky -top-6 z-20 -mx-8 -mt-6 mb-6 px-8 py-3.5 flex items-center gap-3 flex-wrap bg-slate-900 border-b border-indigo-900">
        <button data-preview-back on:click={onPreviewBack} disabled={savingInfo} class="flex items-center gap-2 h-9 px-3 rounded-lg border border-white/10 text-sm text-slate-300 hover:text-white disabled:opacity-50"><Icon name="back" size={16}/>{$t('find.other')}</button>
        <span class="flex-1 min-w-[180px] text-sm text-slate-300">{$t('find.preview_banner', { site: previewSource })}</span>
        <button data-preview-save on:click={onPreviewSave} disabled={savingInfo} class="flex items-center gap-2 h-9 px-4 rounded-lg text-sm font-semibold text-white bg-indigo-500 hover:bg-indigo-400 disabled:opacity-50">
            <Icon name={savingInfo ? 'refresh' : 'check'} size={16} class={savingInfo ? 'animate-spin' : ''}/>{savingInfo ? $t('find.saving') : $t('find.save')}
        </button>
    </div>
{/if}
<div class="flex flex-wrap gap-7 items-start" data-game-detail data-preview={!!preview}>
    <!-- ЛЕВАЯ КОЛОНКА -->
    <div class="flex-[999_1_520px] min-w-0 flex flex-col gap-8">
        <section>
            <div class="relative w-full aspect-video rounded-2xl overflow-hidden bg-slate-900 border border-white/5 group {current ? 'cursor-zoom-in' : ''}"
                 on:click={openCurrent}>
                {#if current}
                    {#key current.src}
                        <!-- Размытая подложка, чтобы вертикальная обложка не висела на пустом фоне -->
                        <img src={mediaSrc(current.src)} alt="" aria-hidden="true"
                             class="absolute inset-0 w-full h-full object-cover scale-110 blur-2xl opacity-30"/>
                        <img src={mediaSrc(current.src)} alt={current.cover ? displayGame.title : $t('detail.screenshots')}
                             on:error={(e) => e.target.style.display = 'none'}
                             class="absolute inset-0 w-full h-full object-contain animate-fade-in transition duration-300 {blurCls($coverBlur, 'cover')} {displayGame.folder_missing ? 'grayscale opacity-60' : ''}"/>
                    {/key}
                    {#if media.length > 1}
                        <button on:click|stopPropagation={() => stepMedia(-1)} aria-label={$t('detail.prev_image')} title={$t('detail.prev_image')}
                                class="absolute left-3 top-1/2 -translate-y-1/2 w-11 h-11 rounded-full flex items-center justify-center text-white bg-black/35 hover:bg-black/55 group-hover:backdrop-blur-md ring-1 ring-inset ring-white/15 opacity-0 group-hover:opacity-100 transition">
                            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 5l-7 7 7 7"/></svg>
                        </button>
                        <button on:click|stopPropagation={() => stepMedia(1)} aria-label={$t('detail.next_image')} title={$t('detail.next_image')}
                                class="absolute right-3 top-1/2 -translate-y-1/2 w-11 h-11 rounded-full flex items-center justify-center text-white bg-black/35 hover:bg-black/55 group-hover:backdrop-blur-md ring-1 ring-inset ring-white/15 opacity-0 group-hover:opacity-100 transition">
                            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 5l7 7-7 7"/></svg>
                        </button>
                        <span class="absolute bottom-3 right-3 px-2 py-0.5 rounded-md text-[11px] text-white/90 bg-black/45 opacity-0 group-hover:opacity-100 transition">{mediaIndex + 1} / {media.length}</span>
                    {/if}
                {:else}
                    <div class="absolute inset-0 flex flex-col items-center justify-center gap-3 text-slate-500 p-6 text-center">
                        <svg class="w-12 h-12 text-slate-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                        <div class="text-slate-300 font-medium">{$t('detail.no_media')}</div>
                        {#if !preview}<div class="text-sm max-w-md">{$t('find.hint')}</div>{/if}
                    </div>
                {/if}
            </div>
            {#if media.length > 1}
                <div class="flex gap-2 mt-3 overflow-x-auto pb-1">
                    {#each media as m, i}
                        <button on:click={() => mediaIndex = i} title={m.cover ? $t('detail.cover') : $t('detail.screenshots')}
                                class="group relative shrink-0 w-28 aspect-video rounded-lg overflow-hidden bg-slate-900 transition {i === mediaIndex ? 'ring-2 ring-indigo-400' : 'ring-1 ring-white/10 opacity-70 hover:opacity-100'}">
                            <img src={mediaSrc(m.src)} alt="" loading="lazy" on:error={(e) => e.target.style.display = 'none'}
                                 class="w-full h-full object-cover {blurCls($coverBlur, 'icon')}"/>
                        </button>
                    {/each}
                </div>
            {/if}
        </section>

        <!-- Скрываем, не размонтируем: после превью остаются запрос, источники и результаты. -->
        <div class:hidden={!!preview} style="order: {noInfo && !dismissed ? 1 : 4}" data-metadata-panel>
            <MetadataPanel {game} {supportedSources} {parsing} onParse={parse} {onReload} {onPreview} onDismiss={dismissInfo} prominent={noInfo && !dismissed}/>
        </div>

        <section class="order-2">
            <h3 class={sectionTitle}>{$t('detail.about')}</h3>
            {#if displayGame.description}
                <p class="text-[15px] text-slate-300 leading-relaxed whitespace-pre-wrap cursor-pointer hover:bg-white/5 p-3 -m-3 rounded-xl transition-colors max-w-3xl"
                   on:click={() => copy(displayGame.description, $t('label.description'))}>{displayGame.description}</p>
            {:else}
                <p class="text-sm text-slate-500">{$t('detail.no_description')}</p>
            {/if}
            {#if displayGame.tags && displayGame.tags.length}
                <div class="flex flex-wrap gap-2 mt-4">
                    {#each displayGame.tags as tag}
                        <button disabled={!!preview} on:click={() => onApplyTag(tag)}
                                class="px-3 py-1 rounded-lg text-xs text-slate-300 bg-slate-900 border border-white/10 hover:border-indigo-400/50 hover:text-white transition-colors">{tag}</button>
                    {/each}
                </div>
            {/if}
        </section>

        <section class="order-3" class:hidden={!!preview}>
            <div class="flex items-center gap-3 mb-3">
                <h3 class="{sectionTitle} !mb-0 flex-1">{$t('detail.notes')}</h3>
                {#if !editingNotes && hasSpoilers}
                    <button on:click={() => revealed = !revealed} class="h-7 px-2.5 rounded-md text-xs text-slate-400 border border-white/10 hover:text-white">
                        {revealed ? $t('detail.hide_spoilers') : $t('detail.show_spoilers')}
                    </button>
                {/if}
                {#if !editingNotes}
                    <button on:click={editNotes} class="h-7 px-2.5 rounded-md text-xs text-slate-400 border border-white/10 hover:text-white">{$t('detail.edit_notes')}</button>
                {/if}
            </div>
            {#if editingNotes}
                <textarea bind:value={notesDraft} rows="5" placeholder={$t('detail.notes_ph')}
                          class="w-full bg-slate-950 border border-white/10 rounded-xl p-3 text-sm text-slate-200 placeholder-slate-600 focus:outline-none focus:border-indigo-400/50 select-text"></textarea>
                <div class="flex items-center gap-2 mt-2">
                    <button on:click={saveNotes} class="h-8 px-4 rounded-lg text-xs font-semibold bg-indigo-500 hover:bg-indigo-400 text-white">{$t('btn.save')}</button>
                    <button on:click={() => editingNotes = false} class="h-8 px-3 rounded-lg text-xs text-slate-400 hover:text-white">{$t('btn.cancel')}</button>
                    <span class="text-xs text-slate-500 ml-2">{$t('detail.notes_hint')}</span>
                </div>
            {:else if displayGame.notes}
                <div class="rounded-xl bg-slate-900/60 border border-white/5 p-4 text-sm text-slate-300 leading-relaxed whitespace-pre-wrap select-text">{#each noteParts as part}{#if part.spoiler}<span on:click={() => revealed = true} class="rounded px-0.5 transition {revealed ? 'bg-white/5' : 'bg-slate-700 text-transparent cursor-pointer select-none'}" title={revealed ? '' : $t('detail.show_spoilers')}>{part.text}</span>{:else}{part.text}{/if}{/each}</div>
            {:else}
                <button on:click={editNotes} class="text-sm text-slate-500 hover:text-slate-300">{$t('detail.notes_empty')}</button>
            {/if}
        </section>


    </div>

    <!-- ПРАВАЯ КОЛОНКА -->
    <aside class="flex-[1_1_340px] max-w-[420px] flex flex-col gap-5">
        <section class="{card} p-6 flex flex-col gap-5">
            <div>
                <h1 class="text-3xl font-bold text-white leading-tight break-words cursor-pointer hover:text-indigo-200 transition-colors"
                    on:click={() => copy(displayGame.title, $t('label.title'))} title={$t('detail.copy_title')}>{displayGame.title}</h1>
                {#if displayGame.author || displayGame.engine}
                    <div class="text-sm text-slate-400 mt-1.5">
                        {#if displayGame.author}<span class="cursor-pointer hover:text-white" on:click={() => copy(displayGame.author, $t('detail.author'))}>{displayGame.author}</span>{/if}{#if displayGame.author && displayGame.engine}{' · '}{/if}{#if displayGame.engine}{displayGame.engine}{/if}
                    </div>
                {/if}
            </div>

            {#if !preview}
            <div class="flex gap-2">
                <button on:click={smartPlay}
                        class="flex-1 h-12 rounded-xl flex items-center justify-center gap-2.5 text-base font-bold transition-colors {displayGame.folder_missing ? 'bg-rose-300 hover:bg-rose-200 text-rose-950' : displayGame.exec_path ? 'bg-indigo-500 hover:bg-indigo-400 text-white' : 'bg-slate-100 hover:bg-white text-slate-950'}">
                    {#if displayGame.exec_path && !displayGame.folder_missing}<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M7 4l13 8-13 8z"/></svg>{/if}
                    {displayGame.folder_missing ? $t('missing.pick') : displayGame.exec_path ? $t('btn.play') : $t('btn.set_exe')}
                </button>
                <button on:click|stopPropagation={onMenu} class={iconBtn} title={$t('card.more')} aria-label={$t('card.more')}>
                    <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.8"/><circle cx="12" cy="12" r="1.8"/><circle cx="19" cy="12" r="1.8"/></svg>
                </button>
                <button on:click={() => onToggleFav(game)} class="{iconBtn} {displayGame.favorite ? '!text-pink-400' : ''}"
                        title={displayGame.favorite ? $t('detail.fav_in') : $t('detail.fav_add')} aria-label={displayGame.favorite ? $t('detail.fav_in') : $t('detail.fav_add')}>
                    <svg class="w-5 h-5" viewBox="0 0 24 24" fill={displayGame.favorite ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><path d="M12 20.3l-1.3-1.2C5.6 14.5 2.5 11.7 2.5 8.3 2.5 5.6 4.6 3.5 7.3 3.5c1.5 0 3 .7 4 1.9 1-1.2 2.5-1.9 4-1.9 2.7 0 4.8 2.1 4.8 4.8 0 3.4-3.1 6.2-8.2 10.8z"/></svg>
                </button>
            </div>

            <div class="grid grid-cols-3 gap-2">
                <div class="rounded-lg bg-slate-950/60 px-3 py-2.5">
                    <div class="text-[11px] text-slate-500">{$t('detail.played')}</div>
                    <div class="text-sm font-semibold text-slate-100 mt-0.5 leading-snug">{fmtPlaytime(displayGame.time_played) || '—'}</div>
                </div>
                <div class="rounded-lg bg-slate-950/60 px-3 py-2.5">
                    <div class="text-[11px] text-slate-500">{$t('detail.last_played')}</div>
                    <div class="text-sm font-semibold text-slate-100 mt-0.5 leading-snug">{lastPlayed}</div>
                </div>
                <button on:click={recalcSize} title={$t('detail.size_hint')}
                        class="rounded-lg bg-slate-950/60 px-3 py-2.5 text-left hover:bg-slate-800 transition-colors">
                    <div class="text-[11px] text-slate-500">{$t('detail.size')}</div>
                    <div class="text-sm font-semibold text-slate-100 mt-0.5 leading-snug">{sizing ? '…' : (fmtSize(displayGame.size_bytes) || '—')}</div>
                </button>
            </div>

            <div>
                <div class="text-xs text-slate-500 mb-2">{$t('status.label')}</div>
                <div class="grid grid-cols-4 gap-1 p-1 rounded-xl bg-slate-950" role="group" aria-label={$t('status.label')}>
                    {#each STATUSES as s}
                        <button on:click={() => setStatus(s.id)} aria-pressed={displayGame.status === s.id} title={$t('status.' + s.id)}
                                class="h-9 rounded-lg text-xs truncate px-1 transition-colors {displayGame.status === s.id ? 'bg-slate-800 font-semibold ' + s.text : 'text-slate-500 hover:text-slate-200'}">{$t('status.short_' + s.id)}</button>
                    {/each}
                </div>
            </div>

            <div class="flex items-center gap-3">
                <span class="text-xs text-slate-500 w-16">{$t('detail.rating')}</span>
                <div class="flex" role="group" aria-label={$t('detail.rating')}>
                    {#each [1, 2, 3, 4, 5] as n}
                        <button on:click={() => setRating(n)} aria-label={$t('detail.rate_n', { n })} title={$t('detail.rate_n', { n })}
                                class="w-7 h-7 flex items-center justify-center transition-colors {n <= (displayGame.rating || 0) ? 'text-amber-300' : 'text-slate-700 hover:text-slate-500'}">
                            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><path d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z"/></svg>
                        </button>
                    {/each}
                </div>
            </div>

            {#if displayGame.update_available}
                <div class="rounded-xl bg-amber-950/60 border border-amber-700/50 p-3.5 flex flex-col gap-2">
                    <div class="flex items-baseline gap-2 flex-wrap">
                        <span class="text-sm text-amber-200 font-semibold">{$t('detail.update_out', { v: displayGame.update_version })}</span>
                        {#if displayGame.version}<span class="text-xs text-amber-100/60">{$t('detail.update_have', { v: displayGame.version })}</span>{/if}
                    </div>
                    {#if displayGame.update_source}<div class="text-xs text-amber-100/60">{$t('detail.update_found', { site: displayGame.update_source })}</div>{/if}
                    <div class="flex gap-2 flex-wrap mt-1">
                        {#if updateSrc}
                            <button on:click={openUpdatePage} class="h-8 px-3 rounded-md text-xs font-bold bg-amber-400 hover:bg-amber-300 text-amber-950 transition-colors">{$t('detail.update_open')} ↗</button>
                            <button on:click={markUpdated} disabled={parsing} title={$t('detail.update_done_hint')}
                                    class="h-8 px-3 rounded-md text-xs text-amber-200 border border-amber-700/60 hover:bg-white/5 transition-colors disabled:opacity-60">
                                {parsing ? $t('meta.downloading') : $t('detail.update_done')}
                            </button>
                        {/if}
                    </div>
                </div>
            {/if}

            {/if}

            <div>
                <div class="text-xs text-slate-500 mb-2">{$t('detail.sources')}</div>
                <div class="flex flex-wrap gap-1.5">
                    {#each links as s}
                        <button on:click={() => OpenURL(s.url)} title={s.url}
                                class="px-2.5 py-1 rounded-md text-xs border transition-colors {s.source === displayGame.primary_source ? 'border-indigo-500/60 text-indigo-200' : 'border-white/10 text-slate-300'} hover:text-white hover:border-white/30">
                            {#if s.source === displayGame.primary_source}★ {/if}{s.source} ↗
                        </button>
                    {/each}
                    {#if !preview}<button on:click={onEdit} class="px-2.5 py-1 rounded-md text-xs border border-dashed border-white/15 text-slate-500 hover:text-slate-200">+ {$t('detail.add_link')}</button>{/if}
                </div>
            </div>
        </section>

        <section class="{card} p-5">
            <h3 class={sectionTitle}>{$t('detail.details')}</h3>
            <div class="grid grid-cols-[104px_minmax(0,1fr)] gap-x-3 gap-y-2.5 text-sm">
                <span class={detailLabel}>{$t('detail.version')}</span>
                <span class="text-slate-100 cursor-pointer hover:text-indigo-200 break-words" on:click={() => copy(displayGame.version, $t('label.version'))}>{displayGame.version || $t('detail.unknown')}</span>
                {#if displayGame.engine}
                    <span class={detailLabel}>{$t('detail.engine')}</span><span class="text-slate-100">{displayGame.engine}</span>
                {/if}
                {#if displayGame.languages && displayGame.languages.length}
                    <span class={detailLabel}>{$t('detail.languages')}</span><span class="text-slate-100">{displayGame.languages.join(', ')}</span>
                {/if}
                {#if !preview}
                {#if addedOn}
                    <span class={detailLabel}>{$t('detail.added')}</span><span class="text-slate-100">{addedOn}</span>
                {/if}
                <span class="{detailLabel} pt-0.5">{$t('detail.collections')}</span>
                <span class="flex flex-wrap gap-1.5">
                    {#each inCollections as c (c.id)}
                        <button on:click={() => onOpenCollection(c.id)} title={c.dynamic ? $t('nav.dynamic') : ''}
                                class="px-2 py-0.5 rounded-md text-xs bg-slate-950/60 text-slate-200 hover:text-white hover:bg-slate-800">{c.name}</button>
                    {/each}
                    <button on:click|stopPropagation={onCollectionsMenu} class="px-2 py-0.5 rounded-md text-xs border border-dashed border-white/15 text-slate-500 hover:text-slate-200">+ {$t('detail.add_to_collection')}</button>
                </span>
                <span class={detailLabel}>{$t('detail.folder')}</span>
                <button on:click={openFolder} class="text-left font-mono text-xs text-indigo-300 hover:text-indigo-100 break-all" title={$t('ctx.open_folder')}>{displayGame.folder_path}</button>
                <span class={detailLabel}>{$t('detail.launch_file')}</span>
                {#if launchName}
                    <button on:click={pickExe} class="text-left font-mono text-xs text-slate-300 hover:text-white break-all" title={$t('btn.set_exe')}>{launchName}</button>
                {:else}
                    <button on:click={pickExe} class="text-left text-xs text-rose-300 hover:text-rose-200">{$t('btn.set_exe')}</button>
                {/if}
                {/if}
            </div>
        </section>
    </aside>
</div>
