<script>
    import { onDestroy } from 'svelte';
    import { t, langStore } from '../i18n.js';
    import { GetGameSessions } from '../../wailsjs/go/main/App.js';
    import { statsRequest, statsDuration, statsDate, statsNumber } from '../lib/statistics.js';
    import WeeklyBars from './WeeklyBars.svelte';
    import Icon from './Icon.svelte';
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

<section data-game-sessions class="order-3 border-t border-white/10 pt-6">
    <div class="flex items-center gap-2 mb-4"><Icon name="chart" size={18}/><h3 class="text-lg font-semibold text-white">{$t('sessions.title')}</h3></div>
    {#if $request.error}
        <p class="text-sm text-rose-300" role="alert">{$t('toast.error', { err: $request.error })}</p>
        <button data-sessions-retry on:click={() => retry++} class="mt-3 px-3 py-2 rounded-md bg-white/5 hover:bg-white/10 text-sm">{$t('stats.retry')}</button>
    {:else if data}
        <p class="text-xs text-slate-400 mb-3">{$t('sessions.eight_weeks')}</p>
        <WeeklyBars weeks={data.weeks || []} zone={calendar.zone} label={$t('sessions.eight_weeks')}/>
        <dl class="flex flex-wrap gap-8 mt-3 mb-5">
            <div><dt class="text-xs text-slate-400">{$t('sessions.count')}</dt><dd data-session-count class="text-xl text-white mt-1 tabular-nums">{statsNumber(data.count, $langStore)}</dd></div>
            <div><dt class="text-xs text-slate-400">{$t('sessions.longest')}</dt><dd class="text-xl text-white mt-1 tabular-nums">{statsDuration(data.longest_seconds, $langStore)}</dd></div>
        </dl>
        {#if data.latest?.length}
            <h4 class="text-xs font-semibold text-slate-400 mb-2">{$t('sessions.latest')}</h4>
            <ol class="divide-y divide-white/5">
                {#each data.latest as session}
                    <li data-session-row class="flex flex-wrap justify-between gap-x-4 gap-y-1 py-2.5 text-sm">
                        <div><time datetime={new Date(session.started_at * 1000).toISOString()}>{statsDate(session.started_at, $langStore, calendar.zone, true)}</time>
                            {#if session.interrupted}<span class="block text-xs text-amber-300 mt-1" title={$t('sessions.interrupted_hint')}>{$t('sessions.interrupted')}</span>{/if}</div>
                        <span class="text-slate-200 tabular-nums">{statsDuration(session.duration_seconds, $langStore)}</span>
                    </li>
                {/each}
            </ol>
        {:else}<p data-sessions-empty class="text-sm text-slate-400">{$t('sessions.empty')}</p>{/if}
        {#if !data.tracking_supported}<p class="text-xs text-slate-400 mt-3">{$t('sessions.untracked')}</p>{/if}
        <p class="text-xs text-slate-500 mt-3">{$t('sessions.legacy')}</p>
    {:else}<p role="status" class="text-sm text-slate-400">{$t('stats.loading')}</p>{/if}
</section>
