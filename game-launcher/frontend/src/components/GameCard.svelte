<script>
    import { t } from '../i18n.js';
    import { mediaSrc, coverStyle } from '../lib/util.js';
    import { coverBlur, blurCls } from '../lib/privacy.js';
    import { statusInfo } from '../lib/status.js';
    import { selection, selecting, toggleSelected, selectRange, cardShape, ctxGameId } from '../lib/view.js';
    export let game;
    export let onOpen = () => {};
    export let onPlay = null;        // если задан и есть exec_path — кнопка запуска
    export let onToggleFav = null;   // если задан — кнопка «избранное»
    export let onDrag = null;        // (game) => ... при начале перетаскивания
    export let onContext = null;     // (game, event) => ... ПКМ

    $: cover = mediaSrc(game.cover_path);
    $: coverCss = coverStyle(game, $cardShape);
    // Тип файла запуска для бейджа: EXE, HTML, BAT, JAR…
    $: launchName = (game.exec_path || '').split(/[\\/]/).pop();
    $: launchExt = launchName.includes('.') ? launchName.split('.').pop().toUpperCase().slice(0, 8) : '▶';
    let coverError = false;
    $: if (cover) coverError = false; // сбрасываем при смене обложки

    function handleDragStart(e) {
        if (!onDrag) return;
        e.dataTransfer.effectAllowed = 'copy';
        try { e.dataTransfer.setData('text/plain', game.id); } catch (_) {}
        onDrag(game);
    }
    // Клик: Ctrl — добавить/убрать из выделения, Shift — диапазон; если уже что-то
    // выделено, обычный клик тоже работает как выделение. Иначе — открыть игру.
    $: selected = $selection.has(game.id);
    function handleClick(e) {
        if (e.shiftKey) { selectRange(e.currentTarget); return; }
        if (e.ctrlKey || e.metaKey || $selecting) { toggleSelected(game.id); return; }
        onOpen();
    }
    // Клавиатура на сфокусированной карточке: Enter — открыть, Ctrl+Enter — играть,
    // Пробел — выделить. Стрелки обрабатывает App (переход между карточками).
    function handleKey(e) {
        if (e.target !== e.currentTarget) return;
        if (e.key === 'Enter') {
            e.preventDefault();
            if ((e.ctrlKey || e.metaKey) && onPlay) onPlay(game); else onOpen();
        } else if (e.key === ' ') {
            e.preventDefault();
            toggleSelected(game.id);
        }
    }
    function handleContext(e) {
        if (!onContext) return;
        e.preventDefault();
        e.stopPropagation();
        onContext(game, e);
    }
    $: canPlay = onPlay && game.exec_path && !game.folder_missing;
    $: st = statusInfo(game.status);
    // Меню ⋯ этой карточки открыто — держим её в состоянии наведения
    $: active = $ctxGameId === game.id;
    // Как в Epic: кнопки быстро выезжают — верхние сверху, нижняя снизу; .is-active держит их видимыми
    const shown = 'group-hover:opacity-100 group-focus-visible:opacity-100 group-[.is-active]:opacity-100';
    const slideTop = `opacity-0 -translate-y-1.5 group-hover:translate-y-0 group-focus-visible:translate-y-0 group-[.is-active]:translate-y-0 ${shown} transition duration-150 ease-out`;
    const slideBottom = `opacity-0 translate-y-1.5 group-hover:translate-y-0 group-focus-visible:translate-y-0 group-[.is-active]:translate-y-0 ${shown} transition duration-150 ease-out`;
    // Полупрозрачная «стеклянная» кнопка поверх обложки
    // backdrop-blur — только при наведении: размытие на сотнях скрытых кнопок сильно тормозит прокрутку
    const glassBtn = 'w-9 h-9 rounded-lg flex items-center justify-center text-white bg-white/15 hover:bg-white/25 group-hover:backdrop-blur-md group-[.is-active]:backdrop-blur-md ring-1 ring-inset ring-white/20 transition-colors';
</script>

<div
        data-game-id={game.id}
        tabindex="0"
        role="button"
        aria-pressed={selected}
        class="group relative cursor-pointer rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-indigo-300 focus-visible:ring-offset-4 focus-visible:ring-offset-slate-950 {active ? 'is-active' : ''}"
        draggable={!!onDrag}
        on:dragstart={handleDragStart}
        on:contextmenu={handleContext}
        on:click={handleClick}
        on:keydown={handleKey}>

    <div class="relative w-full {$cardShape === 'wide' ? 'aspect-video' : 'aspect-[3/4]'} rounded-xl overflow-hidden bg-slate-900 ring-1 transition duration-200 group-hover:shadow-[0_12px_28px_rgba(0,0,0,0.5)] group-[.is-active]:shadow-[0_12px_28px_rgba(0,0,0,0.5)] {selected ? 'ring-2 ring-indigo-400' : 'ring-white/5 group-hover:ring-white/30 group-[.is-active]:ring-white/30'}">
        {#if cover && !coverError}
            <img src={cover} alt={game.title} style={coverCss} on:error={() => coverError = true} loading="lazy" decoding="async"
                 class="absolute inset-0 w-full h-full transition duration-500 {blurCls($coverBlur, 'card')} {game.folder_missing ? 'grayscale opacity-50' : ''}"/>
        {:else}
            <div class="absolute inset-0 flex flex-col items-center justify-center text-slate-600 gap-2 p-4 text-center">
                <svg class="w-8 h-8" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                <span class="text-xs">{$t('nav.no_cover')}</span>
            </div>
        {/if}

        {#if $selecting}
            <!-- Режим выделения: чекбокс вместо кнопок -->
            <span class="absolute top-2.5 left-2.5 w-7 h-7 rounded-md flex items-center justify-center text-sm font-bold ring-2 transition-colors {selected ? 'bg-indigo-500 ring-indigo-300 text-white' : 'bg-black/50 ring-white/40 text-transparent'}">✓</span>
        {:else}
            <!-- Мягкие тени сверху и снизу под кнопками (не сплошное затемнение) -->
            <div class="absolute inset-x-0 top-0 h-1/2 bg-gradient-to-b from-black/55 to-transparent pointer-events-none opacity-0 {shown} transition-opacity duration-150"></div>
            <div class="absolute inset-x-0 bottom-0 h-3/5 bg-gradient-to-t from-black/70 to-transparent pointer-events-none opacity-0 {shown} transition-opacity duration-150"></div>

            {#if game.favorite}
                <!-- Метка избранного видна и без наведения -->
                <span class="absolute top-2.5 left-2.5 w-7 h-7 rounded-md flex items-center justify-center text-pink-300 bg-black/55 group-hover:opacity-0 group-[.is-active]:opacity-0 transition-opacity">
                    <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M12 20.3l-1.3-1.2C5.6 14.5 2.5 11.7 2.5 8.3 2.5 5.6 4.6 3.5 7.3 3.5c1.5 0 3 .7 4 1.9 1-1.2 2.5-1.9 4-1.9 2.7 0 4.8 2.1 4.8 4.8 0 3.4-3.1 6.2-8.2 10.8z"/></svg>
                </span>
            {/if}
            <div class="absolute top-2.5 inset-x-2.5 flex justify-between pointer-events-none {slideTop}">
                {#if onToggleFav}
                    <button on:click|stopPropagation={() => onToggleFav(game)}
                            title={game.favorite ? $t('ctx.fav_remove') : $t('detail.fav_add')}
                            aria-label={game.favorite ? $t('ctx.fav_remove') : $t('detail.fav_add')}
                            class="pointer-events-auto {glassBtn} {game.favorite ? '!text-pink-300' : ''}">
                        <svg class="w-[18px] h-[18px]" viewBox="0 0 24 24" fill={game.favorite ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><path d="M12 20.3l-1.3-1.2C5.6 14.5 2.5 11.7 2.5 8.3 2.5 5.6 4.6 3.5 7.3 3.5c1.5 0 3 .7 4 1.9 1-1.2 2.5-1.9 4-1.9 2.7 0 4.8 2.1 4.8 4.8 0 3.4-3.1 6.2-8.2 10.8z"/></svg>
                    </button>
                {:else}<span></span>{/if}
                {#if onContext}
                    <button on:click|stopPropagation={handleContext} title={$t('card.more')} aria-label={$t('card.more')}
                            class="pointer-events-auto {glassBtn}">
                        <svg class="w-[18px] h-[18px]" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.7"/><circle cx="12" cy="12" r="1.7"/><circle cx="19" cy="12" r="1.7"/></svg>
                    </button>
                {/if}
            </div>

            <!-- Большая кнопка снизу: запуск без захода на страницу игры -->
            <div class="absolute inset-x-2.5 bottom-2.5 {slideBottom}">
                {#if canPlay}
                    <button on:click|stopPropagation={() => onPlay(game)} title={$t('btn.play') + ' · Ctrl+Enter'}
                            class="w-full h-10 rounded-lg flex items-center justify-center gap-2 text-sm font-semibold text-white bg-indigo-500 hover:bg-indigo-400 transition-colors">
                        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor"><path d="M7 4l13 8-13 8z"/></svg>
                        {$t('btn.play')}
                    </button>
                {:else if !game.folder_missing && !game.exec_path}
                    <button on:click|stopPropagation={onOpen}
                            class="w-full h-10 rounded-lg flex items-center justify-center text-sm font-medium text-white bg-white/15 hover:bg-white/25 group-hover:backdrop-blur-md group-[.is-active]:backdrop-blur-md ring-1 ring-inset ring-white/20 transition-colors">
                        {$t('btn.set_exe')}
                    </button>
                {/if}
            </div>
        {/if}
    </div>

    <!-- Подпись под обложкой -->
    <div class="pt-2.5 px-0.5 min-w-0">
        <h3 class="text-sm font-medium text-slate-100 truncate {game.folder_missing ? 'opacity-60' : ''}">{game.title}</h3>
        <div class="flex items-center gap-1.5 mt-0.5 text-xs text-slate-500 min-w-0">
            {#if st}
                <span class="shrink-0 w-1.5 h-1.5 rounded-full {st.dot}"></span>
                <span class="shrink-0 {st.text}">{$t('status.short_' + st.id)}</span>
                <span class="shrink-0 text-slate-600">·</span>
            {/if}
            {#if game.update_available}
                <span class="shrink-0 text-amber-300" title={$t('detail.update_to', { v: game.update_version })}>⬆ {game.update_version}</span>
                <span class="shrink-0 text-slate-600">·</span>
            {/if}
            <span class="truncate">{game.version || '—'}</span>
            {#if game.folder_missing}
                <span class="shrink-0 text-amber-400 font-semibold" title={$t('missing.card_hint')}>· ⚠ {$t('missing.badge')}</span>
            {:else if game.exec_path}
                <span class="shrink-0" title={$t('card.launch_hint', { file: launchName })}>· {launchExt}</span>
            {:else}
                <span class="shrink-0 text-rose-400 font-semibold">· {$t('card.no_launch')}</span>
            {/if}
        </div>
    </div>
</div>
