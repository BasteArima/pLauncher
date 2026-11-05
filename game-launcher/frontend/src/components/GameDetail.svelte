<script>
    // Страница игры (режим просмотра), макет A3: слева большая обложка/скриншот и лента
    // миниатюр, справа карточка с «Играть», временем, размером, версией; ниже описание,
    // теги, коллекции, детали и блок метаданных (ссылки на площадки, загрузка по ссылке).
    import { t, tr, langStore } from '../i18n.js';
    import { showToast } from '../lib/ui.js';
    import { mediaSrc, coverStyle, fmtPlaytime, fmtSize, copyText } from '../lib/util.js';
    import { Launch, OpenFolder, SelectExecutable, UpdateGame, UpdateGameMetadata, RecalcGameSize } from '../../wailsjs/go/main/App.js';
    import MetadataPanel from './MetadataPanel.svelte';
    import { coverBlur, blurCls } from '../lib/privacy.js';

    export let game;
    export let supportedSources = [];
    export let manualCollections = [];
    export let onEdit = () => {};
    export let onRemove = () => {};
    export let onIgnore = () => {};
    export let onToggleFav = (g) => {};
    export let onRelink = (g) => {};                     // указать новую папку вручную
    export let onRelinkAll = () => {};                   // окно перепривязки
    export let onToggleCollection = (colId, g) => {};
    export let onCreateCollection = () => {};
    export let onApplyTag = (tag) => {};
    export let onOpenImage = (index) => {};
    export let onReload = async () => {};

    let parsing = false;
    const copy = (text, label) => copyText(text, label, showToast);

    // --- Медиа: обложка + скриншоты в одной ленте; выбранное показывается крупно ---
    $: media = [
        ...(game.cover_path ? [{ src: game.cover_path, cover: true }] : []),
        ...(game.images || []).map((src, i) => ({ src, shot: i })),
    ];
    let mediaIndex = 0;
    let shownId = '';
    $: if (game.id !== shownId) { shownId = game.id; mediaIndex = 0; }
    $: if (mediaIndex >= media.length) mediaIndex = 0;
    $: current = media[mediaIndex];
    // Листать картинки прямо на странице, как в Steam (по кругу)
    function stepMedia(dir) { if (media.length > 1) mediaIndex = (mediaIndex + dir + media.length) % media.length; }
    function openCurrent() {
        if (current && current.shot !== undefined) onOpenImage(current.shot);
        else if ((game.images || []).length) onOpenImage(0);
    }

    let sizing = false;
    async function recalcSize() {
        if (sizing) return;
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
            await onReload(); // «последний запуск» → порядок на главной
        } catch (err) { showToast(tr('toast.launch_fail', { err }), 'error'); }
    }
    async function pickExe() {
        try {
            const path = await SelectExecutable(game.folder_path);
            if (!path) return;
            game.exec_path = path;
            await UpdateGame(game);
            showToast(tr('toast.saved'), 'success');
            await onReload();
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

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

    // «Принять» обновление = перепарсить с площадки, где оно найдено (обновит версию, снимет флаг)
    function acceptUpdate() {
        const src = (game.sources || []).find(s => s.source === game.update_source);
        if (src && src.url) parse(src.url);
    }

    $: lastPlayed = ($langStore, game.last_launched_at
        ? fmtDay(game.last_launched_at * 1000, $langStore)
        : tr('detail.never'));
    // Дата без года, если это текущий год: «5 нояб.» вместо «5 нояб. 2026 г.»
    function fmtDay(ms, lang) {
        const d = new Date(ms);
        const opts = d.getFullYear() === new Date().getFullYear() ? { day: 'numeric', month: 'short' } : { day: 'numeric', month: 'short', year: 'numeric' };
        try { return d.toLocaleDateString(lang, opts); } catch (e) { return d.toLocaleDateString(undefined, opts); }
    }
    // Игра только с папкой (ещё ни разу не парсилась): блок «Найти информацию» сразу под картинками
    $: noInfo = !game.cover_path && !game.description && !(game.sources || []).some(x => x.url);
    $: launchName = (game.exec_path || '').split(/[\\/]/).pop();

    const card = 'rounded-2xl bg-slate-900/60 border border-white/5';
    const sectionTitle = 'text-xs font-semibold text-slate-500 uppercase tracking-wider mb-3';
    const iconBtn = 'h-12 w-12 shrink-0 rounded-xl flex items-center justify-center border border-white/10 bg-slate-900 text-slate-300 hover:bg-slate-800 hover:text-white transition-colors';
    const smallBtn = 'h-9 px-3 rounded-lg flex items-center gap-1.5 text-xs text-slate-400 hover:text-white hover:bg-white/5 transition-colors';
</script>

{#if game.folder_missing}
    <div class="mb-6 rounded-xl bg-rose-950/50 border border-rose-800/50 p-4 flex items-center gap-4 flex-wrap animate-fade-in">
        <div class="flex-1 min-w-[240px]">
            <div class="font-semibold text-rose-100">{$t('missing.banner_title')}</div>
            <div class="text-xs text-rose-100/70 font-mono truncate" title={game.folder_path}>{game.folder_path}</div>
        </div>
        <button on:click={() => onRelink(game)}
                class="shrink-0 h-10 px-4 rounded-lg text-sm font-semibold bg-rose-300 hover:bg-rose-200 text-rose-950 transition-colors">{$t('missing.pick')}</button>
        <button on:click={onRelinkAll}
                class="shrink-0 h-10 px-4 rounded-lg text-sm text-rose-100 border border-rose-700/60 hover:bg-white/5 transition-colors">{$t('missing.auto')}</button>
    </div>
{/if}

<div class="flex flex-wrap gap-7 items-start mb-9">
    <!-- Картинки: крупно выбранная + лента -->
    <section class="flex-[999_1_520px] min-w-0">
        <div class="relative w-full aspect-video rounded-2xl overflow-hidden bg-slate-900 border border-white/5 group {current ? 'cursor-zoom-in' : ''}"
             on:click={openCurrent}>
            {#if current}
                {#key current.src}
                    <!-- Размытая подложка, чтобы вертикальная обложка не висела на пустом фоне -->
                    <img src={mediaSrc(current.src)} alt="" aria-hidden="true"
                         class="absolute inset-0 w-full h-full object-cover scale-110 blur-2xl opacity-30"/>
                    <img src={mediaSrc(current.src)} alt={current.cover ? game.title : $t('detail.screenshots')}
                         on:error={(e) => e.target.style.display = 'none'}
                         class="absolute inset-0 w-full h-full object-contain animate-fade-in transition duration-300 {blurCls($coverBlur, 'cover')} {game.folder_missing ? 'grayscale opacity-60' : ''}"/>
                {/key}
                {#if media.length > 1}
                    <button on:click|stopPropagation={() => stepMedia(-1)} aria-label={$t('detail.prev_image')} title={$t('detail.prev_image')}
                            class="absolute left-3 top-1/2 -translate-y-1/2 w-11 h-11 rounded-full flex items-center justify-center text-white bg-black/35 hover:bg-black/55 backdrop-blur-md ring-1 ring-inset ring-white/15 opacity-0 group-hover:opacity-100 transition">
                        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 5l-7 7 7 7"/></svg>
                    </button>
                    <button on:click|stopPropagation={() => stepMedia(1)} aria-label={$t('detail.next_image')} title={$t('detail.next_image')}
                            class="absolute right-3 top-1/2 -translate-y-1/2 w-11 h-11 rounded-full flex items-center justify-center text-white bg-black/35 hover:bg-black/55 backdrop-blur-md ring-1 ring-inset ring-white/15 opacity-0 group-hover:opacity-100 transition">
                        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 5l7 7-7 7"/></svg>
                    </button>
                    <span class="absolute bottom-3 right-3 px-2 py-0.5 rounded-md text-[11px] text-white/90 bg-black/40 backdrop-blur-md opacity-0 group-hover:opacity-100 transition">{mediaIndex + 1} / {media.length}</span>
                {/if}
            {:else}
                <div class="absolute inset-0 flex flex-col items-center justify-center gap-3 text-slate-500 p-6 text-center">
                    <svg class="w-12 h-12 text-slate-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                    <div class="text-slate-300 font-medium">{$t('detail.no_media')}</div>
                    <div class="text-sm max-w-md">{$t('detail.no_media_hint')}</div>
                </div>
            {/if}
        </div>
        {#if media.length > 1}
            <div class="flex gap-2 mt-3 overflow-x-auto pb-2">
                {#each media as m, i}
                    <button on:click={() => mediaIndex = i} title={m.cover ? $t('detail.cover') : $t('detail.screenshots')}
                            class="relative shrink-0 w-28 aspect-video rounded-lg overflow-hidden bg-slate-900 group transition {i === mediaIndex ? 'ring-2 ring-indigo-400' : 'ring-1 ring-white/10 opacity-70 hover:opacity-100'}">
                        <img src={mediaSrc(m.src)} alt="" on:error={(e) => e.target.style.display = 'none'}
                             class="w-full h-full object-cover {blurCls($coverBlur, 'icon')}"/>
                    </button>
                {/each}
            </div>
        {/if}
    </section>

    <!-- Карточка действий -->
    <aside class="flex-[1_1_340px] max-w-[440px] {card} p-6 flex flex-col gap-5">
        <div>
            <h1 class="text-3xl font-bold text-white leading-tight break-words cursor-pointer hover:text-indigo-200 transition-colors"
                on:click={() => copy(game.title, $t('label.title'))} title={$t('detail.copy_title')}>{game.title}</h1>
            {#if game.author || game.engine}
                <div class="text-sm text-slate-400 mt-1.5">
                    {#if game.author}<span class="cursor-pointer hover:text-white" on:click={() => copy(game.author, $t('detail.author'))}>{game.author}</span>{/if}{#if game.author && game.engine}{' · '}{/if}{#if game.engine}{game.engine}{/if}
                </div>
            {/if}
        </div>

        <div class="flex gap-2">
            <button on:click={smartPlay}
                    class="flex-1 h-12 rounded-xl flex items-center justify-center gap-2.5 text-base font-bold transition-colors {game.folder_missing ? 'bg-rose-300 hover:bg-rose-200 text-rose-950' : game.exec_path ? 'bg-indigo-500 hover:bg-indigo-400 text-white' : 'bg-slate-100 hover:bg-white text-slate-950'}">
                {#if game.exec_path && !game.folder_missing}<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M7 4l13 8-13 8z"/></svg>{/if}
                {game.folder_missing ? $t('missing.pick') : game.exec_path ? $t('btn.play') : $t('btn.set_exe')}
            </button>
            <button on:click={openFolder} class={iconBtn} title={$t('ctx.open_folder')} aria-label={$t('ctx.open_folder')}>
                <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
            </button>
            <button on:click={() => onToggleFav(game)} class="{iconBtn} {game.favorite ? '!text-pink-400' : ''}"
                    title={game.favorite ? $t('detail.fav_in') : $t('detail.fav_add')} aria-label={game.favorite ? $t('detail.fav_in') : $t('detail.fav_add')}>
                <svg class="w-5 h-5" viewBox="0 0 24 24" fill={game.favorite ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="2"><path d="M12 20s-7-4.4-9.3-8.8C1.3 8.2 2.8 5 6 5c2 0 3.2 1.3 6 3.6C14.8 6.3 16 5 18 5c3.2 0 4.7 3.2 3.3 6.2C19 15.6 12 20 12 20z"/></svg>
            </button>
        </div>

        <div class="grid grid-cols-3 gap-2">
            <div class="rounded-lg bg-slate-950/60 px-3 py-2.5">
                <div class="text-[11px] text-slate-500">{$t('detail.played')}</div>
                <div class="text-sm font-semibold text-slate-100 mt-0.5 leading-snug">{fmtPlaytime(game.time_played) || '—'}</div>
            </div>
            <div class="rounded-lg bg-slate-950/60 px-3 py-2.5">
                <div class="text-[11px] text-slate-500">{$t('detail.last_played')}</div>
                <div class="text-sm font-semibold text-slate-100 mt-0.5 leading-snug">{lastPlayed}</div>
            </div>
            <button on:click={recalcSize} disabled={game.folder_missing} title={$t('detail.size_hint')}
                    class="rounded-lg bg-slate-950/60 px-3 py-2.5 text-left hover:bg-slate-800 transition-colors disabled:hover:bg-slate-950/60">
                <div class="text-[11px] text-slate-500">{$t('detail.size')}</div>
                <div class="text-sm font-semibold text-slate-100 mt-0.5 leading-snug">{sizing ? '…' : (fmtSize(game.size_bytes) || '—')}</div>
            </button>
        </div>

        <div class="flex flex-col gap-2.5 text-sm">
            <div class="flex items-center gap-3 flex-wrap">
                <span class="w-24 shrink-0 text-slate-500">{$t('detail.version')}</span>
                <span class="text-slate-100 cursor-pointer hover:text-indigo-200" on:click={() => copy(game.version, $t('label.version'))}>{game.version || $t('detail.unknown')}</span>
            </div>
            {#if game.languages && game.languages.length}
                <div class="flex items-center gap-3">
                    <span class="w-24 shrink-0 text-slate-500">{$t('detail.languages')}</span>
                    <span class="text-slate-100">{game.languages.join(', ')}</span>
                </div>
            {/if}
        </div>

        {#if game.update_available}
            <div class="rounded-xl bg-amber-950/60 border border-amber-700/50 p-3.5 flex flex-col gap-2.5">
                <div class="text-sm text-amber-200 font-semibold">⬆ {$t('detail.update_to', { v: game.update_version })}{game.update_source ? ` · ${game.update_source}` : ''}</div>
                <button on:click={acceptUpdate} disabled={parsing}
                        class="self-start h-8 px-3 rounded-md text-xs font-bold bg-amber-400 hover:bg-amber-300 text-amber-950 transition-colors disabled:opacity-60">{$t('detail.accept_update')}</button>
            </div>
        {/if}

        <div class="flex flex-wrap gap-1 -mx-1 pt-1 border-t border-white/5">
            <button on:click={onEdit} class={smallBtn}>
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h4L19 9l-4-4L4 16z"/></svg>{$t('detail.edit')}
            </button>
            <button on:click={onIgnore} class={smallBtn} title={$t('ignore.hint')}>
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M5.6 5.6l12.8 12.8"/></svg>{$t('ignore.action')}
            </button>
            <button on:click={onRemove} class="{smallBtn} hover:!text-rose-300">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13"/></svg>{$t('detail.remove_from_launcher')}
            </button>
        </div>
    </aside>
</div>

{#if noInfo}
    <MetadataPanel {game} {supportedSources} {parsing} onParse={parse} {onReload} prominent/>
{/if}

<div class="flex flex-wrap gap-7 items-start mb-9">
    <div class="flex-[999_1_520px] min-w-0">
        <h3 class={sectionTitle}>{$t('detail.description')}</h3>
        {#if game.description}
            <p class="text-[15px] text-slate-300 leading-relaxed whitespace-pre-wrap cursor-pointer hover:bg-white/5 p-3 -mx-3 rounded-xl transition-colors max-w-3xl"
               on:click={() => copy(game.description, $t('label.description'))}>{game.description}</p>
        {:else}
            <p class="text-sm text-slate-500">{$t('detail.no_description')}</p>
        {/if}
        {#if game.tags && game.tags.length}
            <div class="flex flex-wrap gap-2 mt-4">
                {#each game.tags as tag}
                    <button on:click={() => onApplyTag(tag)}
                            class="px-3 py-1 rounded-lg text-xs text-slate-300 bg-slate-900 border border-white/10 hover:border-indigo-400/50 hover:text-white transition-colors">{tag}</button>
                {/each}
            </div>
        {/if}
    </div>

    <aside class="flex-[1_1_340px] max-w-[440px] flex flex-col gap-5">
        <section class="{card} p-5">
            <h3 class={sectionTitle}>{$t('detail.collections')}</h3>
            <div class="flex flex-wrap gap-2 items-center">
                {#each manualCollections as c}
                    {@const inside = (c.game_ids || []).includes(game.id)}
                    <button on:click={() => onToggleCollection(c.id, game)}
                            class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs transition-colors {inside ? 'bg-indigo-900 text-indigo-100 ring-1 ring-indigo-500/60' : 'bg-slate-950/60 text-slate-400 hover:text-white'}">
                        <span>{inside ? '✓' : '＋'}</span> {c.name}
                    </button>
                {/each}
                <button on:click={onCreateCollection} class="px-2.5 py-1 rounded-lg text-xs border border-dashed border-white/20 text-slate-400 hover:text-indigo-200 hover:border-indigo-400 transition-colors">
                    {$t('detail.new_collection')}
                </button>
            </div>
            {#if manualCollections.length === 0}
                <p class="text-xs text-slate-500 mt-2">{$t('detail.collections_hint')}</p>
            {/if}
        </section>

        <section class="{card} p-5">
            <h3 class={sectionTitle}>{$t('detail.details')}</h3>
            <div class="grid grid-cols-[96px_minmax(0,1fr)] gap-x-3 gap-y-2.5 text-sm">
                <span class="text-slate-500">{$t('detail.folder')}</span>
                <button on:click={openFolder} class="text-left font-mono text-xs text-indigo-300 hover:text-indigo-100 break-all" title={$t('ctx.open_folder')}>{game.folder_path}</button>
                <span class="text-slate-500">{$t('detail.launch_file')}</span>
                {#if launchName}
                    <button on:click={pickExe} class="text-left font-mono text-xs text-slate-300 hover:text-white break-all" title={$t('btn.set_exe')}>{launchName}</button>
                {:else}
                    <button on:click={pickExe} class="text-left text-xs text-rose-300 hover:text-rose-200">{$t('btn.set_exe')}</button>
                {/if}
            </div>
        </section>
    </aside>
</div>

{#if !noInfo}
    <MetadataPanel {game} {supportedSources} {parsing} onParse={parse} {onReload}/>
{/if}
