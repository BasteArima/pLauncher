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
    fs.writeFileSync(path.join(__dirname, 'out', 'smart-lists-' + name + '.png'), Buffer.from(result.data, 'base64'));
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
  const banner = () => evaluate('document.querySelector("[data-smart-banner]")?.textContent');
  const start = () => click('[data-smart-banner] button');
  try {
    await send('Runtime.enable'); await send('Page.enable'); await send('Emulation.setFocusEmulationEnabled', { enabled: true }); await send('Network.enable');
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1100, deviceScaleFactor: 1, mobile: false });
    await load('home');
    check('empty new lists hidden', await evaluate('document.querySelectorAll("[data-smart-list]").length'), 0);
    await load('smart');
    check('two nonempty smart lists', await evaluate('document.querySelectorAll("[data-smart-list]").length'), 2);
    await click('[data-smart-list=nolaunch]');
    check('empty or missing primary only, excluding missing folders and hidden games', await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId).sort()'), ['g01','g02']);
    check('launch banner and action', (await banner()).includes('Find launch files'));
    await start(); await wait('window.__smartMock.detections.length===1');
    await wait('document.body.textContent.includes("Found 1 of 2")');
    check('detect receives visible list only', await evaluate('window.__smartMock.detections[0].sort()'), ['g01','g02']);
    check('missing nonempty path remains listed', await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId)'), ['g02']);
    await shot('launch');
    await click('[data-smart-list=nosource]');
    check('source list excludes hidden games', await evaluate('document.querySelectorAll("main [data-game-id]").length'), 3);
    await evaluate('localStorage.setItem("plauncher_find_sources",JSON.stringify(["Steam","VNDB"]))');
    await start();
    await wait('window.__smartMock.applies.length===1');
    await wait('document.querySelector("[data-smart-banner]").textContent.includes("1 of 3")');
    check('progress and stop visible', (await banner()).includes('Stop'));
    await click('[data-nav-statistics]');
    await wait('window.__smartMock.searches.length===2');
    check('navigation does not stop search', await evaluate('window.__smartMock.active'));
    await click('[data-smart-list=nosource]');
    await wait('document.body.textContent.includes("Applied 1, manual choice needed 1, errors 1")');
    check('only confident match applied', await evaluate('window.__smartMock.applies'), ['g01']);
    check('games searched sequentially, visible only', await evaluate('window.__smartMock.searches.map(s=>s.id)'), ['g01','g02','g03']);
    check('selected search sites respected', await evaluate('window.__smartMock.searches.every(s=>JSON.stringify(s.sources)===JSON.stringify(["Steam","VNDB"]))'));
    check('one second pause between games', await evaluate('window.__smartMock.searches.slice(1).every((s,i)=>s.at-window.__smartMock.searches[i].at>=950)'));
    check('manual and failed games stay in list', await evaluate('[...document.querySelectorAll("main [data-game-id]")].map(e=>e.dataset.gameId).sort()'), ['g02','g03']);
    await click('main [data-game-id=g02]');
    await wait(visible('[data-find-results]'));
    check('manual results restored without another search', await evaluate('window.__smartMock.searches.length'), 3);
    check('ordinary name panel shows saved result', await evaluate('document.querySelectorAll("[data-find-card]").length'), 1);
    check('correct list navigation label', (await list()).startsWith('No source link'));
    await shot('manual', '[data-find-by-name]');
    await key('Escape'); await click('main [data-game-id=g03]');
    await wait(visible('[data-find-failed]'));
    check('site failure available for manual retry', (await evaluate('document.querySelector("[data-find-failed]").textContent')).includes('Steam'));
    await key('Escape'); await setLanguage('ru');
    check('Russian smart list label', (await banner()).includes('Нет ссылки на сайт'));
    await shot('ru');
    await send('Emulation.setDeviceMetricsOverride', { width: 900, height: 900, deviceScaleFactor: 1, mobile: false });
    check('narrow smart banner has no horizontal overflow', await evaluate('document.documentElement.scrollWidth<=innerWidth'));
    await shot('narrow');

    // Stop while the request is in flight: deliberately late search response.
    await load('smart'); await click('[data-smart-list=nosource]');
    await evaluate('window.__smartMock.delay=650'); await start();
    await wait('window.__smartMock.searches.length===1'); await start();
    await delay(900);
    check('stop discards late search reply', await evaluate('window.__smartMock.applies.length'), 0);
    check('stop prevents next game', await evaluate('window.__smartMock.searches.length'), 1);
    check('stop returns banner to idle', !(await banner()).includes('Stop'));
    await click('main [data-game-id=g01]');
    check('late result not cached', await evaluate('!document.querySelector("[data-find-results]")'));

    // Stop during apply and privacy transitions; unlocking cannot revive old work.
    for (const mode of ['stop', 'lock', 'panic']) {
      await load('smart'); await click('[data-smart-list=nosource]');
      await evaluate('window.__smartMock.saveDelay=700'); await start();
      await wait('window.__findMock.calls.some(c=>c.name==="ApplySmartGameInfo")');
      if (mode==='stop') await start();
      else {
        await evaluate(`window.__mockEmit(${JSON.stringify(mode==='lock'?'privacy-locked':'privacy-panic')})`);
        if (mode==='lock') {
          await evaluate(`(() => { const el=document.querySelector('input[type=password]'); el.value='1234'; el.dispatchEvent(new Event('input',{bubbles:true})); })()`);
          await textButton('Unlock','body');
        }
      }
      await delay(1000);
      check(mode+' prevents in-flight apply', await evaluate('window.__smartMock.applies.length'), 0);
      check(mode+' stops before next game', await evaluate('window.__smartMock.searches.length'), 1);
      if (mode!=='stop') {
        await click('main [data-game-id=g01]');
        check(mode+' clears cached private results', await evaluate('!document.querySelector("[data-find-results]")'));
      }
    }
    // A new run can start immediately after cancellation without old cleanup stopping it.
    await load('smart'); await click('[data-smart-list=nosource]');
    await evaluate('window.__smartMock.delay=450'); await start();
    await wait('window.__smartMock.searches.length===1'); await start(); await start();
    await wait('window.__smartMock.applies.length===1');
    check('restart survives late old cleanup', await evaluate('window.__smartMock.active'));
    await start();
    await load('smart'); await click('[data-smart-list=nosource]');
    await evaluate('localStorage.setItem("plauncher_find_sources","[]")'); await start();
    check('no selected sites makes no requests', await evaluate('window.__smartMock.searches.length'), 0);
    check('no sites explains required choice', await evaluate('document.body.textContent.includes("Select at least one")'));

    await load('smart'); await click('[data-smart-list=nosource]');
    await evaluate('window.__smartMock.delay=250'); await start();
    await wait('window.__smartMock.searches.length===1');
    await evaluate(`window.__smartMock.games[0].sources=[{source:'Steam',url:'https://example.invalid/manual'}]; window.__mockEmit('games-updated')`);
    await wait('window.__smartMock.searches.length===3');
    await wait('!window.__smartMock.active');
    check('manual source added during lookup is preserved', await evaluate('window.__smartMock.games[0].sources[0].url'), 'https://example.invalid/manual');
    check('manual edit does not stop remaining games', await evaluate('window.__smartMock.searches.map(s=>s.id)'), ['g01','g02','g03']);

    // Successful actions remove their last visible entry; hidden games cannot keep it open.
    await load('smart');
    await evaluate(`window.__smartMock.games.forEach(g => { if(g.id==='g02') g.launch_missing=false; if(['g02','g03'].includes(g.id)) g.sources=[{source:'Steam',url:'https://example.invalid/known'}]; }); window.__mockEmit('games-updated')`);
    await click('[data-smart-list=nolaunch]'); await start();
    await wait('!document.querySelector("[data-smart-list=nolaunch]")');
    check('last detected launch file hides its list', await evaluate('!document.querySelector("[data-smart-list=nolaunch]")'));
    check('empty launch action disabled', await evaluate('document.querySelector("[data-smart-banner] button").disabled'));
    await click('[data-smart-list=nosource]'); await start();
    await wait('!document.querySelector("[data-smart-list=nosource]")');
    await wait('document.querySelector("[data-smart-banner] button").disabled');
    check('last applied source hides its list', await evaluate('!document.querySelector("[data-smart-list=nosource]")'));
    check('single game completed', await evaluate('window.__smartMock.searches.map(s=>s.id)'), ['g01']);
    check('no external requests', external, []);
    check('no runtime errors', errors.length, 0);
    fs.writeFileSync(path.join(__dirname,'out','smart-lists-checks.json'),JSON.stringify(checks,null,2));
    console.log('PASS: '+checks.length+' smart list UI checks');
  } catch (error) {
    await shot('failure').catch(()=>{}); console.error(error); console.error(JSON.stringify(errors)); process.exitCode=1;
  } finally { ws.close(); }
})();
