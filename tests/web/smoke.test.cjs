// These tests exercise actual DOM/CSS/xterm with a fixture API. They do not
// replace Go HTTP contract tests, Safari or real-device accessibility acceptance.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');

const {scenario} = require('./harness.cjs');
const {openTools, closeTools} = require('./tools.cjs');

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
  assert.match(await native.innerText(), /Codex · ChatGPT desktop/);
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
  await openTools(page, 'diff', true);
  await page.locator('.session-diff .add').waitFor();
  assert.equal(await page.locator('.session-diff .add').innerText(), '+new\n');
  await closeTools(page);
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
   assert.equal(await page.locator('#mode').isVisible(),true);
   assert.equal(await page.locator('#mode [data-mode="term"]').isVisible(),false);
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
    assert.match(await card.innerText(), /8 calls/);
    assert.equal(await card.locator('.steps-summary').count(),0);
    assert.ok((await toggle.boundingBox()).height >= 44);
    await toggle.click();
    assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
    assert.equal(await card.locator('summary').count(),0,'summary-only records have no empty inspectors');
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
    assert.equal(await card.locator('.step-row .activity-icon').count(),4);
    assert.equal(await card.locator('.step-detail').nth(1).innerText(),'Review the layout without edits');
    assert.equal(await card.locator('.step-detail').nth(2).innerText(),zh?'超时设置 30 秒':'Timeout setting 30s');
    const row=card.locator('.step-inspect').first(),summary=row.locator('summary');
    await summary.focus();await page.keyboard.press('Space');
    await page.waitForFunction(()=>document.querySelector('.step-inspect').open);
    assert.equal(await row.locator('.step-meta').innerText(),zh?'工具\nexec_command\n工作目录\n/tmp/synthetic-workspace':'Tool\nexec_command\nWorking directory\n/tmp/synthetic-workspace');
    const input=row.locator('.step-input');assert.equal(await input.textContent(),command);
    assert.equal(await row.locator('img,script').count(),0,'recorded markup must not execute');
    await input.focus();
    // Let the resized view and native focus-scroll state reach a painted frame.
    await input.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    const scrollStart=await input.evaluate(e=>({x:e.scrollLeft,width:e.clientWidth,extent:e.scrollWidth,focused:e===document.activeElement,windowFocused:document.hasFocus(),visibility:document.visibilityState}));
    await page.evaluate(()=>{window.scrollKeyEvents=[];document.addEventListener('keydown',e=>{if(e.key==='ArrowRight')window.scrollKeyEvents.push({target:e.target.className,active:document.activeElement?.className,prevented:e.defaultPrevented});});});
    await page.keyboard.press('ArrowRight');
    await page.waitForFunction(()=>document.querySelector('.step-input').scrollLeft>0).catch(async error=>{throw new Error('Keyboard scroll: '+JSON.stringify({before:scrollStart,events:await page.evaluate(()=>window.scrollKeyEvents),after:await input.evaluate(e=>({x:e.scrollLeft,width:e.clientWidth,extent:e.scrollWidth,focused:e===document.activeElement,windowFocused:document.hasFocus(),visibility:document.visibilityState,active:document.activeElement?.className}))}),{cause:error});});
    assert.ok(await input.evaluate(e=>e.scrollLeft>0),'keyboard scrolls the recorded command');
    // Native keyboard scrolling animates beyond its first nonzero offset.
    await input.evaluate(e=>new Promise(resolve=>{let timer;function done(){e.removeEventListener('scroll',moved);resolve();}function moved(){clearTimeout(timer);timer=setTimeout(done,150);}e.addEventListener('scroll',moved);moved();}));
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
    assert.equal(await page.getByRole('button',{name:zh?'复制为引用':'Copy as quote',exact:true}).count(),0);
    const quote=page.locator('.assistant-message .bub-act').getByRole('button',{name:zh?'复制':'Copy',exact:true}).first();
    await quote.focus();await page.waitForFunction(()=>getComputedStyle(document.activeElement.closest('.bub-act')).opacity==='1');
    await quote.click();
    await page.waitForFunction(()=>window.copyRecords.length===2);
    assert.equal(await page.evaluate(()=>window.copyRecords[1].text),'Ready for inspection.');
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
      assert.ok(bounds.height<=120,JSON.stringify(bounds));
      for(const b of bounds.buttons){assert.ok(b.x>=0&&b.right<=bounds.width+1,JSON.stringify(b));assert.ok(b.height>=32&&b.height<=44,'controls keep usable targets: '+JSON.stringify(b));}
      const modeCenters=await head.locator('.tile-modes button').evaluateAll(bs=>bs.map(b=>b.getBoundingClientRect().top));
      assert.ok(modeCenters.every(y=>Math.abs(y-modeCenters[0])<=1),'all mode choices share one row');
      assert.equal(await head.locator('.cap-chip').isVisible(),false,'the composer makes writable capability visible');
    }
    assert.equal(fixture.state.sends.length,0);
  });
}

scenario('Chat operates an unparsed picker without visiting Terminal', {viewport: {width: 390, height: 844}, locale: 'en-US', hasTouch: true}, async (page, fixture) => {
  fixture.state.interaction = {text: 'Choose projects\n❯ [ ] Alpha '+ 'details '.repeat(40) + '\n  [x] Beta\nSpace to toggle, Enter to submit', options: [], awaiting_reply: true, truncated: false};
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Review notes', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  assert.equal(await page.locator('.chat-interact').count(), 0);
  assert.equal(await page.locator('.interaction-keys').count(), 0);
  const screen = page.locator('#chat-interaction .interaction-screen');
  await screen.waitFor();
  assert.match(await screen.innerText(), /Choose projects/);
  assert.equal(await screen.evaluate(e => e.scrollWidth > e.clientWidth), true);
  await page.locator('#chat-input .cx-keyboard').click();
  assert.equal(await page.locator('#chat-input textarea').isVisible(), true);
  const sent = page.waitForResponse(r => r.url().includes('/api/send') && r.request().method() === 'POST');
  await page.locator('#chat-input .cx-keys button[data-key="Down"]').click();
  await sent;
  assert.deepEqual(fixture.state.sends[0].key_event, {key:'ArrowDown',modifiers:[]});
  assert.equal('enter' in fixture.state.sends[0], false);
  await page.locator('#chat-interaction .interaction-toggle').click();
  assert.equal(await page.locator('#chat-interaction .interaction-body').count(), 0);
  await page.locator('#chat-interaction .interaction-toggle').click();
  await page.locator('#chat-input .tk-entry').click();
  await page.getByRole('dialog', {name:'Terminal keys'}).waitFor();
  assert.equal(await page.locator('#mode [data-mode="chat"]').evaluate(e=>e.classList.contains('on')),true);
});

scenario('Chat shows choices only while waiting and folds an answered prompt', {viewport: {width: 390, height: 844}, locale: 'en-US', hasTouch: true}, async (page, fixture) => {
  fixture.state.interaction = {text: 'Allow test tool?', options: [{n: 1, label: 'Allow once'}], awaiting_reply: true, truncated: false};
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Review notes', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.locator('#chat-interaction .appr-opt').click();
  assert.equal(await page.locator('#chat-interaction .interaction-toggle').getAttribute('aria-expanded'), 'false');
  assert.equal(await page.locator('#chat-interaction .interaction-body').count(), 0);
  assert.equal(fixture.state.sends.length, 1);
  fixture.state.agents = [{pane_id: '%902', session: 'Web QA', agent: 'Claude Code', task: 'Review notes', status: 'working', source: 'tmux'}];
  await page.locator('#chat-interaction').waitFor({state: 'hidden'});
  assert.equal(await page.locator('.chat-interact').count(), 0);
  assert.equal(await page.locator('#chat-input .tk-entry').isVisible(), true);
});


for (const width of [390, 1280]) {
 scenario(`View question reveals the picker without submitting a draft at ${width}px`, {viewport:{width:390,height:844},locale:'en-US'}, async(page,fixture)=>{
  await page.route('**/api/input-capabilities?*',route=>route.fulfill({json:{version:1,can_input:true,context:{boot:'synthetic',pane_binding:'%901',proof:'synthetic',expires_at:Math.floor(Date.now()/1000)+300},keys:[],legacy_events:{},actions:[{id:'codex.view-question',revision:'question-1',count:1,label:'View question',event:{key:'g',modifiers:['control']}}]}}));
  await page.route('**/api/pane?*',route=>route.fulfill({json:{id:'%901',text:'Question: choose a target\n> Preview\n  Production',cols:80}}));
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  if(width>900)await page.setViewportSize({width,height:850});
  const input=page.locator('.cx-input:visible');await input.fill('Unsent synthetic draft');
  await page.locator('.tk-notice button:visible').getByText('1 question · View',{exact:true}).click();
  await page.locator('#pane:visible .xterm-rows').waitFor();
  await page.waitForFunction(()=>document.querySelector('#pane .xterm-rows')?.textContent.includes('choose a target'));
  assert.equal(await input.inputValue(),'Unsent synthetic draft');
  assert.equal(await page.evaluate(()=>localStorage.getItem('gtmux.paneMode')),'chat');
  assert.equal(fixture.state.sends.length,1);
  assert.deepEqual(fixture.state.sends[0].action,{id:'codex.view-question',revision:'question-1'});
  assert.equal(fixture.state.sends[0].text,undefined);assert.equal(fixture.state.sends[0].enter,undefined);
 });
}

scenario('Workspace conversation keeps drafts, shows identities and inspectable calls', {viewport: {width: 1440, height: 1000}, locale: 'en-US', permissions: ['clipboard-read', 'clipboard-write']}, async (page, fixture) => {
  fixture.state.turns['%901'] = [
    {prompt: 'Investigate retries', time: new Date().toISOString(), agent: 'Codex', segments: [{text: 'Inspecting the retry policy.', steps: [{title: 'exec_command', detail: 'rg retry internal/client.go', input:'rg retry internal/client.go', input_kind:'command'}]}, {text: 'The retry budget is shared.\n\nUse a bounded backoff.'}]},
    {prompt: 'Check the cancellation path too.', from: {kind: 'hq', label: 'HQ'}, agent: 'Claude Code', segments: [{text: 'Cancellation now stops pending retries.', steps: [{title: 'Read', detail: 'internal/cancellation.go',input:'internal/cancellation.go',input_kind:'path'}]}]},
  ];
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  await page.locator('.tile-modes [data-m="chat"]').click();
  await page.getByText('Inspecting the retry policy.', {exact: true}).waitFor();
  await page.locator('.tile-chat .avatar img').first().waitFor({state: 'visible'});
  for (const selector of ['.user-message', '.assistant-message']) {
    const stack = page.locator('.tile-chat ' + selector).first();
    assert.equal(await stack.locator('.ububble .bub-act, .abubble .bub-act').count(), 0);
    const body = await stack.locator('.ububble, .abubble').boundingBox();
    const footer = await stack.locator('.message-meta').boundingBox();
    assert.ok(footer.y >= body.y + body.height + 4, 'actions sit below the message background');
    const target = await stack.getByRole('button', {name: 'Copy', exact: true}).boundingBox();
    assert.ok(target.width >= 36 && target.height >= 36);
  }
  assert.equal(await page.locator('.tile-chat .user-message time').count(), 1);

  assert.equal(await page.locator('.tile-chat .sender-hq').innerText(), 'HQ');
  await page.locator('.tool-steps > .steps-toggle').first().click();
  await page.locator('.step-inspect > summary').first().click();
  assert.equal(await page.locator('.step-input').first().isVisible(), true);
  const copyCall = page.locator('.step-row').first().getByRole('button', {name: 'Copy', exact: true});
  assert.equal(await copyCall.innerText(), '');
  assert.equal(await copyCall.locator('svg').count(), 1);
  await copyCall.click();
  await page.locator('.step-row .clipboard-action button[data-state="done"]').first().waitFor();
  assert.equal(await copyCall.getAttribute('data-state'), 'done');
  assert.equal(await copyCall.getAttribute('title'), 'Copied');
  assert.equal(await page.evaluate(() => navigator.clipboard.readText()), 'rg retry internal/client.go');
  const keyboardCopy = page.locator('.tile-chat .assistant-message').first().getByRole('button', {name: 'Copy', exact: true});
  assert.equal(await keyboardCopy.innerText(), '');
  await keyboardCopy.press('Enter');
  await page.locator('.tile-chat .assistant-message .bub-act button[aria-label="Copy"][data-state="done"]').first().waitFor();
  assert.equal(await page.evaluate(() => navigator.clipboard.readText()), 'Inspecting the retry policy.');
  await page.evaluate(() => { window.qaClipboard = navigator.clipboard; Object.defineProperty(navigator, 'clipboard', {configurable: true, value: undefined}); });
  await page.locator('.tile-chat .assistant-message').first().getByRole('button', {name: 'Copy', exact: true}).click();
  assert.equal(await page.evaluate(() => window.qaClipboard.readText()), 'Inspecting the retry policy.');
  const input = page.locator('.tile .cx-input');
  await input.fill('Keep this unsent draft');
  fixture.state.turns['%901'][1].segments.push({text: 'Live update from the agent.'});
  await page.getByText('Live update from the agent.', {exact: true}).waitFor();
  assert.equal(await input.inputValue(), 'Keep this unsent draft');
  await openTools(page, 'trace', true);
  const trace = page.locator('.session-tool .trace-view');
  await trace.locator('.trace-event.tool').first().click();
  assert.match(await trace.locator('.trace-inspector').innerText(), /rg retry/);
  await trace.getByRole('searchbox').fill('cancellation');
  assert.equal(await trace.locator('.trace-event').count(), 3);
  await trace.getByRole('combobox').selectOption('tool');
  assert.equal(await trace.locator('.trace-event').count(), 1);
  assert.match(await trace.locator('.trace-inspector pre').innerText(), /internal\/cancellation/);
  fixture.state.turns['%901'][1].segments.push({text: 'One more live update'});
  await page.waitForTimeout(2800);
  assert.equal(await trace.getByRole('searchbox').inputValue(), 'cancellation');
  assert.equal(await input.inputValue(), 'Keep this unsent draft');
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'web-workspace-trace.png')});
  }
  await closeTools(page);
  await page.locator('.tile-btn[data-action="focus"]').click();
  await page.locator('#mode [data-mode="chat"]').click();
  await openTools(page);
  await page.locator('.session-tool .trace-event.tool').first().click();
  if (process.env.GTMUX_WEB_EVIDENCE) await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'web-trace-desktop.png')});
  await closeTools(page);
  const focusedInput = page.locator('#chat-input .cx-input');
  await focusedInput.fill('Follow up from focused chat');
  await focusedInput.press('ControlOrMeta+Enter');
  await page.waitForFunction(() => document.querySelector('#chat-input .cx-input').value === '');
  assert.equal(fixture.state.sends.at(-1).id, '%901');
  assert.equal(fixture.state.sends.at(-1).text, 'Follow up from focused chat');
  await focusedInput.fill('Draft across modes');
  await page.locator('#mode [data-mode="term"]').click();
  assert.equal(await page.locator('#pane-input .cx-input').inputValue(), 'Draft across modes');
  await page.locator('#mode [data-mode="chat"]').click();
  assert.equal(await focusedInput.inputValue(), 'Draft across modes');
  await page.locator('#chat').getByText('Inspecting the retry policy.', {exact: true}).waitFor();
  if (process.env.GTMUX_WEB_EVIDENCE) await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'web-chat-desktop.png')});
});

scenario('Narrow chat supports input and touch copy while trace remains within viewport', {viewport: {width: 390, height: 844}, locale: 'zh-CN', hasTouch: true}, async (page, fixture) => {
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.getByText('Initial finding', {exact: true}).waitFor();
  const touchCopy = page.locator('#chat .assistant-message').getByRole('button', {name: '复制', exact: true});
  assert.equal(await touchCopy.isVisible(), true);
  assert.equal(await touchCopy.innerText(), '');
  const touchBox = await touchCopy.boundingBox();
  assert.ok(touchBox.width >= 44 && touchBox.height >= 44);
  for (const key of await page.locator('#chat-input .cx-keys button').all()) {
    const box = await key.boundingBox();
    assert.ok(box.width >= 44 && box.height >= 44, 'touch keys have full targets');
  }

  await page.locator('#chat-input .cx-keyboard').click();
  await page.locator('#chat-input .cx-input').fill('继续检查');
  await page.locator('#chat-input .cx-send').click();
  assert.equal(fixture.state.sends.at(-1).text, '继续检查');
  await openTools(page);
  await page.locator('.trace-event.tool').click();
  assert.equal(await page.locator('.trace-inspector pre').innerText(), 'Synthetic detail');
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 390);
  if (process.env.GTMUX_WEB_EVIDENCE) await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'web-trace-mobile.png')});
  assert.equal(await page.locator('#chat-input').isVisible(),false,'Trace keeps its reading space');
  await closeTools(page);
  await page.locator('#chat-input .cx-keyboard').click();
  await page.locator('#chat-input .cx-input').fill('Private draft before revocation');
  fixture.state.deny = true;
  await page.locator('#gate').waitFor();
  assert.equal(await page.locator('#chat').innerText(), '');
  assert.equal(await page.locator('#chat-input .cx-input').inputValue(), '');
});

scenario('Workspace controls support keyboard access and truthful copy feedback', {viewport: {width: 1440, height: 1000}, locale: 'en-US', permissions: ['clipboard-read', 'clipboard-write']}, async (page, fixture) => {
  await page.goto(fixture.url + '/#code=TEST-1234');
  const session = page.locator('.tree-row').filter({hasText: 'Investigate retries'});
  await session.press('Enter');
  await page.locator('.tile .xterm-rows').waitFor();
  for (const selector of ['#rail-collapse', '.tile-btn[data-action="focus"]']) {
    const target = await page.locator(selector).boundingBox();
    assert.ok(target.width >= 36 && target.height >= 36);
    const icon = await page.locator(selector + ' svg').boundingBox();
    assert.equal(icon.width, 20);
  }
  await page.locator('#wb-snap').click();
  assert.equal(await page.locator('#wb-snap').getAttribute('aria-pressed'), 'true');
  await page.locator('#wb-preset').click();
  assert.equal(await page.locator('.pm-empty').innerText(), 'No saved layouts');
  await page.locator('.pm-save').press('Escape');
  assert.equal(await page.locator('#wb-preset').getAttribute('aria-expanded'), 'false');
  assert.equal(await page.locator('#wb-preset').evaluate(e => e === document.activeElement), true);
  await page.locator('#wb-gear').click();
  await page.locator('#appearance-options input:checked').press('Escape');
  assert.equal(await page.locator('#settings').isVisible(), false);
  assert.equal(await page.locator('#wb-gear').getAttribute('aria-expanded'), 'false');
  assert.equal(await page.locator('#wb-gear').evaluate(e => e === document.activeElement), true);
  await page.keyboard.press('ControlOrMeta+k');
  await page.locator('#cmdk-input').waitFor();
  assert.equal(await page.getByRole('option', {selected: true}).count(), 1);
  await page.locator('#cmdk-input').press('ArrowDown');
  assert.equal(await page.locator('#cmdk-input').getAttribute('aria-activedescendant'), 'cmdk-result-1');
  await page.locator('#cmdk-input').press('Tab');
  assert.equal(await page.locator('#cmdk-input').evaluate(e => e === document.activeElement), true);
  await page.locator('#cmdk-input').press('Escape');
  assert.equal(await page.locator('#cmdk').isVisible(), false);
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.locator('#wb-preset').click();
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'workspace-controls.png')});
    await page.locator('#wb-preset').press('Escape');
  }
  await page.locator('.tile-btn[data-action="focus"]').click();
  await page.locator('#mode [data-mode="term"]').click();
  await page.locator('#term .xterm-rows').waitFor();
  const copy = page.locator('#copy-screen');
  await copy.click();
  await page.locator('#copy-screen[data-state="done"]').waitFor();
  assert.match(await page.evaluate(() => navigator.clipboard.readText()), /Synthetic terminal/);
  await page.evaluate(() => {
    Object.defineProperty(navigator, 'clipboard', {configurable: true, value: {writeText: () => Promise.reject(new Error('Clipboard denied'))}});
    document.execCommand = () => false;
  });
  await copy.click();
  await page.locator('#copy-screen[data-state="error"]').waitFor();
  assert.equal(await page.locator('#screen-copy-status').innerText(), 'Copy failed');
  assert.equal(await copy.getAttribute('title'), 'Copy failed');
});

scenario('Live chat shows current work, retains disclosure state and clears after completion or revocation', {viewport: {width: 1440, height: 1000}, locale: 'zh-CN'}, async (page, fixture) => {
  let captures = 0;
  page.on('request', request => {if (new URL(request.url()).pathname.endsWith('/api/pane')) captures++;});
  fixture.state.screens['%901'] = '\u001b[32mRunning current checks\u001b[0m\nInspecting cancellation\n\n';
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  await page.locator('.tile-modes [data-m="chat"]').click();
  const tileLive = page.locator('.tile .chat-live');
  await tileLive.waitFor();
  assert.equal(await tileLive.evaluate(e => e.open), false);
  await tileLive.locator('summary').click();
  await tileLive.locator('pre').filter({hasText: 'Running current checks'}).waitFor();
  assert.doesNotMatch(await tileLive.locator('pre').innerText(), /Initial finding|\u001b/);
  fixture.state.turns['%901'] = [{prompt: 'New transcript content', response: 'Updated recorded reply'},
    ...Array.from({length: 5}, (_, i) => ({prompt: 'Earlier investigation ' + i, response: ('Recorded history for case ' + i + '.\n\n').repeat(10)}))];
  await page.getByText('Updated recorded reply', {exact: true}).waitFor();
  assert.equal(await tileLive.evaluate(e => e.open), true);
  await tileLive.locator('summary').click();
  const capturesAfterCollapse = captures;
  await page.waitForTimeout(2800);
  assert.equal(await tileLive.evaluate(e => e.open), false);
  assert.equal(captures, capturesAfterCollapse, 'collapsed Live does not poll captures');
  await page.locator('.tile-btn[data-action="focus"]').click();
  await page.locator('#mode [data-mode="chat"]').click();
  const live = page.locator('#chat .chat-live');
  assert.equal(await live.evaluate(e => e.open), false);
  await live.locator('summary').press('Enter');
  await live.locator('pre').filter({hasText: 'Running current checks'}).waitFor();
  await page.waitForFunction(() => {const e = document.querySelector('.conv-scroll'); return e.scrollHeight - e.scrollTop - e.clientHeight < 3;});
  await page.locator('#chat-input .cx-input').fill('Keep this live draft');
  await page.locator('.conv-scroll').evaluate(e => {e.scrollTop = 0;});
  fixture.state.screens['%901'] = 'Current check completed\nPreparing the report';
  await live.locator('pre').filter({hasText: 'Preparing the report'}).waitFor();
  assert.equal(await page.locator('.conv-scroll').evaluate(e => e.scrollTop), 0, 'Live updates preserve history reading position');
  assert.equal(await page.locator('#chat-input .cx-input').inputValue(), 'Keep this live draft');
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'chat-live-expanded.png')});
  }
  await page.locator('#back').click();
  await page.waitForFunction(() => document.querySelector('.tile .chat-live')?.open);
  assert.match(await tileLive.locator('summary').innerText(), /收起/);
  await page.locator('.tile-btn[data-action="focus"]').click();
  await live.locator('pre').filter({hasText: 'Preparing the report'}).waitFor();
  let releaseCapture, captureStarted;
  const heldCapture = new Promise(resolve => {releaseCapture = resolve;});
  const startedCapture = new Promise(resolve => {captureStarted = resolve;});
  let holdCapture = true;
  await page.route('**/api/pane?*', async route => {
    if (holdCapture && new URL(route.request().url()).searchParams.get('id') === '%901') {
      captureStarted(); await heldCapture;
      await route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify({text: 'Late capture from previous session'})});
    } else await route.continue();
  });
  await startedCapture;
  fixture.state.turns['%902'] = [{prompt: 'Another task', response: 'Second session reply'}];
  await page.locator('#pane-next').click();
  await page.getByText('Second session reply', {exact: true}).waitFor();
  holdCapture = false; releaseCapture();
  assert.equal(await page.locator('#chat .chat-live').count(), 0);
  assert.equal(await page.getByText('Late capture from previous session', {exact: true}).count(), 0);
  await page.locator('#pane-prev').click();
  await live.locator('pre').filter({hasText: 'Preparing the report'}).waitFor();
  assert.equal(await page.locator('#chat-input .cx-input').inputValue(), 'Keep this live draft');
  const agent = fixture.state.agents.find(a => a.pane_id === '%901');
  agent.status = 'idle'; agent.since++;
  await live.waitFor({state: 'detached'});
  assert.equal(await page.locator('#chat-input .cx-input').inputValue(), 'Keep this live draft');
  agent.status = 'working'; agent.since++;
  await live.waitFor();
  await live.locator('pre').filter({hasText: 'Preparing the report'}).waitFor();
  fixture.state.deny = true;
  await page.locator('#gate').waitFor();
  assert.equal(await page.locator('.chat-live').count(), 0);
  assert.equal(await page.locator('#chat-input .cx-input').inputValue(), '');
});

scenario('Appearance follows the system and persists an explicit choice without recoloring terminals', {viewport: {width: 1440, height: 1000}, locale: 'zh-CN', colorScheme: 'light'}, async (page, fixture) => {
  await page.goto(fixture.url);
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
  await page.getByRole('button', {name: '设置', exact: true}).click();
  assert.equal(await page.locator('#appearance-options input:checked').inputValue(), 'system');
  await page.emulateMedia({colorScheme: 'dark'});
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'dark');
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
  await page.locator('#appearance-light-radio').check();
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
  await page.reload();
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  await page.locator('.tile .xterm-rows').waitFor();
  assert.equal(await page.locator('.tile .xt').evaluate(e => getComputedStyle(e).backgroundColor), 'rgb(23, 23, 26)');
  await page.locator('.tile-modes [data-m="chat"]').click();
  await page.getByText('Initial finding', {exact: true}).waitFor();
  await page.locator('.tile-btn[data-action="focus"]').click();
  await page.locator('#mode [data-mode="chat"]').click();
  assert.match(await page.title(), /Investigate retries · 对话 · gtmux/);
  assert.equal(await page.locator('link[rel="icon"]').getAttribute('type'), 'image/svg+xml');
  assert.equal((await page.request.get(new URL(await page.locator('link[rel="icon"]').getAttribute('href'), fixture.url + '/').href)).status(), 200);
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'chat-light.png')});
  }
  await page.locator('#gear').click();
  await page.locator('#appearance-dark-radio').check();
  await page.locator('#gear').click();
  if (process.env.GTMUX_WEB_EVIDENCE) await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'chat-dark.png')});
  await page.locator('#gear').click();
  await page.locator('#appearance-system-radio').check();
  await page.emulateMedia({colorScheme: 'light'});
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'light');
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
  await page.locator('#gear').click();
  await page.locator('#back').click();
  await page.setViewportSize({width: 390, height: 844});
  await page.locator('#radar').waitFor();
  await page.locator('#gear').click();
  assert.equal(await page.locator('#settings').isVisible(), true);
});

scenario('Trace tool closes details and returns to the original panel draft', {viewport:{width:1440,height:1000},locale:'zh-CN'},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();
  await page.locator('.tile-modes [data-m="chat"]').click();
  await page.locator('.tile .cx-input').fill('Preserve this draft');
  await openTools(page,'trace',true);
  const trace=page.locator('.session-tool .trace-view');
  await trace.locator('.trace-event.tool').click();
  await trace.getByRole('button',{name:'关闭详情',exact:true}).click();
  fixture.state.turns['%901']=[...fixture.turns(),{prompt:'Next turn',response:'An additional reply'}];
  await trace.locator('.trace-event').filter({hasText:'An additional reply'}).waitFor();
  assert.equal(await trace.locator('.trace-inspector').isVisible(),false);
  const list=await trace.locator('.trace-list').boundingBox(), body=await trace.locator('.trace-body').boundingBox();assert.ok(list.width>=body.width-2);
  await page.setViewportSize({width:390,height:844});
  await trace.locator('.trace-event.tool').click();
  assert.equal(await trace.locator('.trace-list').isVisible(),false);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),390);
  await closeTools(page);
  await page.setViewportSize({width:1440,height:1000});
  assert.equal(await page.locator('.tile .cx-input').inputValue(),'Preserve this draft');
  assert.equal(await page.locator('.tile-modes [data-m="chat"]').getAttribute('aria-pressed'),'true');
});

scenario('Composer stages files, annotates images, recalls input and matches app send controls', {viewport: {width: 1440, height: 1000}, locale: 'zh-CN', hasTouch:true}, async (page, fixture) => {
  fixture.state.agents[0].project = 'Synthetic repo';
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  await page.locator('.tile-modes [data-m="chat"]').click();
  const composer = page.locator('.tile .composer'), input = composer.locator('.cx-input');
  await composer.locator('.cx-keyboard').click();
  await input.fill('first line');
  await input.press('Enter');
  assert.equal(await input.inputValue(), 'first line\n');
  assert.equal(fixture.state.sends.length, 0, 'Return defaults to a newline');
  await input.press('ControlOrMeta+Enter');
  await page.waitForFunction(() => !document.querySelector('.tile .cx-send').classList.contains('sending'));
  assert.equal(fixture.state.sends.at(-1).text, 'first line\n');
  await composer.locator('.cx-note').filter({hasText: '已发送'}).waitFor();
  await page.locator('.tile .pending-prompt').waitFor();
  fixture.state.turns['%901'] = fixture.turns().concat([{prompt: 'first line\n', response: 'Matched output', time: new Date().toISOString()}]);
  await page.locator('.tile .pending-prompt').waitFor({state: 'detached'});
  await composer.locator('.cx-keyboard').click();
  for (const key of ['Enter', 'BSpace']) {
    const button = composer.locator('[data-key="' + key + '"]'); await button.click();
    await button.locator(':scope[data-state="done"]').waitFor();
    assert.equal(fixture.state.sends.at(-1).key, key);
  }
  await input.fill('Long message');
  await composer.getByRole('button', {name: '展开编辑', exact: true}).click();
  const editor = page.getByRole('dialog', {name: '撰写', exact: true});
  await editor.getByRole('textbox').press('End');
  await editor.getByRole('textbox').press('Enter');
  await editor.getByRole('textbox').pressSequentially('More detail');
  await editor.getByRole('button', {name: '收起', exact: true}).click();
  assert.equal(await input.inputValue(), 'Long message\nMore detail');
  await composer.getByRole('button', {name: '输入历史', exact: true}).click();
  await page.getByRole('dialog', {name: '输入历史', exact: true}).locator('.recall-text').first().click();
  assert.match(await input.inputValue(), /first line/);
  assert.equal(await input.evaluate(e => e === document.activeElement), true);
  assert.equal(fixture.state.sends.length, 3, 'recall does not send automatically');
  await input.fill('');
  await composer.getByRole('button', {name: '常用语', exact: true}).click();
  const replies = page.getByRole('dialog', {name: '常用语', exact: true});
  await replies.getByRole('button', {name: '新增常用语', exact: true}).click();
  await replies.getByRole('textbox', {name: '常用语内容'}).fill('检查一下测试结果');
  await replies.getByRole('button', {name: '保存常用语', exact: true}).click();
  await replies.locator('.recall-text').click();
  assert.equal(await input.inputValue(), '检查一下测试结果');
  assert.equal(fixture.state.sends.length, 3);
  async function pick(label, files) {
    await composer.getByRole('button', {name: '添加附件', exact: true}).click();
    const chooser = page.waitForEvent('filechooser');
    await page.getByRole('dialog', {name: '添加附件', exact: true}).getByRole('button', {name: label, exact: true}).click();
    await (await chooser).setFiles(files);
  }
  await input.fill('Inspect files');
  await pick('文件', [{name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('Synthetic notes')}, {name: 'remove.txt', mimeType: 'text/plain', buffer: Buffer.from('Discard this file')}]);
  assert.equal(await composer.locator('.cx-attachment').count(), 2);
  await composer.getByRole('button', {name: '移除附件：remove.txt', exact: true}).click();
  const icon = await fs.readFile(path.resolve(__dirname, '../../assets/agent-icons/codex.png'));
  await pick('图片', {name: 'example.png', mimeType: 'image/png', buffer: icon});
  const markup = page.getByRole('dialog', {name: '标注图片', exact: true});
  await markup.getByRole('button', {name: '方框', exact: true}).click();
  const canvas = markup.locator('canvas');
  await page.waitForFunction(() => document.querySelector('.markup-canvas').width > 1 && !document.querySelector('.image-editor .dialog-primary').disabled);
  const box = await canvas.boundingBox();
  const imageSize = await canvas.evaluate(c => ({width: c.width, height: c.height}));
  await page.mouse.move(box.x + box.width * .2, box.y + box.height * .2); await page.mouse.down();
  await page.mouse.move(box.x + box.width * .7, box.y + box.height * .7); await page.mouse.up();
  await markup.getByRole('button', {name: '打码', exact: true}).click();
  await page.mouse.move(box.x + box.width * .3, box.y + box.height * .3); await page.mouse.down();
  await page.mouse.move(box.x + box.width * .6, box.y + box.height * .6); await page.mouse.up();
  await markup.getByRole('button', {name: '完成', exact: true}).click();
  await markup.waitFor({state: 'detached'});
  assert.match(await composer.locator('.cx-attachments').innerText(), /example-marked\.jpg/);
  await page.waitForFunction(() => Array.from(document.querySelectorAll('.cx-preview img')).some(i => i.alt === 'example-marked.jpg' && i.complete));
  const exported = await composer.locator('img[alt="example-marked.jpg"]').evaluate(image => {
    const canvas = document.createElement('canvas'); canvas.width = image.naturalWidth; canvas.height = image.naturalHeight;
    const context = canvas.getContext('2d'); context.drawImage(image, 0, 0);
    return {width: canvas.width, height: canvas.height, pixel: Array.from(context.getImageData(Math.round(canvas.width * .45), Math.round(canvas.height * .45), 1, 1).data)};
  });
  assert.equal(exported.width, imageSize.width); assert.equal(exported.height, imageSize.height);
  assert.ok(exported.pixel.slice(0, 3).every(c => c < 12), 'redaction is flattened into the exported image');
  assert.equal(fixture.state.uploads.length, 0, 'picking and editing do not upload');
  assert.equal(fixture.state.sends.length, 3);
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'composer-staged-attachments.png')});
  }
  await composer.getByRole('button', {name: '发送', exact: true}).click();
  await page.waitForFunction(() => !document.querySelector('.tile .cx-send').classList.contains('sending'));
  assert.deepEqual(fixture.state.uploads.map(f => f.name), ['notes.txt', 'example-marked.jpg']);
  assert.match(fixture.state.sends.at(-1).text, /Inspect files\n\/tmp\/gtmux-test-uploads\/notes.txt\n\/tmp\/gtmux-test-uploads\/example-marked.jpg/);
  assert.equal(await composer.locator('.cx-attachment').count(), 0);
  fixture.state.uploadStatus = 413; fixture.state.expectedHttpErrors.push(413);
  await composer.locator('.cx-keyboard').click();
  await input.fill('Try this');
  await pick('文件', {name: 'large.txt', mimeType: 'text/plain', buffer: Buffer.from('Synthetic server-size refusal')});
  await composer.getByRole('button', {name: '发送', exact: true}).click();
  await composer.locator('.cx-note').filter({hasText: '服务端大小限制'}).waitFor();
  assert.equal(await input.inputValue(), 'Try this');
  assert.equal(await composer.locator('.cx-attachment').count(), 1);
  assert.equal(fixture.state.sends.length, 4, 'failed uploads never submit a partial message');
  await composer.getByRole('button', {name: '移除附件：large.txt', exact: true}).click();
  fixture.state.uploadStatus = 200;
  await pick('文件', {name: 'small.txt', mimeType: 'text/plain', buffer: Buffer.from('Smaller synthetic file')});
  await composer.getByRole('button', {name: '发送', exact: true}).click();
  await page.waitForFunction(() => !document.querySelector('.tile .cx-send').classList.contains('sending'));
  assert.equal(fixture.state.sends.length, 5);
  await page.locator('#wb-gear').click();
  await page.locator('#return-sends').check();
  await page.locator('#wb-gear').click();
  await composer.locator('.cx-keyboard').click();
  await input.fill('Return preference'); await input.press('Enter');
  await page.waitForFunction(() => !document.querySelector('.tile .cx-send').classList.contains('sending'));
  assert.equal(fixture.state.sends.at(-1).text, 'Return preference');
  await composer.locator('.cx-keyboard').click();
  await input.fill('Persistent scoped draft'); await page.reload();
  await page.waitForFunction(() => document.querySelector('.tile .cx-input')?.value === 'Persistent scoped draft');
  assert.equal(await page.evaluate(() => localStorage.getItem('gtmux.returnSends')), '1');
});

scenario('Mobile composer and speaker profiles expose identity without redundant writable badges', {viewport: {width: 390, height: 844}, locale: 'zh-CN', hasTouch: true}, async (page, fixture) => {
  fixture.state.agents[0].project = 'Synthetic repo';
  fixture.state.turns['%901'] = [{prompt: 'A user message', response: 'Codex reply', agent: 'Codex'}, {prompt: 'HQ instruction', from: {kind: 'hq', label: 'HQ'}, response: 'Done', agent: 'Codex'}, {prompt: 'An older tool', response: 'Historical reply', agent: 'Claude Code'}];
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.getByText('Codex reply', {exact: true}).waitFor();
  assert.equal(await page.locator('#cap').isVisible(), false);
  assert.equal(await page.getByText('可输入', {exact: true}).count(), 0);
  assert.equal(await page.getByRole('button', {name: '引用', exact: true}).count(), 0);
  assert.equal(await page.locator('#gear').innerText(), '');
  assert.equal(await page.locator('#chat-input .cx-row').isVisible(), false);
  await page.locator('#chat-input .cx-keyboard').click();
  await page.locator('#chat-input .cx-input').fill('Mobile draft');
  const sendBox = await page.locator('#chat-input .cx-send').boundingBox();
  assert.ok(sendBox.width >= 44 && sendBox.height >= 44);
  await page.locator('#chat .urow .speaker-avatar').first().click();
  const profile = page.getByRole('dialog', {name: '资料', exact: true});
  assert.match(await profile.innerText(), /人类|碳基决策者/);
  await profile.getByRole('button', {name: '关闭', exact: true}).click();
  await page.locator('#chat .arow .speaker-avatar').first().click();
  assert.match(await profile.innerText(), /Codex/);
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'profile-agent-mobile.png')});
  }
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 390);
  await profile.getByRole('button', {name: '关闭', exact: true}).click();
  await page.locator('#chat .urow .speaker-avatar').nth(1).click();
  assert.match(await profile.innerText(), /HQ|协调 Agent/);
  await profile.getByRole('button', {name: '关闭', exact: true}).click();
  await page.locator('#chat .arow .speaker-avatar').nth(2).click();
  assert.match(await profile.innerText(), /Claude Code|这条消息的记录/);
  assert.doesNotMatch(await profile.innerText(), /Synthetic repo/);
  assert.equal(await profile.getByRole('button', {name: '打开会话', exact: true}).count(), 0);
  await profile.getByRole('button', {name: '关闭', exact: true}).click();
  await page.locator('#chat-input .cx-expand').click();
  const editor = page.getByRole('dialog', {name: '撰写', exact: true});
  await editor.getByRole('textbox').fill('Mobile long message');
  await page.setViewportSize({width: 390, height: 500});
  await page.waitForFunction(() => document.querySelector('.compose-editor').getBoundingClientRect().bottom <= window.innerHeight + 1);
  const editorSend = await editor.getByRole('button', {name: '发送', exact: true}).boundingBox();
  assert.ok(editorSend.y + editorSend.height <= 500);
  await page.setViewportSize({width: 390, height: 844});
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 390);
  if (process.env.GTMUX_WEB_EVIDENCE) await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'composer-editor-mobile.png')});
  await editor.getByRole('button', {name: '收起', exact: true}).click();
  assert.equal(await page.locator('#chat-input .cx-input').inputValue(), 'Mobile long message');
  await page.locator('#chat .arow .speaker-avatar').first().click();
  await profile.waitFor();
  fixture.state.deny = true;
  await page.locator('#gate').waitFor();
  assert.equal(await page.locator('.browser-dialog').count(), 0);
  assert.equal(await page.evaluate(() => Object.keys(localStorage).filter(k => k.startsWith('gtmux.compose:')).length), 0);
});

scenario('Clipboard images and dropped files stay staged until an explicit send', {viewport: {width: 1280, height: 900}, locale: 'zh-CN', permissions: ['clipboard-read', 'clipboard-write']}, async (page, fixture) => {
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  await page.locator('.tile-modes [data-m="chat"]').click();
  const composer = page.locator('.tile .composer');
  const png = await fs.readFile(path.resolve(__dirname, '../../assets/agent-icons/codex.png'));
  await page.evaluate(async base64 => {
    const bytes = Uint8Array.from(atob(base64), c => c.charCodeAt(0));
    await navigator.clipboard.write([new ClipboardItem({'image/png': new Blob([bytes], {type: 'image/png'})})]);
  }, png.toString('base64'));
  await composer.getByRole('button', {name: '添加附件', exact: true}).click();
  await page.getByRole('dialog', {name: '添加附件', exact: true}).getByRole('button', {name: '从剪贴板粘贴', exact: true}).click();
  const markup = page.getByRole('dialog', {name: '标注图片', exact: true});
  await markup.getByRole('button', {name: '使用原图', exact: true}).click();
  await composer.locator('.cx-preview img').waitFor();
  const transfer = await page.evaluateHandle(() => {const data = new DataTransfer(); data.items.add(new File(['Synthetic dropped note'], 'dropped.txt', {type: 'text/plain'})); return data;});
  await composer.dispatchEvent('drop', {dataTransfer: transfer});
  assert.equal(await composer.locator('.cx-attachment').count(), 2);
  assert.equal(fixture.state.uploads.length, 0);
  assert.equal(fixture.state.sends.length, 0);
  await composer.locator('.cx-input').fill('Review both attachments');
  await composer.getByRole('button', {name: '发送', exact: true}).click();
  await page.waitForFunction(() => !document.querySelector('.tile .cx-send').classList.contains('sending'));
  assert.equal(fixture.state.uploads.length, 2);
  assert.equal(fixture.state.sends.length, 1);
  assert.match(fixture.state.sends[0].text, /clipboard-.*\.png/);
  assert.match(fixture.state.sends[0].text, /dropped\.txt/);
});
