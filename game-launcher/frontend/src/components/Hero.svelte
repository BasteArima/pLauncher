<script>
    // Баннер «Продолжить» на главной.
    import { t } from '../i18n.js';
    import { mediaSrc, coverStyle, fmtPlaytime } from '../lib/util.js';
    import { coverBlur, blurCls } from '../lib/privacy.js';

    export let game;
    export let onOpen = (g) => {};
    export let onPlay = (g) => {};
    export let onToggleFav = (g) => {};
</script>

<div class="relative h-[176px] rounded-2xl overflow-hidden ring-1 ring-white/10 mb-7 cursor-pointer animate-rise-in group"
     on:click={() => onOpen(game)}>
    {#if game.cover_path}
        <img src={mediaSrc(game.cover_path)} alt="" class="absolute inset-0 w-full h-full object-cover object-center scale-110 opacity-50 transition-transform duration-700 group-hover:scale-105 {$coverBlur === 'none' ? 'blur-sm' : blurCls($coverBlur, 'bg')}"/>
    {/if}
    <div class="absolute inset-0 bg-gradient-to-r from-[#0a0912] via-[#0a0912]/75 to-transparent"></div>
    <div class="absolute inset-0 bg-gradient-to-t from-[#0a0912] via-transparent to-transparent"></div>

    <button on:click|stopPropagation={() => onToggleFav(game)}
            title={game.favorite ? $t('ctx.fav_remove') : $t('detail.fav_add')}
            class="absolute top-3 right-3 w-8 h-8 rounded-full flex items-center justify-center backdrop-blur-sm transition-all {game.favorite ? 'bg-pink-500/90 text-white' : 'bg-black/40 text-white/80 hover:bg-black/60'}">
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill={game.favorite ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="2"><path d="M12 21s-7.5-4.6-10-9.3C.5 8.5 2 5 5.5 5 7.7 5 9 6.5 12 9c3-2.5 4.3-4 6.5-4C22 5 23.5 8.5 22 11.7 19.5 16.4 12 21 12 21z"/></svg>
    </button>

    <div class="relative h-full flex items-center gap-5 px-6 py-4">
        {#if game.cover_path}
            <img src={mediaSrc(game.cover_path)} alt={game.title}
                 style={coverStyle(game)} on:error={(e) => e.target.style.display = 'none'}
                 class="hidden sm:block h-full aspect-[3/4] rounded-lg ring-1 ring-white/15 shadow-2xl shrink-0 transition duration-300 {blurCls($coverBlur, 'cover')}"/>
        {/if}
        <div class="min-w-0">
            <div class="text-[10px] uppercase tracking-widest text-indigo-300 mb-1 font-semibold">
                {game.last_launched_at ? $t('hero.continue') : $t('hero.recent')}
            </div>
            <h2 class="text-2xl font-black text-white drop-shadow-lg mb-1.5 line-clamp-1">{game.title}</h2>
            <div class="flex items-center gap-2 text-xs text-slate-300 mb-3">
                <span>{game.version || '—'}</span>
                {#if game.languages && game.languages.length}
                    <span class="text-slate-600">•</span>
                    <span class="truncate">{game.languages.join(', ')}</span>
                {/if}
                {#if game.time_played > 0}
                    <span class="text-slate-600">•</span>
                    <span>🕒 {fmtPlaytime(game.time_played)}</span>
                {/if}
            </div>
            <div class="flex items-center gap-2.5">
                <button on:click|stopPropagation={() => onPlay(game)}
                        class="flex items-center gap-2 bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-400 hover:to-green-500 text-white font-bold py-2 px-6 rounded-lg shadow-lg shadow-emerald-900/40 transition-all hover:-translate-y-0.5">
                    <svg class="w-4 h-4 fill-current" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                    {game.exec_path ? $t('btn.play') : $t('btn.set_exe')}
                </button>
                <button on:click|stopPropagation={() => onOpen(game)}
                        class="glass hover:bg-white/10 text-white font-semibold py-2 px-5 rounded-lg transition-colors text-sm">
                    {$t("btn.details")}
                </button>
            </div>
        </div>
    </div>
</div>
