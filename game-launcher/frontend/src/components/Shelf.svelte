<script>
    import GameCard from './GameCard.svelte';
    import { cardSize } from '../lib/view.js';

    export let title;
    export let items = [];
    export let onOpen = () => {};
    export let onPlay = null;
    export let onToggleFav = null;
    export let onDrag = null;
    export let onContext = null;

    let row;
    function scroll(dir) {
        if (row) row.scrollBy({ left: dir * row.clientWidth * 0.85, behavior: 'smooth' });
    }
</script>

{#if items.length}
    <div class="mb-9">
        <div class="flex items-center justify-between mb-3">
            <h3 class="text-lg font-bold text-white tracking-tight">
                {title}<span class="text-slate-500 text-sm font-medium ml-2">{items.length}</span>
            </h3>
            <div class="flex gap-1.5">
                <button on:click={() => scroll(-1)} aria-label="Назад"
                        class="w-8 h-8 rounded-full glass hover:bg-white/10 text-slate-200 flex items-center justify-center transition-colors text-lg leading-none">‹</button>
                <button on:click={() => scroll(1)} aria-label="Вперёд"
                        class="w-8 h-8 rounded-full glass hover:bg-white/10 text-slate-200 flex items-center justify-center transition-colors text-lg leading-none">›</button>
            </div>
        </div>
        <div bind:this={row} class="flex gap-4 overflow-x-auto pb-3 -mx-1 px-1">
            {#each items as g (g.id)}
                <div class="shrink-0" style="width:{Math.round($cardSize * 0.93)}px">
                    <GameCard game={g} onOpen={() => onOpen(g)} {onPlay} {onToggleFav} {onDrag} {onContext} />
                </div>
            {/each}
        </div>
    </div>
{/if}
