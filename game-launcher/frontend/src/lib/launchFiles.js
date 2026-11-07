// Comparison only: keep original paths when saving. Windows paths ignore case; POSIX paths do not.
export function fileKey(path = '') {
    const windows = /^[a-z]:[\\/]|^\\\\/i.test(path);
    const parts = [];
    for (const part of path.replaceAll('\\', '/').split('/')) {
        if (part === '.') continue;
        if (part === '..' && parts.length && parts[parts.length - 1] !== '..') parts.pop();
        else parts.push(part);
    }
    const key = parts.join('/');
    return windows ? key.toLowerCase() : key;
}
export const fileName = path => (path || '').split(/[\\/]/).pop();
export const launchFiles = game => [
    ...(game.exec_path ? [{ path: game.exec_path, label: game.exec_label || '', primary: true, index: -1 }] : []),
    ...(game.launch_files || []).map((file, index) => ({ ...file, primary: false, index })),
];
export const fileLabel = (file, key, translate) => file.label || (key ? translate(key) : fileName(file.path));
export const freeSlots = game => (game.launch_files || []).length > 8 ? 0 : 8 - (game.launch_files || []).length + (game.exec_path ? 0 : 1);
