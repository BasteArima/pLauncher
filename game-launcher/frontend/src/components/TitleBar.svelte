<script>
    // Свой титлбар безрамочного окна (перетаскивание через --wails-draggable).
    import { t } from '../i18n.js';
    import { WindowMinimise, WindowToggleMaximise, Quit } from '../../wailsjs/runtime/runtime';
</script>

<div class="h-9 shrink-0 flex items-center justify-between glass-strong border-b border-white/5 select-none z-30" style="--wails-draggable:drag" on:dblclick={() => WindowToggleMaximise()}>
    <div class="flex items-center gap-2 px-3 text-slate-400 text-xs font-semibold tracking-wide">
        <span class="text-indigo-400">◆</span> pLauncher
    </div>
    <div class="flex items-center h-full" style="--wails-draggable:no-drag">
        <button on:click={() => WindowMinimise()} class="h-full px-4 hover:bg-white/10 text-slate-400 hover:text-white transition-colors" title={$t("titlebar.min")} aria-label={$t("titlebar.min")}>
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12h14"/></svg>
        </button>
        <button on:click={() => WindowToggleMaximise()} class="h-full px-4 hover:bg-white/10 text-slate-400 hover:text-white transition-colors" title={$t("titlebar.max")} aria-label={$t("titlebar.max")}>
            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="1"/></svg>
        </button>
        <button on:click={() => Quit()} class="h-full px-4 hover:bg-red-600 text-slate-400 hover:text-white transition-colors" title={$t("titlebar.close")} aria-label={$t("titlebar.close")}>
            <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 6l12 12M18 6L6 18"/></svg>
        </button>
    </div>
</div>
