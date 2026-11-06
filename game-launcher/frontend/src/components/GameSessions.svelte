<script>
    import { onDestroy } from 'svelte';
    import { t, langStore } from '../i18n.js';
    import { GetGameSessions } from '../../wailsjs/go/main/App.js';
    import { statsRequest, statsDuration, statsNumber } from '../lib/statistics.js';
    import WeeklyBars from './WeeklyBars.svelte';
    export let game;
    export let visibleGames = [];
    export let scopeKey = '';
    export let revision = 0;
    export let calendar;
    const request = statsRequest();
    let retry = 0;
    $: key = `${scopeKey}:${game.id}`;
    $: { game; visibleGames; revision; calendar; retry;
        request.load(key, () => GetGameSessions(game.id, visibleGames.map(g => g.id), calendar.zone)); }
    $: data = $request.key === key ? $request.data : null;
    onDestroy(request.destroy);
</script>

<section data-game-sessions class="rounded-2xl bg-slate-900/60 border border-white/5 p-5 min-w-0">
    <div class="flex items-center justify-between gap-2 mb-3">
        <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider">{$t('sessions.title')}</h3>
        <span class="text-[10px] text-slate-500">{$t('sessions.eight_weeks')}</span>
    </div>
    {#if $request.error}
        <p class="text-xs text-rose-300" role="alert">{$t('toast.error', { err: $request.error })}</p>
        <button data-sessions-retry on:click={() => retry++} class="mt-3 px-3 py-2 rounded-md bg-white/5 hover:bg-white/10 text-sm">{$t('stats.retry')}</button>
    {:else if data}
        {#if !data.tracking_supported}
            <p data-sessions-untracked class="text-xs text-slate-400">{$t('sessions.untracked')}</p>
        {:else}
            <WeeklyBars compact weeks={data.weeks || []} zone={calendar.zone} label={$t('sessions.eight_weeks')}/>
            {#if data.count}
                <p data-session-count class="text-xs text-slate-400 mt-2">{$t('sessions.summary', { count: statsNumber(data.count, $langStore), duration: statsDuration(data.longest_seconds, $langStore) })}</p>
            {:else}
                <p data-sessions-empty class="text-xs text-slate-400 mt-2">{$t('sessions.empty')}</p>
            {/if}
        {/if}
    {:else}<p role="status" class="text-xs text-slate-400">{$t('stats.loading')}</p>{/if}
</section>
