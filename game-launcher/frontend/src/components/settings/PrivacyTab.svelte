<script>
    // Приватность: размытие обложек, PIN-код и автоблокировка, кнопка паники,
    // скрытые коллекции. Всё сохраняется сразу при изменении.
    import { t, tr } from '../../i18n.js';
    import { showToast } from '../../lib/ui.js';
    import { blurMode, pinError } from '../../lib/privacy.js';
    import { GetPrivacy, SavePrivacy, SetPin } from '../../../wailsjs/go/main/App.js';
    import Section from './Section.svelte';
    import HotkeyInput from '../HotkeyInput.svelte';
    import { btn, input, choice } from './styles.js';

    export let privacy;                        // результат GetPrivacy (bind)
    export let hiddenCount = 0;                // скрытых коллекций
    export let showHidden = false;
    export let onToggleHidden = () => {};
    export let onLockNow = () => {};

    async function save(patch) {
        try {
            await SavePrivacy({ ...privacy, ...patch });
        } catch (err) {
            showToast(tr('privacy.save_fail', { err }), 'error');
        }
        privacy = await GetPrivacy();
        blurMode.set(privacy.blur_mode);
    }

    // --- PIN ---
    let pinMode = '';                          // '' | set | change | remove
    let pinOld = '', pinNew = '', pinRepeat = '';
    let pinBusy = false;
    function openPin(mode) { pinMode = mode; pinOld = pinNew = pinRepeat = ''; }
    async function submitPin() {
        if (pinMode !== 'remove' && pinNew !== pinRepeat) { showToast(tr('privacy.pin_mismatch'), 'error'); return; }
        pinBusy = true;
        try {
            await SetPin(pinOld, pinMode === 'remove' ? '' : pinNew);
            showToast(pinMode === 'remove' ? tr('privacy.pin_removed') : tr('privacy.pin_saved'), 'success');
            pinMode = '';
            privacy = await GetPrivacy();
        } catch (err) {
            showToast(pinError(err), 'error');
        } finally { pinBusy = false; }
    }

    const blurOptions = [['none', 'privacy.blur_none'], ['hover', 'privacy.blur_hover'], ['always', 'privacy.blur_always']];
    const idleOptions = [0, 5, 15, 30, 60];
</script>

<Section title={$t('privacy.covers')} hint={$t('privacy.covers_hint')}>
    <div class="grid grid-cols-3 gap-2 mb-3">
        {#each blurOptions as [value, label]}
            <button on:click={() => save({ blur_mode: value })} class={choice(privacy.blur_mode === value)}>
                <span class="text-sm font-semibold text-white">{$t(label)}</span>
            </button>
        {/each}
    </div>
    <label class="flex items-center gap-2 text-sm text-slate-300 cursor-pointer">
        <input type="checkbox" checked={privacy.start_discreet} on:change={(e) => save({ start_discreet: e.target.checked })} class="accent-indigo-500 w-4 h-4"/>
        {$t('privacy.start_discreet')}
    </label>
</Section>

<Section title={$t('privacy.pin')} hint={$t('privacy.pin_hint')}>
    <div class="flex items-center gap-2 mb-2">
        <span class="text-sm flex-1 {privacy.has_pin ? 'text-emerald-300' : 'text-slate-400'}">
            {privacy.has_pin ? '🔒 ' + $t('privacy.pin_on') : $t('privacy.pin_off')}
        </span>
        {#if privacy.has_pin}
            <button on:click={onLockNow} class={btn}>{$t('privacy.lock_now')}</button>
            <button on:click={() => openPin('change')} class={btn}>{$t('privacy.pin_change')}</button>
            <button on:click={() => openPin('remove')} class="{btn} text-rose-300">{$t('privacy.pin_remove')}</button>
        {:else}
            <button on:click={() => openPin('set')} class={btn}>{$t('privacy.pin_set')}</button>
        {/if}
    </div>

    {#if pinMode}
        <form on:submit|preventDefault={submitPin} class="glass rounded-xl p-3 mb-3 flex flex-col gap-2">
            {#if pinMode !== 'set'}
                <input type="password" bind:value={pinOld} placeholder={$t('privacy.pin_current')} autocomplete="off" class={input}/>
            {/if}
            {#if pinMode !== 'remove'}
                <input type="password" bind:value={pinNew} placeholder={$t('privacy.pin_new')} autocomplete="off" class={input}/>
                <input type="password" bind:value={pinRepeat} placeholder={$t('privacy.pin_repeat')} autocomplete="off" class={input}/>
            {/if}
            <div class="flex gap-2 justify-end">
                <button type="button" on:click={() => pinMode = ''} class={btn}>{$t('btn.cancel')}</button>
                <button type="submit" disabled={pinBusy} class="text-sm font-bold text-white bg-indigo-600 hover:bg-indigo-500 px-4 py-2 rounded-lg transition-colors disabled:opacity-50">
                    {pinMode === 'remove' ? $t('privacy.pin_remove') : $t('btn.save')}
                </button>
            </div>
        </form>
    {/if}

    <div class="flex items-center gap-3 {privacy.has_pin ? '' : 'opacity-50 pointer-events-none'}">
        <span class="text-sm text-slate-300 flex-1">{$t('privacy.idle_lock')}</span>
        <select value={privacy.idle_lock_min} on:change={(e) => save({ idle_lock_min: +e.target.value })} class={input}>
            {#each idleOptions as m}<option value={m}>{m ? $t('privacy.minutes', { n: m }) : $t('privacy.off')}</option>{/each}
        </select>
    </div>
</Section>

<Section title={$t('privacy.panic')} hint={$t('privacy.panic_hint')}>
    <label class="flex items-center gap-2 text-sm text-slate-200 cursor-pointer mb-3">
        <input type="checkbox" checked={privacy.panic_enabled} on:change={(e) => save({ panic_enabled: e.target.checked })} class="accent-indigo-500 w-4 h-4"/>
        {$t('privacy.panic_enable')}
    </label>
    <div class="flex flex-col gap-3 {privacy.panic_enabled ? '' : 'opacity-50'}">
        <div class="flex items-center gap-3">
            <span class="text-sm text-slate-300 w-36 shrink-0">{$t('privacy.panic_keys')}</span>
            <HotkeyInput label={privacy.panic_label}
                         onChange={(hk) => save({ panic_mods: hk.mods, panic_vk: hk.vk, panic_label: hk.label })}/>
        </div>
        <div class="flex items-center gap-3">
            <span class="text-sm text-slate-300 w-36 shrink-0">{$t('privacy.panic_action')}</span>
            <select value={privacy.panic_action} on:change={(e) => save({ panic_action: e.target.value })} class="flex-1 {input}">
                <option value="hide">{$t('privacy.panic_hide')}</option>
                <option value="minimize">{$t('privacy.panic_minimize')}</option>
            </select>
        </div>
        <label class="flex items-center gap-2 text-sm cursor-pointer {privacy.has_pin ? 'text-slate-300' : 'text-slate-500'}">
            <input type="checkbox" checked={privacy.lock_on_panic} disabled={!privacy.has_pin}
                   on:change={(e) => save({ lock_on_panic: e.target.checked })} class="accent-indigo-500 w-4 h-4"/>
            {$t('privacy.panic_lock')}{#if !privacy.has_pin}<span class="text-xs">&nbsp;({$t('privacy.needs_pin')})</span>{/if}
        </label>
    </div>
</Section>

<Section title={$t('privacy.hidden')} hint={$t('privacy.hidden_hint')}>
    <div class="flex items-center gap-2">
        <span class="text-sm text-slate-300 flex-1">{$t('privacy.hidden_count', { n: hiddenCount })}</span>
        <button on:click={onToggleHidden} disabled={!hiddenCount} class={btn}>
            {showHidden ? $t('privacy.hidden_hide') : $t('privacy.hidden_show')}
        </button>
    </div>
</Section>
