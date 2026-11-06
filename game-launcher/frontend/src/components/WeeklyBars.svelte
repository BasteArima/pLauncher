<script>
    import { t, langStore } from '../i18n.js';
    import { statsDuration, statsDate, statsMonth } from '../lib/statistics.js';
    export let weeks = [];
    export let zone = 'UTC';
    export let label = '';
    let picked = null;
    $: maximum = Math.max(1, ...weeks.map(w => w.seconds));
    $: { weeks; picked = null; }
    // Textual language/zone dependencies are deliberate (Svelte 3).
    const description = (w, lang, tz) => `${statsDate(w.start, lang, tz)} – ${statsDate(w.end - 1, lang, tz)} · ${statsDuration(w.seconds, lang)}`;
    function move(e, i) {
        const index = e.key === 'ArrowRight' ? Math.min(i + 1, weeks.length - 1) : e.key === 'ArrowLeft' ? Math.max(0, i - 1) : e.key === 'Home' ? 0 : e.key === 'End' ? weeks.length - 1 : -1;
        if (index < 0) return;
        e.preventDefault(); e.currentTarget.parentElement.children[index].focus();
    }
</script>

<div class="weekly-chart" data-week-chart={weeks.length}>
    <div class="overflow-x-auto pt-1 pb-1">
        <div class:year={weeks.length > 8}>
            <div class="bars" role="group" aria-label={label}>
                {#each weeks as week, i}
                    <button type="button" class="bar-slot" aria-label={description(week, $langStore, zone)}
                            data-week-seconds={week.seconds}
                            on:mouseenter={() => picked = week} on:mouseleave={() => picked = null}
                            on:focus={() => picked = week} on:blur={() => picked = null} on:keydown={(e) => move(e, i)}>
                        <span class="bar" class:empty={!week.seconds} style="height:{week.seconds ? Math.max(2, week.seconds / maximum * 100) : 2}%"></span>
                    </button>
                {/each}
            </div>
            <div class="months" aria-hidden="true">
                {#each weeks as week, i}
                    <span><span class="month-label">{i === 0 || (weeks.length <= 8 ? i === weeks.length - 1 : statsMonth(week.start, $langStore, zone) !== statsMonth(weeks[i - 1].start, $langStore, zone)) ? statsMonth(week.start, $langStore, zone) : ''}</span></span>
                {/each}
            </div>
        </div>
    </div>
    <p data-week-tooltip class="text-xs text-slate-400 mt-2 min-h-[2rem]" aria-live="polite">{picked ? description(picked, $langStore, zone) : $t('stats.chart_hint')}</p>
</div>

<style>
    .bars { display: grid; grid-auto-flow: column; grid-auto-columns: minmax(0, 1fr); gap: 5px; height: 132px; border-bottom: 1px solid rgba(255,255,255,.12); }
    .year { min-width: 560px; }
    .bar-slot { height: 100%; display: flex; align-items: flex-end; justify-content: center; min-width: 0; border-radius: 3px 3px 0 0; }
    .bar-slot:focus-visible { outline: 2px solid #c4b5fd; outline-offset: 2px; }
    .bar { width: 100%; max-width: 48px; background: #8b7cc8; border-radius: 3px 3px 0 0; }
    .bar.empty { background: #4c4b56; }
    .bar-slot:hover .bar, .bar-slot:focus .bar { background: #b3a5e1; }
    .months { display: grid; grid-auto-flow: column; grid-auto-columns: minmax(0, 1fr); gap: 5px; margin-top: 8px; font-size: 10px; color: #91909b; }
    .months > span { position: relative; height: 15px; }
    .month-label { position: absolute; left: 0; white-space: nowrap; }
    .months > span:last-child .month-label { left: auto; right: 0; }
</style>
