// Список горячих клавиш для справки (F1) и вкладки настроек.
// Сами обработчики — в App.svelte (handleKeydown), GameCard, Lightbox, ConfirmDialog.
export const HOTKEY_GROUPS = [
    { title: 'hk.g_nav', keys: [
        ['Ctrl+F', 'hk.search'],
        ['← ↑ → ↓', 'hk.arrows'],
        ['Enter', 'hk.open'],
        ['Ctrl+Enter', 'hk.play'],
        ['Esc / Backspace', 'hk.back'],
        ['F5', 'hk.scan'],
        ['Ctrl+,', 'hk.settings'],
        ['F1', 'hk.help'],
    ] },
    { title: 'hk.g_sel', keys: [
        ['Ctrl+Click', 'hk.sel_toggle'],
        ['Shift+Click', 'hk.sel_range'],
        ['Space', 'hk.sel_focused'],
        ['Ctrl+A', 'hk.sel_all'],
        ['Delete', 'hk.sel_delete'],
        ['Esc', 'hk.sel_clear'],
    ] },
    { title: 'hk.g_view', keys: [
        ['Ctrl + / Ctrl −', 'hk.zoom'],
        ['Ctrl+0', 'hk.zoom_reset'],
        ['Ctrl+Wheel', 'hk.zoom_wheel'],
    ] },
    { title: 'hk.g_privacy', keys: [
        ['Ctrl+H', 'hk.discreet'],
        ['Ctrl+Shift+H', 'hk.hidden'],
        ['Ctrl+L', 'hk.lock'],
    ] },
];
