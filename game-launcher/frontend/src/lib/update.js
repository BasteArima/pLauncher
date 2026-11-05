// Обновление самого лаунчера: состояние для плашки в заголовке и окна обновления.
// Загрузка и установка — на бэкенде (updater_install.go), сюда приходит событие "launcher-download".
import { writable, get } from 'svelte/store';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import {
    CheckLauncherUpdate, StartLauncherDownload, CancelLauncherDownload, GetLauncherDownload,
    ApplyLauncherUpdate, GetLauncherRestartBlocker,
} from '../../wailsjs/go/main/App.js';
import { lsGet, lsSet } from './util.js';
import { discreet } from './privacy.js';

const CHECK_INTERVAL = 6 * 3600 * 1000;
const RESTART_DELAY = 3;                 // секунд до автоперезапуска: успеть снять галочку
const BLOCKER_POLL = 5000;

export const updateInfo = writable(null);          // CheckLauncherUpdate()
export const download = writable({ state: 'idle' }); // LauncherDownload
export const updateModalOpen = writable(false);
export const restartIn = writable(0);              // обратный отсчёт автоперезапуска
export const restartBlocker = writable('');        // game | hidden | editing | ''
export const restarting = writable(false);

function prefStore(key) {
    const s = writable(lsGet(key, '1') === '1');
    s.subscribe(v => lsSet(key, v ? '1' : '0'));
    return s;
}
export const autoRestart = prefStore('plauncher_upd_autorestart');
export const autoCheck = prefStore('plauncher_upd_autocheck');

let busyCheck = () => '';
let timer = null;
let postponed = false;

// busy() — что ещё мешает перезапуску на стороне интерфейса (открытый редактор игры).
export function initUpdater({ appVersion, busy }) {
    busyCheck = busy;
    EventsOn('launcher-download', (d) => download.set(d));
    GetLauncherDownload().then(d => download.set(d)).catch(() => {});

    // Готово и стоит галочка: перезапуск, как только ничего не мешает
    download.subscribe(scheduleAutoRestart);
    autoRestart.subscribe(scheduleAutoRestart);

    const check = () => { if (get(autoCheck)) checkUpdate().catch(() => {}); }; // нет сети — молча
    if (appVersion !== 'dev') {
        check();
        setInterval(check, CHECK_INTERVAL);
    }
}

export async function checkUpdate() {
    const info = await CheckLauncherUpdate();
    updateInfo.set(info);
    return info;
}

export async function startDownload() {
    await StartLauncherDownload();
}

export function cancelDownload() {
    CancelLauncherDownload();
}

export async function restartNow() {
    stopAutoRestart();
    restarting.set(true);
    try { await ApplyLauncherUpdate(get(discreet)); }
    catch (err) { restarting.set(false); throw err; }
}

export async function currentBlocker() {
    return busyCheck() || await GetLauncherRestartBlocker().catch(() => '');
}

// «При следующем запуске»: только на этот раз, сама настройка галочки не меняется.
export function postponeRestart() {
    postponed = true;
    stopAutoRestart();
}

function stopAutoRestart() {
    clearTimeout(timer);
    timer = null;
    restartIn.set(0);
    restartBlocker.set('');
}

function scheduleAutoRestart() {
    if (timer || get(restarting)) {
        if (!get(autoRestart) || get(download).state !== 'ready') stopAutoRestart();
        return;
    }
    if (!postponed && get(autoRestart) && get(download).state === 'ready') waitForFreeMoment();
}

async function waitForFreeMoment() {
    timer = setTimeout(waitForFreeMoment, BLOCKER_POLL);
    const blocker = await currentBlocker();
    if (!timer) return;                  // галочку сняли, пока ждали ответа
    restartBlocker.set(blocker);
    if (blocker) return;
    clearTimeout(timer);
    tick(RESTART_DELAY);
}

function tick(left) {
    restartIn.set(left);
    if (left <= 0) {
        restartNow().catch(err => console.error(err));
        return;
    }
    timer = setTimeout(() => tick(left - 1), 1000);
}
