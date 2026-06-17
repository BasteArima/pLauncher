<script>
    import { t } from './i18n.js';
    export let game;
    export let onOpen = () => {};
    export let onPlay = null;        // если задан и есть exec_path — кнопка запуска
    export let onToggleFav = null;   // если задан — кнопка «избранное»
    export let discreet = false;     // блюр обложки (дискретный режим)
    export let onDrag = null;        // (game) => ... при начале перетаскивания
    export let onContext = null;     // (game, event) => ... ПКМ

    $: cover = game.cover_path ? `/media/${game.cover_path.replaceAll('\\', '/')}` : '';
    $: coverCss = `object-fit:${game.cover_fit || 'cover'};object-position:${game.cover_pos || '50% 20%'}`;
    let coverError = false;
    $: if (cover) coverError = false; // сбрасываем при смене обложки

    function handleDragStart(e) {
        if (!onDrag) return;
        e.dataTransfer.effectAllowed = 'copy';
        try { e.dataTransfer.setData('text/plain', game.id); } catch (_) {}
        onDrag(game);
    }
    function handleContext(e) {
        if (!onContext) return;
        e.preventDefault();
        e.stopPropagation();
        onContext(game, e);
    }
</script>

<div
        class="group relative cursor-pointer rounded-xl overflow-hidden ring-1 ring-white/10 hover:ring-indigo-400/50 bg-white/5 transition-all duration-300 hover:-translate-y-1.5 hover:shadow-2xl hover:shadow-indigo-900/50 {game.update_available ? 'ring-2 ring-amber-400/70' : ''}"
        draggable={!!onDrag}
        on:dragstart={handleDragStart}
        on:contextmenu={handleContext}
        on:click={onOpen}>

    <div class="aspect-[3/4] w-full bg-slate-900/60 relative">
        {#if cover && !coverError}
            <img src={cover} alt={game.title} style={coverCss} on:error={() => coverError = true}
                 class="absolute inset-0 w-full h-full transition-transform duration-500 group-hover:scale-105 {discreet ? 'blur-2xl scale-110' : ''}"/>
        {:else}
            <div class="absolute inset-0 flex flex-col items-center justify-center text-slate-600 gap-2 p-4 text-center">
                <span class="text-3xl opacity-40">🎮</span>
                <span class="text-sm leading-snug line-clamp-3">{discreet ? '•••' : game.title}</span>
            </div>
        {/if}

        {#if onToggleFav}
            <button
                    on:click|stopPropagation={() => onToggleFav(game)}
                    title={game.favorite ? $t('ctx.fav_remove') : $t('detail.fav_add')}
                    class="absolute top-2.5 left-2.5 w-9 h-9 rounded-full flex items-center justify-center transition-all backdrop-blur-sm {game.favorite ? 'bg-pink-500/90 text-white opacity-100' : 'bg-black/40 text-white/80 opacity-0 group-hover:opacity-100 hover:bg-black/60'}">
                <svg class="w-4.5 h-4.5" viewBox="0 0 24 24" fill={game.favorite ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="2"><path d="M12 21s-7.5-4.6-10-9.3C.5 8.5 2 5 5.5 5 7.7 5 9 6.5 12 9c3-2.5 4.3-4 6.5-4C22 5 23.5 8.5 22 11.7 19.5 16.4 12 21 12 21z"/></svg>
            </button>
        {/if}

        {#if onPlay && game.exec_path}
            <button
                    on:click|stopPropagation={() => onPlay(game)}
                    title={$t('btn.play')}
                    class="absolute top-2.5 right-2.5 w-10 h-10 rounded-full bg-emerald-500/90 hover:bg-emerald-400 text-white flex items-center justify-center opacity-0 group-hover:opacity-100 translate-y-1 group-hover:translate-y-0 transition-all shadow-lg backdrop-blur-sm">
                <svg class="w-5 h-5 fill-current ml-0.5" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
            </button>
        {/if}

        <div class="absolute inset-x-0 bottom-0 pt-10 pb-2.5 px-3 bg-gradient-to-t from-black/95 via-black/55 to-transparent pointer-events-none">
            <h3 class="font-bold text-white text-sm line-clamp-1 drop-shadow">{game.title}</h3>
            <div class="flex items-center justify-between mt-1">
                <span class="text-[11px] text-slate-300 truncate pr-2 flex items-center gap-1">
                    {#if game.update_available}<span class="text-amber-300 font-bold">⬆</span>{/if}{game.version || '—'}
                </span>
                {#if game.exec_path}
                    <span class="shrink-0 px-1.5 py-0.5 rounded text-[9px] font-bold tracking-widest bg-emerald-500/20 text-emerald-300 ring-1 ring-emerald-400/30">EXE</span>
                {:else}
                    <span class="shrink-0 px-1.5 py-0.5 rounded text-[9px] font-bold tracking-widest bg-rose-500/20 text-rose-300 ring-1 ring-rose-400/30">NO EXE</span>
                {/if}
            </div>
        </div>
    </div>
</div>
