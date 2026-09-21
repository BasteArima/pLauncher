<script>
    // Сетка карточек игр (полка «Все игры» и результаты фильтра/поиска).
    import GameCard from './GameCard.svelte';

    export let items = [];
    export let discreet = false;
    export let onOpen = (g) => {};
    export let onPlay = null;
    export let onToggleFav = null;
    export let onDrag = null;
    export let onContext = null;
</script>

<div class="grid gap-5 grid-cols-[repeat(auto-fill,minmax(170px,1fr))] {$$props.class || ''}">
    {#each items as game, i (game.id)}
        <div class="animate-rise-in" style="animation-delay:{Math.min(i, 14) * 30}ms">
            <GameCard {game} onOpen={() => onOpen(game)} {onPlay} {onToggleFav} {discreet} {onDrag} {onContext} />
        </div>
    {/each}
</div>
