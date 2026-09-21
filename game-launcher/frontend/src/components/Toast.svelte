<script>
    import { toast } from '../lib/ui.js';
</script>

{#if $toast.show}
    <div class="fixed bottom-8 right-8 p-4 rounded-lg shadow-2xl flex items-center gap-3 z-[90] animate-fade-in transition-all {$toast.type === 'error' ? 'bg-red-900 border border-red-500 text-red-100' : 'bg-slate-800 border-l-4 border-emerald-500 text-slate-200'}">
        {#if $toast.type === 'error'}
            <span class="text-red-400 text-xl">⚠</span>
        {:else}
            <span class="text-emerald-400 text-xl">✓</span>
        {/if}
        <span class="font-semibold tracking-wide">{$toast.message}</span>
    </div>
{/if}
