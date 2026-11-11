<script>
    import { onDestroy, tick } from 'svelte';
    import { t, langStore } from '../i18n.js';
    import { GetLibraryStats } from '../../wailsjs/go/main/App.js';
    import { statsRequest, statsDuration, statsHours, statsNumber, statsSize } from '../lib/statistics.js';
    import { statusInfo } from '../lib/status.js';
    import WeeklyBars from './WeeklyBars.svelte';
    import Icon from './Icon.svelte';
    export let games = [];
    export let scopeKey = '';
    export let revision = 0;
    export let calendar;
    export let active = true;
    export let period = 'month';
    export let onSelect = () => {};
    export let onReady = () => {};
    const request = statsRequest();
    let retry = 0, allEngines = false;
    $: key = scopeKey;
    async function load(key, games, revision, calendar, active, retry, period) {
        const loaded = await request.load(key, () => GetLibraryStats(games.map(g => g.id), period, calendar.zone), active);
        if (loaded) { await tick(); onReady(); }
    }
    $: load(key, games, revision, calendar, active, retry, period);
    $: data = $request.key === key ? $request.data : null;
    $: gameMap = new Map(games.map(g => [g.id, g]));
    $: top = (data?.top_games || []).filter(g => gameMap.has(g.game_id));
    $: topMax = Math.max(1, ...top.map(g => g.seconds));
    $: engines = data?.engines || [];
    $: shownEngines = allEngines || engines.length <= 5 ? engines : [...engines.slice(0, 5), { name: null, count: engines.slice(5).reduce((n, e) => n + e.count, 0) }];
    const panel = 'rounded-xl border border-white/10 bg-slate-900/60 p-5 min-w-0';
    onDestroy(request.destroy);
</script>

<div data-statistics class="max-w-6xl mx-auto pb-4">
    <div class="flex items-center gap-3 mb-2"><Icon name="chart" size={26}/><h1 class="text-2xl font-semibold text-white">{$t('stats.title')}</h1></div>
    <p class="text-sm text-slate-400 mb-6">{$t('stats.scope')}</p>
    {#if $request.error}
        <div class={panel} role="alert"><p>{$t('toast.error', { err: $request.error })}</p>
            <button data-stats-retry on:click={() => retry++} class="mt-4 px-4 py-2 rounded-md bg-indigo-500 hover:bg-indigo-400 text-white text-sm">{$t('stats.retry')}</button></div>
    {:else if data}
        {#if !games.length}<p data-stats-empty class="text-sm text-slate-400 mb-4">{$t('stats.empty_library')}</p>{/if}
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4" data-stat-totals>
            {#each [['all', 'stats.all_time', data.all_time_seconds], ['month', 'stats.month', data.month_seconds], ['week', 'stats.week', data.week_seconds]] as [id, label, seconds]}
                <div class={panel}><p class="text-xs text-slate-400 mb-2">{$t(label)}</p><p data-total={id} data-seconds={seconds} class="text-3xl font-semibold tracking-tight text-white tabular-nums">{statsHours(seconds, $langStore)}</p></div>
            {/each}
        </div>
        <p class="text-xs text-slate-500 mt-3 mb-6">{$t('stats.recorded_only')}</p>
        <section class="{panel} mb-5">
            <div class="flex flex-wrap justify-between gap-2 mb-5"><h2 class="font-semibold text-white">{$t('stats.activity')}</h2><span class="text-xs text-slate-400">{$t('stats.year')}</span></div>
            <WeeklyBars weeks={data.weeks || []} zone={calendar.zone} label={$t('stats.activity')}/>
        </section>
        <section class="{panel} mb-5" data-stats-top>
            <div class="flex flex-wrap justify-between items-center gap-3 mb-5">
                <h2 class="font-semibold text-white">{$t('stats.top')}</h2>
                <div class="flex rounded-lg border border-white/10 p-1 gap-1" role="group" aria-label={$t('stats.period')}>
                    {#each ['week', 'month', 'all'] as choice}<button data-stats-period={choice} aria-pressed={period === choice} on:click={() => period = choice}
                        class="px-3 py-1.5 rounded-md text-xs {period === choice ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-white'}">{$t(choice === 'all' ? 'stats.all_time' : 'stats.' + choice)}</button>{/each}
                </div>
            </div>
            {#if $request.loading}
                <p role="status" class="text-sm text-slate-400 py-4" style="min-height:{Math.max(70, top.length * 70)}px">{$t('stats.loading')}</p>
            {:else if top.length}
                <ol class="space-y-2">{#each top as entry, i}
                    <li><button data-stats-game={entry.game_id} on:click={() => onSelect(gameMap.get(entry.game_id))} class="w-full text-left rounded-lg px-2 py-2.5 hover:bg-white/5 focus-visible:ring-2 focus-visible:ring-indigo-400">
                        <div class="flex gap-3 items-center text-sm mb-2"><span class="w-5 text-slate-500 tabular-nums">{statsNumber(i + 1, $langStore)}</span>
                            <span class="flex-1 min-w-0 truncate"><span class="spoiler text-slate-200">{gameMap.get(entry.game_id).title}</span></span><span class="tabular-nums text-slate-400">{statsDuration(entry.seconds, $langStore)}</span><Icon name="chevron-right" size={14}/></div>
                        <div class="h-1 bg-white/5 rounded-full ml-8"><div class="h-full bg-indigo-400/70 rounded-full" style="width:{entry.seconds / topMax * 100}%"></div></div>
                    </button></li>
                {/each}</ol>
            {:else}<p data-stats-no-activity class="text-sm text-slate-400 py-4">{$t('stats.no_activity')}</p>{/if}
        </section>
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
            <section class={panel}><h2 class="font-semibold text-white mb-5">{$t('stats.statuses')}</h2>
                <ul class="space-y-4">{#each data.statuses || [] as status}<li>
                    <div class="flex justify-between gap-2 text-sm mb-2"><span class={statusInfo(status.name)?.text || 'text-slate-400'}>{status.name ? $t('status.' + status.name) : $t('stats.no_status')}</span><span>{statsNumber(status.count, $langStore)}</span></div>
                    <div class="h-1 rounded-full bg-white/5"><div class="h-full rounded-full {statusInfo(status.name)?.dot || 'bg-slate-500'}" style="width:{status.count / Math.max(1, games.length) * 100}%"></div></div>
                </li>{/each}</ul>
            </section>
            <section class={panel}><h2 class="font-semibold text-white mb-5">{$t('stats.engines')}</h2>
                <ul class="space-y-4">{#each shownEngines as engine}<li class="flex justify-between gap-2 text-sm"><span class="truncate">{engine.name === null ? $t('stats.other') : engine.name || $t('detail.unknown')}</span><span>{statsNumber(engine.count, $langStore)}</span></li>{/each}</ul>
                {#if engines.length > 5}<button data-all-engines on:click={() => allEngines = !allEngines} class="text-xs text-indigo-300 hover:text-white mt-5">{allEngines ? $t('stats.show_less') : $t('stats.show_all')}</button>{/if}
            </section>
            <section class={panel}><h2 class="font-semibold text-white mb-3">{$t('stats.disk')}</h2><p class="text-3xl font-semibold text-white mb-4" data-disk-bytes={data.disk.bytes}>{statsSize(data.disk.bytes, $langStore)}</p>
                <dl class="space-y-2 text-sm">{#each [['stats.measured', data.disk.measured], ['stats.unknown_sizes', data.disk.unknown], ['stats.missing_folders', data.disk.missing]] as [label, count]}<div class="flex justify-between gap-2"><dt class="text-slate-400">{$t(label)}</dt><dd>{statsNumber(count, $langStore)}</dd></div>{/each}</dl>
                <p class="text-xs text-slate-500 mt-4">{$t('stats.disk_hint')}</p>
            </section>
            <section class={panel}><h2 class="font-semibold text-white mb-5">{$t('stats.library_info')}</h2>
                <dl class="space-y-5">{#each [['stats.updates', data.updates], ['stats.no_cover', data.no_cover], ['stats.no_source', data.no_source]] as [label, count]}<div class="flex justify-between gap-3"><dt class="text-sm text-slate-400">{$t(label)}</dt><dd class="text-lg text-white tabular-nums">{statsNumber(count, $langStore)}</dd></div>{/each}</dl>
            </section>
        </div>
    {:else}<div class="{panel} min-h-[360px]" role="status">{$t('stats.loading')}</div>{/if}
</div>
