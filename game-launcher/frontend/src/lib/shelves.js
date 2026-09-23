// Полки главной (как в Steam) и вычисление коллекций — чистые функции.
import { tr } from '../i18n.js';
import { lsJSON } from './util.js';

export function newShelfId() { return 's_' + Math.random().toString(36).slice(2, 9); }

export function defaultShelves() {
    return [
        { id: newShelfId(), type: 'favorites' },
        { id: newShelfId(), type: 'continue' },
        { id: newShelfId(), type: 'added' },
        { id: newShelfId(), type: 'all' },
    ];
}

// Сохранённые полки из localStorage (или набор по умолчанию).
// Раздел «Вся библиотека» может быть только один — лишние (из старых сохранений) отбрасываем.
export function loadShelves() {
    const raw = lsJSON('plauncher_shelves', null);
    if (Array.isArray(raw) && raw.length) {
        let seenAll = false;
        return raw
            .filter(s => s.type !== 'all' || (!seenAll && (seenAll = true)))
            .map(s => ({ id: s.id || newShelfId(), type: s.type, tag: s.tag, collectionId: s.collectionId }));
    }
    return defaultShelves();
}

function shelfTitle(s) {
    if (s.type === 'tag') return s.tag || tr('layout.opt.tag');
    if (['continue', 'added', 'favorites', 'all'].includes(s.type)) return tr('shelf.' + s.type);
    return tr('shelf.section');
}

// d — готовые списки: { recentlyPlayed, recentlyAdded, favoriteGames, allGamesSorted, games, collectionsView }
// Полки для отрисовки. Единственное исключение: если сразу под баннером «Продолжить»
// (первая непустая полка) стоит «Продолжить играть», игру из баннера в ней не повторяем —
// иначе одна и та же игра идёт два раза подряд. Ниже по странице полка показывается целиком.
export function renderShelves(shelves, d, heroGame) {
    const built = shelves.map(s => buildShelf(s, d));
    const first = built.find(s => s.items.length);
    if (heroGame && first && first.type === 'continue' && first.items[0] === heroGame) {
        first.items = first.items.slice(1);
    }
    return built;
}

export function buildShelf(s, d) {
    let items, title = shelfTitle(s);
    switch (s.type) {
        case 'continue':  items = d.recentlyPlayed; break;
        case 'added':     items = d.recentlyAdded; break;
        case 'favorites': items = d.favoriteGames; break;
        case 'all':       items = d.allGamesSorted; break;
        case 'tag':       items = d.games.filter(g => (g.tags || []).includes(s.tag)); break;
        case 'collection': {
            const col = (d.collectionsView || []).find(c => c.id === s.collectionId);
            items = col ? col.items : [];
            title = col ? col.name : tr('layout.opt.collection');
            break;
        }
        default:          items = [];
    }
    return { ...s, title, items };
}

// Игры коллекции: ручная — по game_ids; динамическая — совпавшие по тегам ∪ вручную добавленные
export function buildCol(c, allGames) {
    const ids = new Set(c.game_ids || []);
    let items;
    if (c.type === 'dynamic') {
        const tags = c.tags || [];
        items = allGames.filter(g => ids.has(g.id) || (tags.length && (g.tags || []).some(t => tags.includes(t))));
    } else {
        items = allGames.filter(g => ids.has(g.id));
    }
    return { ...c, items };
}
