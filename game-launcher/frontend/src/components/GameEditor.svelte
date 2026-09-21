<script>
    // Режим редактирования игры. Меняет переданный объект game (bind:game в родителе);
    // сохранение/отмену решает родитель через onSave/onCancel.
    import { t, tr } from '../i18n.js';
    import { showToast, askConfirm } from '../lib/ui.js';
    import { mediaSrc, coverStyle, coverPosXY } from '../lib/util.js';
    import {
        SelectCoverImage, CopyCoverToData, SelectScreenshots, CopyScreenshotToData, SelectExecutable,
    } from '../../wailsjs/go/main/App.js';

    export let game;
    export let supportedSources = [];
    export let onSave = () => {};
    export let onCancel = () => {};

    // --- Обложка ---
    async function selectCover() {
        try {
            const path = await SelectCoverImage();
            if (path) game.cover_path = await CopyCoverToData(game.id, path);
        } catch (err) { showToast(tr('toast.cover_fail', { err }), 'error'); }
    }
    async function removeCover() {
        const ok = await askConfirm({ title: tr('dlg.remove_cover_title'), confirmText: tr('btn.delete'), danger: true });
        if (ok) game.cover_path = '';
    }
    function setCoverFit(f) { game.cover_fit = f; }
    function setCoverPosX(v) { game.cover_pos = `${v}% ${coverPosXY(game).y}%`; }
    function setCoverPosY(v) { game.cover_pos = `${coverPosXY(game).x}% ${v}%`; }

    // --- Файл запуска ---
    async function selectExecutable() {
        try {
            const path = await SelectExecutable(game.folder_path); // диалог откроется в папке игры
            if (path) game.exec_path = path;
        } catch (err) { showToast(tr('toast.error', { err }), 'error'); }
    }

    // --- Теги ---
    let newTag = '';
    function addTag() {
        const tag = newTag.trim();
        if (!tag) return;
        const tags = game.tags || [];
        if (!tags.includes(tag)) game.tags = [...tags, tag];
        newTag = '';
    }
    function removeTag(tag) { game.tags = (game.tags || []).filter(x => x !== tag); }

    // --- Скриншоты ---
    async function addScreenshots() {
        try {
            const paths = await SelectScreenshots();
            if (!paths || paths.length === 0) return;
            const added = [];
            for (const p of paths) added.push(await CopyScreenshotToData(game.id, p));
            game.images = [...(game.images || []), ...added];
        } catch (err) { showToast(tr('toast.screens_fail', { err }), 'error'); }
    }
    async function removeScreenshot(index) {
        const ok = await askConfirm({ title: tr('dlg.remove_screenshot_title'), confirmText: tr('btn.delete'), danger: true });
        if (ok) game.images = game.images.filter((_, i) => i !== index);
    }

    // --- Ссылки по площадкам ---
    function sourceUrl(g, platform) {
        const s = (g.sources || []).find(x => x.source === platform);
        return s ? s.url : '';
    }
    function setSourceUrl(platform, url) {
        url = (url || '').trim();
        const list = game.sources ? [...game.sources] : [];
        const i = list.findIndex(x => x.source === platform);
        if (i >= 0) {
            if (url) list[i] = { ...list[i], url };
            else list.splice(i, 1);
        } else if (url) {
            list.push({ source: platform, url, last_version: '' });
        }
        game.sources = list;
        // Основной источник: первый со ссылкой, если не задан или его ссылку убрали
        if (!list.find(x => x.source === game.primary_source)) {
            game.primary_source = list.length ? list[0].source : '';
        }
    }
    function setPrimarySource(platform) { game.primary_source = platform; }

    const inputCls = 'bg-slate-900/50 text-white border border-slate-600 rounded-md px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none';
    const labelCls = 'text-slate-400 font-semibold uppercase tracking-wider text-xs w-20';
</script>

<div class="flex flex-col lg:flex-row gap-8">
    <div class="w-full lg:w-[260px] shrink-0">
        {#if game.cover_path}
            <div class="aspect-[3/4] w-full rounded-xl ring-1 ring-white/10 overflow-hidden relative group mb-4">
                <img src={mediaSrc(game.cover_path)} alt="cover" style={coverStyle(game)} on:error={(e) => e.target.style.display = 'none'} class="w-full h-full"/>
                <div class="absolute inset-0 bg-black/70 flex flex-col gap-3 items-center justify-center opacity-0 group-hover:opacity-100 transition-all backdrop-blur-sm">
                    <button on:click|stopPropagation={selectCover} class="bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-2 px-6 rounded-lg w-3/4 transition-colors">{$t("cover.change")}</button>
                    <button on:click|stopPropagation={removeCover} class="bg-red-600 hover:bg-red-500 text-white font-bold py-2 px-6 rounded-lg w-3/4 transition-colors">{$t("cover.remove")}</button>
                </div>
            </div>

            <div class="glass rounded-xl p-3 mb-4">
                <div class="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2">{$t("cover.view")}</div>
                <div class="flex gap-1.5 mb-3">
                    {#each [['cover','cover.fill'],['contain','cover.fit'],['fill','cover.stretch']] as opt}
                        <button on:click={() => setCoverFit(opt[0])}
                                class="flex-1 px-1 py-1.5 rounded-lg text-xs transition-colors {(game.cover_fit || 'cover') === opt[0] ? 'bg-indigo-500/30 text-white ring-1 ring-indigo-400/40' : 'bg-white/5 text-slate-300 hover:bg-white/10'}">{$t(opt[1])}</button>
                    {/each}
                </div>
                {#if (game.cover_fit || 'cover') !== 'fill'}
                    <div class="flex items-center gap-2 mb-1.5">
                        <span class="text-xs text-slate-500 w-5 text-center">↔</span>
                        <input type="range" min="0" max="100" step="1" value={coverPosXY(game).x} on:input={(e) => setCoverPosX(e.target.value)} class="flex-1 accent-indigo-500"/>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="text-xs text-slate-500 w-5 text-center">↕</span>
                        <input type="range" min="0" max="100" step="1" value={coverPosXY(game).y} on:input={(e) => setCoverPosY(e.target.value)} class="flex-1 accent-indigo-500"/>
                    </div>
                {/if}
            </div>
        {:else}
            <div class="aspect-[3/4] w-full rounded-xl ring-1 ring-white/10 bg-white/5 flex flex-col items-center justify-center text-slate-600 mb-4">
                <span class="text-6xl mb-4 opacity-30">🖼️</span>
                <button on:click|stopPropagation={selectCover} class="border-2 border-indigo-600/50 hover:border-indigo-500 text-indigo-400 hover:text-indigo-300 font-bold py-2 px-6 rounded-lg transition-all">{$t("cover.add")}</button>
            </div>
        {/if}

        <div class="flex flex-col gap-2">
            <button on:click={onSave} class="w-full bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-2.5 rounded-xl transition-colors">{$t("btn.save")}</button>
            <button on:click={onCancel} class="w-full bg-white/5 hover:bg-white/10 text-slate-300 font-bold py-2.5 rounded-xl transition-colors border border-white/10">{$t("btn.cancel")}</button>
        </div>
    </div>

    <div class="flex-1 min-w-0">
        <input type="text" bind:value={game.title} class="w-full text-3xl font-black glass text-white rounded-xl px-4 py-3 mb-4 focus:outline-none focus:ring-2 focus:ring-indigo-400/40" placeholder={$t("edit.title_ph")}/>

        <div class="flex flex-col gap-3 mb-6 glass rounded-xl p-4">
            <div class="flex items-center gap-3">
                <span class={labelCls}>{$t("edit.version_label")}</span>
                <input type="text" bind:value={game.version} class="{inputCls} w-40" placeholder="1.0"/>
            </div>
            <div class="flex items-center gap-3">
                <span class={labelCls}>{$t("edit.author_label")}</span>
                <input type="text" bind:value={game.author} class="{inputCls} flex-1" placeholder={$t("edit.author_ph")}/>
            </div>
            <div class="flex items-center gap-3">
                <span class={labelCls}>{$t("edit.engine_label")}</span>
                <input type="text" bind:value={game.engine} class="{inputCls} w-40" placeholder="Ren'Py"/>
            </div>
            <div class="flex items-center gap-3">
                <span class={labelCls}>{$t("edit.exe_label")}</span>
                <input type="text" bind:value={game.exec_path} class="{inputCls} flex-1 font-mono text-xs" placeholder="C:\Games\Game\run.exe" />
                <button on:click={selectExecutable} class="bg-white/10 hover:bg-white/20 text-slate-200 px-3 py-1.5 rounded-md border border-white/10 transition-colors" title={$t("edit.choose_file")}>📁</button>
            </div>
            <div class="flex items-start gap-3">
                <span class="{labelCls} pt-2">{$t("edit.tags_label")}</span>
                <div class="flex-1">
                    <div class="flex flex-wrap gap-2 mb-2">
                        {#each game.tags || [] as tag}
                            <span class="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs bg-indigo-500/20 text-indigo-200 ring-1 ring-indigo-400/20">
                                {tag}
                                <button on:click={() => removeTag(tag)} class="text-indigo-300 hover:text-white" title={$t("btn.remove")}>✕</button>
                            </span>
                        {/each}
                    </div>
                    <input type="text" bind:value={newTag} on:keydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addTag(); } }}
                           class="{inputCls} w-full" placeholder={$t("edit.add_tag_ph")}/>
                </div>
            </div>
        </div>

        <textarea bind:value={game.description} rows="6" class="w-full glass text-slate-300 rounded-xl px-4 py-4 mb-6 text-lg leading-relaxed focus:outline-none focus:ring-2 focus:ring-indigo-400/40 resize-y" placeholder={$t("edit.desc_ph")}></textarea>

        <h3 class="text-sm font-bold text-slate-400 mb-3 uppercase tracking-wider">{$t("detail.screenshots")}</h3>
        <div class="flex gap-4 overflow-x-auto pb-3">
            {#each game.images || [] as img, i}
                <div class="relative flex-shrink-0 w-64 aspect-video bg-slate-900 rounded-lg ring-1 ring-white/10 overflow-hidden group">
                    <img src={mediaSrc(img)} alt="screenshot" on:error={(e) => e.target.style.display = 'none'} class="w-full h-full object-cover"/>
                    <button on:click|stopPropagation={() => removeScreenshot(i)} class="absolute top-2 right-2 bg-red-600/90 hover:bg-red-500 text-white rounded-full w-8 h-8 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity text-sm font-bold shadow-lg" title={$t("edit.remove_screenshot")}>✕</button>
                </div>
            {/each}
            <div class="flex-shrink-0 w-64 aspect-video glass rounded-lg border-2 border-dashed border-white/15 hover:border-indigo-500 flex flex-col items-center justify-center cursor-pointer transition-colors text-slate-400 hover:text-indigo-400" on:click={addScreenshots}>
                <span class="text-4xl mb-1 font-light">+</span>
                <span class="text-xs font-bold uppercase tracking-wider">{$t("edit.add")}</span>
            </div>
        </div>

        <h3 class="text-sm font-bold text-slate-400 mt-6 mb-1 uppercase tracking-wider">{$t("edit.sources_label")}</h3>
        <p class="text-xs text-slate-500 mb-3">{$t("edit.sources_hint")}</p>
        <div class="flex flex-col gap-2 mb-6">
            {#each supportedSources as s}
                {@const url = sourceUrl(game, s.name)}
                <div class="flex items-center gap-2">
                    <button type="button" disabled={!url} on:click={() => setPrimarySource(s.name)}
                            title={$t("edit.primary_source")}
                            class="w-7 text-center text-lg transition-colors {game.primary_source === s.name ? 'text-amber-400' : 'text-slate-600 hover:text-slate-300'} disabled:opacity-30 disabled:hover:text-slate-600">
                        {game.primary_source === s.name ? '★' : '☆'}
                    </button>
                    <span class="w-36 shrink-0 text-sm text-slate-300 truncate">{s.name}</span>
                    <input type="text" value={url} on:change={(e) => setSourceUrl(s.name, e.target.value)}
                           placeholder={s.domain}
                           class="{inputCls} flex-1 font-mono text-xs"/>
                </div>
            {/each}
        </div>
    </div>
</div>
