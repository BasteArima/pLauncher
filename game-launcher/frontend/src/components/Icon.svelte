<script>
    // Линейная иконка из набора lib/icons.js. Цвет — от родителя (currentColor) или через class.
    import { iconSvg } from '../lib/icons.js';
    export let name = '';
    export let size = 16;
    export let fill = false;
    let cls = '';
    export { cls as class };
    $: html = iconSvg(name, size, fill);
</script>

{#if html}<span class="inline-flex items-center justify-center shrink-0 {cls}" aria-hidden="true">{@html html}</span>{/if}
