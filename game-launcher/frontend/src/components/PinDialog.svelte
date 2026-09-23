<script>
    // Ввод PIN-кода.
    // mode='lock'   — полноэкранная блокировка лаунчера (закрыть нельзя), Unlock на бэкенде;
    // mode='verify' — модальная проверка (показ скрытых коллекций), VerifyPin, можно отменить.
    import { onMount, tick } from 'svelte';
    import { t } from '../i18n.js';
    import { pinError } from '../lib/privacy.js';
    import { Unlock, VerifyPin } from '../../wailsjs/go/main/App.js';

    export let mode = 'lock';
    export let onSuccess = () => {};
    export let onCancel = () => {};

    let pin = '';
    let error = '';
    let busy = false;
    let input;

    onMount(async () => { await tick(); input && input.focus(); });

    async function submit() {
        if (!pin || busy) return;
        busy = true; error = '';
        try {
            if (mode === 'lock') await Unlock(pin); else await VerifyPin(pin);
            pin = '';
            onSuccess();
        } catch (err) {
            error = pinError(err);
            pin = '';
            await tick(); input && input.focus();
        } finally { busy = false; }
    }

    function onKey(e) {
        if (e.key === 'Enter') { e.preventDefault(); submit(); }
        else if (e.key === 'Escape' && mode === 'verify') { e.preventDefault(); onCancel(); }
    }
</script>

<!-- Экран блокировки начинается под титлбаром: окно можно свернуть/закрыть -->
<div class="fixed flex items-center justify-center animate-fade-in {mode === 'lock' ? 'inset-x-0 bottom-0 top-9 z-[200] aurora-bg' : 'inset-0 z-[85] bg-slate-950/80 backdrop-blur-md'}"
     on:click={() => mode === 'verify' && onCancel()}>
    <div class="w-[360px] max-w-[90vw] glass-strong rounded-2xl shadow-2xl p-7 text-center" on:click|stopPropagation>
        <div class="text-4xl mb-3">🔒</div>
        <h2 class="text-xl font-black text-white mb-1">{mode === 'lock' ? $t('lock.title') : $t('lock.verify_title')}</h2>
        <p class="text-sm text-slate-400 mb-5">{mode === 'lock' ? $t('lock.sub') : $t('lock.verify_sub')}</p>
        <input bind:this={input} type="password" bind:value={pin} on:keydown={onKey} autocomplete="off"
               placeholder="••••"
               class="w-full text-center text-2xl tracking-[0.4em] bg-slate-900/60 border border-white/10 text-white rounded-xl px-4 py-3 focus:outline-none focus:ring-2 focus:ring-indigo-400/40"/>
        <p class="text-xs text-rose-300 h-5 mt-2">{error}</p>
        <div class="flex gap-2 mt-2">
            {#if mode === 'verify'}
                <button on:click={onCancel} class="flex-1 py-2.5 rounded-lg font-semibold text-slate-300 bg-white/5 hover:bg-white/10 border border-white/10 transition-colors">{$t('btn.cancel')}</button>
            {/if}
            <button on:click={submit} disabled={!pin || busy}
                    class="flex-1 py-2.5 rounded-lg font-bold text-white bg-indigo-600 hover:bg-indigo-500 transition-colors disabled:opacity-50">
                {$t('lock.unlock')}
            </button>
        </div>
    </div>
</div>
