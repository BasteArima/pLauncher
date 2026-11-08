<script>
    import { toast } from '../lib/ui.js';
    import { t } from '../i18n.js';
    import Icon from './Icon.svelte';
</script>

{#if $toast.show}
    <div class="fixed bottom-8 right-8 max-w-[calc(100vw-4rem)] p-4 rounded-lg shadow-2xl flex flex-wrap items-center gap-3 z-[90] animate-fade-in transition-all {$toast.type === 'error' ? 'bg-red-900 border border-red-500 text-red-100' : 'bg-slate-800 border-l-4 border-emerald-500 text-slate-200'}">
        {#if $toast.type === 'error'}
            <Icon name="alert" size={20} class="text-red-300"/>
        {:else}
            <Icon name="check" size={20} class="text-emerald-400"/>
        {/if}
        <span class="font-semibold tracking-wide">{$toast.message}</span>
        {#if $toast.action}<button data-toast-action on:click={$toast.action.run} class="text-sm underline underline-offset-4">{$t($toast.action.key, $toast.action.params)}</button>{/if}
    </div>
{/if}
