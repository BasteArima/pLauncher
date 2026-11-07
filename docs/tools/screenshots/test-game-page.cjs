// Run against build-ui.cjs and a dedicated headless Edge. UI_PORT/CDP_PORT override 5393/9334.
// Screenshots and assertion results stay in git-ignored out/.
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
    fs.writeFileSync(path.join(__dirname, 'out', 'game-page-' + name + '.png'), Buffer.from(result.data, 'base64'));
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
  const title = () => evaluate('document.querySelector("main h1")?.textContent.trim()');
  const list = () => evaluate('document.querySelector("[data-game-list]")?.textContent.trim()');
  const setLanguage = async lang => {
    await key(',', 'Comma', 2);
    await wait('!!Array.from(document.querySelectorAll("select")).find(el=>el.querySelector("option[value=en]"))');
    await evaluate(`(() => { const el=Array.from(document.querySelectorAll('select')).find(el=>el.querySelector('option[value=en]')); el.value=${JSON.stringify(lang)}; el.dispatchEvent(new Event('change',{bubbles:true})); })()`);
    await key('Escape');
    await delay(80);
  };
  try {
    await send('Runtime.enable'); await send('Page.enable'); await send('Network.enable');
    await send('Emulation.setTimezoneOverride', { timezoneId: 'UTC' });
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false });
    await load();
    await textButton('Playing');
    const ordered = await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId)');
    check('status list has three games', ordered.length, 3);
    await click('main [data-game-id="' + ordered[1] + '"]');
    await wait(visible('[data-game-list]'));
    check('origin and position', await list(), 'Playing · 2 of 3');
    const middle = await title();
    await click('[data-game-next]');
    check('last position and boundary', await list(), 'Playing · 3 of 3');
    check('next disabled at end', await evaluate('document.querySelector("[data-game-next]").disabled'));
    await key('ArrowLeft', 'ArrowLeft', 1); await delay(100);
    check('Alt-left restores game and origin', await title(), middle);
    check('Alt-left restores position', await list(), 'Playing · 2 of 3');
    await key('ArrowRight', 'ArrowRight', 1); await delay(100);
    check('Alt-right restores neighbour', await list(), 'Playing · 3 of 3');
    await send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'back', x: 800, y: 450, clickCount: 1 });
    await send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'back', x: 800, y: 450, clickCount: 1 }); await delay(100);
    check('mouse back restores origin', await list(), 'Playing · 2 of 3');
    await send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'forward', x: 800, y: 450, clickCount: 1 });
    await send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'forward', x: 800, y: 450, clickCount: 1 }); await delay(100);
    check('mouse forward restores origin', await list(), 'Playing · 3 of 3');
    await click('[data-game-prev]'); await click('[data-game-prev]');
    check('previous disabled at start', await evaluate('document.querySelector("[data-game-prev]").disabled'));
    await key('Escape'); await delay(100);
    await click('main [data-game-id=g01]');
    await wait(visible('[data-update-found]'));
    check('date shown beside update source', await evaluate('document.querySelector("[data-update-found]").parentElement.textContent.startsWith("Found on F95zone · ")'));
    check('note date visible', await evaluate('document.querySelector("[data-notes-edited]").textContent.startsWith("Edited ")'));
    const noteDate = await evaluate('document.querySelector("[data-notes-edited]").textContent');
    await shot('wide');
    // Match the exported board width, keeping the app sidebar and responsive layout.
    await send('Emulation.setDeviceMetricsOverride', { width: 1016, height: 872, deviceScaleFactor: 1, mobile: false });
    await shot('mockup-width');
    await send('Emulation.setDeviceMetricsOverride', { width: 1290, height: 1500, deviceScaleFactor: 1, mobile: false });
    await evaluate('document.querySelector("main").scrollTop=0');
    await delay(80);
    const mainShot = await send('Page.captureScreenshot', { format: 'png', clip: { x: 274, y: 48, width: 1016, height: 1452, scale: 1 } });
    fs.writeFileSync(path.join(__dirname, 'out', 'game-page-main.png'), Buffer.from(mainShot.data, 'base64'));
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false });
    await textButton('Edit', 'main section');
    await textButton('Save');
    check('unchanged note keeps date', await evaluate('document.querySelector("[data-notes-edited]").textContent'), noteDate);
    await textButton('Edit', 'main section');
    await evaluate(`(() => {const e=document.querySelector('main textarea'); e.value='Changed note';e.dispatchEvent(new Event('input',{bubbles:true}));})()`);
    await textButton('Save');
    check('changed note gets new date', await evaluate('document.querySelector("[data-notes-edited]").textContent !== ' + JSON.stringify(noteDate)));
    await textButton("I've updated the game");
    check('accept update clears notice', await evaluate('!document.querySelector("[data-update-found]")'));
    check('accept update does not change notes date', await evaluate('document.querySelector("[data-notes-edited]").textContent.startsWith("Edited ")'));
    await setLanguage('ru');
    check('origin translates reactively', (await list()).startsWith('Играю · '));
    check('note date translates reactively', await evaluate('document.querySelector("[data-notes-edited]").textContent.startsWith("Изменено ")'));
    await shot('ru');
    await setLanguage('en');
    await key('F1'); await delay(100);
    check('F1 lists all added shortcuts', await evaluate('["Alt+← / Alt+→","Mouse 4 / 5","Ctrl+S","Cancel edit mode","Back from game preview"].every(t=>document.body.innerText.includes(t))'));
    await shot('hotkeys');
    await key('Escape');
    await key(',', 'Comma', 2); await delay(100);
    check('settings share added shortcuts', await evaluate('document.body.innerText.includes("Back from game preview") && document.body.innerText.includes("Mouse 4 / 5")'));
    await key('Escape');
    const openEditor = async () => {
      await click('[data-launch-menu]');
      await wait(visible('[data-context-menu]'));
      await textButton('Edit', '.fixed');
      await wait(visible('#game-about'));
    };
    await openEditor();
    check('editor hides neighbour controls', await evaluate('!document.querySelector("[data-game-list]").getClientRects().length'));
    await evaluate(`(() => {const e=document.querySelector('#game-about');e.value='Edited description';e.dispatchEvent(new Event('input',{bubbles:true}));})()`);
    await key('s', 'KeyS', 2); await wait('!document.querySelector("#game-about")');
    check('Ctrl-S saves editor', await evaluate('document.querySelector("main").innerText.includes("Edited description")'));
    await openEditor();
    await evaluate(`(() => {const e=document.querySelector('#game-about');e.value='Discarded description';e.dispatchEvent(new Event('input',{bubbles:true}));})()`);
    await key('Escape'); await textButton('Discard', '.fixed');
    check('Esc discards editor changes', await evaluate('!document.querySelector("#game-about") && document.querySelector("main").innerText.includes("Edited description") && !document.querySelector("main").innerText.includes("Discarded description")'));
    check('editor retains list context', (await list()).startsWith('Playing · '));
    await load();
    await textButton('Weekend picks', 'aside');
    const collectionOrder = await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId)');
    await click('main [data-game-id="'+collectionOrder[0]+'"]');
    check('collection origin', await list(), 'Weekend picks · 1 of 4');
    await click('[data-game-next]');
    check('collection order', await title(), 'Campus Days');
    check('collection position', await list(), 'Weekend picks · 2 of 4');
    await key('Escape');
    await key('f', 'KeyF', 2);
    await evaluate(`(() => {const e=document.querySelector('input[placeholder="Search by title…"]');e.value='s';e.dispatchEvent(new Event('input',{bubbles:true}));e.blur();})()`);
    await delay(150);
    const searchOrder = await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId)');
    await click('main [data-game-id="'+searchOrder[0]+'"]');
    check('search origin', (await list()).startsWith('Search results: s · 1 of '));
    await click('[data-game-next]');
    await key('ArrowLeft', 'ArrowLeft', 1); await delay(100);
    check('history preserves search text', await evaluate(`document.querySelector('input[placeholder="Search by title…"]').value`), 's');
    await key('Escape');
    check('history preserves search order', await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId)'), searchOrder);
    await load();
    await evaluate(`(() => {const e=document.querySelector('main select');e.value='title_asc';e.dispatchEvent(new Event('change',{bubbles:true}));})()`);
    await delay(80);
    const sorted = await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId)');
    const titles = await evaluate('window.go.main.App.GetGames().then(g=>Object.fromEntries(g.map(x=>[x.id,x.title])))');
    await click('main [data-game-id="'+sorted[0]+'"]');
    await click('[data-game-next]');
    check('neighbour follows selected sort', await title(), titles[sorted[1]]);
    await load('stats');
    await click('main [data-game-id=g01]');
    check('hidden games absent from origin', (await list()).endsWith('of 9'));
    await key('Escape');
    await evaluate(`Array.from(document.querySelectorAll('aside button')).find(e=>e.textContent.trim()==='Summer Heat').click()`);
    check('sidebar collection uses its own order', await list(), 'Weekend picks · 2 of 3');
    await click('[data-game-prev]');
    check('sidebar neighbour follows alphabetical order', await title(), 'Campus Days');
    await load();
    await evaluate(`localStorage.setItem('plauncher_home_view','shelves')`);
    await send('Page.navigate', { url: origin + '/?shot=home&keep=1' });
    await wait('document.body?.dataset.ready === "1"');
    await evaluate('document.querySelector("main [data-game-id=g01]").focus()');
    await key('Enter');
    check('shelf carries its own list context', await list(), 'Favorites · 1 of 3');
    await click('[data-game-next]');
    check('shelf neighbour order', await title(), 'Campus Days');
    await load('stats');
    await click('[data-nav-statistics]'); await wait(visible('[data-stats-game]'));
    await click('[data-stats-game]');
    check('outside library lists has no neighbour block', await evaluate('!document.querySelector("[data-game-list]")'));
    check('no external requests', external, []);
    check('no runtime errors', errors.length, 0);
    fs.writeFileSync(path.join(__dirname, 'out', 'game-page-checks.json'), JSON.stringify(checks, null, 2));
    console.log('PASS: ' + checks.length + ' game page UI checks');
  } catch (error) {
    await shot('failure').catch(() => {});
    console.error(error); console.error(JSON.stringify(errors)); process.exitCode=1;
  } finally { ws.close(); }
})();
