// Compact and prominent metadata headers: English/Russian, down to the native 900px minimum.
// Run against build-ui.cjs with UI_PORT/CDP_PORT (defaults 5393/9334).
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
  try {
    await send('Runtime.enable'); await send('Page.enable'); await send('Emulation.setFocusEmulationEnabled', { enabled: true });
    for (const mode of ['detail', 'find']) {
      await send('Emulation.setDeviceMetricsOverride', {width:1440,height:1000,deviceScaleFactor:1,mobile:false});
      await send('Page.navigate', {url:origin+'/?shot='+mode});
      await wait('document.body?.dataset.ready === "1"');
      for (const language of ['en','ru']) {
        await key(',', 'Comma', 2);
        await wait('!!document.querySelector("select option[value=ru]")');
        await evaluate(`(() => { const el=[...document.querySelectorAll('select')].find(s=>s.querySelector('option[value=ru]')); el.value='${language}'; el.dispatchEvent(new Event('change',{bubbles:true})); })()`);
        await key('Escape');
        for (const width of [1440,1280,1024,900]) {
          await send('Emulation.setDeviceMetricsOverride', {width,height:1000,deviceScaleFactor:1,mobile:false});
          await delay(100);
          await evaluate('document.fonts.ready.then(()=>true)');
          const bounds=await evaluate(`(() => {
            const tabs=document.querySelector('[data-find-tab=name]').parentElement;
            const header=tabs.parentElement, heading=header.querySelector('h3');
            header.scrollIntoView({block:'center'});
            const t=tabs.getBoundingClientRect(), h=heading.getBoundingClientRect(), p=header.getBoundingClientRect();
            return {sameRow:Math.abs((h.top+h.height/2)-(t.top+t.height/2))<2, fits:t.right<=p.right+1 && h.left>=p.left-1 && h.right<t.left && header.scrollWidth<=header.clientWidth+1, singleLine: h.height <= parseFloat(getComputedStyle(heading).lineHeight)+1, unclipped:heading.scrollWidth<=heading.clientWidth+1 && [...tabs.children].every(b=>b.scrollWidth<=b.clientWidth+1)};
          })()`);
          check(`${mode} ${language} ${width}: aligned and unclipped`, bounds, {sameRow:true, fits:true, singleLine:true, unclipped:true});
          await click('[data-find-tab=link]');
          check(`${mode} ${language} ${width}: link tab works`, await evaluate('document.querySelector("[data-find-tab=link]").getAttribute("aria-pressed")==="true"'));
          await click('[data-find-tab=name]');
          check(`${mode} ${language} ${width}: name tab works`, await evaluate('document.querySelector("[data-find-tab=name]").getAttribute("aria-pressed")==="true"'));
          await shot('header-'+mode+'-'+language+'-'+width, '[data-metadata-panel]');
        }
      }
    }
    check('no runtime errors',errors,[]);
    fs.writeFileSync(path.join(__dirname,'out','metadata-layout-checks.json'),JSON.stringify({checks,errors},null,2));
    console.log(`PASS: ${checks.length} metadata layout checks`);
  } finally { ws.close(); }
})().catch(err => { console.error(err); process.exitCode=1; });
