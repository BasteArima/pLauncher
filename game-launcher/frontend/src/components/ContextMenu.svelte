<script>
    // Кастомное контекстное меню (нативное отключено).
    // items: [{label, icon?, danger?, action}] | {sep:true} | {label, icon?, submenu:[{label, checked?, action}]}
    export let x = 0;
    export let y = 0;
    export let items = [];
    export let onClose = () => {};

    function run(action) { action(); onClose(); }
</script>

<div class="fixed inset-0 z-[99]" on:click={onClose} on:contextmenu|preventDefault={onClose}></div>
<div class="fixed z-[100] w-[232px] glass-strong rounded-xl shadow-2xl py-1.5 text-sm animate-fade-in" style="left:{x}px; top:{y}px">
    {#each items as it}
        {#if it.sep}
            <div class="h-px bg-white/10 my-1.5 mx-2"></div>
        {:else if it.submenu}
            <div class="relative group/sub">
                <button class="w-full flex items-center gap-2.5 px-3 py-1.5 text-left text-slate-200 hover:bg-white/10 transition-colors">
                    {#if it.icon}<span class="w-4 text-center text-xs">{it.icon}</span>{/if}
                    <span class="flex-1">{it.label}</span>
                    <span class="text-slate-500">›</span>
                </button>
                <div class="absolute left-full top-0 -ml-1 w-[210px] glass-strong rounded-xl shadow-2xl py-1.5 hidden group-hover/sub:block max-h-72 overflow-y-auto">
                    {#each it.submenu as sub}
                        <button on:click={() => run(sub.action)}
                                class="w-full flex items-center gap-2 px-3 py-1.5 text-left text-slate-200 hover:bg-white/10 transition-colors">
                            <span class="w-4 text-center text-xs text-emerald-400">{sub.checked ? '✓' : ''}</span>
                            <span class="flex-1 truncate">{sub.label}</span>
                        </button>
                    {/each}
                </div>
            </div>
        {:else}
            <button on:click={() => run(it.action)}
                    class="w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors {it.danger ? 'text-rose-300 hover:bg-rose-500/20' : 'text-slate-200 hover:bg-white/10'}">
                {#if it.icon}<span class="w-4 text-center text-xs">{it.icon}</span>{/if}
                <span class="flex-1">{it.label}</span>
            </button>
        {/if}
    {/each}
</div>
