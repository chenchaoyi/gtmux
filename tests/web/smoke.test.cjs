// These tests exercise actual DOM/CSS/xterm with a fixture API. They do not
// replace Go HTTP contract tests, Safari or real-device accessibility acceptance.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {chromium} = require('playwright');
const {start} = require('./fixture.cjs');

function scenario(name, options, run) {
  test(name, {timeout: 45000}, async () => {
    const fixture = await start();
    let browser, context, page;
    const errors = [];
    try {
      browser = await chromium.launch({headless: true, executablePath: process.env.GTMUX_TEST_CHROMIUM || undefined});
      context = await browser.newContext(options);
      context.setDefaultTimeout(6000);
      await context.tracing.start({screenshots: true, snapshots: true});
      page = await context.newPage();
      page.on('pageerror', error => errors.push(error.message));
      await run(page, fixture);
      assert.equal(await page.title(), 'gtmux');
      assert.equal(errors.length, 0, errors.join('\n'));
    } catch (error) {
      const output = path.join(__dirname, 'results', name.replace(/[^a-z0-9]+/gi, '-'));
      await fs.mkdir(output, {recursive: true});
      if (page) await page.screenshot({path: path.join(output, 'failure.png'), fullPage: true}).catch(() => {});
      if (context) await context.tracing.stop({path: path.join(output, 'trace.zip')}).catch(() => {});
      await fs.writeFile(path.join(output, 'failure.txt'), String(error.stack) + '\nPage errors:\n' + errors.join('\n'));
      throw error;
    } finally {
      if (browser) await browser.close();
      await fixture.close();
    }
  });
}

scenario('Desktop enrollment, sidebar, restore and live chat', {viewport: {width: 1280, height: 850}, locale: 'en-US'}, async (page, fixture) => {
  await page.goto(fixture.url); // Direct path without trailing slash.
  assert.equal(await page.locator('#gate').isVisible(), true);
  await page.locator('#gate-code-input').fill('WRONG-1234');
  await page.locator('#gate-code-go').click();
  await page.locator('#gate-code-why').waitFor();
  assert.equal(await page.locator('#gate-code-go').isEnabled(), true);
  await page.locator('#gate-code-input').fill('TEST-1234');
  await page.locator('#gate-code-go').click();
  await page.locator('.tree-row').first().waitFor();
  const native = page.locator('.tree-row.native');
  assert.match(await native.innerText(), /Status only/);
  await native.dblclick();
  assert.equal(await page.locator('.tile').count(), 0);
  await page.locator('#rail-collapse').click();
  await page.locator('#rail-tab').click();
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  await page.locator('.tile .cx-input').waitFor();
  await page.locator('.tile-modes [data-m="chat"]').click();
  await page.getByText('Initial finding', {exact: true}).waitFor();
  const updated = fixture.turns();
  updated[0].segments[0].text = 'Updated live segment';
  fixture.state.turns['%901'] = updated;
  await page.getByText('Updated live segment', {exact: true}).waitFor();
  await page.locator('.tile-modes [data-m="diff"]').click();
  await page.locator('.tile-diff .add').waitFor();
  assert.equal(await page.locator('.tile-diff .add').innerText(), '+new\n');
  await page.locator('.tile-modes [data-m="term"]').click();
  await page.locator('.tile .xterm-rows').waitFor();
  await page.locator('.tile-name').click();
  for (let i = 0; i < 60; i++) await page.locator('.tile-body').click({position: {x: 100, y: 60}});
  await page.locator('#max-exit').click();
  assert.equal(await page.locator('#board').evaluate(e => e.classList.contains('maximized')), false);
});

scenario('Narrow Chinese read-only share and shell layout', {viewport: {width: 390, height: 844}, locale: 'zh-CN'}, async (page, fixture) => {
  fixture.state.share = {all: false, input: false, panes: [], view_panes: ['%901', '%902', '%903']};
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('.row:not(.native)').first().waitFor();
  assert.equal(new URL(page.url()).hash, '');
  await page.locator('#radar').getByText('Review notes', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.locator('.appr-card').waitFor();
  assert.match(await page.locator('.appr-hint').innerText(), /只读/);
  await page.locator('#mode [data-mode="term"]').click();
  assert.equal(await page.locator('#pane-input').isVisible(), false);
  await page.locator('#back').click();
  await page.locator('#panes-btn').click();
  await page.locator('#panes-search').fill('%904');
  await page.locator('.pb-row').click();
  for (const width of [320, 390, 799]) {
    await page.setViewportSize({width, height: 844});
    await page.waitForFunction(w => document.documentElement.scrollWidth === w, width);
    assert.equal(await page.locator('#mode').isVisible(), false);
  }
  assert.equal(fixture.state.sends.length, 0);
});

scenario('Idle rows never date an unknown completion from window activity', {viewport: {width: 390, height: 844}, locale: 'en-US'}, async (page, fixture) => {
  const now = Math.floor(Date.now() / 1000);
  await page.route('**/api/agents', route => route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify([
    {pane_id: '%901', agent: 'Codex', task: 'Unknown completion', status: 'idle', since: 0, activity_at: now, source: 'tmux'},
    {pane_id: '%902', agent: 'Codex', task: 'Known completion', status: 'idle', since: now - 86400, activity_at: now, source: 'tmux'},
    {pane_id: '%903', agent: 'Codex', task: 'Working fallback', status: 'working', activity_at: now, source: 'tmux'},
  ])}));
  await page.goto(fixture.url + '/#code=TEST-1234');
  const unknown = page.locator('#radar .row').filter({hasText: 'Unknown completion'});
  await unknown.waitFor();
  assert.equal(await unknown.locator('.time').count(), 0);
  assert.equal(await page.locator('#radar .row').filter({hasText: 'Known completion'}).locator('.time').innerText(), '1d');
  assert.match(await page.locator('#radar .row').filter({hasText: 'Working fallback'}).locator('.time').innerText(), /^\d+s$/);
});

scenario('Revocation clears private content and permits re-enrollment', {viewport: {width: 1280, height: 850}, locale: 'en-US'}, async (page, fixture) => {
  await page.goto(fixture.url + '/#code=TEST-1234');
  for (let i = 0; i < 2; i++) {
    await page.locator('.tree-row').first().waitFor();
    await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
    await page.locator('.tile').waitFor();
    fixture.state.deny = true;
    await page.locator('#gate').waitFor();
    assert.equal(await page.locator('.tile').count(), 0);
    assert.equal(await page.locator('#identity').isVisible(), false);
    assert.equal(await page.locator('#gate-code-go').isEnabled(), true);
    fixture.state.deny = false;
    await page.locator('#gate-code-input').fill('TEST-1234');
    await page.locator('#gate-code-go').click();
    await page.locator('#workbench').waitFor();
  }
});

scenario('Slow chat reads stay single and render when they finish', {viewport: {width: 390, height: 844}, locale: 'en-US'}, async (page, fixture) => {
  let release;
  const held = new Promise(resolve => {release = resolve;});
  let calls = 0;
  await page.route('**/api/transcript?*', async route => {
    calls++;
    await held;
    await route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify([{prompt: 'Synthetic slow read', response: 'Delayed current reply'}])});
  });
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.waitForTimeout(3500);
  assert.equal(calls, 1, 'a pending transcript must not start another request');
  release();
  await page.getByText('Delayed current reply', {exact: true}).waitFor();
  assert.equal(await page.locator('#chat').isVisible(), true);
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'web-slow-chat.png')});
  }
});


for (const viewport of [{width:1280,height:850},{width:390,height:844}]) {
 scenario(`DSH external reader and scoped follow ${viewport.width}`,{viewport,locale:'en-US'},async(page,fixture)=>{
   fixture.state.agents=[{agent:'DeepSeek Harness',icon:'synthetic-dsh.png',source:'native',session_id:'synthetic-dsh',home_id:'synthetic-dsh-home',can_read_conversation:true,current_client:'unconfirmed',status:'working',task:'DSH Web research'}];
   await page.goto(fixture.url+'/#code=TEST-1234');
   const row=page.locator(viewport.width>=900?'.tree-row.native':'.row.native');await row.waitFor();await row.click();
   await page.getByText('Initial finding',{exact:true}).waitFor();
   assert.match(await page.locator('.external-controls').innerText(),/Read only/);
   assert.match(await page.locator('.external-controls').innerText(),/Earlier messages omitted/);
   assert.equal(await page.locator('#mode').isVisible(),false);
   assert.equal(await page.locator('#pane-input').isVisible(),false);
   assert.equal(await page.locator('.external-controls input').nth(1).isDisabled(),true);
   await page.locator('.external-controls input').nth(0).check();
   await page.waitForFunction(()=>!document.querySelectorAll('.external-controls input')[1].disabled);
   assert.equal(fixture.state.follow.hq,true);assert.equal(fixture.state.follow.notify,false);assert.equal(fixture.state.follow.knowledge,false);
   await page.locator('.external-controls input').nth(2).check();
   await page.waitForFunction(()=>!document.querySelectorAll('.external-controls input')[0].disabled);
   assert.equal(fixture.state.follow.knowledge,true);
   assert.equal(await page.locator('.chat-state .avatar img').evaluate(img=>img.naturalWidth>0),true);
   assert.equal(fixture.state.sends.length,0);
   await page.screenshot({path:`/tmp/gtmux-dsh-web-${viewport.width}.png`,fullPage:false});
   await page.locator('#back').click();await row.waitFor();
 });
}

for (const width of [320, 1280]) {
  scenario(`Structured activity at ${width}px`, {viewport: {width, height: 850}, locale: 'en-US'}, async (page, fixture) => {
    const steps = [
      ...Array.from({length: 6}, (_, i) => ({kind: 'tool', title: 'js', detail: `Inspect layout ${i + 1}`})),
      {kind: 'tool', title: 'wait', detail: '<img src=x onerror=alert(1)>'},
      {kind: 'tool', title: 'tool_' + 'long'.repeat(40), detail: '/tmp/' + 'unbroken'.repeat(30)},
    ];
    fixture.state.turns['%901'] = [{prompt: 'Inspect commands', response: 'Review activity', segments: [{text: 'Review activity', steps}]}];
    await page.goto(fixture.url);
    await page.locator('#gate-code-input').fill('TEST-1234');
    await page.locator('#gate-code-go').click();
    if (width > 900) {
      // Go through the narrow radar to enter the same focused Chat on either width.
      await page.setViewportSize({width: 390, height: 850});
    }
    await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
    await page.getByRole('button', {name: 'Chat', exact: true}).click();
    if (width > 900) await page.setViewportSize({width, height: 850});
    const card = page.locator('#chat .tool-steps');
    const toggle = card.getByRole('button', {name: 'Activity, 8 calls'});
    await toggle.waitFor();
    assert.equal(await toggle.getAttribute('aria-expanded'), 'false');
    assert.equal(await card.locator('.step-row').count(), 0);
    assert.match(await card.innerText(), /Run code ×6/);
    assert.ok((await toggle.boundingBox()).height >= 44);
    await toggle.click();
    assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
    assert.deepEqual(await card.locator('.step-name').allTextContents(), steps.map(s => s.title));
    assert.equal(await card.locator('img').count(), 0);
    assert.equal(await card.locator('.step-detail').nth(6).innerText(), steps[6].detail);
    const bounds = await card.evaluate(node => ({width: node.clientWidth, scroll: node.scrollWidth}));
    assert.ok(bounds.scroll <= bounds.width + 1, JSON.stringify(bounds));
    fixture.state.turns['%901'][0].segments[0].steps = [...steps, {kind:'tool',title:'exec',detail:'go test ./...'}];
    await card.getByRole('button', {name: 'Activity, 9 calls'}).waitFor();
    assert.equal(await card.locator('.step-row').count(), 9, 'reader expansion survives a live update');
    await card.getByRole('button', {name: 'Activity, 9 calls'}).click();
    assert.equal(await card.locator('.step-row').count(), 0);
  });
}

for (const options of [
  {viewport:{width:320,height:850},locale:'en-US'},
  {viewport:{width:390,height:844},locale:'zh-CN',hasTouch:true,isMobile:true},
  {viewport:{width:1280,height:850},locale:'en-US'},
]) {
  const width=options.viewport.width, zh=options.locale==='zh-CN';
  scenario(`Activity inspector and clipboard at ${width}px`,options,async(page,fixture)=>{
    const command='node check.js --message="<img src=x onerror=alert(1)>" --long='+ 'argument'.repeat(50)+'\n'+Array.from({length:35},(_,i)=>'# recorded line '+i).join('\n');
    const steps=[
      {title:'exec_command',operation:'command',detail:'node check.js',input:command,input_kind:'command',workdir:'/tmp/synthetic-workspace'},
      {title:'spawn_agent',input_kind:'parameters',input:'{"message":"Review the layout without edits"}',detail:'{"message":"Review the layout without edits"}'},
      {title:'wait_agent',input_kind:'parameters',input:'{"timeout_ms":30000}'},
      {title:'read_file',input_kind:'parameters',input:'{"path":"docs/README.md"}',details_truncated:true},
    ];
    fixture.state.turns['%901']=[{prompt:'Inspect the records',segments:[{text:'Ready for inspection.',steps}]}];
    await page.goto(fixture.url+'/#code=TEST-1234');
    if(width>=900)await page.setViewportSize({width:390,height:850});
    await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
    await page.locator('#mode [data-mode="chat"]').click();
    if(width>=900)await page.setViewportSize(options.viewport);
    const card=page.locator('.tool-steps'),toggle=card.locator('.steps-toggle');
    await toggle.waitFor();await toggle.focus();await page.keyboard.press('Enter');
    assert.equal(await toggle.evaluate(e=>e===document.activeElement),true,'group activation retains focus');
    assert.equal(await card.locator('.activity-icon').count(),4);
    assert.equal(await card.locator('.step-detail').nth(1).innerText(),'Review the layout without edits');
    assert.equal(await card.locator('.step-detail').nth(2).innerText(),zh?'超时设置 30 秒':'Timeout setting 30s');
    const row=card.locator('.step-inspect').first(),summary=row.locator('summary');
    await summary.focus();await page.keyboard.press('Space');
    await page.waitForFunction(()=>document.querySelector('.step-inspect').open);
    assert.equal(await row.locator('.step-meta').innerText(),zh?'工具\nexec_command\n工作目录\n/tmp/synthetic-workspace':'Tool\nexec_command\nWorking directory\n/tmp/synthetic-workspace');
    const input=row.locator('.step-input');assert.equal(await input.textContent(),command);
    assert.equal(await row.locator('img,script').count(),0,'recorded markup must not execute');
    await input.focus();
    await input.evaluate(e=>{e.dataset.scrollReady='false';e.addEventListener('scrollend',()=>{e.dataset.scrollReady='true';},{once:true});});
    await page.keyboard.press('ArrowRight');
    await page.waitForFunction(()=>document.querySelector('.step-input').dataset.scrollReady==='true');
    assert.ok(await input.evaluate(e=>e.scrollLeft>0),'keyboard scrolls the recorded command');
    await input.evaluate(e=>{e.scrollLeft=120;e.scrollTop=60;});
    fixture.state.turns['%901'][0].segments[0].steps[1].input='{"message":"Updated review purpose"}';
    await card.getByText('Updated review purpose',{exact:true}).waitFor();
    assert.equal(await row.evaluate(e=>e.open),true,'row expansion survives changed transcript');
    const retained=await input.evaluate(e=>({focus:e===document.activeElement,x:e.scrollLeft,y:e.scrollTop}));
    assert.deepEqual(retained,{focus:true,x:120,y:60});
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'no page overflow');
    await page.evaluate(()=>{
      window.copyRecords=[];
      Object.defineProperty(navigator,'clipboard',{configurable:true,value:undefined});
      document.execCommand=cmd=>{window.copyRecords.push({cmd,text:document.activeElement.value});return true;};
    });
    const copy=row.getByRole('button',{name:zh?'复制':'Copy',exact:true});
    if(options.hasTouch){assert.ok((await copy.boundingBox()).width>=44);assert.equal(await page.locator('.bub-act').first().evaluate(e=>getComputedStyle(e).opacity),'1','touch needs no hover');}
    await copy.click();
    assert.equal(await page.evaluate(()=>window.copyRecords[0].text),command,'copy keeps exact recorded bytes');
    assert.equal(await copy.evaluate(e=>e===document.activeElement),true,'fallback returns focus to the copy control');
    assert.equal(await row.getByRole('status').innerText(),zh?'已复制':'Copied');
    // A rejected API also takes the fallback; both failure paths must stay honest.
    await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:()=>Promise.reject(new Error('denied'))}}));
    const quote=page.locator('.bub-act').getByRole('button',{name:zh?'复制为引用':'Copy as quote',exact:true});
    await quote.focus();await page.waitForFunction(()=>getComputedStyle(document.activeElement.closest('.bub-act')).opacity==='1');
    await quote.click();
    await page.waitForFunction(()=>window.copyRecords.length===2);
    assert.equal(await page.evaluate(()=>window.copyRecords[1].text),'> Ready for inspection.');
    await page.evaluate(()=>{Object.defineProperty(navigator,'clipboard',{configurable:true,value:undefined});document.execCommand=()=>false;});
    await copy.click();assert.equal(await row.getByRole('status').innerText(),zh?'复制失败':'Copy failed');
    await card.locator('.step-inspect').nth(3).locator('summary').click();
    await card.getByText(zh?'此记录已在来源处截断。':'This record was truncated at the source.',{exact:true}).waitFor();
    const collapse=page.getByRole('button',{name:zh?'折叠全部':'Collapse all',exact:true});
    assert.equal(await collapse.evaluate(e=>getComputedStyle(e).borderStyle),'none');
    await collapse.click();assert.equal(await card.locator('.step-row').count(),0);
    assert.equal(fixture.state.sends.length,0,'reading, copying and folding never send input');
  });
}

for (const locale of ['en-US','zh-CN']) {
  scenario(`Compact tile controls stay readable in ${locale}`,{viewport:{width:1280,height:850},locale},async(page,fixture)=>{
    await page.goto(fixture.url+'/#code=TEST-1234');
    await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();
    const head=page.locator('.tile-head');await head.waitFor();
    for(const width of [360,280,600]) {
      await page.locator('.tile').evaluate((e,w)=>{e.style.width=w+'px';},width);
      const bounds=await head.evaluate(e=>{
        const r=e.getBoundingClientRect();return{width:e.clientWidth,scroll:e.scrollWidth,height:r.height,buttons:[...e.querySelectorAll('button')].map(b=>{const x=b.getBoundingClientRect();return{x:x.left-r.left,right:x.right-r.left,height:x.height,centerY:(x.top+x.bottom)/2,text:b.textContent};})};
      });
      assert.ok(bounds.scroll<=bounds.width+1,JSON.stringify(bounds));
      assert.ok(bounds.height<=80,JSON.stringify(bounds));
      for(const b of bounds.buttons){assert.ok(b.x>=0&&b.right<=bounds.width+1,JSON.stringify(b));assert.ok(b.height<=24,'button labels stay on one line: '+JSON.stringify(b));}
      assert.ok(bounds.buttons.every(b=>Math.abs(b.centerY-bounds.buttons[0].centerY)<=1),'mode/fullscreen/close controls wrap as one group');
      assert.equal(await head.locator('.cap-chip').isVisible(),true,'input scope stays visible');
    }
    assert.equal(fixture.state.sends.length,0);
  });
}

scenario('Chat operates an unparsed picker without visiting Terminal', {viewport: {width: 390, height: 844}, locale: 'en-US'}, async (page, fixture) => {
  fixture.state.interaction = {text: 'Choose projects\n❯ [ ] Alpha '+ 'details '.repeat(40) + '\n  [x] Beta\nSpace to toggle, Enter to submit', options: [], awaiting_reply: true, truncated: false};
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Review notes', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  const screen = page.locator('#chat-interaction .interaction-screen');
  await screen.waitFor();
  assert.match(await screen.innerText(), /Choose projects/);
  assert.equal(await screen.evaluate(e => e.scrollWidth > e.clientWidth), true);
  assert.equal(await page.locator('#chat-input textarea').isVisible(), true);
  const sent = page.waitForResponse(r => r.url().includes('/api/send') && r.request().method() === 'POST');
  await page.locator('#chat-interaction .interaction-keys').getByRole('button', {name: 'Space', exact:true}).click();
  await sent;
  assert.deepEqual(fixture.state.sends[0].key_event, {key:'Space',modifiers:[]});
  assert.equal('enter' in fixture.state.sends[0], false);
  await page.locator('#chat-interaction .interaction-toggle').click();
  assert.equal(await page.locator('#chat-interaction .interaction-body').count(), 0);
  await page.locator('#chat-interaction .interaction-toggle').click();
  await page.locator('#chat-interaction .interaction-keys').getByRole('button', {name:'More keys',exact:true}).click();
  await page.getByRole('dialog', {name:'Terminal keys'}).waitFor();
  assert.equal(await page.locator('#mode [data-mode="chat"]').evaluate(e=>e.classList.contains('on')),true);
});
