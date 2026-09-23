<script>
    // Сетка карточек игр (полка «Все игры» и результаты фильтра/поиска).
    import GameCard from './GameCard.svelte';
    import { cardSize } from '../lib/view.js';

    export let items = [];
    export let onOpen = (g) => {};
    export let onPlay = null;
    export let onToggleFav = null;
    export let onDrag = null;
    export let onContext = null;
</script>

<div class="grid gap-5 {$$props.class || ''}" style="grid-template-columns:repeat(auto-fill,minmax({$cardSize}px,1fr))">
    {#each items as game, i (game.id)}
        <div class="animate-rise-in" style="animation-delay:{Math.min(i, 14) * 30}ms">
            <GameCard {game} onOpen={() => onOpen(game)} {onPlay} {onToggleFav} {onDrag} {onContext} />
        </div>
    {/each}
</div>
