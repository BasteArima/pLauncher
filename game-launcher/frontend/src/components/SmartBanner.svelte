<script>
    // Плашка над умным списком (обновления, без обложки, папка не найдена):
    // что это за игры и одно действие сразу для всех.
    export let tone = 'info';          // info | update | danger
    export let title = '';
    export let text = '';
    export let action = '';            // подпись кнопки; пусто — без кнопки
    export let busy = false;
    export let onAction = () => {};

    const TONES = {
        info: 'bg-indigo-950 border-indigo-800 text-indigo-100',
        update: 'bg-amber-950/60 border-amber-700/50 text-amber-100',
        danger: 'bg-rose-950/50 border-rose-800/50 text-rose-100',
    };
    const BTN = {
        info: 'bg-slate-100 hover:bg-white text-slate-950',
        update: 'bg-amber-400 hover:bg-amber-300 text-amber-950',
        danger: 'bg-rose-300 hover:bg-rose-200 text-rose-950',
    };
</script>

<div data-smart-banner class="flex items-center gap-4 flex-wrap rounded-xl border px-5 py-4 mb-6 animate-fade-in {TONES[tone]}">
    <div class="flex-1 min-w-[260px]">
        <div class="font-semibold">{title}</div>
        {#if text}<div class="text-sm opacity-80 mt-0.5">{text}</div>{/if}
    </div>
    {#if action}
        <button on:click={onAction} disabled={busy}
                class="h-10 px-4 rounded-lg text-sm font-semibold transition-colors disabled:opacity-60 {BTN[tone]}">{action}</button>
    {/if}
</div>
