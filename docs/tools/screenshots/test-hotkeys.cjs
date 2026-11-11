// Hotkey rebinding and discreet-mode spoiler regression. Run against build-ui.cjs and a dedicated
// headless Edge (see README); UI_PORT/CDP_PORT override 5393/9334. Results stay in git-ignored out/.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

(async () => {
  const tabs = await (await fetch('http://127.0.0.1:' + (process.env.CDP_PORT || '9334') + '/json')).json();
  const origin = 'http://127.0.0.1:' + (process.env.UI_PORT || '5393');
  const tab = tabs.find(t => t.url.startsWith(origin)) || tabs.find(t => t.url === 'about:blank');
  const ws = new WebSocket(tab.webSocketDebuggerUrl);
  await new Promise(resolve => ws.addEventListener('open', resolve, { once: true }));
  let seq = 0;
  const pending = new Map(), errors = [], checks = [], external = [];
  ws.addEventListener('message', e => {
    const message = JSON.parse(e.data);
    if (message.id) { const p = pending.get(message.id); pending.delete(message.id); message.error ? p.reject(message.error) : p.resolve(message.result); }
    if (message.method === 'Network.requestWillBeSent' && /^https?:/.test(message.params.request.url) && !message.params.request.url.startsWith(origin + '/')) external.push(message.params.request.url);
    if (message.method === 'Runtime.exceptionThrown') errors.push(message.params.exceptionDetails);
  });
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++seq; pending.set(id, { resolve, reject }); ws.send(JSON.stringify({ id, method, params }));
  });
  const evaluate = async expression => {
    const result = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
    if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails));
    return result.result.value;
  };
  const wait = async expression => {
    for (let i = 0; i < 100; i++) { if (await evaluate(expression)) return; await delay(50); }
    throw new Error('Timed out: ' + expression);
  };
  const visible = selector => `!!document.querySelector(${JSON.stringify(selector)})?.getClientRects().length`;
  const click = async selector => {
    await wait(visible(selector));
    const point = await evaluate(`(() => { const el = document.querySelector(${JSON.stringify(selector)}); el.scrollIntoView({block:'center'}); const r=el.getBoundingClientRect(); return {x:r.x+r.width/2,y:r.y+r.height/2}; })()`);
    await send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1, ...point });
    await send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...point });
  };
  const key = async (key, code = key, modifiers = 0) => {
    await send('Input.dispatchKeyEvent', { type: 'keyDown', key, code, modifiers, text: key === 'Enter' ? '\r' : undefined, windowsVirtualKeyCode: key === 'Escape' ? 27 : key === 'Enter' ? 13 : undefined });
    await send('Input.dispatchKeyEvent', { type: 'keyUp', key, code, modifiers });
  };
  const shot = async (name, selector) => {
    if (selector) await evaluate(`document.querySelector(${JSON.stringify(selector)}).scrollIntoView({block:'start'})`);
    else await evaluate('document.querySelector("main").scrollTop=0');
    await delay(80);
    const result = await send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(__dirname, 'out', 'hotkeys-' + name + '.png'), Buffer.from(result.data, 'base64'));
  };
  const check = (name, actual, expected = true) => { assert.deepEqual(actual, expected, name); checks.push(name); };
  const textButton = async (text, scope = 'main') => {
    await evaluate(`(() => { const el=[...document.querySelectorAll(${JSON.stringify(scope + ' button')})].find(e=>e.textContent.trim().replace(/ +[0-9]+$/, '')===${JSON.stringify(text)}); if(!el) throw Error('Button not found: '+${JSON.stringify(text)}); el.click(); })()`);
    await delay(100);
  };
  const load = async (scenario = 'home') => {
    await send('Page.navigate', { url: origin + '/?shot=' + scenario });
    await wait('document.body?.dataset.ready === "1"');
  };
  const discreetOn = () => evaluate('document.documentElement.classList.contains("is-discreet")');
  // Текст под «спойлером» прозрачен и покрыт фоном с точками
  const hidden = selector => evaluate(`(() => { const el=document.querySelector(${JSON.stringify(selector)}); if(!el) return 'missing'; const s=getComputedStyle(el); return s.color==='rgba(0, 0, 0, 0)' && s.backgroundImage.includes('svg'); })()`);
  const hint = id => evaluate(`document.querySelector('[data-hotkey-row="${id}"] [data-hotkey-hint]')?.textContent || ''`);
  const label = id => evaluate(`document.querySelector('[data-hotkey="${id}"]')?.textContent.trim()`);
  const CTRL = 2, SHIFT = 8;
  try {
    await send('Runtime.enable'); await send('Page.enable'); await send('Emulation.setFocusEmulationEnabled', { enabled: true }); await send('Network.enable');
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
    await load();

    // --- Дискретный режим: названия под спойлером, статус и тип файла видны ---
    check('discreet off by default', await discreetOn(), false);
    check('card title readable when off', await hidden('[data-game-id] h3'), false);
    await key('h', 'KeyH', CTRL); await delay(80);
    check('Ctrl+H enables discreet mode', await discreetOn());
    check('card title hidden', await hidden('[data-game-id] h3'));
    check('card version hidden', await hidden('[data-game-id] h3 + div .spoiler'));
    check('sidebar game title hidden', await hidden('aside .spoiler.truncate'));
    check('card status stays readable', await evaluate('(() => { const el=[...document.querySelectorAll("[data-game-id] h3 + div span")].find(s=>!s.classList.contains("spoiler") && s.textContent.trim().length>2); return !!el && getComputedStyle(el).color!=="rgba(0, 0, 0, 0)"; })()'));
    await shot('discreet');
    await click('[data-game-id]'); await wait(visible('main h1'));
    check('game page title hidden', await hidden('main h1 .spoiler'));
    await key('Escape'); await delay(80);
    await key('h', 'KeyH', CTRL); await delay(80);
    check('Ctrl+H disables discreet mode', await discreetOn(), false);
    check('card title readable again', await hidden('[data-game-id] h3'), false);

    // --- Переназначение в настройках ---
    await key(',', 'Comma', CTRL); await wait(visible('[data-hotkey="discreet"]'));
    check('settings list is editable', await evaluate('document.querySelectorAll("button[data-hotkey]").length'), 16);
    await click('[data-hotkey="discreet"]'); await delay(50);
    check('capture prompt shown', await label('discreet'), 'Press a combination…');
    await key('l', 'KeyL', CTRL); await delay(50);
    check('conflict reported', await hint('discreet'), 'Already used: Lock (if a PIN is set)');
    await key('j', 'KeyJ'); await delay(50);
    check('modifier required', await hint('discreet'), 'Add Ctrl or Alt (or use an F key)');
    await key('c', 'KeyC', CTRL); await delay(50);
    check('text shortcuts reserved', await hint('discreet'), 'This combination is reserved');
    await key('Escape'); await delay(50);
    check('Esc cancels capture, settings stay open', await label('discreet'), 'Ctrl+H');
    await click('[data-hotkey="discreet"]'); await delay(50);
    await key('о', 'KeyJ', CTRL); await delay(50);   // русская раскладка: решает физическая клавиша
    check('new combination saved', await label('discreet'), 'Ctrl+J');
    check('stored in localStorage', await evaluate('localStorage.getItem("plauncher_hotkeys")'), '{"discreet":["Ctrl+J"]}');
    check('reset button for changed row', await evaluate(`!!document.querySelector('[data-hotkey-row="discreet"] [data-hotkey-reset]')`));
    await click('[data-hotkey="search"]'); await delay(50);
    await key('k', 'KeyK', CTRL | SHIFT); await delay(50);
    check('search rebound', await label('search'), 'Ctrl+Shift+K');
    await shot('settings', '[data-hotkey-row="search"]');
    await key('Escape'); await delay(80);

    // --- Новые сочетания действуют, старые — нет ---
    await key('h', 'KeyH', CTRL); await delay(80);
    check('old discreet combination inactive', await discreetOn(), false);
    await key('j', 'KeyJ', CTRL); await delay(80);
    check('new discreet combination works', await discreetOn());
    await key('j', 'KeyJ', CTRL); await delay(80);
    check('titlebar tooltip follows binding', await evaluate('[...document.querySelectorAll("button[title]")].some(b=>b.title.endsWith(" · Ctrl+J"))'));
    check('search hint follows binding', await evaluate('[...document.querySelectorAll("label span")].some(s=>s.textContent.trim()==="Ctrl Shift K")'));
    await key('л', 'KeyK', CTRL | SHIFT); await delay(80);
    check('rebound search focuses input', await evaluate('document.activeElement?.tagName === "INPUT"'));
    await evaluate('document.activeElement.blur()');

    // --- Сохраняется после перезапуска, справка F1 показывает текущие ---
    await send('Page.navigate', { url: origin + '/?shot=home&keep=1' });
    await wait('document.body?.dataset.ready === "1"');
    await key('F1', 'F1'); await wait(visible('kbd[data-hotkey="discreet"]'));
    check('F1 shows rebound combination', await evaluate(`document.querySelector('kbd[data-hotkey="discreet"]').textContent`), 'Ctrl+J');
    await key('F1', 'F1'); await delay(80);

    // --- Снять сочетание и сбросить всё ---
    await key(',', 'Comma', CTRL); await wait(visible('[data-hotkey="lock"]'));
    await evaluate(`document.querySelector('[data-hotkey-row="lock"] [data-hotkey-clear]').click()`); await delay(50);
    check('cleared combination shows Not set', await label('lock'), 'Not set');
    await click('[data-hotkey-reset-all]'); await delay(100);
    await key('Enter', 'Enter'); await delay(100);
    check('reset all restores defaults', [await label('discreet'), await label('search'), await label('lock')], ['Ctrl+H', 'Ctrl+F', 'Ctrl+L']);
    check('reset all clears storage', await evaluate('localStorage.getItem("plauncher_hotkeys")'), '{}');
    await key('Escape'); await delay(80);
    await key('h', 'KeyH', CTRL); await delay(80);
    check('default discreet works after reset', await discreetOn());

    check('no external requests', external, []);
    check('no runtime errors', errors.length, 0);
    fs.writeFileSync(path.join(__dirname, 'out', 'hotkeys-checks.json'), JSON.stringify(checks, null, 2));
    console.log('PASS: ' + checks.length + ' hotkey and discreet-mode checks');
  } catch (error) {
    await shot('failure').catch(() => {});
    console.error(error); console.error(JSON.stringify(errors)); process.exitCode=1;
  } finally { ws.close(); }
})();
