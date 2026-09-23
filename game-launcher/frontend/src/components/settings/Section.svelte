<script>
    // Блок настроек: заголовок, необязательная подсказка и содержимое.
    export let title = '';
    export let hint = '';
</script>

<section class="mb-7 last:mb-0">
    <h3 class="text-xs font-bold text-slate-400 uppercase tracking-wider mb-1">{title}</h3>
    {#if hint}<p class="text-xs text-slate-500 mb-3">{hint}</p>{:else}<div class="mb-2"></div>{/if}
    <slot/>
</section>
