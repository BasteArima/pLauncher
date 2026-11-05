// Простой рендер заметок к релизу: списки, заголовки, абзацы, **жирный**, `код`.
// Всё экранируется, ссылки показываются текстом: HTML из описания релиза в интерфейс не попадает.
function esc(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function inline(s) {
    return esc(s)
        .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
        .replace(/\*\*([^*]+)\*\*/g, '<b class="text-slate-200">$1</b>')
        .replace(/`([^`]+)`/g, '<code class="text-indigo-200">$1</code>');
}

export function renderNotes(md) {
    const out = [];
    let list = false;
    const closeList = () => { if (list) { out.push('</ul>'); list = false; } };
    let continuation = false;            // продолжение пункта списка на следующей строке
    for (const raw of (md || '').split('\n')) {
        const line = raw.trim();
        if (!line) { closeList(); continuation = false; continue; }
        const item = /^[-*]\s+(.*)$/.exec(line);
        if (item) {
            if (!list) { out.push('<ul class="list-disc pl-4 space-y-1">'); list = true; }
            out.push(`<li>${inline(item[1])}</li>`);
            continuation = true;
        } else if (list && continuation) {
            out[out.length - 1] = out[out.length - 1].replace('</li>', ` ${inline(line)}</li>`);
        } else {
            closeList();
            const head = /^#{1,6}\s+(.*)$/.exec(line);
            if (head) out.push(`<p class="font-bold text-slate-200 mt-2">${inline(head[1])}</p>`);
            else out.push(`<p>${inline(line)}</p>`);
        }
    }
    closeList();
    return out.join('');
}
