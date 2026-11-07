// Run against build-ui.cjs on :5393 and a dedicated headless Edge on :9334.
// Screenshots and assertion results stay in git-ignored out/.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

(async () => {
  const origin = 'http://127.0.0.1:' + (process.env.UI_PORT || '5393');
  const tabs = await (await fetch('http://127.0.0.1:' + (process.env.CDP_PORT || '9334') + '/json')).json();
  const tab = tabs.find(t => t.url.startsWith(origin)) || tabs.find(t => t.url === 'about:blank');
  const ws = new WebSocket(tab.webSocketDebuggerUrl);
  await new Promise(resolve => ws.addEventListener('open', resolve, { once: true }));
  let seq = 0;
  const pending = new Map(), errors = [], checks = [];
  ws.addEventListener('message', e => {
    const message = JSON.parse(e.data);
    if (message.id) { const p = pending.get(message.id); pending.delete(message.id); message.error ? p.reject(message.error) : p.resolve(message.result); }
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
    fs.writeFileSync(path.join(__dirname, 'out', 'find-' + name + '.png'), Buffer.from(result.data, 'base64'));
  };
  const check = (name, actual, expected = true) => { assert.deepEqual(actual, expected, name); checks.push(name); };
  const snapshot = () => evaluate('window.go.main.App.GetGames()');
  const choose = '[data-hit-url]';
  const preview = '[data-info-preview]';
  const load = async (extra = '') => {
    await send('Page.navigate', { url: origin + '/?shot=find' + extra });
    await wait('document.body?.dataset.ready === "1"');
    await wait(visible('[data-find-tab=name]'));
    if (!await evaluate(visible('[data-find-query]'))) await click('[data-find-tab=name]');
    await wait(visible('[data-find-query]'));
    await wait('document.querySelector("[data-find-query]").value === "Moonlit Harbor"');
  };
  try {
    await send('Runtime.enable'); await send('Page.enable'); await send('Emulation.setFocusEmulationEnabled', { enabled: true });
    await send('Page.bringToFront');
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
    await load();
    check('folder suggestion and version hint', await evaluate('document.querySelector("[data-find-by-name]").innerText.includes("0.3")'));
    check('prominent defaults to name', await evaluate('document.querySelector("[data-find-tab=name]").getAttribute("aria-pressed")'), 'true');
    await shot('search');
    await click('[data-find-query]'); await key('Enter');
    await wait(visible('[data-find-results]'));
    check('four hits from two sources', await evaluate('document.querySelectorAll("[data-hit-url]").length'), 4);
    check('one best match', await evaluate('document.querySelectorAll("[data-find-best]").length'), 1);
    check('failed source visible', await evaluate('document.querySelector("[data-find-failed]").innerText.includes("F95zone")'));
    await shot('results', '[data-metadata-panel]');
    check('landscape result covers', await evaluate('[...document.querySelectorAll("[data-find-card] img")].every(img=>Math.abs(img.clientWidth/img.clientHeight-16/9)<0.03)'));
    check('reasons replace best label', await evaluate('!!document.querySelector("[data-find-reason]") && !document.querySelector("[data-find-results]").innerText.includes("Most likely")'));
    check('best action is white', await evaluate('getComputedStyle(document.querySelector("button[data-find-best]")).backgroundColor==="rgb(255, 255, 255)"'));
    check('unavailable source metadata omitted', await evaluate('![...document.querySelectorAll("[data-find-card]")].some(el=>el.innerText.includes("updated") || el.innerText.includes("Ren\'Py"))'));
    await evaluate('window.__findMock.failedSource=""');
    await click('[data-find-search]'); await wait('document.querySelectorAll("[data-hit-url]").length===6');
    check('F95 engine and update date displayed', await evaluate(`[...document.querySelectorAll('[data-find-card]')].filter(el=>el.innerText.includes('F95zone')).every(el=>el.innerText.includes("Ren'Py") && el.innerText.includes('updated Oct 30'))`));
    await shot('source-metadata', '[data-metadata-panel]');
    await evaluate('window.__findMock.failedSource="F95zone"');
    await click('[data-find-search]'); await wait('document.querySelectorAll("[data-hit-url]").length===4');
    const original = await snapshot();
    await click(choose); await wait(visible(preview));
    check('preview omits launch controls', await evaluate('!document.querySelector("[data-launch-menu], [data-launch-details], [data-launch-editor]")'));
    check('preview omits play sessions', await evaluate('!document.querySelector("[data-game-sessions]")'));
    check('preview leaves library unchanged', await snapshot(), original);
    check('preview fallback version', await evaluate('document.querySelector("[data-game-detail]").innerText.includes("0.3")'));
    check('preview has no play/status/notes mutations', await evaluate('!Array.from(document.querySelectorAll("[data-game-detail] button")).filter(e=>e.getClientRects().length).some(e=>/^(Play|Want|Playing|Edit notes|Add link)$/.test(e.textContent.trim()))'));
    check('remote images load directly', await evaluate('Array.from(document.querySelectorAll("[data-game-detail] img")).every(e=>!e.src.includes("/media/http"))'));
    await wait('document.querySelector("[data-game-detail] img")?.naturalWidth > 0');
    await shot('preview');
    await key('Enter', 'Enter', 2);
    check('Ctrl+Enter cannot launch from preview', await evaluate('!window.__findMock.calls.some(c=>["Launch","LaunchGameFile"].includes(c.name))'));
    await key(',', 'Comma', 2);
    await wait('!!Array.from(document.querySelectorAll("select")).find(el=>el.querySelector("option[value=ru]"))');
    await evaluate('(() => { const el=Array.from(document.querySelectorAll("select")).find(el=>el.querySelector("option[value=ru]")); el.value="ru"; el.dispatchEvent(new Event("change",{bubbles:true})); })()');
    await key('Escape');
    check('preview reacts to language change', await evaluate('document.querySelector("[data-info-preview]").innerText.includes("Найдено на Steam")'));
    await shot('preview-ru');
    await key(',', 'Comma', 2);
    await wait('!!Array.from(document.querySelectorAll("select")).find(el=>el.querySelector("option[value=en]"))');
    await evaluate('(() => { const el=Array.from(document.querySelectorAll("select")).find(el=>el.querySelector("option[value=en]")); el.value="en"; el.dispatchEvent(new Event("change",{bubbles:true})); })()');
    await key('Escape');
    await key('h', 'KeyH', 2);
    check('preview discreet blur', await evaluate('getComputedStyle(document.querySelector("[data-game-detail] img.object-contain")).filter.includes("blur")'));
    await shot('blur');
    await click('[data-game-detail] img.object-contain');
    await wait(visible('img[alt="fullscreen"]'));
    check('lightbox inherits blur', await evaluate('getComputedStyle(document.querySelector("img[alt=fullscreen]")).filter.includes("blur")'));
    await key('Escape'); await wait('!document.querySelector("img[alt=fullscreen]")');
    check('Esc closes lightbox before preview', await evaluate(visible(preview)));
    await key('h', 'KeyH', 2);
    await key('Escape'); await wait('!document.querySelector("[data-info-preview]")');
    check('results survive Esc', await evaluate('document.querySelectorAll("[data-hit-url]").length'), 4);
    await click(choose); await wait(visible(preview));
    await key('Backspace'); await wait('!document.querySelector("[data-info-preview]")');
    check('Backspace returns from preview with results intact', await evaluate('document.querySelectorAll("[data-hit-url]").length'), 4);
    await click(choose); await wait(visible(preview));
    await send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'back', x: 800, y: 450, clickCount: 1 });
    await send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'back', x: 800, y: 450, clickCount: 1 });
    await wait('!document.querySelector("[data-info-preview]")');
    check('mouse back returns to results', await evaluate(visible('[data-find-results]')));
    await click(choose); await wait(visible(preview));
    await evaluate('window.__findMock.saveError=true');
    await click('[data-preview-save]'); await wait('!document.querySelector("[data-preview-save]").disabled');
    check('save error keeps preview and library', await snapshot(), original);
    await evaluate('window.__findMock.saveError=false');
    await click('[data-preview-save]'); await wait('!document.querySelector("[data-info-preview]")');
    const saved = (await snapshot()).find(g=>g.id==='g01');
    check('saved title', saved.title, 'Moonlit Harbor');
    check('saved local cover', saved.cover_path, 'covers/g01.svg');
    check('saved details, merged tags and folder version', !!saved.description && saved.images.length === 2 && saved.tags.includes('Manual') && saved.tags.includes('Mystery') && saved.version === '0.3');
    check('saved source', saved.sources[0].source, 'Steam');
    check('saved title displayed', await evaluate('document.querySelector("main h1").textContent'), 'Moonlit Harbor');
    await shot('saved');
    await shot('compact', '[data-metadata-panel]');
    check('only ApplyGameInfo writes', await evaluate('!window.__findMock.calls.some(c=>["UpdateGame","UpdateGameMetadata","Launch"].includes(c.name))'));

    await load();
    await click('[data-find-dismiss]');
    check('dismissal persisted', await evaluate('JSON.parse(localStorage.plauncher_noinfo_dismissed).includes("g01")'));
    check('compact search remains', await evaluate(visible('[data-find-tab=name]')));
    await key('Escape');
    await click('[data-game-id="g01"]');
    await wait(visible('[data-find-tab=link]'));
    check('dismissed reopens compact link tab', await evaluate('document.querySelector("[data-find-tab=link]").getAttribute("aria-pressed")'), 'true');
    check('prominent dismissal remains hidden', await evaluate('!document.querySelector("[data-find-dismiss]")'));
    await click('[data-find-tab=name]');
    await click('[data-find-source=Steam]');
    check('source preference stored', await evaluate('JSON.parse(localStorage.plauncher_find_sources).includes("Steam")'), false);
    await load('&keep=1');
    // Compact after reload: search still accessible, preferences preserved.
    check('source preference survives reopen', await evaluate('document.querySelector("[data-find-source=Steam]").getAttribute("aria-pressed")'), 'false');
    await click('[data-find-source=F95zone]'); await click('[data-find-source=VNDB]');
    check('empty source set disables search', await evaluate('document.querySelector("[data-find-search]").disabled'));
    await click('[data-find-source=Steam]');
    await evaluate('window.__findMock.empty=true');
    await click('[data-find-search]'); await wait(visible('[data-find-empty]'));
    check('empty result guidance', await evaluate('document.querySelector("[data-find-empty]").innerText.includes("shorter name")'));
    await evaluate('window.__findMock.empty=false; window.__findMock.previewError=true');
    await click('[data-find-search]'); await wait(visible(choose));
    await click(choose); await wait('!document.querySelector("[data-hit-url]").disabled');
    check('preview error leaves results', await evaluate('!document.querySelector("[data-info-preview]")'));
    await evaluate('window.__findMock.previewError=false; window.__findMock.previewDelay=700');
    await click(choose);
    await evaluate('window.__mockEmit("privacy-locked")');
    await delay(900);
    check('late preview discarded after lock', await evaluate('!document.querySelector("[data-game-detail]") && !document.querySelector("[data-info-preview]")'));
    check('no runtime errors', errors, []);
    fs.writeFileSync(path.join(__dirname, 'out', 'find-checks.json'), JSON.stringify({ checks, errors }, null, 2));
    console.log(`PASS: ${checks.length} checks; screenshots in docs/tools/screenshots/out/find-*.png`);
  } finally { ws.close(); }
})().catch(err => { console.error(err); process.exitCode = 1; });
