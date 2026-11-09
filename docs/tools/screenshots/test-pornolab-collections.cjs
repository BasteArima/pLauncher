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
    fs.writeFileSync(path.join(__dirname, 'out', 'collection-' + name + '.png'), Buffer.from(result.data, 'base64'));
  };
  const check = (name, actual, expected = true) => { assert.deepEqual(actual, expected, name); checks.push(name); };
  const raw = 'https://pornolab.net/forum/viewtopic.php?t=3006469';
  const preview = '[data-info-preview]', list = '[data-collection-results]';
  const fill = (selector, value) => evaluate(`(() => { const el=document.querySelector(${JSON.stringify(selector)}); el.value=${JSON.stringify(value)}; el.dispatchEvent(new Event('input',{bubbles:true})); })()`);
  const state = expression => evaluate('window.__collectionMock.' + expression);
  const textButton = text => evaluate(`Array.from(document.querySelectorAll('button')).find(el=>el.textContent.trim()===${JSON.stringify(text)}).click()`);
  const load = async (compact = false) => {
    await send('Page.navigate', { url: origin + '/?shot=collection' });
    await wait('document.body?.dataset.ready === "1"'); await wait(visible('[data-find-tab=link]'));
    if (compact) { await click('[data-find-dismiss]'); await key('Escape'); await click('main [data-game-id=g01]'); }
    await click('[data-find-tab=link]');
  };
  const submit = async (url = raw) => { await fill('[data-find-link]', url); await click('[data-find-parse]'); };
  const rows = n => wait(`document.querySelectorAll('[data-collection-card]').length===${n} && !document.querySelector('[data-collection-loading]')`);
  const choose = async (index = 2) => { await click(`[data-collection-use="${index}"]`); await wait(visible(preview)); };
  const language = async lang => {
    await key(',', 'Comma', 2);
    await wait('!!Array.from(document.querySelectorAll("select")).find(el=>el.querySelector("option[value=en]"))');
    await evaluate(`(() => { const el=Array.from(document.querySelectorAll('select')).find(el=>el.querySelector('option[value=en]')); el.value=${JSON.stringify(lang)}; el.dispatchEvent(new Event('change',{bubbles:true})); })()`);
    await key('Escape');
  };
  const unlock = async () => { await fill('input[type=password]', '1234'); await textButton('Unlock'); await wait('!document.querySelector("input[type=password]")'); };
  const startBulk = async () => {
    await key('Escape'); await click('[data-bulk-list=nocover]'); await click('[data-smart-banner] button');
    await wait(visible('.fixed.inset-0[class~="z-[80]"]')); await key('Enter');
  };
  await send('Runtime.enable'); await send('Page.enable');
  await send('Emulation.setFocusEmulationEnabled', { enabled: true }); await send('Page.bringToFront');
  await send('Emulation.setDeviceMetricsOverride', { width: 1426, height: 1000, deviceScaleFactor: 1, mobile: false });
  try {
    for (const compact of [false, true]) {
      await load(compact); await submit(); await rows(2);
      const label = compact ? 'compact: ' : 'prominent: ';
      check(label+'no automatic preview or save', await state('calls.filter(c=>c.name!=="resolve").length'), 0);
      check(label+'rank keeps document identity', await evaluate('[...document.querySelectorAll("[data-collection-card]")].map(e=>e.dataset.collectionCard)'), ['2','1']);
      check(label+'one confident reason', await evaluate('document.querySelectorAll("[data-collection-reason]").length'), 1);
      check(label+'filled confident button', await evaluate('getComputedStyle(document.querySelector("[data-collection-best]")).backgroundColor'), 'rgb(255, 255, 255)');
      check(label+'16:9 thumbnails', await evaluate('[...document.querySelectorAll("[data-collection-card] img")].every(el=>Math.abs(el.clientWidth/el.clientHeight-16/9)<.03)'));
      check(label+'version year developer', await evaluate('document.querySelector("[data-collection-card]").textContent.includes("0.3 · 2026 · Harbor Studio")'));
      await click('[data-collection-open="2"]');
      check(label+'open source preserves selection fragment',await evaluate('window.__findMock.calls.filter(c=>c.name==="OpenURL").at(-1).args[0]'),raw+'#game=2');
      await shot(compact?'compact':'two-en', list);
      await choose(); check(label+'selected metadata in preview', await evaluate('document.querySelector("main h1").textContent'), 'Moonlit Harbor');
      check(label+'preview does not save', await state('saved.length'), 0);
      await key('Escape'); await wait('!document.querySelector("[data-info-preview]")');
      check(label+'Esc retains list and link', await evaluate(`document.querySelectorAll('[data-collection-card]').length===2 && document.querySelector('[data-find-link]').value===${JSON.stringify(raw)}`));
      await choose(1); await click('[data-preview-back]'); await choose();
      await click('[data-preview-save]'); await wait('!document.querySelector("[data-info-preview]")');
      check(label+'saved exact section', await state('saved'), [{ id:'g01', index:2 }]);
      check(label+'saved source identity', await state('games[0].sources[0]'), { source:'Pornolab', url:raw+'#game=2', selected_title:'Moonlit Harbor', last_version:'0.3' });
      check(label+'one page fetch across list/back/save', await state('pageGets'), 1);
    }
    await load(); await state('count=3'); await submit(); await rows(3);
    await shot('three-en', list); await language('ru');
    check('runtime Russian reason', await evaluate('document.querySelector("[data-collection-reason]").textContent'), 'Похоже на вашу папку');
    check('Russian collection action matches specification',await evaluate('document.querySelector("[data-collection-use]").textContent.trim()'),'Использовать');
    await shot('three-ru', list);
    await language('en'); await state('best=false'); await submit(); await rows(3);
    check('unconfident list has no suggestion', await evaluate('!document.querySelector("[data-collection-best]") && !document.querySelector("[data-collection-reason]")'));
    await state('fallback=true'); await submit(); await rows(3);
    check('fallback labels use original index', await evaluate('[...document.querySelectorAll("[data-collection-title]")].map(el=>el.textContent)'), ['Game 2','Game 1','Game 3']);
    check('fallback shows distinguishing description', await evaluate('document.querySelector("[data-collection-card]").textContent.includes("location 2")'));
    await language('ru'); check('fallback Russian label', await evaluate('document.querySelector("[data-collection-title]").textContent'), 'Игра 2');
    await shot('fallback-ru', list);

    await load(); await submit(); await rows(2); await key('h','KeyH',2);
    await send('Input.dispatchMouseEvent',{type:'mouseMoved',x:0,y:0}); await delay(200);
    check('all result covers discreet', await evaluate('[...document.querySelectorAll("[data-collection-card] img")].every(el=>getComputedStyle(el).filter.includes("blur"))'));
    await shot('blur', list); await choose();
    check('preview cover discreet', await evaluate('getComputedStyle(document.querySelector("[data-game-detail] img.object-contain")).filter.includes("blur")'));
    await click('[data-game-detail] img.object-contain'); await wait(visible('img[alt=fullscreen]'));
    check('lightbox cover discreet', await evaluate('getComputedStyle(document.querySelector("img[alt=fullscreen]")).filter.includes("blur")'));
    await key('Escape'); check('lightbox Esc retains preview', await evaluate(visible(preview)));
    await key('Escape'); await key('h','KeyH',2);
    await key(',','Comma',2); await textButton('Privacy'); await textButton('Until hover'); await key('Escape');
    await send('Input.dispatchMouseEvent',{type:'mouseMoved',x:0,y:0}); await delay(200);
    check('hover mode starts blurred', await evaluate('getComputedStyle(document.querySelector("[data-collection-card] img")).filter.includes("blur")'));
    const point = await evaluate('(() => {const r=document.querySelector("[data-collection-card]").getBoundingClientRect(); return {x:r.x+40,y:r.y+40};})()');
    await send('Input.dispatchMouseEvent',{type:'mouseMoved',...point}); await delay(200);
    check('hover reveals only that row', await evaluate('[...document.querySelectorAll("[data-collection-card] img")].map(el=>getComputedStyle(el).filter)'), ['blur(0px)','blur(40px)']);

    await load(); await state('resolveError=true'); await submit(); await wait(visible('[data-collection-error]'));
    check('resolve failure retains input', await evaluate(`document.querySelector('[data-find-link]').value===${JSON.stringify(raw)}`));
    await state('resolveError=false'); await click('[data-find-parse]'); await rows(2);
    await state('previewError="parse.request"'); await click('[data-collection-use="2"]'); await wait('!document.querySelector("[data-collection-use]").disabled');
    check('preview failure keeps choices', await evaluate(visible(list)));
    await state('previewError=""; window.__collectionMock.changed=true'); await click('[data-collection-use="2"]'); await rows(3);
    check('membership change requests selection again', await evaluate('!!document.querySelector("[data-collection-error]") && !document.querySelector("[data-info-preview]")'));
    await delay(3100); await shot('reselect',list); await choose(); await state('saveError="parse.request"'); await click('[data-preview-save]');
    await wait('!document.querySelector("[data-preview-save]").disabled'); check('save failure keeps preview', await evaluate(visible(preview)));
    check('save failure does not write', await state('saved.length'),0);
    await state('saveError="parse.collection_game_missing"'); await click('[data-preview-save]'); await wait('!document.querySelector("[data-info-preview]")'); await rows(3);
    check('missing selected game shows localized guidance', await evaluate('document.querySelector("[data-collection-error]").textContent.includes("choose")'));
    await state('saveError=""'); await choose(1); await click('[data-preview-save]'); await wait('!document.querySelector("[data-info-preview]")');
    check('reselection saves new index', await state('saved[0].index'),1);
    await load(); await submit(); await rows(2); await state('expired=true'); await choose();
    check('expiry without membership change silently previews', await state('pageGets'),2);

    await load(); await submit(raw+'#game=1'); await wait(visible(preview));
    check('new selected link previews requested index', await evaluate('document.querySelector("main h1").textContent'),'Quiet Town');
    await load(); await evaluate(`window.__collectionMock.games[0].sources=[{source:'Pornolab',url:${JSON.stringify(raw+'#game=1')},selected_title:'Moonlit Harbor'}]`);
    // Refresh the library object through the existing focus reload.
    await evaluate('window.__mockEmit("games-updated")'); await delay(150);
    await submit(raw+'#game=1'); await wait(visible(preview));
    check('saved title follows changed document index', await state('calls.find(c=>c.name==="preview").index'),2);
    await load(); await evaluate(`window.__collectionMock.games[0].sources=[{source:'Pornolab',url:${JSON.stringify(raw+'#game=2')},selected_title:'Removed game'}]`);
    await evaluate('window.__mockEmit("games-updated")'); await delay(150); await submit(raw+'#game=2'); await rows(2);
    check('missing saved title never guesses', await state('calls.filter(c=>c.name==="preview").length'),0);
    check('missing saved title exposes reselection', await evaluate(visible('[data-collection-error]')));

    for (const phase of ['resolve','preview','save']) for (const cancel of ['navigate','lock','panic']) {
      await load();
      if (phase!=='resolve') { await submit(); await rows(2); }
      if (phase==='save') await choose();
      await state(`${phase==='resolve'?'resolveDelay':phase==='preview'?'previewDelay':'saveDelay'}=600`);
      if (phase==='resolve') await submit(); else await click(phase==='preview'?'[data-collection-use="2"]':'[data-preview-save]');
      if (cancel==='navigate') {
        await click('[data-nav-statistics]');
      } else { await evaluate(`window.__mockEmit('privacy-${cancel==='lock'?'locked':'panic'}')`); if(cancel==='lock') await unlock(); }
      await delay(750);
      check(`${phase}/${cancel}: no late content`,await evaluate('!document.querySelector("[data-info-preview]") && !document.querySelector("[data-collection-results]")'));
      check(`${phase}/${cancel}: no late save`,await state('saved.length'),0);
    }
    await load(); await state('resolveDelay=600'); await submit(); await click('[data-find-tab=name]'); await delay(750);
    check('tab switch ignores late result',await evaluate('!document.querySelector("[data-collection-results]")'));

    await load(); await submit(); await rows(2); await click('[data-find-tab=name]');
    await fill('[data-find-query]',raw.replace('3006469','3006470')); await click('[data-find-tab=link]');
    check('URL handoff discards previous release rows',await evaluate('!document.querySelector("[data-collection-results]")'));
    await load(); await submit(); await rows(2); await choose(); await state('beginDelay=600');
    await click('[data-preview-save]'); await click('[data-nav-statistics]'); await delay(750);
    check('late begin token never starts save',await state('calls.filter(c=>c.name==="save").length'),0);
    check('late begin token is cancelled',await state('applyActive'),false);
    await load();
    await evaluate(`window.__collectionMock.games[0].sources=[{source:'Pornolab',url:${JSON.stringify(raw+'#game=2')},selected_title:'Moonlit Harbor'}]; window.__mockEmit('games-updated')`);
    await wait(visible('[data-check-source=Pornolab]'));
    await state('checkDelay=600; window.__collectionMock.checkError="parse.collection_game_missing"');
    await click('[data-check-source=Pornolab]');
    const replacement = raw.replace('3006469','3006470');
    await submit(replacement); await rows(2); await delay(750);
    check('late update error preserves replacement URL',await evaluate('document.querySelector("[data-find-link]").value'),replacement);
    check('late update error does not reopen old release',await state('calls.filter(c=>c.name==="resolve").map(c=>c.url)'),[replacement]);
    check('late update error leaves new list usable',await evaluate('!document.querySelector("[data-collection-error]")'));

    await load(); await startBulk(); await wait(visible('[data-toast-action]'));
    check('bulk summary distinguishes manual count',await evaluate('document.querySelector("[data-toast-action]").textContent.trim()'),'Manual choice needed: 1');
    check('bulk skips hidden game and handles independent source',await state('calls.filter(c=>c.name==="reparse").map(c=>c.id)'),['g02','g03']);
    check('manual item has no metadata write',await state('games[1].sources[0].url'),raw);
    await click('[data-toast-action]'); await wait('document.querySelectorAll("main [data-game-id]").length===1');
    check('manual summary opens affected game only',await evaluate('document.querySelector("main [data-game-id]").dataset.gameId'),'g02');
    await shot('manual-list'); await click('main [data-game-id=g02]'); await rows(2);
    check('handoff reuses list without resolve request',await state('calls.filter(c=>c.name==="resolve").length'),0);
    await choose(); await click('[data-preview-save]'); await wait('!document.querySelector("[data-info-preview]")'); await key('Escape');
    check('saved item leaves manual list',await evaluate('document.querySelectorAll("main [data-game-id]").length'),0);
    for (const cancel of ['stop','lock','panic']) {
      await load(); await state('saveDelay=600'); await startBulk(); await wait('window.__collectionMock.calls.some(c=>c.name==="reparse")');
      if(cancel==='stop') await click('[data-smart-banner] button');
      else { await evaluate(`window.__mockEmit('privacy-${cancel==='lock'?'locked':'panic'}')`); if(cancel==='lock') await unlock(); }
      await delay(750);
      check('bulk '+cancel+' skips next item',await state('calls.filter(c=>c.name==="reparse").length'),1);
      check('bulk '+cancel+' drops late manual handoff',await evaluate('!document.querySelector("[data-toast-action]") && !document.querySelector("[data-manual-summary]")'));
    }
    await load(); await submit(); await rows(2);
    await send('Emulation.setDeviceMetricsOverride',{width:900,height:900,deviceScaleFactor:1,mobile:false}); await shot('narrow',list);
    check('narrow list fits viewport',await evaluate('[...document.querySelectorAll("[data-collection-card]")].every(el=>el.getBoundingClientRect().right<=innerWidth)'));
    check('no runtime exceptions',errors,[]);
    fs.writeFileSync(path.join(__dirname,'out','collection-checks.json'),JSON.stringify({checks,errors},null,2));
    console.log(`PASS: ${checks.length} collection checks; screenshots in out/collection-*.png`);
  } finally { ws.close(); }
})().catch(err=>{console.error(err);process.exitCode=1;});
