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

// Сохранённые полки из localStorage (или набор по умолчанию)
export function loadShelves() {
    const raw = lsJSON('plauncher_shelves', null);
    if (Array.isArray(raw) && raw.length) {
        return raw.map(s => ({ id: s.id || newShelfId(), type: s.type, tag: s.tag, collectionId: s.collectionId }));
    }
    return defaultShelves();
}

function shelfTitle(s) {
    if (s.type === 'tag') return s.tag || tr('layout.opt.tag');
    if (['continue', 'added', 'favorites', 'all'].includes(s.type)) return tr('shelf.' + s.type);
    return tr('shelf.section');
}

// d — готовые списки: { recentlyPlayed, recentlyAdded, favoriteGames, allGamesSorted, games, collectionsView }
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
