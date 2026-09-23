<script>
    // Сортировка сетки «Вся библиотека» и результатов фильтра/поиска.
    // (Полки главной сортируются по своему смыслу, поэтому сортировка живёт здесь, а не в шапке.)
    import { t } from '../i18n.js';

    export let sortBy = 'added_desc';        // bind
</script>

<div class="flex items-center gap-2 text-sm text-slate-400">
    <span class="text-xs uppercase tracking-wider">{$t('sort.label')}</span>
    <select bind:value={sortBy} class="bg-slate-900/60 border border-white/10 text-slate-200 text-sm rounded-lg px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-indigo-400/40">
        <option value="added_desc">{$t('sort.added_desc')}</option>
        <option value="added_asc">{$t('sort.added_asc')}</option>
        <option value="played_desc">{$t('sort.played')}</option>
        <option value="playtime_desc">{$t('sort.playtime')}</option>
        <option value="size_desc">{$t('sort.size')}</option>
        <option value="title_asc">{$t('sort.title')}</option>
    </select>
</div>
