const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');
const {openTools,closeTools} = require('./tools.cjs');

for(const width of [390,1440]) scenario(`Session section casing matches App headings at ${width}`,{viewport:{width,height:900},locale:'en-US'},async(page,fixture)=>{
  fixture.state.agents=[
    {pane_id:'%901',agent:'Codex',status:'waiting',task:'Needs a reply'},
    {pane_id:'%902',agent:'Claude Code',status:'idle',error:true,error_text:'Recorded error',task:'Retry failed'},
    {pane_id:'%903',agent:'Codex',role:'supervisor',status:'idle',task:'HQ'},
    {pane_id:'%904',agent:'Codex',status:'working',task:'Checking'},
    {pane_id:'%905',agent:'Kimi Code',status:'idle',task:'Finished'},
    {pane_id:'',source:'native',agent:'Codex',session_id:'external-case',current_client:'unconfirmed',can_read_conversation:true,task:'External example'},
    {pane_id:'',source:'native',agent:'Codex',session_id:'elsewhere-case',task:'Status only'},
  ];
  await page.goto(fixture.url+'/#code=TEST-1234');
  const root=page.locator(width<900?'#radar':'#tree');
  const labels=await root.locator('.section-name').allTextContents();
  for(const expected of ['NEEDS ATTENTION','ERRORED','WORKING','IDLE','ELSEWHERE','EXTERNAL CONVERSATIONS']) assert.ok(labels.includes(expected),expected);
  if(width>=900)assert.ok(labels.includes('HEADQUARTERS'));
  const toggle=root.locator('.section-toggle.working');
  assert.equal(await toggle.locator('.section-action').innerText(),'Hide');
  assert.equal(await toggle.locator('.section-action').evaluate(e=>getComputedStyle(e).textTransform),'none');
  await toggle.click();assert.equal(await toggle.locator('.section-action').innerText(),'Show');
  assert.equal(await toggle.locator('.section-name').innerText(),'WORKING');
  for(const heading of await root.locator('.section-toggle').all())assert.ok((await heading.boundingBox()).height>=44);
  if(process.env.GTMUX_WEB_EVIDENCE)await page.screenshot({path:require('node:path').join(process.env.GTMUX_WEB_EVIDENCE,'section-casing-'+width+'.png')});
  assert.equal(fixture.state.sends.length,0);
});

scenario('Empty workspace has a clear first action and a scene-specific attributed excerpt',{viewport:{width:1440,height:900},locale:'zh-CN'},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.getByRole('heading',{name:'从一个会话开始'}).waitFor();
  assert.match(await page.locator('.workspace-quote').innerText(),/初始之际/);
  assert.doesNotMatch(await page.locator('.workspace-quote').innerText(),/思考交给机器|turned their thinking/);
  assert.equal(await page.locator('.workspace-quote a').getAttribute('href'),'https://sites.prh.com/dune/read-an-excerpt');
  const heading=await page.locator('.workspace-welcome h1').evaluate(e=>Number.parseFloat(getComputedStyle(e).fontSize));assert.ok(heading>=24);
  await page.getByRole('button',{name:'选择会话',exact:true}).click();
  await page.locator('#cmdk-input').fill('Investigate');
  await page.locator('#cmdk-input').press('Enter');await page.locator('.tile').waitFor();
});

scenario('Only the actual supervisor is pinned above status groups',{viewport:{width:1440,height:900},locale:'en-US'},async(page,fixture)=>{
  fixture.state.agents[2].task='Coordinate release';fixture.state.agents[0].task='HQ is only a task name';
  await page.goto(fixture.url+'/#code=TEST-1234');
  assert.equal(await page.locator('.tree-hq .tr-name').innerText(),'Coordinate release');
  assert.equal(await page.locator('.tree-hq .tree-row').count(),1);
  assert.equal(await page.locator('.tree-hq .hq-brand-mark rect').count(),3);
  assert.equal(await page.locator('#wb-surface').count(),0);
  assert.equal(await page.locator('#tree > :first-child').getAttribute('class'),'tree-hq');
});

for(const width of [320,390,768,1440]) scenario(`Focused Chat is a conversation with session tools at ${width}`,{viewport:{width,height:900},locale:'en-US',hasTouch:width<800},async(page,fixture)=>{
  fixture.state.turns['%901']=Array.from({length:100},(_,i)=>({session_id:'long',prompt:'Request '+i+' — '+('readable context '.repeat(15)),segments:[{text:'A complete answer for request '+i,steps:[{title:'wait',operation:'wait'}]}]}));
  await page.goto(fixture.url+'/#code=TEST-1234');
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();await page.locator('.tile [data-action="focus"]').click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.locator('#chat .assistant-message').first().waitFor();
  const toolbar = await page.locator('#session-toolbar').boundingBox();
  const title = await page.locator('#title').boundingBox();
  const modeBox = await page.locator('#mode').boundingBox();
  const toolsBox = await page.locator('#session-actions').boundingBox();
  assert.ok(toolbar.y >= title.y + title.height, 'mode and tools stay below identity');
  assert.ok(modeBox.x < 60, 'modes stay at the left edge in focus');
  assert.ok(toolsBox.x >= modeBox.x + modeBox.width + 8, 'tool actions are separate from presentation modes');
  assert.ok(Math.abs(toolsBox.y-modeBox.y) < 3);
  assert.equal(await page.locator('#session-tools, #font-dn, #font-up').count(), 0);
  for (const button of await page.locator('#session-actions button').all()) assert.ok((await button.boundingBox()).height >= 44);

  assert.equal(await page.locator('#chat .turn-rail').count(),0);
  assert.deepEqual(await page.locator('#mode button').allTextContents(),['Chat','Terminal']);
  const box=await page.locator('#chat .conversation-main').boundingBox();assert.ok(box.width>=Math.min(width-24,850));
  assert.ok(box.height>=300);assert.equal(await page.locator('#chat .cturn').count(),100);
  if(width<800) await page.locator('#chat-input .cx-keyboard').click();
  const input=page.locator('#chat-input .cx-input');await input.fill('Keep this draft');
  await page.locator('#chat .conv-scroll').evaluate(e=>{e.scrollTop=1234;});
  const readingPosition=await page.locator('#chat .conv-scroll').evaluate(e=>e.scrollTop);
  await openTools(page,'diff');await page.locator('.session-diff .add').waitFor();await closeTools(page);
  assert.equal(await input.inputValue(),'Keep this draft');
  await openTools(page,'trace');await page.locator('.trace-event').first().waitFor();await closeTools(page);
  assert.equal(await input.inputValue(),'Keep this draft');
  assert.equal(await page.locator('#chat .conv-scroll').evaluate(e=>e.scrollTop),readingPosition);
  assert.equal(await page.locator('#mode [data-mode="chat"]').getAttribute('aria-pressed'),'true');
  await page.locator('#chat .steps-toggle').last().click();
  assert.equal(await page.locator('#chat .step-inspect summary').count(),0,'a wait with no input does not open an empty inspector');
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
});

scenario('Appearance selects keep a 44px target and inset arrows in both themes',{viewport:{width:390,height:844},locale:'zh-CN'},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#gear').click();
  for(const theme of ['light','dark']){
    await page.locator('#appearance-options input[value="'+theme+'"]').check();
    for(const select of await page.locator('#settings select').all()){
      const style=await select.evaluate(e=>{const s=getComputedStyle(e);return {height:e.getBoundingClientRect().height,left:parseFloat(s.paddingLeft),right:parseFloat(s.paddingRight),arrow:s.backgroundPosition,font:s.fontFamily};});
      assert.ok(style.height>=44&&style.left>=12&&style.right>=40);assert.match(style.arrow,/14px/);assert.match(style.font,/-apple-system/);
    }
  }
});

scenario('Conversation images reuse authenticated App document metadata and verified bytes',{viewport:{width:390,height:844},locale:'en-US'},async(page,fixture)=>{
  fixture.state.agents[0].home_id='image-home';fixture.state.turns['%901']=[{session_id:'image-session',prompt:'![Image #1](/tmp/synthetic.png)\nPlease inspect this image',response:'Recorded answer'}];
  await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();
  const attachment=page.locator('.conversation-file');await attachment.waitFor();assert.equal(fixture.state.documentReads||0,0,'reading a conversation does not snapshot files automatically');
  await attachment.click();await page.locator('.document-image').waitFor();
  await page.waitForFunction(()=>document.querySelector('.document-image').naturalWidth===1);
  const firstURL=await page.locator('.document-image').getAttribute('src');assert.match(firstURL,/^blob:/);
  await page.locator('.conversation-document .dialog-close').click();await page.locator('.conversation-document').waitFor({state:'detached'});
  fixture.state.corruptDocument=true;await attachment.click();await page.getByText('Could not read the file. Check the connection and retry.',{exact:true}).waitFor();assert.equal(await page.locator('.document-image').count(),0);
  fixture.state.deny=true;await page.locator('#gate').waitFor();assert.equal(await page.locator('.conversation-document').count(),0);
  assert.equal(fixture.state.sends.length,0);
});

scenario('Legacy auto-open settings do not open panels when a session needs input',{viewport:{width:1440,height:900},locale:'en-US'},async(page,fixture)=>{
  await page.addInitScript(()=>localStorage.setItem('gtmux.board',JSON.stringify({surface:true,tiles:[]})));
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('#tree .tree-row').first().waitFor();
  fixture.state.agents[0].status='waiting';
  await page.waitForTimeout(2300);
  assert.equal(await page.locator('.tile').count(),0);
  assert.equal(await page.locator('#wb-surface').count(),0);
});

for(const width of [390,1440]) scenario(`Browser title follows session tools and returns to Chat at ${width}`,{viewport:{width,height:900},locale:'en-US'},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();await page.locator('.tile [data-action="focus"]').click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  assert.equal(await page.title(),'Investigate retries · Chat · gtmux');
  await openTools(page,'trace');assert.equal(await page.title(),'Investigate retries · Trace · gtmux');await closeTools(page);
  assert.equal(await page.title(),'Investigate retries · Chat · gtmux');
  await openTools(page,'diff');assert.equal(await page.title(),'Investigate retries · Diff · gtmux');await closeTools(page);
  assert.equal(await page.title(),'Investigate retries · Chat · gtmux');
  assert.equal(fixture.state.sends.length,0);
});

scenario('Browser tool title returns to the workspace or Terminal without changing presentation',{viewport:{width:1440,height:900},locale:'en-US'},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();
  assert.equal(await page.title(),'Agent workspace · gtmux');
  for(const tool of ['trace','diff']){
    await openTools(page,tool,true);assert.equal(await page.title(),'Investigate retries · '+(tool==='trace'?'Trace':'Diff')+' · gtmux');await closeTools(page);
    assert.equal(await page.title(),'Agent workspace · gtmux');
  }
  await page.locator('.tile [data-action="focus"]').click();
  await page.locator('#mode [data-mode="term"]').click();
  assert.equal(await page.title(),'Investigate retries · Terminal · gtmux');
  await openTools(page,'trace');await closeTools(page);
  assert.equal(await page.title(),'Investigate retries · Terminal · gtmux');
  assert.equal(fixture.state.sends.length,0);
});

for(const [width,locale] of [[320,'zh-CN'],[390,'en-US'],[1440,'en-US']]) scenario(`Compact settings expose direct appearance choices and bounded call rows at ${width}`,{viewport:{width,height:900},locale,hasTouch:width<800},async(page,fixture)=>{
  fixture.state.turns['%901']=[{prompt:'Review this fixture',segments:[{text:'Reading the fixture',steps:Array.from({length:12},(_,i)=>({title:'read_file',operation:'read',input:'/fixture/analysis-'+i+'.md',input_kind:'path'}))},{text:'The fixture is ready'}]}];
  await page.goto(fixture.url+'/#code=TEST-1234');
  await (width>=900?page.locator('#wb-gear'):page.locator('#gear')).click();
  const panel=page.locator('#settings'), group=page.locator('#appearance-options');
  assert.equal(await group.getByRole('radio').count(),3);assert.equal(await panel.locator('#appearance-sel').count(),0);
  for(const value of ['light','dark','system']){const radio=group.locator('input[value="'+value+'"]');assert.equal(await radio.isVisible(),true);const hit=await radio.boundingBox();assert.ok(hit.height>=44&&hit.width>=44);await radio.check();assert.equal(await radio.isChecked(),true);}
  await page.emulateMedia({colorScheme:'dark'});await page.waitForFunction(()=>document.documentElement.dataset.theme==='dark');
  await page.emulateMedia({colorScheme:'light'});await page.waitForFunction(()=>document.documentElement.dataset.theme==='light');
  await group.locator('input[value="system"]').focus();await group.locator('input[value="system"]').press('ArrowRight');assert.equal(await group.locator('input[value="light"]').isChecked(),true);
  await page.reload();await (width>=900?page.locator('#wb-gear'):page.locator('#gear')).click();assert.equal(await group.locator('input[value="light"]').isChecked(),true);
  const rect=await panel.boundingBox();assert.ok(rect.height<=480,`settings height ${rect.height} should fit a compact reading panel`);assert.ok(rect.x>=0&&rect.x+rect.width<=width);assert.ok(rect.y+rect.height<=900);
  const toggle=await panel.locator('#return-sends').boundingBox();assert.ok(toggle.height>=44&&toggle.width>=44);
  await panel.locator('#settings-reset').click();assert.equal(await group.locator('input[value="system"]').isChecked(),true);
  if(process.env.GTMUX_WEB_EVIDENCE)await page.screenshot({path:require('node:path').join(process.env.GTMUX_WEB_EVIDENCE,'compact-settings-'+width+'.png')});
  await panel.locator('#settings-close').click();
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile [data-action="focus"]').click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();await page.locator('#chat .steps-toggle').click();
  const rows=page.locator('#chat .steps-list .step-toggle');assert.equal(await rows.count(),12);
  for(const row of await rows.all()){const box=await row.boundingBox();assert.ok(box.height>=44&&box.height<=46,`call target ${box.height} preserves touch size without doubled padding`);}
  assert.equal(await page.locator('#chat .step-row').first().evaluate(e=>e.getBoundingClientRect().height),await rows.first().evaluate(e=>e.getBoundingClientRect().height));
  await rows.first().click();assert.equal(await page.locator('#chat .step-input').first().innerText(),'/fixture/analysis-0.md');
  if(process.env.GTMUX_WEB_EVIDENCE)await page.screenshot({path:require('node:path').join(process.env.GTMUX_WEB_EVIDENCE,'compact-calls-'+width+'.png')});
  assert.equal(fixture.state.sends.length,0);
});

scenario('External conversations share compact calls and preserve full input',{viewport:{width:1440,height:900},locale:'zh-CN'},async(page,fixture)=>{
 fixture.state.agents=[{pane_id:'',source:'native',agent:'DeepSeek Harness',session_id:'external-density',home_id:'fixture-home',current_client:'unconfirmed',can_read_conversation:true,status:'idle',task:'外部会话密度示例'}];
 fixture.state.externalSessions={'external-density\u0000fixture-home':[{session_id:'external-density',agent:'DeepSeek Harness',prompt:'检查示例报告',segments:[{text:'撰写分析报告',steps:Array.from({length:13},(_,i)=>({title:i%2?'read_file':'edit_file',operation:i%2?'read':'edit',input:'/fixture/analysis.md',input_kind:'path'}))},{text:'示例报告已完成'}]}]};
 await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#tree').getByText('外部会话密度示例',{exact:true}).click();
 await page.locator('#chat .steps-toggle').click();const rows=page.locator('#chat .step-toggle');assert.equal(await rows.count(),13);
 for(const row of await rows.all())assert.ok((await row.boundingBox()).height<=46);
 await page.locator('#chat .conv-scroll').evaluate(e=>{e.scrollTop=0;});
 if(process.env.GTMUX_WEB_EVIDENCE)await page.screenshot({path:require('node:path').join(process.env.GTMUX_WEB_EVIDENCE,'compact-external-calls.png')});
 await rows.first().click();assert.equal(await page.locator('#chat .step-input').first().innerText(),'/fixture/analysis.md');assert.equal(fixture.state.sends.length,0);
});
