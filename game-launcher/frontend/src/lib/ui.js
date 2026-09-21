// Глобальные UI-сервисы: тосты и диалог подтверждения.
// Рендерятся один раз в App (<Toast/>, <ConfirmDialog/>), вызываются из любого компонента.
import { writable, get } from 'svelte/store';

// --- Тост ---
export const toast = writable({ show: false, message: '', type: 'success' });
let toastTimer = null;

export function showToast(message, type = 'success') {
    toast.set({ show: true, message, type });
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toast.update(t => ({ ...t, show: false })), 3000);
}

// --- Подтверждение (вместо нативного confirm) ---
export const confirmState = writable({ show: false, title: '', message: '', confirmText: 'OK', danger: false });
let pendingResolve = null;

// askConfirm возвращает Promise<boolean>: true — подтвердили, false — отмена.
export function askConfirm({ title = '', message = '', confirmText = 'OK', danger = false } = {}) {
    if (pendingResolve) pendingResolve(false); // предыдущий диалог считается отменённым
    return new Promise((resolve) => {
        pendingResolve = resolve;
        confirmState.set({ show: true, title, message, confirmText, danger });
    });
}

export function closeConfirm(result) {
    const resolve = pendingResolve;
    pendingResolve = null;
    confirmState.update(s => ({ ...s, show: false }));
    if (resolve) resolve(result);
}

export function isConfirmOpen() {
    return get(confirmState).show;
}
