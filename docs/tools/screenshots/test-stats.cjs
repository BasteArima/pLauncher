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
    fs.writeFileSync(path.join(__dirname, 'out', 'stats-' + name + '.png'), Buffer.from(result.data, 'base64'));
  };
  const check = (name, actual, expected = true) => { assert.deepEqual(actual, expected, name); checks.push(name); };
  const ready = () => wait('!!document.querySelector("[data-total=all]") && !document.querySelector("[data-stats-top] [role=status]")');
  const load = async () => {
    await send('Page.navigate', { url: origin + '/?shot=stats' });
    await wait('document.body?.dataset.ready === "1"');
    await click('[data-nav-statistics]'); await ready();
  };
  const setLanguage = async lang => {
    await key(',', 'Comma', 2);
    await wait('!!Array.from(document.querySelectorAll("select")).find(el=>el.querySelector("option[value=en]"))');
    await evaluate(`(() => { const el=Array.from(document.querySelectorAll('select')).find(el=>el.querySelector('option[value=en]')); el.value=${JSON.stringify(lang)}; el.dispatchEvent(new Event('change',{bubbles:true})); })()`);
    await key('Escape');
  };
  const reveal = async () => {
    await key('h', 'KeyH', 10);
    if (await evaluate(visible('input[type=password]'))) {
      await evaluate(`(() => { const el=document.querySelector('input[type=password]');el.value='1234';el.dispatchEvent(new Event('input',{bubbles:true})); })()`);
      await key('Enter');
    }
  };
  try {
    await send('Runtime.enable'); await send('Page.enable'); await send('Network.enable');
    await send('Emulation.setTimezoneOverride', { timezoneId: 'UTC' });
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
    await load();
    check('whole visible library scope excludes manual/dynamic/pinned hidden games', await evaluate('window.__statsMock.calls.filter(c=>c.method==="library").at(-1).ids'), ['g01','g02','g03','g04','g05','g06','g07','g08','g09']);
    check('all time retains old minutes', await evaluate('Number(document.querySelector("[data-total=all]").dataset.seconds)'), 548400);
    check('fixed month/week totals split the month boundary', await evaluate('[...document.querySelectorAll("[data-total]")].map(e=>Number(e.dataset.seconds))'), [548400,101520,96120]);
    check('disk excludes unknown/missing folders but includes measured zero', await evaluate('Number(document.querySelector("[data-disk-bytes]").dataset.diskBytes)'), 30*1024**3);
    check('52 weekly bars', await evaluate('document.querySelectorAll("[data-statistics] [data-week-seconds]").length'), 52);
    check('no hidden metadata or library controls in statistics', await evaluate('!document.querySelector("[data-statistics]").innerText.includes("SecretEngine") && !Array.from(document.querySelectorAll("main select")).length'));
    check('wide chart has no unnecessary horizontal scroll', await evaluate('(()=>{const e=document.querySelector("[data-week-chart] .overflow-x-auto");return e.scrollWidth<=e.clientWidth+1})()'));
    await shot('overview');
    await click('[data-all-engines]');
    check('all engines expand', await evaluate('document.querySelector("[data-statistics]").innerText.includes("Flash")'));
    await shot('library', '[data-all-engines]');
    await click('[data-stats-period=week]'); await ready();
    await click('[data-stats-period=all]'); await ready();
    check('old game ranks first all-time', await evaluate('document.querySelector("[data-stats-game]").dataset.statsGame'), 'g03');
    await evaluate('document.querySelector("[data-stats-game=g03]").scrollIntoView({block:"center"})');
    await delay(80);
    const beforeScroll = await evaluate('document.querySelector("main").scrollTop');
    await click('[data-stats-game=g03]');
    await wait('!!document.querySelector("[data-sessions-empty]")');
    check('old game has no invented sessions', await evaluate('document.querySelector("[data-session-count]").textContent'), '0');
    check('legacy total remains on game page', await evaluate('document.querySelector("[data-game-detail]").innerText.includes("100")'));
    await shot('legacy', '[data-game-sessions]');
    await key('Escape'); await ready();
    check('period survives game back', await evaluate('document.querySelector("[data-stats-period=all]").getAttribute("aria-pressed")'), 'true');
    check('scroll survives game back', await evaluate(`Math.abs(document.querySelector('main').scrollTop-${beforeScroll})<2`));
    await click('[data-stats-period=month]'); await ready();
    await click('[data-stats-game=g01]'); await wait('document.querySelectorAll("[data-session-row]").length===5');
    check('eight weekly bars on game page', await evaluate('document.querySelectorAll("[data-game-sessions] [data-week-seconds]").length'), 8);
    check('history count and interrupted session', await evaluate('document.querySelector("[data-session-count]").textContent === "7" && document.querySelector("[data-game-sessions]").innerText.includes("Interrupted")'));
    await shot('sessions', '[data-game-sessions]');
    await click('[data-game-sessions] [data-week-seconds]');
    await key('ArrowRight');
    check('keyboard chart navigation and tooltip', await evaluate('document.activeElement.hasAttribute("data-week-seconds") && document.querySelector("[data-game-sessions] [data-week-tooltip]").innerText.includes("2026")'));
    await evaluate('window.__statsMock.sessionError=true;window.__mockEmit("sessions-updated")');
    await wait(visible('[data-sessions-retry]'));
    check('session error clears old counts', await evaluate('!document.querySelector("[data-session-count]")'));
    await evaluate('window.__statsMock.sessionError=false'); await click('[data-sessions-retry]');
    await wait('document.querySelectorAll("[data-session-row]").length===5');
    check('session retry restores latest five', true);
    // The Add link action opens the full editor without changing any game field.
    await evaluate(`Array.from(document.querySelectorAll('[data-game-detail] button')).find(e=>e.textContent.includes('Add link')).dataset.testEdit='1'`);
    await click('[data-test-edit]');
    await wait('!document.querySelector("[data-game-detail]")');
    check('editor omits Sessions', await evaluate('!document.querySelector("[data-game-sessions]")'));
    await key('Escape'); await wait(visible('[data-game-sessions]'));
    await wait('document.querySelectorAll("[data-session-row]").length===5');
    await setLanguage('ru');
    check('session labels react to language', await evaluate('document.querySelector("[data-game-sessions]").innerText.includes("Прервана")'));
    await shot('sessions-ru', '[data-game-sessions]');
    await key('Escape'); await ready();
    check('statistics reacts to language', await evaluate('document.querySelector("[data-statistics] h1").textContent'), 'Статистика');
    await shot('overview-ru');
    await setLanguage('en');
    const oldMonth = await evaluate('Number(document.querySelector("[data-total=month]").dataset.seconds)');
    await evaluate('window.__statsMock.complete("g01")'); await ready();
    await wait(`Number(document.querySelector('[data-total=month]').dataset.seconds)===${oldMonth+120}`);
    check('completed session refreshes statistics', true);
    await reveal(); await ready();
    await wait('Number(document.querySelector("[data-total=all]").dataset.seconds)>1000000');
    check('reveal includes hidden games', await evaluate('window.__statsMock.calls.filter(c=>c.method==="library").at(-1).ids.length'), 12);
    await evaluate('window.__statsMock.delay=500');
    await click('[data-stats-period=week]');
    await reveal();
    check('hide clears cached hidden totals immediately', await evaluate('!document.querySelector("[data-total=all]") && !document.querySelector("[data-statistics]").innerText.includes("SecretEngine")'));
    await ready(); await delay(600);
    check('late revealed response cannot restore hidden totals', await evaluate('Number(document.querySelector("[data-total=all]").dataset.seconds)'), 548520);
    check('hidden engines cleared', await evaluate('!document.querySelector("[data-statistics]").innerText.includes("SecretEngine")'));
    await evaluate('window.__statsMock.delay=50;window.__statsMock.error=true;window.__mockEmit("sessions-updated")');
    await wait(visible('[data-stats-retry]'));
    check('statistics error does not look like zero', await evaluate('!document.querySelector("[data-total=all]")'));
    await evaluate('window.__statsMock.error=false');await click('[data-stats-retry]');await ready();
    await evaluate('window.__statsMock.emptyActivity=true;window.__mockEmit("sessions-updated")');
    await wait(visible('[data-stats-no-activity]'));
    check('no activity preserves all-time total', await evaluate('Number(document.querySelector("[data-total=all]").dataset.seconds)'), 548520);
    await evaluate('window.__statsMock.emptyActivity=false;window.__mockEmit("sessions-updated")'); await ready();
    await send('Emulation.setTimezoneOverride', { timezoneId: 'America/New_York' });
    await evaluate('window.dispatchEvent(new Event("focus"))');
    await wait('window.__statsMock.calls.filter(c=>c.method==="library").at(-1).zone==="America/New_York"'); await ready();
    check('timezone change refetches aggregates', true);
    await send('Emulation.setTimezoneOverride', { timezoneId: 'UTC' });
    await evaluate('window.dispatchEvent(new Event("focus"))'); await ready();
    const calls = await evaluate('window.__statsMock.calls.length');
    await evaluate('window.__statsMock.advanceDay()');
    await wait(`window.__statsMock.calls.length>${calls}`); await ready();
    check('local day change refreshes calendar', true);
    await send('Emulation.setDeviceMetricsOverride', { width: 900, height: 800, deviceScaleFactor: 1, mobile: false });
    await shot('narrow');
    check('no page-wide horizontal overflow', await evaluate('document.querySelector("main").scrollWidth <= document.querySelector("main").clientWidth+1'));
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
    await reveal(); await ready();
    await wait('Number(document.querySelector("[data-total=all]").dataset.seconds)>1000000');
    await evaluate('window.__statsMock.delay=500'); await click('[data-stats-period=all]');
    await evaluate('window.__mockEmit("privacy-panic")'); await delay(700); await ready();
    check('panic discards late hidden results', await evaluate('Number(document.querySelector("[data-total=all]").dataset.seconds)'), 548520);
    await click('[data-stats-period=week]');
    await evaluate('window.__mockEmit("privacy-locked")'); await delay(700);
    await evaluate('window.__mockEmit("session-recording-error","private-session-recording")');
    check('recording error stays silent while locked', await evaluate('!document.body.innerText.includes("private-session-recording")'));
    check('lock clears statistics and discards late result', await evaluate('!document.querySelector("[data-statistics]") && !document.querySelector("[data-game-sessions]")'));
    await evaluate(`(() => { const el=document.querySelector('input[type=password]');el.value='1234';el.dispatchEvent(new Event('input',{bubbles:true})); })()`);await key('Enter');await ready();
    check('unlock reloads visible scope', await evaluate('window.__statsMock.calls.filter(c=>c.method==="library").at(-1).ids.length'), 9);
    // Search exits Statistics but keeps the full visible library available for aggregates.
    const title = await evaluate('(async () => (await window.go.main.App.GetGames()).find(g=>g.id==="g04").title)()');
    await key('f', 'KeyF', 2); await delay(50);
    await send('Input.insertText', { text: title });
    await wait(visible('[data-game-id=g04]'));
    check('search returns to library', await evaluate('!document.querySelector("[data-statistics]")'));
    await click('[data-game-id=g04]'); await wait(visible('[data-sessions-empty]'));
    check('empty untracked game explains recording', await evaluate('document.querySelector("[data-game-sessions]").innerText.includes("cannot be tracked")'));
    await click('[data-nav-statistics]'); await ready();
    check('statistics ignores previous search scope', await evaluate('window.__statsMock.calls.filter(c=>c.method==="library").at(-1).ids.length'), 9);
    await click('[data-stats-game=g01]'); await wait(visible('[data-session-count]'));
    await key('ArrowLeft', 'ArrowLeft', 1); await ready();
    check('Alt+Left returns to Statistics', await evaluate('!document.querySelector("[data-game-detail]")'));
    await key('ArrowRight', 'ArrowRight', 1); await wait(visible('[data-session-count]'));
    check('Alt+Right reopens game', true);
    await send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'back', x: 800, y: 450, clickCount: 1 });
    await send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'back', x: 800, y: 450, clickCount: 1 }); await ready();
    check('mouse back returns to Statistics', await evaluate('!document.querySelector("[data-game-detail]")'));
    await evaluate(`Array.from(document.querySelectorAll('aside button')).find(e=>e.textContent.trim()==='Weekend picks').dataset.testCollection='1'`);
    await click('[data-test-collection]'); await wait(visible('[data-game-id=g01]'));
    check('collection grid is scoped', await evaluate('document.querySelectorAll("main [data-game-id]").length'), 3);
    await click('[data-nav-statistics]'); await ready();
    check('statistics ignores collection scope', await evaluate('window.__statsMock.calls.filter(c=>c.method==="library").at(-1).ids.length'), 9);
    await evaluate('window.__statsMock.delay=500');
    await click('[data-stats-game=g01]'); await wait(visible('[data-game-sessions]'));
    await click('[data-nav-statistics]'); await delay(600); await ready();
    check('late game history discarded after navigation', await evaluate('!document.querySelector("[data-game-sessions]")'));
    await evaluate('window.__mockEmit("session-recording-error", "[sessions.storage] fixture error")');
    await wait('document.body.innerText.includes("Could not read or save play sessions.")');
    check('recording error uses localized toast', true);
    await evaluate('window.__statsMock.emptyLibrary()'); await ready();
    check('empty library has zero totals and guidance', await evaluate('!!document.querySelector("[data-stats-empty]") && Number(document.querySelector("[data-total=all]").dataset.seconds)===0'));
    await shot('empty');
    check('no external requests', external, []);
    check('no runtime errors', errors.length, 0);
    fs.writeFileSync(path.join(__dirname, 'out', 'stats-checks.json'), JSON.stringify(checks, null, 2));
    console.log('PASS: ' + checks.length + ' statistics UI checks');
  } catch (error) {
    await shot('failure').catch(() => {});
    console.error(error); console.error(JSON.stringify(errors)); process.exitCode=1;
  } finally { ws.close(); }
})();
