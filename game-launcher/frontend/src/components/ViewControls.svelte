<script>
    // Сортировка сетки (все игры, коллекция, фильтр, поиск).
    // Полки сортируются по своему смыслу, поэтому сортировка живёт здесь, а не в шапке.
    import { t } from '../i18n.js';

    export let sortBy = 'added_desc';        // bind
</script>

<label class="flex items-center gap-2 text-[13px] text-slate-500">
    {$t('sort.label')}
    <select bind:value={sortBy} class="h-8 bg-slate-900 border border-white/10 text-slate-100 text-[13px] rounded-lg pl-2.5 pr-7 focus:outline-none focus:border-indigo-400/60 cursor-pointer">
        <option value="added_desc">{$t('sort.added_desc')}</option>
        <option value="added_asc">{$t('sort.added_asc')}</option>
        <option value="played_desc">{$t('sort.played')}</option>
        <option value="playtime_desc">{$t('sort.playtime')}</option>
        <option value="size_desc">{$t('sort.size')}</option>
        <option value="title_asc">{$t('sort.title')}</option>
    </select>
</label>
