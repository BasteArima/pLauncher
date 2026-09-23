<script>
    // Страница игры (режим просмотра): шапка с обложкой и кнопками, описание,
    // скриншоты, коллекции и блок метаданных.
    import { t, tr } from '../i18n.js';
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
        if (!game.exec_path) {
            try {
                const path = await SelectExecutable(game.folder_path);
                if (!path) return;
                game.exec_path = path;
                await UpdateGame(game);
                showToast(tr('toast.saved'), 'success');
                await onReload();
            } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
            return;
        }
        try {
            await Launch(game.id, game.exec_path, game.folder_path);
            await onReload(); // «последний запуск» → порядок на главной
        } catch (err) { showToast(tr('toast.launch_fail', { err }), 'error'); }
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

    const chip = 'glass px-3 py-1 rounded-full';
    const chipBtn = 'glass px-3 py-1 rounded-full cursor-pointer hover:bg-white/10 transition-colors';
</script>

{#if game.folder_missing}
    <div class="mb-5 rounded-xl bg-amber-500/10 ring-1 ring-amber-400/40 p-4 flex items-center gap-4 animate-fade-in">
        <span class="text-2xl text-amber-400">⚠</span>
        <div class="flex-1 min-w-0">
            <div class="font-bold text-amber-200">{$t('missing.banner_title')}</div>
            <div class="text-xs text-amber-100/70 font-mono truncate" title={game.folder_path}>{game.folder_path}</div>
        </div>
        <button on:click={() => onRelink(game)}
                class="shrink-0 text-sm font-bold bg-amber-400/90 hover:bg-amber-300 text-black px-4 py-2 rounded-lg transition-colors">📁 {$t('missing.pick')}</button>
        <button on:click={onRelinkAll}
                class="shrink-0 text-sm font-semibold text-amber-200 hover:text-white bg-white/5 hover:bg-white/10 px-4 py-2 rounded-lg border border-amber-400/30 transition-colors">{$t('missing.auto')}</button>
    </div>
{/if}

<div class="relative rounded-2xl overflow-hidden ring-1 ring-white/10 mb-8 min-h-[340px] flex">
    {#if game.cover_path}
        <img src={mediaSrc(game.cover_path)} alt="" class="absolute inset-0 w-full h-full object-cover object-center scale-110 opacity-40 {$coverBlur === 'none' ? 'blur-lg' : 'blur-3xl'}"/>
    {/if}
    <div class="absolute inset-0 bg-gradient-to-r from-[#0a0912] via-[#0a0912]/80 to-[#0a0912]/40"></div>
    <div class="absolute inset-0 bg-gradient-to-t from-[#0a0912] to-transparent"></div>

    <div class="relative flex gap-8 p-8 w-full">
        {#if game.cover_path}
            <img src={mediaSrc(game.cover_path)} alt={game.title}
                 style={coverStyle(game)} on:error={(e) => e.target.style.display = 'none'}
                 class="w-[210px] aspect-[3/4] rounded-xl ring-1 ring-white/15 shadow-2xl shrink-0 hidden md:block transition duration-300 {blurCls($coverBlur, 'self')}"/>
        {/if}
        <div class="flex-1 min-w-0 flex flex-col">
            <h1 class="text-4xl xl:text-5xl font-black text-white drop-shadow-xl mb-3 break-words cursor-pointer hover:text-indigo-200 transition-colors" on:click={() => copy(game.title, $t('label.title'))} title={$t("detail.copy_title")}>
                {game.title}
            </h1>
            <div class="flex items-center flex-wrap gap-2.5 text-slate-300 font-medium mb-3">
                <span class={chipBtn} on:click={() => copy(game.version, $t('label.version'))}>
                    {$t("detail.version")}: <span class="text-white">{game.version || $t("detail.unknown")}</span>
                </span>
                {#if game.update_available}
                    <span class="flex items-center gap-2 px-3 py-1 rounded-full bg-amber-500/15 text-amber-200 ring-1 ring-amber-400/40">
                        ⬆ {$t('detail.update_to', { v: game.update_version })}{game.update_source ? ` · ${game.update_source}` : ''}
                        <button on:click={acceptUpdate} disabled={parsing}
                                class="text-xs font-bold bg-amber-400/90 hover:bg-amber-300 text-black px-2 py-0.5 rounded-full transition-colors disabled:opacity-60">
                            {$t("detail.accept_update")}
                        </button>
                    </span>
                {/if}
                {#if game.engine}
                    <span class={chip}>{$t("detail.engine")}: <span class="text-white">{game.engine}</span></span>
                {/if}
                {#if game.author}
                    <span class={chipBtn} on:click={() => copy(game.author, $t('detail.author'))}>
                        {$t("detail.author")}: <span class="text-white">{game.author}</span>
                    </span>
                {/if}
                {#if game.languages && game.languages.length}
                    <span class={chip}>{game.languages.join(', ')}</span>
                {/if}
                {#if game.time_played > 0}
                    <span class={chip}>🕒 {fmtPlaytime(game.time_played)}</span>
                {/if}
                {#if !game.folder_missing}
                    <span class={chipBtn} on:click={recalcSize} title={$t('detail.size_hint')}>
                        💾 {sizing ? $t('meta.checking') : (fmtSize(game.size_bytes) || $t('detail.size_unknown'))}
                    </span>
                {/if}
            </div>
            {#if game.tags && game.tags.length}
                <div class="flex flex-wrap gap-2 mb-5">
                    {#each game.tags as tag}
                        <button on:click={() => onApplyTag(tag)}
                                class="px-3 py-1 rounded-full text-sm bg-indigo-500/15 text-indigo-200 ring-1 ring-indigo-400/20 hover:bg-indigo-500/30 transition-colors">{tag}</button>
                    {/each}
                </div>
            {/if}
            <div class="flex items-center flex-wrap gap-2.5 mt-auto">
                <button on:click={smartPlay}
                        class="flex items-center justify-center gap-2.5 font-black py-3.5 px-10 rounded-xl text-lg tracking-wide transition-all hover:-translate-y-0.5 {game.folder_missing ? 'bg-gradient-to-r from-amber-400 to-amber-500 hover:from-amber-300 hover:to-amber-400 text-black' : game.exec_path ? 'bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-400 hover:to-green-500 text-white shadow-lg shadow-emerald-900/40' : 'bg-gradient-to-r from-orange-500 to-red-500 hover:from-orange-400 hover:to-red-400 text-white'}">
                    <svg class="w-5 h-5 fill-current" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                    {game.folder_missing ? $t('missing.pick') : game.exec_path ? $t('btn.play') : $t('btn.set_exe')}
                </button>
                <button on:click={openFolder} class="glass hover:bg-white/10 text-slate-200 font-semibold py-3.5 px-5 rounded-xl transition-colors" title={$t("ctx.open_folder")}>📁 {$t("btn.folder")}</button>
                <button on:click={() => onToggleFav(game)} class="{game.favorite ? 'bg-pink-500/80 text-white' : 'glass hover:bg-white/10 text-slate-200'} font-semibold py-3.5 px-5 rounded-xl transition-colors" title={game.favorite ? $t('detail.fav_in') : $t('detail.fav_add')}>
                    {game.favorite ? '❤️' : '🤍'}
                </button>
                <button on:click={onEdit} class="glass hover:bg-white/10 text-slate-200 font-semibold py-3.5 px-5 rounded-xl transition-colors" title={$t("detail.edit")}>✏️</button>
                <button on:click={onIgnore} class="glass hover:bg-amber-900/40 text-slate-400 hover:text-amber-300 font-semibold py-3.5 px-5 rounded-xl transition-colors" title={$t('ignore.hint')}>🚫</button>
                <button on:click={onRemove} class="glass hover:bg-red-900/50 text-slate-400 hover:text-red-400 font-semibold py-3.5 px-5 rounded-xl transition-colors" title={$t("detail.remove_from_launcher")}>🗑️</button>
            </div>
        </div>
    </div>
</div>

{#if game.description}
    <div class="mb-8">
        <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">{$t("detail.description")}</h3>
        <p class="text-lg text-slate-300 leading-relaxed whitespace-pre-wrap cursor-pointer hover:bg-white/5 p-4 -mx-4 rounded-xl transition-all" on:click={() => copy(game.description, $t('label.description'))}>
            {game.description}
        </p>
    </div>
{/if}

{#if game.images && game.images.length > 0}
    <div class="mb-8">
        <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">{$t("detail.screenshots")}</h3>
        <div class="flex gap-4 overflow-x-auto pb-3">
            {#each game.images as img, i}
                <div class="relative flex-shrink-0 w-72 aspect-video bg-slate-900 rounded-lg ring-1 ring-white/10 overflow-hidden group hover:ring-indigo-400/50 transition-all">
                    <img src={mediaSrc(img)} alt="screenshot" on:error={(e) => e.target.style.display = 'none'} class="w-full h-full object-cover cursor-pointer group-hover:scale-105 transition duration-500 {blurCls($coverBlur, 'icon')}" on:click={() => onOpenImage(i)}/>
                </div>
            {/each}
        </div>
    </div>
{/if}

<div class="mb-8">
    <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">{$t("detail.collections")}</h3>
    <div class="flex flex-wrap gap-2 items-center">
        {#each manualCollections as c}
            {@const inside = (c.game_ids || []).includes(game.id)}
            <button on:click={() => onToggleCollection(c.id, game)}
                    class="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm transition-colors {inside ? 'bg-indigo-500/25 text-white ring-1 ring-indigo-400/40' : 'bg-white/5 text-slate-300 hover:bg-white/10'}">
                <span class="text-xs">{inside ? '✓' : '＋'}</span> {c.name}
            </button>
        {/each}
        <button on:click={onCreateCollection} class="px-3 py-1.5 rounded-full text-sm border border-dashed border-white/20 text-slate-400 hover:text-indigo-300 hover:border-indigo-500 transition-colors">
            {$t("detail.new_collection")}
        </button>
    </div>
    {#if manualCollections.length === 0}
        <p class="text-xs text-slate-500 mt-2">{$t("detail.collections_hint")}</p>
    {/if}
</div>

<MetadataPanel {game} {supportedSources} {parsing} onParse={parse} {onReload}/>
