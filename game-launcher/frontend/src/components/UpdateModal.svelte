<script>
    // Окно обновления лаунчера: что нового → загрузка с прогрессом → перезапуск.
    // Загрузка не зависит от окна: его можно закрыть, ход виден на плашке в заголовке.
    import { t, tr, localizeError } from '../i18n.js';
    import { fmtSize } from '../lib/util.js';
    import { renderNotes } from '../lib/markdown.js';
    import { askConfirm } from '../lib/ui.js';
    import {
        updateInfo, download, updateModalOpen, autoRestart, restartIn, restartBlocker, restarting,
        startDownload, cancelDownload, restartNow, currentBlocker, postponeRestart,
    } from '../lib/update.js';
    import { OpenLauncherReleasePage } from '../../wailsjs/go/main/App.js';

    let actionError = '';

    $: info = $updateInfo;
    $: dl = $download;
    $: state = dl.state || 'idle';
    $: releases = (info && info.releases) || [];
    $: percent = dl.total > 0 ? Math.min(100, Math.round(dl.done / dl.total * 100)) : 0;
    $: shownError = actionError || (state === 'error' ? localizeError(dl.error) : '');
    $: blockerText = $restartBlocker === 'game' ? $t('upd.wait_game')
        : $restartBlocker === 'editing' ? $t('upd.wait_editing') : '';

    function close() { updateModalOpen.set(false); }

    async function update() {
        actionError = '';
        try { await startDownload(); }
        catch (err) { actionError = localizeError(err); }
    }

    async function restart() {
        actionError = '';
        // Игра продолжит работать, но её время не запишется: лаунчер, который его считал, закроется
        if (await currentBlocker() === 'game') {
            const ok = await askConfirm({ title: tr('upd.confirm_game_title'), message: tr('upd.confirm_game_msg'), confirmText: tr('upd.restart_now') });
            if (!ok) return;
        }
        try { await restartNow(); }
        catch (err) { actionError = localizeError(err); }
    }

    const btn = 'text-sm px-4 py-2 rounded-lg transition-colors';
    const secondary = btn + ' text-slate-300 hover:text-white bg-white/5 hover:bg-white/10 border border-white/10';
    const primary = btn + ' font-bold text-white bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50';
</script>

<div class="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/80 backdrop-blur-md animate-fade-in" on:click={close}>
    <div class="w-[520px] max-w-[92vw] max-h-[84vh] flex flex-col glass-strong rounded-2xl shadow-2xl p-6" on:click|stopPropagation>
        {#if state === 'ready'}
            <h2 class="text-xl font-bold text-white mb-1">{$t('upd.ready_title', { v: dl.version })}</h2>
            <p class="text-sm text-slate-400 mb-4">{$t('upd.ready_sub')}</p>
            {#if $restarting}
                <p class="text-sm text-indigo-200 mb-4">{$t('upd.restarting')}</p>
            {:else if $restartIn > 0}
                <p class="text-sm text-indigo-200 mb-4">{$t('upd.restarting_in', { s: $restartIn })}</p>
            {:else if $autoRestart && blockerText}
                <p class="text-sm text-amber-200 mb-4">{blockerText}</p>
            {/if}
            <label class="flex items-center gap-2 text-sm text-slate-300 mb-4 cursor-pointer">
                <input type="checkbox" bind:checked={$autoRestart} class="accent-indigo-500"/> {$t('upd.auto_restart')}
            </label>
            {#if shownError}<p class="text-xs text-rose-300 break-words mb-3">{shownError}</p>{/if}
            <div class="flex justify-end gap-2">
                <button on:click={() => { postponeRestart(); close(); }} disabled={$restarting} class={secondary}>{$t('upd.next_launch')}</button>
                <button on:click={restart} disabled={$restarting} class={primary}>{$t('upd.restart_now')}</button>
            </div>

        {:else if state === 'downloading'}
            <h2 class="text-xl font-bold text-white mb-3">{$t('upd.downloading', { v: dl.version })}</h2>
            <div class="h-2 rounded-full bg-white/10 overflow-hidden mb-2">
                <div class="h-full bg-indigo-400 transition-all duration-150" style="width:{percent}%"></div>
            </div>
            <div class="flex justify-between text-xs text-slate-400 mb-4">
                <span>{$t('upd.progress', { done: fmtSize(dl.done) || '0', total: fmtSize(dl.total) || '?' })}</span>
                {#if dl.speed > 0}<span>{$t('upd.speed', { speed: fmtSize(dl.speed) })}</span>{/if}
            </div>
            <label class="flex items-center gap-2 text-sm text-slate-300 mb-4 cursor-pointer">
                <input type="checkbox" bind:checked={$autoRestart} class="accent-indigo-500"/> {$t('upd.auto_restart')}
            </label>
            <div class="flex justify-end gap-2">
                <button on:click={cancelDownload} class={secondary}>{$t('btn.cancel')}</button>
                <button on:click={close} class={secondary}>{$t('upd.minimize')}</button>
            </div>

        {:else if info}
            <h2 class="text-xl font-bold text-white mb-1">{$t('upd.modal_title', { v: info.latest })}</h2>
            <p class="text-sm text-slate-400 mb-4">
                {$t('upd.current', { v: info.current })}{#if info.size > 0}{' · '}{fmtSize(info.size)}{/if}
            </p>
            {#if releases.length}
                <div class="flex-1 min-h-0 overflow-y-auto text-sm text-slate-300 space-y-2 mb-3 pr-1 select-text">
                    {#each releases as r}
                        {#if releases.length > 1}<p class="font-bold text-white">{r.version}</p>{/if}
                        {@html renderNotes(r.notes)}
                    {/each}
                </div>
            {/if}
            <button on:click={() => OpenLauncherReleasePage()} class="self-start text-xs text-indigo-300 hover:text-white mb-4">{$t('upd.all_changes')} ↗</button>

            {#if info.can_install}
                <label class="flex items-center gap-2 text-sm text-slate-300 mb-4 cursor-pointer">
                    <input type="checkbox" bind:checked={$autoRestart} class="accent-indigo-500"/> {$t('upd.auto_restart')}
                </label>
                {#if shownError}<p class="text-xs text-rose-300 break-words mb-3">{shownError}</p>{/if}
                <div class="flex justify-end gap-2">
                    <button on:click={close} class={secondary}>{$t('upd.later')}</button>
                    <button on:click={update} class={primary}>{shownError ? $t('upd.retry') : $t('upd.update')}</button>
                </div>
            {:else}
                <p class="text-xs text-amber-200 mb-4">{info.manual_reason === 'readonly' ? $t('upd.manual_readonly') : $t('upd.manual_os')}</p>
                <div class="flex justify-end gap-2">
                    <button on:click={close} class={secondary}>{$t('upd.later')}</button>
                    <button on:click={() => OpenLauncherReleasePage()} class={primary}>{$t('upd.download')} ↗</button>
                </div>
            {/if}
        {/if}
    </div>
</div>
