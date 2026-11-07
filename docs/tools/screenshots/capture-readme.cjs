// Capture only after the fictional UI reaches its requested state; never accept a blank/loading shot.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const [url, output, size, port] = process.argv.slice(2);
const [width, height] = size.split(',').map(Number);
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

(async () => {
  let tabs;
  for (let i = 0; i < 100; i++) {
    try { tabs = await (await fetch(`http://127.0.0.1:${port}/json`)).json(); if (tabs.length) break; } catch {}
    await delay(100);
  }
  assert(tabs?.length, 'Dedicated screenshot browser did not start');
  const ws = new WebSocket(tabs[0].webSocketDebuggerUrl);
  await new Promise(resolve => ws.addEventListener('open', resolve, { once: true }));
  let seq = 0;
  const pending = new Map(), errors = [], external = [];
  ws.addEventListener('message', e => {
    const msg = JSON.parse(e.data);
    if (msg.id) { const p = pending.get(msg.id); pending.delete(msg.id); msg.error ? p.reject(msg.error) : p.resolve(msg.result); }
    if (msg.method === 'Runtime.exceptionThrown') errors.push(msg.params.exceptionDetails.text);
    if (msg.method === 'Network.requestWillBeSent' && /^https?:/.test(msg.params.request.url) && !msg.params.request.url.startsWith(new URL(url).origin + '/')) external.push(msg.params.request.url);
  });
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++seq; pending.set(id, { resolve, reject }); ws.send(JSON.stringify({ id, method, params }));
  });
  const evaluate = async expression => {
    const r = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
    if (r.exceptionDetails) throw Error(JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  try {
    await send('Runtime.enable'); await send('Page.enable'); await send('Network.enable');
    await send('Emulation.setFocusEmulationEnabled', { enabled: true });
    await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false });
    await send('Page.navigate', { url });
    const isUI = url.startsWith('http:');
    let ready = false;
    for (let i = 0; i < 150; i++) {
      ready = await evaluate(isUI ? 'document.body?.dataset.ready === "1"' : 'document.readyState === "complete" && !!document.querySelector(".fan")');
      if (ready) break;
      assert.equal(errors.length, 0, 'Scenario failed: ' + errors.join('; '));
      await delay(100);
    }
    assert(ready, 'Screenshot scenario never became ready: ' + url);
    await evaluate('document.fonts.ready.then(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))))');
    assert.deepEqual(errors, [], 'Browser exceptions');
    assert.deepEqual(external, [], 'Screenshot must use local fictional assets only');
    assert(await evaluate('[...document.images].filter(i=>i.getClientRects().length).every(i=>i.complete && i.naturalWidth>0)'), 'Unloaded or broken visible image');
    const shot = new URL(url).searchParams.get('shot');
    const states = {
      home: 'document.querySelectorAll("main [data-game-id]").length===12',
      detail: '!!document.querySelector("[data-update-found]") && !!document.querySelector("[data-notes-edited]") && !!document.querySelector("[data-launch-details]") && !!document.querySelector("[data-game-list]")',
      discreet: '!!document.querySelector("img.blur-xl, img.blur-2xl")',
      bulk: 'document.body.textContent.includes("Selected: 4")',
      settings: '!!document.querySelector("[role=dialog]") || document.body.textContent.includes("Panic button")',
      find: 'document.querySelectorAll("[data-find-card]").length>0',
      stats: 'document.querySelectorAll("[data-total]").length===3 && !!document.querySelector("[data-stats-top]")',
      launch: 'document.querySelectorAll("[data-launch-menu-file]").length===3',
      editor: '!!document.querySelector("[data-launch-editor]") && !!document.querySelector("[data-launch-candidates] input:checked")',
      smart: 'document.querySelectorAll("[data-smart-list]").length===2 && document.querySelector("[data-smart-banner]").textContent.includes("1 of 3")',
    };
    if (isUI) assert(await evaluate(states[shot]), 'Wrong screenshot state: ' + shot);
    const screenshot = await send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(output, Buffer.from(screenshot.data, 'base64'));
    console.log(`Verified ${shot || 'banner'}: ${width}x${height}, local assets, no browser errors`);
  } finally { ws.close(); }
})().catch(err => { console.error(err); process.exitCode = 1; });
