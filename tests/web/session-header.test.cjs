const {test, after} = require('node:test');
const assert = require('node:assert/strict');
const {chromium} = require('playwright');
const {start} = require('./fixture.cjs');
const {browserPool} = require('./browser-pool.cjs');
const pool = browserPool(() => chromium.launch({headless:true, executablePath:process.env.GTMUX_TEST_CHROMIUM || undefined}));
after(() => pool.close());
const tile = (page, name) => page.locator('.tile').filter({has:page.locator('.tile-name').getByText(name, {exact:true})});
function scenario(name, options, run) {
  test(name, {timeout:45000}, async () => {
    const fixture = await start(), browser = await pool.get(), context = await browser.newContext(options), page = await context.newPage();
    context.setDefaultTimeout(8000);
    const errors=[], outside=[];
    page.on('pageerror', e => errors.push(e.message));
    page.on('console', m => {if (['warning','error'].includes(m.type()) && !/status of 401/.test(m.text())) errors.push(m.text());});
    page.on('request', r => {if (/^https?:/.test(r.url()) && !r.url().startsWith(new URL(fixture.url).origin+'/')) outside.push(r.url());});
    try {
      await run(page, fixture);
      assert.equal(new URL(page.url()).origin, new URL(fixture.url).origin);
      assert.match(await page.title(), / · gtmux$/);
      assert.deepEqual(errors, []);
      assert.deepEqual(outside, []);
      assert.equal(fixture.state.sends.length, 0);
    } finally {await context.close(); await fixture.close();}
  });
}
async function board(page, fixture, rows) {
  fixture.state.agents=rows.map((a,i)=>({pane_id:'%'+(901+i),agent:'Codex',session:'Synthetic headers',since:Math.floor(Date.now()/1000),source:'tmux',...a}));
  await page.addInitScript(tiles => {
    if (window.top === window) localStorage.setItem('gtmux.board',JSON.stringify({railW:232,railCollapsed:true,snap:false,tiles}));
  }, fixture.state.agents.map((a,i)=>({id:a.pane_id,x:8+i*348,y:8,w:340,h:680,mode:'chat'})));
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('.tile-head[data-session-state]').first().waitFor();
}
async function state(element, value) {await element.locator('.tile-head').evaluate((e,s)=>{if(e.dataset.sessionState!==s)throw new Error('unexpected initial header state '+e.dataset.sessionState);},value);}
async function waitState(page, selector, value) {await page.waitForFunction(([s,v])=>document.querySelector(s)?.dataset.sessionState===v,[selector,value]);}
async function contrast(head) {
  const measured = await head.evaluate(e => {
    const rgb = value => value.match(/[\d.]+/g).map(Number);
    const luminance = channels => channels.slice(0, 3).map(v => v / 255)
      .map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4)
      .reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0);
    const style = getComputedStyle(e);
    const labels = e.querySelectorAll('.session-state, .tile-name, .tile-id, .cap-chip, #title, #sub, .conn, .session-actions button, .tile-modes button, #mode button');
    const ratios = [...labels].filter(n => n.getBoundingClientRect().width).map(n => {
      const layers = [];
      for (let parent = n; parent !== e; parent = parent.parentElement) layers.unshift(rgb(getComputedStyle(parent).backgroundColor));
      const bg = layers.reduce((under, [r, g, b, alpha = 1]) => [r, g, b].map((c, i) => c * alpha + under[i] * (1 - alpha)), rgb(style.backgroundColor));
      const a = luminance(rgb(getComputedStyle(n).color)), b = luminance(bg);
      return {label: n.textContent, ratio: (Math.max(a, b) + .05) / (Math.min(a, b) + .05)};
    });
    return {bg: style.backgroundColor, stripe: style.boxShadow, ratios};
  });
  assert.match(measured.bg, /^rgb\(/, 'header background is opaque');
  assert.notEqual(measured.stripe, 'none');
  assert.ok(measured.ratios.length >= 3);
  assert.ok(measured.ratios.every(({ratio}) => ratio >= 4.5), JSON.stringify(measured));
  return measured.bg;
}
for (const theme of ['light','dark']) scenario('WHS-002 mixed panels '+theme,{viewport:{width:1800,height:950},locale:'en-US',colorScheme:theme},async(page,fixture)=>{
  await board(page,fixture,[{task:'Active',status:'working'},{task:'Question',status:'waiting'},{task:'Finished',status:'idle'},{task:'Failed',status:'idle',error:true},{task:'Unknown',status:''}]);
  const backgrounds = [];
  for (const [name,value,label] of [['Active','working','working'],['Question','waiting','needs attention'],['Finished','idle','idle'],['Failed','errored','errored'],['Unknown','unknown','status unavailable']]) {
    const panel=tile(page,name),head=panel.locator('.tile-head');await state(panel,value);
    assert.equal(await head.locator('.session-state').innerText(),label);
    backgrounds.push(await contrast(head));
  }
  assert.equal(new Set(backgrounds).size,5);
  assert.equal(await tile(page,'Question').evaluate(e => getComputedStyle(e).animationName), 'none');
  const active=tile(page,'Active'), draft=active.locator('.cx-input');await draft.fill('Retain this unsent caption');
  const reader = active.locator('.tile-chat');
  await reader.getByText('Initial finding', {exact:true}).waitFor();
  await reader.evaluate(e => { e.dataset.retained = 'same reader'; });
  fixture.state.agents[0].status='waiting';
  await page.waitForFunction(()=>document.querySelector('.tile .tile-head').dataset.sessionState==='waiting');
  await state(tile(page,'Finished'),'idle');await state(tile(page,'Failed'),'errored');
  fixture.state.agents[0].error=true;await waitState(page,'.tile .tile-head','errored');
  fixture.state.agents[0].error=false;fixture.state.agents[0].status='idle';await waitState(page,'.tile .tile-head','idle');
  fixture.state.agents.shift();
  await waitState(page,'.tile .tile-head','unknown');
  assert.equal(await draft.inputValue(),'Retain this unsent caption');
  assert.equal(await reader.getAttribute('data-retained'),'same reader');
  assert.equal(await active.locator('.tile-head .badge').isVisible(),false);
});
scenario('WHS-003 narrow board maximize restore and focused modes',{viewport:{width:1440,height:950},locale:'en-US'},async(page,fixture)=>{
  const name='Long conversation identity '+('readable title '.repeat(10));
  await board(page,fixture,[{task:name,status:'working'}]);
  const panel=page.locator('.tile'),head=panel.locator('.tile-head');
  const original=await panel.evaluate(e=>({width:e.style.width,height:e.style.height}));
  const boxes=await head.evaluate(e=>{const box=n=>{const r=n.getBoundingClientRect();return {x:r.x,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height};};return {head:box(e),identity:box(e.querySelector('.tile-identity')),caption:box(e.querySelector('.session-state')),controls:box(e.querySelector('.tile-window-controls')),buttons:[...e.querySelectorAll('button')].map(box)};});
  assert.ok(boxes.caption.right<=boxes.identity.right+.5); assert.ok(boxes.identity.right<=boxes.controls.x+.5);
  assert.ok(boxes.buttons.every(b=>b.x>=boxes.head.x && b.right<=boxes.head.right+.5 && b.height>=32));
  await head.locator('.tile-name').click();await page.locator('.tile.max').waitFor();await state(panel,'working');
  await head.getByRole('button',{name:'Restore layout'}).click();
  assert.deepEqual(await panel.evaluate(e=>({width:e.style.width,height:e.style.height})),original);
  const draft=panel.locator('.cx-input');await draft.fill('Board draft');
  await head.locator('[data-tool="trace"]').click();await page.locator('.session-tool').waitFor();
  await page.locator('.session-tool > .dialog-head .dialog-close').click();assert.equal(await draft.inputValue(),'Board draft');
  const open = head.getByRole('button',{name:'Open conversation'});
  await page.locator('.session-tool').waitFor({state:'detached'});
  await page.locator('#rail-tab').focus();
  await tabTo(page, open);
  const focus = await open.evaluate(button => {
    const {outlineStyle, outlineWidth, outlineColor} = getComputedStyle(button);
    return {active: document.activeElement === button && button.matches(':focus-visible'), outlineStyle, outlineWidth, outlineColor};
  });
  assert.ok(focus.active && focus.outlineStyle === 'solid' && parseFloat(focus.outlineWidth) >= 2);
  assert.ok(!(await head.evaluate(e => getComputedStyle(e).boxShadow)).includes(focus.outlineColor));
  await page.keyboard.press('Enter');
  await page.locator('#chat').getByText('Initial finding',{exact:true}).waitFor();
  await waitState(page,'#bar','working');
  await contrast(page.locator('#bar'));
  await page.locator('#mode [data-mode="chat"]').focus();
  await page.keyboard.press('Tab');
  assert.ok(await page.locator('#mode [data-mode="term"]').evaluate(button =>
    button.matches(':focus-visible') && parseFloat(getComputedStyle(button).outlineOffset) < 0));
  await page.keyboard.press('Enter');
  await page.locator('#pane').waitFor({state:'visible'});
  let transcripts=0; page.on('request',r=>{if(r.url().includes('/api/transcript?')) transcripts++;});
  fixture.state.agents[0].status='waiting';await waitState(page,'#bar','waiting');
  assert.equal(transcripts,0,'focused Terminal does not poll a hidden Chat');
  const before=await contrast(page.locator('#bar'));
  await page.emulateMedia({colorScheme:'dark'});
  await page.waitForFunction(bg=>getComputedStyle(document.querySelector('#bar')).backgroundColor!==bg,before);
  await contrast(page.locator('#bar'));
  await page.locator('#back').click();await page.locator('#workbench').waitFor({state:'visible'});
  assert.equal(await page.locator('#bar').getAttribute('data-session-state'),null);
});
for (const theme of ['light','dark']) scenario('WHS-003 mobile readonly focused modes '+theme,{viewport:{width:390,height:950},locale:'zh-CN',colorScheme:theme,hasTouch:true},async(page,fixture)=>{
  fixture.state.share.input=false;fixture.state.agents[0].task='手机上很长的会话名称与来源信息';
  await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#radar').getByText(fixture.state.agents[0].task,{exact:true}).click();
  await waitState(page,'#bar','working');
  const caption=page.locator('#session-state');assert.equal(await caption.innerText(),'运行中');
  const box=await caption.boundingBox();assert.ok(box.x>=0 && box.x+box.width<=390);
  await page.locator('#pane-ro').waitFor({state:'visible'});
  assert.equal(await page.locator('#pane-input').isVisible(),false);
  await contrast(page.locator('#bar'));
  const mode=page.locator('#mode [data-mode="chat"]');await mode.click();await page.locator('#chat').waitFor({state:'visible'});
  fixture.state.agents[0].status='waiting';await waitState(page,'#bar','waiting');assert.equal(await caption.innerText(),'待处理');
  await page.locator('#chat-ro').waitFor({state:'visible'});
  assert.equal(await page.locator('#chat-input .cx-input').isVisible(),false);
  await contrast(page.locator('#bar'));
  await page.locator('#back').click();await page.locator('#radar').waitFor({state:'visible'});assert.equal(await page.locator('#bar').getAttribute('data-session-state'),null);
});
scenario('WHS-004 neutral native terminal reduced motion and retired delayed status',{viewport:{width:1440,height:950},locale:'en-US',reducedMotion:'reduce'},async(page,fixture)=>{
  await board(page,fixture,[{task:'Neutral terminal',agent:'',status:'waiting',error:true},{task:'Unverified',status:''},{task:'Needs review',status:'waiting'},{task:'Quiet process',status:'running'}]);
  await state(tile(page,'Neutral terminal'),'terminal');await state(tile(page,'Unverified'),'unknown');
  const running = tile(page,'Quiet process');
  await state(running,'running');
  assert.equal(await running.locator('.session-state').innerText(),'idle');
  const waiting=tile(page,'Needs review');assert.equal(await waiting.evaluate(e=>getComputedStyle(e).animationName),'none');
  assert.equal(await waiting.locator('.tile-head').evaluate(e=>getComputedStyle(e).transitionDuration),'0s');
  await waiting.locator('[data-action="focus"]').click();await page.locator('#chat').waitFor({state:'visible'});
  const held = Promise.withResolvers(), intercepted = Promise.withResolvers(), settled = Promise.withResolvers();
  let holding = true;
  await page.route('**/api/agents', async route => {
    if (!holding) { await route.continue(); return; }
    holding = false;
    const response = await route.fetch();
    intercepted.resolve();
    await held.promise;
    try { await route.fulfill({response}); } catch (error) {
      // An aborted browser request may discard the intercepted response.
      if (!/abort|interception|cancell/i.test(error.message)) throw error;
    } finally { settled.resolve(); }
  });
  try {
    await intercepted.promise;
    await page.locator('#back').click();
    await page.locator('#workbench').waitFor({state:'visible'});
    const home = page.waitForResponse(response => response.url().endsWith('/api/agents') && response.ok());
    held.resolve();
    await settled.promise;
    await (await home).finished();
    assert.equal(await page.locator('#bar').getAttribute('data-session-state'),null);
    assert.equal(await page.locator('#session-state').isVisible(),false);
    fixture.state.agents.push({pane_id:'',agent:'Codex',task:'Native history',source:'native',client:'deepseek-harness',session_id:'synthetic-native-history',home_id:'synthetic-dsh-home',can_read_conversation:true,status:'idle'});
    await page.locator('#rail-tab').click();await page.locator('#tree').getByText('Native history',{exact:true}).dblclick();
    const native=tile(page,'Native history');await state(native,'idle');assert.equal(await native.locator('.cx-input').count(),0);
    await native.locator('[data-action="focus"]').click();await waitState(page,'#bar','idle');
    fixture.state.deny=true;await page.locator('#gate').waitFor({state:'visible'});
    assert.equal(await page.locator('#bar').getAttribute('data-session-state'),null);assert.equal(await page.locator('#session-state').isVisible(),false);
  } finally { held.resolve(); }
});

async function tabTo(page, target) {
  for (let step = 0; step < 8; step++) {
    await page.keyboard.press('Tab');
    if (await target.evaluate(button => document.activeElement === button)) return;
  }
  assert.fail('Tab did not reach Open conversation from the workbench sidebar');
}
