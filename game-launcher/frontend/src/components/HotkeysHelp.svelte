<script>
    // Окно-справка по горячим клавишам (F1).
    import { t } from '../i18n.js';
    import HotkeyList from './HotkeyList.svelte';
    export let panicLabel = '';
    export let onClose = () => {};
</script>

<div class="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={onClose}>
    <div class="w-[680px] max-w-[92vw] max-h-[85vh] overflow-y-auto glass-strong rounded-2xl shadow-2xl p-7" on:click|stopPropagation>
        <div class="flex items-center justify-between mb-5">
            <h2 class="text-2xl font-black text-white">⌨ {$t('hk.title')}</h2>
            <button on:click={onClose} class="text-slate-400 hover:text-white text-3xl leading-none">&times;</button>
        </div>
        <HotkeyList {panicLabel}/>
    </div>
</div>
