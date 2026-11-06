// Статусы прохождения: один список для страницы игры, чипсов над сеткой и подписи карточки.
// Пустой статус ("") — игра не начата. Подписи — в locales: status.<id> и status.short_<id>.
export const STATUSES = [
    { id: 'playing', text: 'text-emerald-300', dot: 'bg-emerald-400' },
    { id: 'want', text: 'text-violet-300', dot: 'bg-violet-300' },
    { id: 'finished', text: 'text-sky-300', dot: 'bg-sky-400' },
    { id: 'dropped', text: 'text-slate-300', dot: 'bg-slate-500' },
];

export const statusInfo = (id) => STATUSES.find(s => s.id === id) || null;
