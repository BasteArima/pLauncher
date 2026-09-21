<script>
    import { t } from '../i18n.js';
    import { confirmState, closeConfirm } from '../lib/ui.js';

    // Enter/Space — подтверждение, Escape — отмена. Остальные обработчики клавиш
    // в App пропускают ввод, пока диалог открыт (isConfirmOpen).
    function onKey(e) {
        if (!$confirmState.show) return;
        if (e.key === 'Escape') { e.preventDefault(); closeConfirm(false); }
        else if (e.key === 'Enter' || e.key === ' ' || e.code === 'Space') { e.preventDefault(); closeConfirm(true); }
    }
</script>

<svelte:window on:keydown={onKey}/>

{#if $confirmState.show}
    <div class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={() => closeConfirm(false)}>
        <div class="w-[440px] max-w-[90vw] glass-strong rounded-2xl shadow-2xl p-6" on:click|stopPropagation>
            <h3 class="text-xl font-black text-white mb-2">{$confirmState.title}</h3>
            {#if $confirmState.message}
                <p class="text-slate-400 text-sm whitespace-pre-line mb-6">{$confirmState.message}</p>
            {:else}
                <div class="mb-6"></div>
            {/if}
            <div class="flex gap-3 justify-end">
                <button on:click={() => closeConfirm(false)} class="px-5 py-2.5 rounded-lg font-semibold text-slate-300 bg-slate-700/60 hover:bg-slate-600 border border-slate-600/50 transition-colors">
                    {$t("btn.cancel")}
                </button>
                <button on:click={() => closeConfirm(true)} class="px-5 py-2.5 rounded-lg font-bold text-white transition-colors {$confirmState.danger ? 'bg-red-600 hover:bg-red-500' : 'bg-indigo-600 hover:bg-indigo-500'}">
                    {$confirmState.confirmText}
                </button>
            </div>
        </div>
    </div>
{/if}
