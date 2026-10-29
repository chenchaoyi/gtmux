const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');
const {openTools,closeTools} = require('./tools.cjs');

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
    await page.locator('#appearance-sel').selectOption(theme);
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
