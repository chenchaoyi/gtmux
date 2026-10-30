// Actual embedded UI with public transcript-shaped synthetic data only.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');
const {openTools, closeTools} = require('./tools.cjs');

const command = 'rg -n "retry|cancel" internal/client\n  printf "保持缩进\\n"';
function record(agent, session = 'synthetic-current', prompt = 'Review retries') {
  return {session_id: session, agent, time: '2026-10-28T14:20:00Z', prompt,
    segments: [{text: 'Inspecting cancellation', steps: [{title: 'shell', detail: 'Inspect retry code', input: command, input_kind: 'command', workdir: '/workspace/retry-demo', operation: 'shell'}]},
      {text: '## Review result\n\n**Cancellation** stops pending retries.'}]};
}
async function openTrace(page, fixture, width, task = 'Trace QA') {
  await page.goto(fixture.url + '/#code=TEST-1234');
  if (width >= 900) {
    await page.locator('#tree').getByText(task, {exact:true}).dblclick();
    await page.locator('.tile-modes [data-m="chat"]').click();
    await page.locator('.tile-btn[data-action="focus"]').click();
  } else await page.locator('#radar').getByText(task, {exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await openTools(page);
  await page.locator('.session-tool .trace-event').first().waitFor();
  return page.locator('.session-tool .trace-view');
}
function agentFixture(fixture, agent = 'Codex', turns = [record(agent)]) {
  fixture.state.agents = [{pane_id:'%901', agent, task:'Trace QA', session:'QA', status:'working', since:Math.floor(Date.now()/1000), source:'tmux'}];
  fixture.state.turns['%901'] = turns;
}
async function evidence(page, name) {
  if (!process.env.GTMUX_WEB_EVIDENCE) return;
  await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive:true});
  await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,name+'.png')});
}

for (const agent of ['Codex','Claude Code','Kimi','OpenCode','DeepSeek Harness']) {
  for (const width of [390,1440]) {
    scenario(`Trace public calls for ${agent} at ${width}`, {viewport:{width,height:900},locale:'zh-CN',hasTouch:width<700}, async(page,fixture)=>{
      agentFixture(fixture,agent);
      const trace=await openTrace(page,fixture,width);
      assert.match(await trace.locator('.trace-status').innerText(),/运行中/);
      assert.equal(await page.locator('#chat-input').isVisible(),false);
      await trace.locator('.trace-event.tool').click();
      assert.equal(await trace.locator('.trace-toolbar').isVisible(),width>=700);
      assert.equal(await trace.locator('.trace-overview').isVisible(),width>=700);
      assert.equal(await trace.locator('.trace-source').innerText(),command);
      assert.match(await trace.locator('.trace-workdir').innerText(),/\/workspace\/retry-demo/);
      assert.equal(await trace.locator('.trace-list').isVisible(),width>=700);
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
      await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.traceCopy=text;}}}));
      await trace.getByRole('button',{name:'复制',exact:true}).click();
      assert.equal(await page.evaluate(()=>window.traceCopy),command);
      await trace.getByRole('button',{name:'关闭详情',exact:true}).click();
      assert.equal(await trace.locator('.trace-toolbar').isVisible(),true);
      assert.equal(await trace.locator('.trace-inspector').isVisible(),false);
      assert.equal(await trace.locator('.trace-event.tool').evaluate(e=>e===document.activeElement),true);
      assert.equal(fixture.state.sends.length,0);
      if(agent==='Codex') await evidence(page,'trace-'+width+'-list');
    });
  }
}

for (const width of [320,768]) {
  scenario(`Trace responsive controls, reading and empty filters at ${width}`, {viewport:{width,height:844},locale:'zh-CN',hasTouch:true}, async(page,fixture)=>{
    agentFixture(fixture);
    const trace=await openTrace(page,fixture,width);
    await trace.locator('.trace-event.tool').click();
    const detail=await trace.locator('.trace-inspector').boundingBox();
    assert.ok(detail.x>=0&&detail.x+detail.width<=width+1);
    for(const control of await trace.locator('.trace-detail-head button').all()) {
      const box=await control.boundingBox();assert.ok(box.width>=44&&box.height>=44);
    }
    await trace.getByRole('button',{name:'关闭详情',exact:true}).click();
    await trace.getByRole('searchbox',{name:'搜索轨迹',exact:true}).fill('no match at all');
    await trace.getByText('没有匹配的记录。',{exact:true}).waitFor();
    assert.equal(await trace.locator('.trace-inspector').isVisible(),false);
    await trace.getByRole('button',{name:'清除筛选',exact:true}).click();
    assert.equal(await trace.locator('.trace-event').count(),4);
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
  });
}

scenario('Trace preserves selected calls across history prepend, append and status changes', {viewport:{width:1440,height:900},locale:'en-US'},async(page,fixture)=>{
  const initial=record('Codex');agentFixture(fixture,'Codex',[initial]);
  const trace=await openTrace(page,fixture,1440);
  await trace.locator('.trace-event.tool').click();
  const key=await trace.locator('.trace-event.tool').getAttribute('data-key');
  fixture.state.turns['%901']=[{...record('Claude Code','previous','Older task'),time:'2026-10-27T12:00:00Z'},initial,record('Codex','synthetic-current','New task')];
  await trace.locator('.trace-count').filter({hasText:'12 records'}).waitFor();
  assert.equal(await trace.locator('.trace-event.on').getAttribute('data-key'),key);
  assert.equal(await trace.locator('.trace-source').innerText(),command);
  assert.equal(await trace.locator('.trace-session option').count(),3);
  const old=await trace.locator('.trace-session option').nth(1).getAttribute('value');
  await trace.locator('.trace-session').selectOption(old);
  assert.equal(await trace.locator('.trace-event').count(),4);
  assert.equal(await trace.locator('.trace-status').innerText(),'History');
  assert.equal(await trace.locator('.trace-latest').isVisible(),false);
  await trace.locator('.trace-session').selectOption('all');
  fixture.state.agents[0].status='idle';
  await trace.locator('.trace-status').filter({hasText:'idle'}).waitFor();
  await evidence(page,'trace-hq-history');
});

scenario('Trace pauses a streamed detail while text is selected and resumes after selection clears', {viewport:{width:1280,height:850},locale:'en-US'},async(page,fixture)=>{
  agentFixture(fixture,'Codex',[{session_id:'stream',prompt:'Work',segments:[{text:'Selected original reply'}]}]);
  const trace=await openTrace(page,fixture,1280);
  await trace.locator('.trace-event.assistant').click();
  await page.evaluate(()=>{const text=document.querySelector('.session-tool .trace-prose p').firstChild;const range=document.createRange();range.setStart(text,0);range.setEnd(text,8);const selection=getSelection();selection.removeAllRanges();selection.addRange(range);});
  fixture.state.turns['%901'][0].segments[0].text='Selected original reply with a new streamed ending';
  await page.waitForTimeout(3000);
  assert.equal(await trace.locator('.trace-prose').innerText(),'Selected original reply');
  assert.equal(await page.evaluate(()=>getSelection().toString()),'Selected');
  await page.evaluate(()=>getSelection().removeAllRanges());
  await trace.locator('.trace-prose').getByText('Selected original reply with a new streamed ending',{exact:true}).waitFor();
  await page.evaluate(()=>{const text=document.querySelector('.session-tool .trace-prose p').firstChild;const range=document.createRange();range.selectNodeContents(text);getSelection().addRange(range);});
  await trace.locator('.trace-event.user').click();
  assert.equal(await trace.locator('.trace-detail-title').innerText(),'User message');
  assert.equal(await trace.locator('.trace-prose').innerText(),'Work','explicit navigation supersedes an old text selection');
});

scenario('Trace bounds long-history DOM, searches all records and reaches the final call with a keyboard', {viewport:{width:1280,height:850},locale:'en-US'},async(page,fixture)=>{
  const turns=Array.from({length:2501},(_,i)=>({session_id:'long',time:new Date(1793180000000+i*1000).toISOString(),prompt:'Request '+i,segments:[{steps:[{title:'Read',detail:'record '+i}]}]}));
  turns.at(-1).segments[0].steps[0].input='Final uniquely searchable command';
  agentFixture(fixture,'Codex',turns);
  const trace=await openTrace(page,fixture,1280);
  assert.equal(await trace.locator('.trace-count').innerText(),'5002 records');
  assert.ok(await trace.locator('.trace-event').count()<=200);
  assert.ok(await trace.locator('.trace-mark').count()<=120);
  await trace.locator('.trace-scrubber').focus();await page.keyboard.press('Home');
  assert.equal(await trace.locator('.trace-event.on').innerText(),'Message\nUser message\nRequest 0');
  await page.keyboard.press('End');
  assert.match(await trace.locator('.trace-event.on').innerText(),/Final uniquely searchable command/);
  assert.equal(await trace.locator('.trace-source').innerText(),'Final uniquely searchable command');
  await trace.getByRole('searchbox',{name:'Search trace',exact:true}).fill('uniquely searchable');
  assert.equal(await trace.locator('.trace-event').count(),1);
  assert.equal(await trace.locator('.trace-count').innerText(),'1 / 5002 records');
});

scenario('Trace handles tools-only, old summaries, missing clocks and no records without invented data', {viewport:{width:390,height:844},locale:'en-US',hasTouch:true},async(page,fixture)=>{
  agentFixture(fixture,'Kimi',[{session_id:'minimal',segments:[{steps:[{title:'unknown-provider-tool',detail:'Recorded summary only',details_truncated:true}]}]}]);
  const trace=await openTrace(page,fixture,390);
  await trace.locator('.trace-event.tool').click();
  assert.equal(await trace.locator('.trace-source').innerText(),'Recorded summary only');
  assert.match(await trace.locator('.trace-truncated').innerText(),/recorded portion/);
  assert.doesNotMatch(await trace.locator('.trace-meta').innerText(),/Invalid|1970|Turn started/);
  fixture.state.turns['%901']=[];
  await trace.getByText('No conversation records yet.',{exact:true}).waitFor();
  assert.equal(await trace.locator('.trace-inspector').isVisible(),false);
  assert.equal(await trace.locator('.trace-toggle').isDisabled(),true);
});

scenario('External DSH Trace stays read-only, scoped and clears on revocation', {viewport:{width:390,height:844},locale:'zh-CN',hasTouch:true},async(page,fixture)=>{
  fixture.state.agents=[{source:'native',agent:'DeepSeek Harness',task:'External Trace QA',session_id:'synthetic-dsh',home_id:'synthetic-dsh-home',can_read_conversation:true,status:'working',current_client:'unconfirmed'}];
  const requests=[];page.on('request',r=>{if(r.url().includes('/api/'))requests.push(r.url());});
  await page.goto(fixture.url+'/#code=TEST-1234');
  assert.match(await page.locator('.row.native').innerText(),/只读/);
  await page.locator('.row.native').click();
  await openTools(page);
  const trace=page.locator('.session-tool .trace-view');await trace.locator('.trace-event.tool').waitFor();
  assert.equal(await page.locator('#mode [data-mode="term"]').isVisible(),false);
  assert.equal(await page.locator('#chat-input').isVisible(),false);
  await trace.locator('.trace-event.tool').click();
  assert.ok(requests.some(url=>url.includes('/session/transcript?session_id=synthetic-dsh&home_id=synthetic-dsh-home')));
  assert.ok(requests.every(url=>!url.includes('/api/pane?')&&!url.includes('/api/send')));
  fixture.state.deny=true;
  await page.locator('#gate').waitFor();
  assert.equal(await page.locator('#chat').innerText(),'');
  assert.equal(await page.locator('.trace-view').count(),0);
  assert.equal(fixture.state.sends.length,0);
});

scenario('A delayed Trace response cannot replace the next pane or its mode', {viewport:{width:1280,height:850},locale:'en-US'},async(page,fixture)=>{
  agentFixture(fixture,'Codex',[record('Codex','old-session','Old task')]);
  fixture.state.agents.push({pane_id:'%902',agent:'Claude Code',task:'Next trace session',session:'QA',status:'idle',source:'tmux'});
  fixture.state.turns['%902']=[{session_id:'new-session',agent:'Claude Code',prompt:'New task',response:'New owner response'}];
  let hold=false,release,started;
  const held=new Promise(r=>{release=r;}),pending=new Promise(r=>{started=r;});
  await page.route('**/api/transcript?*',async route=>{
    if(hold&&new URL(route.request().url()).searchParams.get('id')==='%901') { started();await held;await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify([{prompt:'Late old task',response:'Late old response'}])}).catch(()=>{}); }
    else await route.continue();
  });
  await openTrace(page,fixture,1280);hold=true;await pending;
  await closeTools(page);
  await page.locator('#pane-next').click();
  await openTools(page);
  await page.locator('.session-tool .trace-event').filter({hasText:'New owner response'}).waitFor();
  assert.equal(await page.locator('#mode [data-mode="chat"]').getAttribute('aria-pressed'),'true');
  hold=false;release();await page.waitForTimeout(150);
  assert.match(await page.locator('.session-tool').innerText(),/New owner response/);
  assert.doesNotMatch(await page.locator('.session-tool').innerText(),/Late old response|Late old task/);
  assert.equal(fixture.state.sends.length,0);
});

scenario('External Trace keeps same-ID conversations in different homes separate', {viewport:{width:390,height:844},locale:'en-US',hasTouch:true},async(page,fixture)=>{
  fixture.state.agents=['home-a','home-b'].map(home=>({source:'native',agent:'DeepSeek Harness',task:home,session_id:'same-id',home_id:home,can_read_conversation:true,status:'idle',current_client:'unconfirmed'}));
  fixture.state.externalSessions={'same-id\u0000home-a':[{prompt:'Home A question',response:'Home A reply'}],'same-id\u0000home-b':[{prompt:'Home B question',response:'Home B reply'}]};
  let hold=false,release,started;
  const held=new Promise(r=>{release=r;}),pending=new Promise(r=>{started=r;});
  await page.route('**/api/session/transcript?*',async route=>{
    if(hold&&new URL(route.request().url()).searchParams.get('home_id')==='home-a') {started();await held;await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify([{prompt:'Late A',response:'Late home A reply'}])}).catch(()=>{});}
    else await route.continue();
  });
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('#radar').getByText('home-a',{exact:true}).click();
  await openTools(page);
  await page.locator('.session-tool .trace-event').filter({hasText:'Home A reply'}).waitFor();
  hold=true;await pending;
  await closeTools(page);
  await page.locator('#back').click();
  await page.locator('#radar').getByText('home-b',{exact:true}).click();
  await openTools(page);
  await page.locator('.session-tool .trace-event').filter({hasText:'Home B reply'}).waitFor();
  hold=false;release();await page.waitForTimeout(150);
  assert.match(await page.locator('.session-tool').innerText(),/Home B reply/);
  assert.doesNotMatch(await page.locator('.session-tool').innerText(),/Home A|Late A|Late home A/);
  assert.equal(await page.locator('#chat-input').isVisible(),false);
  assert.equal(fixture.state.sends.length,0);
});

for (const width of [390,1440]) scenario(`Trace replaces removed session detail even while old source is selected at ${width}`,{viewport:{width,height:900},locale:'en-US',hasTouch:width<700},async(page,fixture)=>{
  agentFixture(fixture,'Codex',[record('Codex','old-root','Old root task')]);
  const trace=await openTrace(page,fixture,width);
  await trace.locator('.trace-event.tool').click();
  await trace.locator('.trace-source').focus();
  await page.evaluate(()=>{const source=document.querySelector('.session-tool .trace-source');const range=document.createRange();range.selectNodeContents(source);getSelection().removeAllRanges();getSelection().addRange(range);});
  const next=record('Codex','new-root','New root task');next.segments[0].steps[0].input='New root exact command';
  fixture.state.turns['%901']=[next];
  await trace.locator('.trace-meta').filter({hasText:'new-root'}).waitFor();
  assert.doesNotMatch(await trace.locator('.trace-inspector').innerText(),/old-root|retry-demo/);
  assert.equal(await trace.locator('.trace-prose').innerText(),'Review result\n\nCancellation stops pending retries.');
  assert.equal(fixture.state.sends.length,0);
});

scenario('Idle Trace opens latest without a follow mode and preserves a historical viewport through refresh', {viewport:{width:1280,height:850},locale:'en-US'},async(page,fixture)=>{
  const initial=Array.from({length:80},(_,i)=>record('Codex','stable','Task '+i));
  agentFixture(fixture,'Codex',initial);fixture.state.agents[0].status='idle';
  const trace=await openTrace(page,fixture,1280);
  assert.match(await trace.locator('.trace-event.on').innerText(),/Review result/);
  assert.equal(await trace.locator('.trace-latest').isVisible(),false);
  assert.equal(await trace.locator('.trace-latest').getAttribute('aria-pressed'),null);
  assert.equal(await trace.getByRole('button',{name:'Follow latest',exact:true}).count(),0);
  const list=trace.locator('.trace-list');
  await list.evaluate(el=>{el.scrollTop=el.scrollHeight*.45;el.dispatchEvent(new Event('scroll'));});
  const anchor=await list.evaluate(el=>{const top=el.getBoundingClientRect().top;const row=[...el.querySelectorAll('.trace-event')].find(n=>n.getBoundingClientRect().bottom>top+8);return {key:row.dataset.key,y:row.getBoundingClientRect().top};});
  fixture.state.turns['%901']=[record('Codex','stable','PREPENDED TASK'),...initial,record('Codex','stable','APPENDED TASK')];
  await trace.locator('.trace-count').filter({hasText:'328 records'}).waitFor();
  const after=await trace.locator('.trace-event[data-key="'+anchor.key+'"]').boundingBox();
  assert.ok(Math.abs(after.y-anchor.y)<=2,'refresh must retain the visible record pixel anchor');
  await trace.getByRole('button',{name:'Jump to latest',exact:true}).click();
  assert.equal(await trace.locator('.trace-latest').isVisible(),false);
  await list.evaluate(el=>{el.scrollTop=el.scrollHeight*.6;el.dispatchEvent(new Event('scroll'));});
  assert.equal(await trace.locator('.trace-latest').isVisible(),true);
  await list.evaluate(el=>{el.scrollTop=el.scrollHeight;el.dispatchEvent(new Event('scroll'));});
  fixture.state.turns['%901']=[...fixture.state.turns['%901'],{...record('Codex','stable','TAIL APPEND'),segments:[{text:'TAIL FOLLOW ACCEPTED'}]}];
  await trace.locator('.trace-event.on').filter({hasText:'TAIL FOLLOW ACCEPTED'}).waitFor();
});


for (const width of [390, 1440]) {
  scenario(`Trace preserves modified shortcuts at ${width}`, {viewport:{width,height:900},locale:'zh-CN',hasTouch:width<700}, async(page,fixture)=>{
    agentFixture(fixture);
    const trace=await openTrace(page,fixture,width);
    await trace.locator('.trace-event.tool').click();
    await page.evaluate(()=>{
      window.traceShortcutEvents=[];
      document.querySelector('.trace-view').addEventListener('keydown',event=>{
        if (!(event.metaKey||event.ctrlKey||event.altKey)||!['ArrowLeft','ArrowRight','ArrowDown','Home'].includes(event.key)) return;
        window.traceShortcutEvents.push(event.defaultPrevented);
        // Observe the component's handling, then fence browser navigation in this fixture.
        event.preventDefault();event.stopPropagation();
      });
    });
    for (const shortcut of ['Meta+ArrowLeft','Control+Home','Alt+ArrowRight']) {
      await trace.locator('.trace-overview-mode').click();
      await trace.locator('.trace-overview-mode').focus();
      await page.keyboard.press(shortcut);
      assert.equal(await trace.locator('.trace-overview-mode').getAttribute('aria-selected'),'true',shortcut+' must not change the selected detail');
      assert.equal(await trace.locator('.trace-overview-mode').evaluate(node=>node===document.activeElement),true);
    }
    await page.keyboard.press('ArrowRight');
    assert.equal(await trace.locator('.trace-preview-mode').getAttribute('aria-selected'),'true','unmodified arrows still navigate details');
    await trace.locator('.trace-close').click();
    const selected=trace.locator('.trace-event[aria-pressed="true"]');
    const key=await selected.getAttribute('data-key');
    await selected.focus();await page.keyboard.press('Meta+ArrowDown');
    assert.equal(await trace.locator('.trace-event[aria-pressed="true"]').getAttribute('data-key'),key,'modified arrows must not change the selected record');
    assert.deepEqual(await page.evaluate(()=>window.traceShortcutEvents),[false,false,false,false]);
    await page.keyboard.press('ArrowDown');
    assert.notEqual(await trace.locator('.trace-event[aria-pressed="true"]').getAttribute('data-key'),key,'unmodified arrows still navigate records');
    assert.equal(fixture.state.sends.length,0);
  });
}

for (const recordIDs of [false, true]) scenario(`Trace keeps repeated clockless calls on their original turn (record IDs ${recordIDs})`, {viewport:{width:1440,height:900},locale:'en-US'}, async(page,fixture)=>{
  agentFixture(fixture);
  const turn = i => ({...(recordIDs ? {record_id:'log:'+i} : {}),session_id:'synthetic-current',agent:'Codex',prompt:'Continue',segments:[{text:'Inspecting',steps:[{title:'exec_command',input:'echo item-'+i,operation:'command'}]}]});
  fixture.state.turns['%901']=[turn(1),turn(2)];
  const trace=await openTrace(page,fixture,1440);
  await trace.locator('.trace-event.tool').last().click();await trace.locator('.trace-source-mode').click();await trace.locator('.trace-count').click();
  const selected=await trace.locator('.trace-event.tool[aria-pressed="true"]').getAttribute('data-key');
  fixture.state.turns['%901']=[turn(0),turn(1),turn(2),turn(3)];
  await page.waitForFunction(()=>document.querySelectorAll('.trace-view .trace-event.tool').length===4);
  assert.equal(await trace.locator('.trace-event.tool[aria-pressed="true"]').getAttribute('data-key'),selected);
  assert.equal(await trace.locator('.trace-source').innerText(),'echo item-2');assert.equal(fixture.state.sends.length,0);
});
