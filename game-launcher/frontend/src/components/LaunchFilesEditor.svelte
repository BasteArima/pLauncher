<script>
    import { onDestroy, tick } from 'svelte';
    import { t, tr } from '../i18n.js';
    import { SelectLaunchFile, FindLaunchFileCandidates, GetLaunchFileLabel } from '../../wailsjs/go/main/App.js';
    import { fileKey, fileName, fileLabel, launchFiles, freeSlots } from '../lib/launchFiles.js';
    import Icon from './Icon.svelte';

    export let game;
    let alive = true, generation = 0, labelGeneration = 0;
    let keys = {}, error = '', picking = false, discovering = false, picker = false;
    let candidates = [], checked = [], incomplete = false, panel, findButton;
    $: files = launchFiles(game);
    $: remaining = freeSlots(game);
    $: signature = JSON.stringify(files.map(f => f.path));
    $: loadLabels(signature);
    onDestroy(() => { alive = false; generation++; labelGeneration++; });

    async function loadLabels(signature) {
        const ticket = ++labelGeneration;
        try {
            const entries = await Promise.all(JSON.parse(signature).map(async path => [fileKey(path), await GetLaunchFileLabel(path)]));
            if (alive && ticket === labelGeneration) keys = Object.fromEntries(entries);
        } catch (err) { /* Filename remains a usable fallback; discovery errors are shown separately. */ }
    }
    function setLabel(file, value) {
        if (file.primary) game.exec_label = value;
        else game.launch_files = game.launch_files.map((f, i) => i === file.index ? { ...f, label: value } : f);
    }
    function promote(file) {
        if (file.primary) return;
        const extras = [...(game.launch_files || [])];
        if (game.exec_path) extras[file.index] = { path: game.exec_path, label: game.exec_label || '' };
        else extras.splice(file.index, 1);
        game = { ...game, exec_path: file.path, exec_label: file.label || '', launch_files: extras };
    }
    function remove(file) {
        if (file.primary) game = { ...game, exec_path: '', exec_label: '' };
        else game.launch_files = game.launch_files.filter((_, i) => i !== file.index);
        error = '';
    }
    function listed(path, rows = files) { return rows.some(f => fileKey(f.path) === fileKey(path)); }
    function append(paths) {
        if (paths.some(path => listed(path))) { error = tr('launch.already_added'); return false; }
        if (paths.length > freeSlots(game)) { error = tr('err.launch.limit', { n: 8 }); return false; }
        const next = { ...game, launch_files: [...(game.launch_files || [])] };
        for (const path of paths) {
            if (!next.exec_path) { next.exec_path = path; next.exec_label = ''; }
            else next.launch_files.push({ path, label: '' });
        }
        game = next;
        error = '';
        return true;
    }
    async function addFile() {
        if (picking || !remaining) return;
        const ticket = ++generation;
        picking = true; error = '';
        try {
            const path = await SelectLaunchFile(game.id);
            if (alive && ticket === generation && path) append([path]);
        } catch (err) { if (alive && ticket === generation) error = tr('toast.error', { err }); }
        finally { if (alive && ticket === generation) picking = false; }
    }
    async function discover() {
        const ticket = ++generation;
        picker = true; discovering = true; error = ''; candidates = []; checked = []; incomplete = false;
        await tick(); panel?.focus();
        try {
            const result = await FindLaunchFileCandidates(game.id);
            if (!alive || ticket !== generation || !result) return;
            const seen = new Set();
            candidates = (result.files || []).filter(f => {
                const key = fileKey(f.path);
                if (seen.has(key)) return false;
                seen.add(key); return true;
            });
            incomplete = result.incomplete;
        } catch (err) { if (alive && ticket === generation) error = tr('toast.error', { err }); }
        finally { if (alive && ticket === generation) discovering = false; }
    }
    function closePicker() { generation++; picker = false; discovering = false; checked = []; error = ''; findButton?.focus(); }
    function addChecked() { if (append(checked)) closePicker(); }
    function pickerKeys(e) { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closePicker(); } }
    const button = 'h-8 px-2.5 rounded-lg border border-white/10 text-xs text-slate-300 hover:text-white hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1.5';
</script>

<section id="launch-files-editor" data-launch-editor class="min-w-0 flex flex-col gap-2.5 scroll-mt-20">
    <h3 class="text-xs font-semibold text-slate-500 uppercase tracking-wider">{$t('launch.files')}</h3>
    <p class="text-xs text-slate-500">{$t('launch.tracking_hint')}</p>
    {#if !game.exec_path}<p data-no-primary class="text-xs text-amber-200">{$t('launch.no_primary')}</p>{/if}
    <div class="flex flex-col gap-2">
        {#each files as file (file.path)}
            <div data-launch-edit-row data-path={file.path} class="rounded-lg border border-white/10 bg-slate-950 p-2 flex gap-2 items-start min-w-0">
                <button data-make-primary on:click={() => promote(file)} aria-pressed={file.primary}
                        title={$t(file.primary ? 'launch.primary' : 'launch.make_primary')} aria-label={$t(file.primary ? 'launch.primary' : 'launch.make_primary') + ': ' + fileName(file.path)}
                        class="w-6 h-7 shrink-0 flex items-center justify-center {file.primary ? 'text-indigo-300' : 'text-slate-600 hover:text-slate-300'}"><Icon name="star" size={16} fill={file.primary}/></button>
                <div class="flex-1 min-w-0 flex flex-col gap-1.5">
                    <span title={file.path} class="text-xs font-mono text-slate-300 truncate">{fileName(file.path)}</span>
                    <span class="text-[10px] text-slate-500 truncate" title={file.path}>{file.path}</span>
                    <input data-launch-label value={file.label || ''} on:input={e => setLabel(file, e.target.value)}
                           aria-label={$t('launch.label') + ': ' + fileName(file.path)} placeholder={fileLabel({ ...file, label: '' }, keys[fileKey(file.path)], $t)}
                           class="w-full min-w-0 h-7 px-2 rounded border border-white/10 bg-slate-900 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-400 select-text"/>
                </div>
                <button data-remove-file on:click={() => remove(file)} title={$t('btn.remove')} aria-label={$t('btn.remove') + ': ' + fileName(file.path)}
                        class="w-6 h-7 shrink-0 flex items-center justify-center text-slate-500 hover:text-rose-300"><Icon name="x" size={14}/></button>
            </div>
        {/each}
    </div>
    <p class="text-[11px] text-slate-500">{$t('launch.label_hint')}</p>
    <div class="flex flex-wrap gap-2">
        <button data-launch-add on:click={addFile} disabled={picking || picker || !remaining} class={button}><Icon name="plus" size={14}/>{$t('launch.add')}</button>
        <button data-launch-find bind:this={findButton} on:click={discover} disabled={picking || picker || !remaining} class={button}><Icon name="search" size={14}/>{$t('launch.find')}</button>
    </div>
    {#if remaining === 0}<p data-launch-limit class="text-xs text-amber-200">{$t('err.launch.limit', { n: 8 })}</p>{/if}
    {#if !picker && error}<p role="alert" class="text-xs text-rose-300">{error}</p>{/if}
    {#if picker}
        <div data-launch-candidates bind:this={panel} tabindex="-1" on:keydown={pickerKeys} class="rounded-lg bg-slate-950 border border-white/10 p-3 flex flex-col gap-2.5 focus:outline-none">
            <h4 class="text-xs font-semibold text-slate-200">{$t('launch.find')}</h4>
            {#if discovering}<p role="status" class="text-xs text-slate-400">{$t('launch.searching')}</p>
            {:else if error}
                <p role="alert" class="text-xs text-rose-300">{error}</p>
                <button data-launch-retry on:click={discover} class={button}>{$t('launch.retry')}</button>
            {:else}
                {#if incomplete}<p data-launch-incomplete class="text-xs text-amber-200">{$t('launch.incomplete')}</p>{/if}
                {#if !candidates.length}<p data-launch-empty class="text-xs text-slate-500">{$t('launch.empty')}</p>{/if}
                <div class="max-h-64 overflow-y-auto flex flex-col gap-1.5">
                    {#each candidates as file (file.path)}
                        {@const exists = listed(file.path, files)}
                        <label class="flex gap-2 items-start p-1 min-w-0 {exists ? 'opacity-50' : ''}" title={file.path}>
                            <input type="checkbox" value={file.path} bind:group={checked} disabled={exists || (!checked.includes(file.path) && checked.length >= remaining)} class="mt-1 accent-indigo-500"/>
                            <span class="flex-1 min-w-0 text-xs text-slate-200">
                                <span class="block truncate">{fileLabel(file, file.label_key, $t)}</span>
                                <span class="block truncate text-[10px] text-slate-500">{file.relative_path || fileName(file.path)}</span>
                                {#if exists}<span class="text-[10px] text-slate-500">{$t('launch.already_added')}</span>{/if}
                            </span>
                        </label>
                    {/each}
                </div>
                <p data-launch-selected class="text-xs text-slate-500" aria-live="polite">{$t('launch.selected', { n: checked.length, remaining })}</p>
            {/if}
            <div class="flex gap-2 flex-wrap">
                <button data-launch-add-selected on:click={addChecked} disabled={!checked.length || discovering || !!error || checked.length > remaining} class="{button} bg-indigo-500 !text-white">{$t('launch.add_selected')}</button>
                <button data-launch-cancel on:click={closePicker} class={button}>{$t('btn.cancel')}</button>
            </div>
        </div>
    {/if}
</section>
