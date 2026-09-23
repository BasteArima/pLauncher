<script>
    // Кастомное контекстное меню (нативное отключено).
    // items: [{label, icon?, hint?, danger?, action}] | {sep:true} | {label, icon?, submenu:[{label, checked?, action}]}
    export let x = 0;
    export let y = 0;
    export let items = [];
    export let width = 232;
    export let onClose = () => {};

    const SUB_W = 210;
    // Подменю рисуется соседним элементом, а не внутри меню: вложенный backdrop-filter
    // не размывает то, что за границами родителя, и стекло выглядело бы иначе.
    let sub = null;                         // {idx, items, x, y}
    let closeTimer;

    function run(action) { action(); onClose(); }
    function openSub(e, idx, subItems) {
        clearTimeout(closeTimer);
        const r = e.currentTarget.getBoundingClientRect();
        let sx = r.right - 2;
        if (sx + SUB_W > window.innerWidth - 8) sx = r.left - SUB_W + 2;   // не влезает справа — слева
        const h = Math.min(subItems.length * 32 + 12, 288);                // max-h-72
        const sy = Math.max(8, Math.min(r.top - 6, window.innerHeight - h - 8));
        sub = { idx, items: subItems, x: sx, y: sy };
    }
    function leaveSub() { closeTimer = setTimeout(() => sub = null, 180); }
    function keepSub() { clearTimeout(closeTimer); }
    function closeSub() { clearTimeout(closeTimer); sub = null; }
</script>

<div class="fixed inset-0 z-[99]" on:click={onClose} on:contextmenu|preventDefault={onClose}></div>
<div class="fixed z-[100] glass-strong rounded-xl shadow-2xl py-1.5 text-sm animate-fade-in" style="left:{x}px; top:{y}px; width:{width}px">
    {#each items as it, i}
        {#if it.sep}
            <div class="h-px bg-white/10 my-1.5 mx-2"></div>
        {:else if it.submenu}
            <button on:mouseenter={(e) => openSub(e, i, it.submenu)} on:mouseleave={leaveSub}
                    on:click={(e) => openSub(e, i, it.submenu)}
                    class="w-full flex items-center gap-2.5 px-3 py-1.5 text-left text-slate-200 transition-colors {sub && sub.idx === i ? 'bg-white/10' : 'hover:bg-white/10'}">
                {#if it.icon}<span class="w-4 text-center text-xs">{it.icon}</span>{/if}
                <span class="flex-1">{it.label}</span>
                <span class="text-slate-500">›</span>
            </button>
        {:else}
            <button on:click={() => run(it.action)} on:mouseenter={closeSub}
                    class="w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors {it.danger ? 'text-rose-300 hover:bg-rose-500/20' : 'text-slate-200 hover:bg-white/10'}">
                {#if it.icon}<span class="w-4 text-center text-xs">{it.icon}</span>{/if}
                <span class="flex-1">{it.label}</span>
                {#if it.hint}<span class="text-[11px] text-slate-500 shrink-0">{it.hint}</span>{/if}
            </button>
        {/if}
    {/each}
</div>
{#if sub}
    <div class="fixed z-[101] glass-strong rounded-xl shadow-2xl py-1.5 text-sm max-h-72 overflow-y-auto"
         style="left:{sub.x}px; top:{sub.y}px; width:{SUB_W}px"
         on:mouseenter={keepSub} on:mouseleave={leaveSub}>
        {#each sub.items as s}
            <button on:click={() => run(s.action)}
                    class="w-full flex items-center gap-2 px-3 py-1.5 text-left text-slate-200 hover:bg-white/10 transition-colors">
                <span class="w-4 text-center text-xs text-emerald-400">{s.checked ? '✓' : ''}</span>
                <span class="flex-1 truncate">{s.label}</span>
            </button>
        {/each}
    </div>
{/if}
