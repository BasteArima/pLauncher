<script>
    // Полноэкранный просмотр скриншотов. Сам обрабатывает клавиши (Esc, ←/→)
    // и боковые кнопки мыши; App на это время свои обработчики пропускает.
    import { mediaSrc } from '../lib/util.js';

    export let images = [];
    export let index = 0;
    export let onClose = () => {};

    function next(e) { if (e) e.stopPropagation(); index = (index + 1) % images.length; }
    function prev(e) { if (e) e.stopPropagation(); index = (index - 1 + images.length) % images.length; }

    function onKey(e) {
        if (e.key === 'Escape') { e.preventDefault(); onClose(); }
        else if (e.key === 'ArrowRight') { e.preventDefault(); next(); }
        else if (e.key === 'ArrowLeft') { e.preventDefault(); prev(); }
    }
    // Боковые кнопки мыши: 3 — «Назад» (закрыть), 4 — «Вперёд» (следующий)
    function onMouse(e) {
        if (e.button === 3) { e.preventDefault(); onClose(); }
        else if (e.button === 4) { e.preventDefault(); next(); }
    }
</script>

<svelte:window on:keydown={onKey} on:mousedown={onMouse}/>

<div class="fixed inset-0 bg-slate-900/95 backdrop-blur-md z-[95] flex items-center justify-center p-10 animate-fade-in" on:click={onClose}>
    <button class="absolute top-6 right-8 text-slate-400 hover:text-white text-5xl font-light transition-colors z-50" on:click={onClose}>&times;</button>

    <div class="absolute left-0 top-0 bottom-0 w-1/4 flex items-center justify-start pl-8 group cursor-pointer" on:click={prev}>
        <div class="text-white/30 group-hover:text-white text-7xl transition-colors drop-shadow-2xl">&#10094;</div>
    </div>

    <img src={mediaSrc(images[index])} alt="fullscreen"
         class="max-w-[80vw] max-h-[80vh] object-contain rounded-lg shadow-2xl border border-slate-700"
         on:click|stopPropagation/>

    <div class="absolute right-0 top-0 bottom-0 w-1/4 flex items-center justify-end pr-8 group cursor-pointer" on:click={next}>
        <div class="text-white/30 group-hover:text-white text-7xl transition-colors drop-shadow-2xl">&#10095;</div>
    </div>

    <div class="absolute bottom-6 text-slate-400 font-semibold tracking-widest bg-slate-900/80 px-4 py-1 rounded-full">
        {index + 1} / {images.length}
    </div>
</div>
