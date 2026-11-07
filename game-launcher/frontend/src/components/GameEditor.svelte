<script>
    // Режим редактирования — та же страница игры (макет A4), но поля редактируются на месте.
    // Меняет переданный объект game (bind:game в родителе); сохранение/отмену решает родитель.
    // Слева: картинки (обложка отдельно, скриншоты — порядок перетаскиванием), блок «Обложка»
    // с живыми превью карточек (тянуть — сдвиг, колесо — зум) и ползунками, описание и теги.
    // Справа: название, автор, движок, версия, языки, файл запуска, ссылки на сайты (★ — для обновлений).
    import { t, tr } from '../i18n.js';
    import { showToast, askConfirm } from '../lib/ui.js';
    import { mediaSrc, coverStyle, coverPosXY, coverZoom, COVER_ZOOM_MAX } from '../lib/util.js';
    import Icon from './Icon.svelte';
    import LaunchFilesEditor from './LaunchFilesEditor.svelte';
    import {
        SelectCoverImage, CopyCoverToData, SelectScreenshots, CopyScreenshotToData,
    } from '../../wailsjs/go/main/App.js';

    export let game;
    export let supportedSources = [];
    export let dirty = false;
    export let onSave = () => {};
    export let onCancel = () => {};

    // --- Что показано крупно: 'cover' или индекс скриншота ---
    let sel = game.cover_path ? 'cover' : (game.images || []).length ? 0 : 'cover';
    $: images = game.images || [];
    $: if (sel !== 'cover' && sel >= images.length) sel = images.length ? images.length - 1 : 'cover';
    $: shown = sel === 'cover' ? game.cover_path : images[sel];

    // --- Обложка ---
    async function selectCover() {
        try {
            const path = await SelectCoverImage();
            if (path) { game.cover_path = await CopyCoverToData(game.id, path); sel = 'cover'; }
        } catch (err) { showToast(tr('toast.cover_fail', { err }), 'error'); }
    }
    async function removeCover() {
        const ok = await askConfirm({ title: tr('dlg.remove_cover_title'), confirmText: tr('btn.delete'), danger: true });
        if (ok) game.cover_path = '';
    }
    function useAsCover(path) { game.cover_path = path; sel = 'cover'; }

    $: fit = game.cover_fit || 'cover';
    // Кадровка у широкой и вертикальной карточки независимая. active — какую сейчас правят
    // ползунки (выбирается кнопками или просто касанием превью).
    let active = 'wide';
    $: pos = coverPosXY(game, active);
    $: zoom = coverZoom(game, active);
    $: tallOwn = !!game.cover_pos_tall;
    // Пока у вертикальной нет своей кадровки, она повторяет широкую. Перед первой правкой
    // любой из них вертикальная получает копию — дальше они меняются каждая сама по себе.
    function detachTall() {
        if (game.cover_pos_tall) return;
        const p = coverPosXY(game, 'tall');
        game.cover_pos_tall = `${p.x}% ${p.y}%`;
        game.cover_zoom_tall = coverZoom(game, 'tall');
    }
    function setPos(x, y) {
        detachTall();
        const v = `${Math.round(x)}% ${Math.round(y)}%`;
        if (active === 'tall') game.cover_pos_tall = v; else game.cover_pos = v;
    }
    function setZoom(z) {
        detachTall();
        const v = Math.round(Math.max(1, Math.min(COVER_ZOOM_MAX, z)) * 100) / 100;
        if (active === 'tall') game.cover_zoom_tall = v; else game.cover_zoom = v;
    }
    function resetCover() { game.cover_pos = ''; game.cover_zoom = 0; game.cover_pos_tall = ''; game.cover_zoom_tall = 0; }
    function tallLikeWide() { game.cover_pos_tall = ''; game.cover_zoom_tall = 0; }
    const clamp = (v) => Math.max(0, Math.min(100, v));

    // Тянуть превью: картинка едет за курсором (точка фокуса смещается в обратную сторону)
    let dragging = false;
    function dragStart(e, shape) {
        if (e.button !== 0) return;
        active = shape;
        if (fit === 'fill') return;
        e.preventDefault();
        const r = e.currentTarget.getBoundingClientRect();
        const p = coverPosXY(game, shape);
        const start = { x: e.clientX, y: e.clientY, px: p.x, py: p.y };
        dragging = true;
        const k = 1 / coverZoom(game, shape);          // при зуме картинка «тяжелее» — двигаем медленнее
        const move = (ev) => setPos(clamp(start.px - (ev.clientX - start.x) / r.width * 100 * k),
                                    clamp(start.py - (ev.clientY - start.y) / r.height * 100 * k));
        const up = () => { dragging = false; window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up); };
        window.addEventListener('mousemove', move);
        window.addEventListener('mouseup', up);
    }
    function wheel(e, shape) {
        active = shape;
        if (fit !== 'fill') setZoom(coverZoom(game, shape) - e.deltaY * 0.0015);
    }

    // --- Скриншоты: добавить, удалить, порядок перетаскиванием ---
    async function addScreenshots() {
        try {
            const paths = await SelectScreenshots();
            if (!paths || paths.length === 0) return;
            const added = [];
            for (const p of paths) added.push(await CopyScreenshotToData(game.id, p));
            game.images = [...images, ...added];
        } catch (err) { showToast(tr('toast.screens_fail', { err }), 'error'); }
    }
    async function removeScreenshot(index) {
        const ok = await askConfirm({ title: tr('dlg.remove_screenshot_title'), confirmText: tr('btn.delete'), danger: true });
        if (ok) game.images = images.filter((_, i) => i !== index);
    }
    let dragIndex = null;
    let overIndex = null;
    function dropShot(i) {
        if (dragIndex === null || dragIndex === i) { dragIndex = overIndex = null; return; }
        const list = [...images];
        const [m] = list.splice(dragIndex, 1);
        list.splice(i, 0, m);
        game.images = list;
        sel = i;
        dragIndex = overIndex = null;
    }

    // --- Языки: строкой через запятую ---
    let langText = (game.languages || []).join(', ');
    function setLangs(v) { langText = v; game.languages = v.split(',').map(x => x.trim()).filter(Boolean); }

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

    function scrollToCover() { const el = document.getElementById('cover-settings'); if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' }); }

    const card = 'rounded-2xl bg-slate-900/60 border border-white/5';
    const sectionTitle = 'text-xs font-semibold text-slate-500 uppercase tracking-wider';
    const field = 'h-9 w-full px-2.5 rounded-lg bg-slate-950 border border-white/10 text-slate-100 text-sm focus:outline-none focus:border-indigo-400/60 select-text';
    const fieldLabel = 'text-xs text-slate-500';
    const glassBtn = 'h-9 px-3.5 rounded-lg text-sm text-white bg-slate-950/75 ring-1 ring-inset ring-white/20 hover:bg-slate-950/90 transition-colors';
    const thumbCls = (on) => 'relative shrink-0 w-28 aspect-video rounded-lg overflow-hidden bg-slate-900 transition ' + (on ? 'ring-2 ring-indigo-400' : 'ring-1 ring-white/10 opacity-80 hover:opacity-100');
    const segBtn = (on) => 'h-7 px-3 rounded-md text-xs transition-colors ' + (on ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-100');
</script>

<!-- Полоса режима редактирования: всегда сверху -->
<div class="sticky -top-6 z-20 -mx-8 -mt-6 mb-6 px-8 py-3.5 flex items-center gap-3.5 flex-wrap bg-slate-900 border-b border-indigo-900">
    <span class="w-2 h-2 rounded-full bg-indigo-400"></span>
    <span class="font-semibold text-white">{$t('edit.editing')}</span>
    <span class="text-sm text-slate-400">{dirty ? $t('edit.unsaved') : $t('edit.no_changes')}</span>
    <div class="flex-1"></div>
    <span class="text-xs text-slate-500">{$t('edit.shortcuts')}</span>
    <button on:click={onCancel} class="h-9 px-4 rounded-lg border border-white/10 text-sm text-slate-300 hover:text-white hover:bg-white/5">{$t('btn.cancel')}</button>
    <button on:click={onSave} class="h-9 px-4 rounded-lg text-sm font-semibold text-white bg-indigo-500 hover:bg-indigo-400">{$t('edit.save')}</button>
</div>

<div class="flex flex-wrap gap-7 items-start">
    <!-- ЛЕВАЯ КОЛОНКА -->
    <div class="flex-[999_1_520px] min-w-0 flex flex-col gap-7">
        <section class="flex flex-col gap-3">
            <div class="flex items-baseline gap-3 flex-wrap">
                <h3 class={sectionTitle}>{$t('edit.pictures')}</h3>
                <span class="text-xs text-slate-500">{$t('edit.pictures_hint')}</span>
            </div>
            <div class="relative w-full aspect-video rounded-2xl overflow-hidden bg-slate-900 border border-white/5">
                {#if shown}
                    {#key shown}
                        <img src={mediaSrc(shown)} alt="" aria-hidden="true" class="absolute inset-0 w-full h-full object-cover scale-110 blur-2xl opacity-30"/>
                        <img src={mediaSrc(shown)} alt="" on:error={(e) => e.target.style.display = 'none'} class="absolute inset-0 w-full h-full object-contain animate-fade-in"/>
                    {/key}
                    {#if sel === 'cover'}
                        <span class="absolute top-3.5 left-3.5 px-2.5 py-0.5 rounded-full text-xs font-semibold text-white bg-indigo-500">{$t('edit.cover')}</span>
                        <button on:click={scrollToCover} class="absolute right-3.5 bottom-3.5 {glassBtn}">{$t('edit.cover_settings')} ↓</button>
                    {:else}
                        <span class="absolute top-3.5 left-3.5 px-2.5 py-0.5 rounded-full text-xs text-slate-100 bg-slate-950/75">{$t('detail.screenshots')} · {sel + 1} / {images.length}</span>
                        <div class="absolute right-3.5 bottom-3.5 flex gap-2">
                            <button on:click={() => useAsCover(images[sel])} class={glassBtn}>★ {$t('edit.use_as_cover')}</button>
                            <button on:click={() => removeScreenshot(sel)} class="{glassBtn} !text-rose-200">{$t('cover.remove')}</button>
                        </div>
                    {/if}
                {:else}
                    <div class="absolute inset-0 flex flex-col items-center justify-center gap-3 text-slate-500">
                        <Icon name="image" size={48} class="text-slate-600"/>
                        <button on:click={selectCover} class="h-9 px-4 rounded-lg text-sm font-semibold text-indigo-200 border border-indigo-500/60 hover:bg-indigo-950">{$t('cover.add')}</button>
                    </div>
                {/if}
            </div>
            <div class="flex gap-2 items-center overflow-x-auto pb-1">
                <!-- Обложка отдельно от скриншотов -->
                {#if game.cover_path}
                    <button on:click={() => sel = 'cover'} class={thumbCls(sel === 'cover')} title={$t('edit.cover')}>
                        <img src={mediaSrc(game.cover_path)} alt="" class="w-full h-full object-cover"/>
                        <span class="absolute left-1.5 top-1.5 px-1.5 rounded text-[10px] font-semibold text-white bg-indigo-500">{$t('edit.cover')}</span>
                    </button>
                {:else}
                    <button on:click={selectCover} class="shrink-0 w-28 aspect-video rounded-lg border border-dashed border-indigo-500/50 text-xs text-indigo-200 hover:bg-indigo-950">{$t('cover.add')}</button>
                {/if}
                <span class="w-px h-10 bg-white/10 mx-1 shrink-0"></span>
                {#each images as img, i (img)}
                    <button draggable="true" on:click={() => sel = i}
                            on:dragstart={(e) => { dragIndex = i; try { e.dataTransfer.setData('text/plain', 'shot'); } catch (_) {} }}
                            on:dragover|preventDefault={() => overIndex = i}
                            on:dragleave={() => { if (overIndex === i) overIndex = null; }}
                            on:drop|preventDefault={() => dropShot(i)}
                            on:dragend={() => dragIndex = overIndex = null}
                            class="{thumbCls(sel === i)} {overIndex === i && dragIndex !== i ? 'ring-2 !ring-indigo-300' : ''} {dragIndex === i ? 'opacity-40' : ''} cursor-grab">
                        <img src={mediaSrc(img)} alt="" loading="lazy" draggable="false" class="w-full h-full object-cover pointer-events-none"/>
                    </button>
                {/each}
                <button on:click={addScreenshots} class="shrink-0 w-28 aspect-video rounded-lg border border-dashed border-white/20 text-xs text-slate-400 hover:text-white hover:border-white/40">+ {$t('edit.add')}</button>
            </div>
        </section>

        <!-- Обложка: картинка, замена/удаление, как она обрезается в карточках -->
        {#if game.cover_path}
            <section id="cover-settings" class="{card} p-5 flex flex-col gap-4 scroll-mt-20">
                <div class="flex items-center gap-3 flex-wrap">
                    <h3 class="{sectionTitle} flex-1">{$t('edit.cover')}</h3>
                    <div class="flex gap-0.5 p-0.5 rounded-lg bg-slate-950 border border-white/10" role="group" aria-label={$t('cover.view')}>
                        {#each [['cover', 'cover.fill'], ['contain', 'cover.fit'], ['fill', 'cover.stretch']] as opt}
                            <button on:click={() => game.cover_fit = opt[0]} class={segBtn(fit === opt[0])}>{$t(opt[1])}</button>
                        {/each}
                    </div>
                    <button on:click={resetCover} class="h-8 px-3 rounded-md text-xs text-slate-400 border border-white/10 hover:text-white">{$t('edit.reset')}</button>
                </div>

                <div class="flex gap-5 items-start flex-wrap">
                    <div class="w-28 flex flex-col gap-2">
                        <div class="relative w-full aspect-[2/3] rounded-lg overflow-hidden bg-slate-950 border border-white/10">
                            <img src={mediaSrc(game.cover_path)} alt="" class="absolute inset-0 w-full h-full object-contain"/>
                        </div>
                        <button on:click={selectCover} class="h-8 rounded-md text-xs text-slate-100 bg-slate-800 border border-white/10 hover:bg-slate-700">{$t('edit.replace')}</button>
                        <button on:click={removeCover} class="h-8 rounded-md text-xs text-rose-200 border border-rose-800/60 hover:bg-rose-950/50">{$t('cover.remove')}</button>
                    </div>
                    {#each [['aspect-video w-72', 'edit.wide_card', 'wide'], ['aspect-[3/4] w-36', 'edit.tall_card', 'tall']] as pv}
                        <div class="flex flex-col gap-1.5">
                            <!-- Как в библиотеке: тянуть — сдвинуть, колесо — приблизить; выделена та, что правят ползунки -->
                            <div class="relative {pv[0]} rounded-xl overflow-hidden bg-slate-950 select-none {active === pv[2] ? 'ring-2 ring-indigo-400' : 'ring-1 ring-white/10 hover:ring-white/30'} {fit === 'fill' ? 'cursor-pointer' : dragging ? 'cursor-grabbing' : 'cursor-grab'}"
                                 on:mousedown={(e) => dragStart(e, pv[2])} on:wheel|preventDefault|nonpassive={(e) => wheel(e, pv[2])} role="img" aria-label={$t(pv[1])}>
                                <img src={mediaSrc(game.cover_path)} alt="" draggable="false" style={coverStyle(game, pv[2])} class="absolute inset-0 w-full h-full pointer-events-none"/>
                            </div>
                            <span class="text-xs {active === pv[2] ? 'text-indigo-300' : 'text-slate-500'}">{$t(pv[1])}</span>
                        </div>
                    {/each}
                </div>

                <!-- Какую карточку правят ползунки -->
                <div class="flex items-center gap-3 flex-wrap">
                    <div class="flex gap-0.5 p-0.5 rounded-lg bg-slate-950 border border-white/10" role="group" aria-label={$t('edit.cover')}>
                        <button on:click={() => active = 'wide'} class={segBtn(active === 'wide')}>{$t('edit.wide_card')}</button>
                        <button on:click={() => active = 'tall'} class={segBtn(active === 'tall')}>{$t('edit.tall_card')}</button>
                    </div>
                    {#if active === 'tall' && tallOwn}
                        <button on:click={tallLikeWide} class="h-7 px-2.5 rounded-md text-xs text-slate-400 hover:text-white">{$t('edit.tall_same')}</button>
                    {/if}
                </div>

                <div class="grid grid-cols-[104px_minmax(0,1fr)_52px] gap-x-3 gap-y-2.5 items-center max-w-xl">
                    <label for="cover-x" class="text-xs text-slate-400">{$t('edit.horizontal')}</label>
                    <input id="cover-x" type="range" min="0" max="100" step="1" value={pos.x} disabled={fit === 'fill'}
                           on:input={(e) => setPos(+e.target.value, pos.y)} class="w-full accent-indigo-500 disabled:opacity-40"/>
                    <span class="text-xs text-slate-200 text-right tabular-nums">{pos.x}%</span>
                    <label for="cover-y" class="text-xs text-slate-400">{$t('edit.vertical')}</label>
                    <input id="cover-y" type="range" min="0" max="100" step="1" value={pos.y} disabled={fit === 'fill'}
                           on:input={(e) => setPos(pos.x, +e.target.value)} class="w-full accent-indigo-500 disabled:opacity-40"/>
                    <span class="text-xs text-slate-200 text-right tabular-nums">{pos.y}%</span>
                    <label for="cover-z" class="text-xs text-slate-400">{$t('edit.zoom')}</label>
                    <input id="cover-z" type="range" min="1" max={COVER_ZOOM_MAX} step="0.05" value={zoom} disabled={fit === 'fill'}
                           on:input={(e) => setZoom(+e.target.value)} class="w-full accent-indigo-500 disabled:opacity-40"/>
                    <span class="text-xs text-slate-200 text-right tabular-nums">{zoom.toFixed(2)}×</span>
                </div>
                <p class="text-xs text-slate-500 leading-relaxed">{$t('edit.cover_hint')}</p>
            </section>
        {/if}

        <section class="flex flex-col gap-3">
            <label for="game-about" class={sectionTitle}>{$t('detail.about')}</label>
            <textarea id="game-about" bind:value={game.description} rows="6" placeholder={$t('edit.desc_ph')}
                      class="w-full rounded-xl px-4 py-3 bg-slate-900/60 border border-white/10 text-[15px] leading-relaxed text-slate-200 placeholder-slate-600 focus:outline-none focus:border-indigo-400/50 resize-y select-text"></textarea>
            <div class="flex flex-wrap gap-2 items-center">
                {#each game.tags || [] as tag (tag)}
                    <span class="h-7 pl-2.5 pr-1 rounded-lg flex items-center gap-1 text-xs text-slate-200 bg-slate-900 border border-white/10">
                        {tag}
                        <button on:click={() => removeTag(tag)} aria-label={$t('btn.remove') + ': ' + tag} class="w-5 h-5 rounded flex items-center justify-center text-slate-500 hover:text-white"><Icon name="x" size={11}/></button>
                    </span>
                {/each}
                <input type="text" bind:value={newTag} placeholder={$t('edit.add_tag_ph')} aria-label={$t('edit.tags_label')}
                       on:keydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addTag(); } }}
                       class="h-7 w-48 px-2.5 rounded-lg bg-transparent border border-dashed border-white/20 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-400/60 select-text"/>
            </div>
        </section>
    </div>

    <!-- ПРАВАЯ КОЛОНКА: те же места, что на странице игры, но поля редактируются -->
    <aside class="flex-[1_1_340px] max-w-[420px] min-w-0 flex flex-col gap-5">
        <section class="{card} p-6 flex flex-col gap-3.5">
            <label class="flex flex-col gap-1.5">
                <span class={fieldLabel}>{$t('edit.title_label')}</span>
                <input type="text" bind:value={game.title} placeholder={$t('edit.title_ph')}
                       class="h-12 px-3 rounded-xl bg-slate-950 border border-white/10 text-xl font-bold text-white focus:outline-none focus:border-indigo-400/60 select-text"/>
            </label>
            <div class="grid grid-cols-2 gap-2.5">
                <label class="flex flex-col gap-1.5"><span class={fieldLabel}>{$t('detail.author')}</span>
                    <input type="text" bind:value={game.author} placeholder={$t('edit.author_ph')} class={field}/></label>
                <label class="flex flex-col gap-1.5"><span class={fieldLabel}>{$t('detail.engine')}</span>
                    <input type="text" bind:value={game.engine} placeholder="Ren'Py" class={field}/></label>
                <label class="flex flex-col gap-1.5"><span class={fieldLabel}>{$t('detail.version')}</span>
                    <input type="text" bind:value={game.version} placeholder="1.0" class={field}/></label>
                <label class="flex flex-col gap-1.5"><span class={fieldLabel}>{$t('detail.languages')}</span>
                    <input type="text" value={langText} on:input={(e) => setLangs(e.target.value)} placeholder="EN, RU" class={field}/></label>
            </div>
            <LaunchFilesEditor bind:game/>
        </section>

        <section class="{card} p-5 flex flex-col gap-2.5">
            <h3 class={sectionTitle}>{$t('detail.sources')}</h3>
            <p class="text-xs text-slate-500 leading-relaxed">{$t('edit.sources_hint')}</p>
            {#each supportedSources as s}
                {@const url = sourceUrl(game, s.name)}
                <div class="flex items-center gap-2">
                    <button type="button" disabled={!url} on:click={() => game.primary_source = s.name} title={$t('edit.primary_source')} aria-label={$t('edit.primary_source')}
                            class="w-7 h-7 shrink-0 text-base transition-colors {game.primary_source === s.name ? 'text-amber-300' : 'text-slate-600 hover:text-slate-300'} disabled:opacity-30">★</button>
                    <span class="w-24 shrink-0 text-sm text-slate-300 truncate">{s.name}</span>
                    <input type="text" value={url} on:change={(e) => setSourceUrl(s.name, e.target.value)} placeholder={s.domain}
                           class="{field} !h-8 font-mono text-[11px] flex-1 min-w-0"/>
                </div>
            {/each}
        </section>

        <p class="text-xs text-slate-500 leading-relaxed px-1">{$t('edit.personal_hint')}</p>
    </aside>
</div>
