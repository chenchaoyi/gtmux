const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const {scenario}=require('./harness.cjs');
const evidence=async(page,name)=>{if(process.env.GTMUX_WEB_EVIDENCE){fs.mkdirSync(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,name+'.png')});}};

for(const width of [390,1440]) scenario(`App-aligned roster status, summaries and persistent sections at ${width}`,{viewport:{width,height:900},locale:'zh-CN'},async(page,fixture)=>{
  fixture.state.agents[0]={...fixture.state.agents[0],status:'idle',error:true,error_text:"You've hit your weekly limit · resets Monday",task:'MP analysis dev',session:'analysis'};
  await page.goto(fixture.url+'/#code=TEST-1234');
  const root=page.locator(width<900?'#radar':'#tree');
  const row=root.locator(width<900?'.row':'.tree-row').filter({hasText:'MP analysis dev'});
  assert.match(await row.innerText(),/You've hit your weekly limit/);
  assert.doesNotMatch(await row.innerText(),/analysis · %901/);
  assert.equal(await row.locator('.badge').getAttribute('aria-label'),'出错');
  assert.equal(await row.locator('.badge circle').first().getAttribute('fill'),'#F59E0B');
  assert.match(await root.locator(width<900?'.row':'.tree-row').filter({hasText:'Review notes'}).innerText(),/Web QA · %902/);
  const button=root.locator('[data-section="errored"]');await button.click();
  assert.equal(await button.getAttribute('aria-expanded'),'false');assert.equal(await row.isVisible(),false);
  fixture.state.agents[0].error_text='A new recorded error';await page.waitForTimeout(2400);
  assert.equal(await root.locator('[data-section="errored"]').getAttribute('aria-expanded'),'false');
  await page.reload();assert.equal(await root.locator('[data-section="errored"]').getAttribute('aria-expanded'),'false');
  await root.locator('[data-section="errored"]').press('Enter');assert.equal(await row.isVisible(),true);assert.match(await row.innerText(),/A new recorded error/);
  await evidence(page,'aligned-roster-'+width);
  assert.equal(fixture.state.sends.length,0);
});

for(const width of [320,390,1440]) scenario(`Settings change actual Chat and Terminal typography, immediately and after reload at ${width}`,{viewport:{width,height:900},locale:'en-US'},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile [data-action="focus"]').click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();await page.locator('#chat .md').first().waitFor();
  await page.locator('#gear').click();
  const range=page.locator('#chat-size-rng');await range.fill('22');await range.dispatchEvent('input');
  assert.equal(await page.locator('#chat .md').first().evaluate(e=>getComputedStyle(e).fontSize),'22px');
  await page.locator('#return-sends').check();assert.match(await page.locator('#return-send-hint').innerText(),/Shift \+ Enter/);
  await evidence(page,'settings-chat-'+width);
  await page.locator('#settings-close').click();assert.equal(await page.locator('#gear').evaluate(e=>e===document.activeElement),true);
  await page.locator('#mode [data-mode="term"]').click();await page.locator('#term .xterm-screen').waitFor();
  const before=await page.locator('#term .xterm-screen').boundingBox();
  await page.locator('#gear').click();await page.locator('#size-rng').fill('22');await page.locator('#size-rng').dispatchEvent('input');
  await page.waitForTimeout(100);
  const cell=await page.locator('#term .xterm-char-measure-element').first().evaluate(e=>getComputedStyle(e).fontSize);
  assert.equal(cell,'22px');assert.equal(await page.locator('.terminal-font-preview').evaluate(e=>getComputedStyle(e).fontSize),'22px');
  await page.locator('#font-sel').selectOption('JetBrains Mono');assert.match(await page.locator('#term .xterm-char-measure-element').first().evaluate(e=>getComputedStyle(e).fontFamily),/JetBrains Mono/);
  await page.locator('#settings-close').click();await page.reload();
  // Focus is session-local; reopen the synthetic pane after refreshing the workspace.
  if(width>=900){await page.locator('.tile').first().waitFor();await page.locator('.tile [data-action="focus"]').first().click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="term"]').click();
  await page.waitForFunction(()=>getComputedStyle(document.querySelector('#term .xterm-char-measure-element')).fontSize==='22px');
  await page.locator('#mode [data-mode="chat"]').click();assert.equal(await page.locator('#chat .md').first().evaluate(e=>getComputedStyle(e).fontSize),'22px');
  await page.locator('#gear').click();
  const box=await page.locator('#settings').boundingBox();assert.ok(box.x>=0&&box.x+box.width<=width&&box.y+box.height<=900);
  await page.locator('#settings-reset').click();assert.equal(await page.locator('#chat .md').first().evaluate(e=>getComputedStyle(e).fontSize),'14px');
  assert.equal(await page.locator('#return-sends').isChecked(),false);await page.locator('#settings').press('Escape');
  if(width>=900){await page.locator('#back').click();await page.locator('.tile .xterm-screen').waitFor();assert.equal(await page.locator('.tile .xterm-char-measure-element').first().evaluate(e=>getComputedStyle(e).fontSize),'14px');}
  assert.ok(before.height>0);assert.equal(fixture.state.sends.length,0);
});

scenario('Human profiles share App identity and quote without irrelevant browser permissions',{viewport:{width:390,height:844},locale:'zh-CN'},async(page,fixture)=>{
  const source=JSON.parse(fs.readFileSync(path.join(__dirname,'../../mobileapp/src/ui/participantPresentation.json'),'utf8'));
  const scope={window:{}};vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../../internal/server/web/participant-presentation.js'),'utf8'),scope);
  assert.equal(JSON.stringify(scope.window.GtmuxParticipantPresentation),JSON.stringify(source));
  await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();
  await page.locator('#chat .urow > .speaker-avatar').click();const dialog=page.locator('.participant-profile');
  assert.match(await dialog.innerText(),/人类|碳基决策者/);assert.match(await dialog.locator('.profile-quote').innerText(),/曾经，人们把思考交给机器/);
  assert.doesNotMatch(await dialog.innerText(),/当前浏览器权限|所有者|发送者的个人资料/);
  await evidence(page,'profile-human-mobile');await dialog.getByRole('button',{name:'关闭',exact:true}).click();
});

for(const [width,touch] of [[390,false],[1440,false],[390,true],[1440,true]]) scenario(`Touch input tools follow input device at ${width} touch=${touch}`,{viewport:{width,height:900},locale:'en-US',hasTouch:touch},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile .composer').waitFor();assert.equal(await page.locator('.tile .cx-keys').isVisible(),touch);await page.locator('.tile [data-action="focus"]').click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();const keys=page.locator('#chat-input .cx-keys');assert.equal(await keys.isVisible(),touch);
  if(touch) await keys.locator('.cx-keyboard').click();
  const input=page.locator('#chat-input .cx-input');await input.fill('A preserved keyboard draft');
  await page.setViewportSize({width:width===390?1440:390,height:900});assert.equal(await keys.isVisible(),touch);assert.equal(await input.inputValue(),'A preserved keyboard draft');
  assert.equal(await page.locator('#chat-input .cx-attach').isVisible(),true);assert.equal(await page.locator('#chat-input .cx-send').isVisible(),true);
  assert.equal(fixture.state.sends.length,0);
});

scenario('Chat retains recorded code and wait calls in compact summaries and expanded detail',{viewport:{width:390,height:844},locale:'en-US',hasTouch:true},async(page,fixture)=>{
 fixture.state.turns['%901']=[{session_id:'code-fixture',prompt:'Check the code runner',segments:[{text:'Running the checks',steps:[{title:'exec',operation:'code',input:'const result = await tools.check({});\ntext(result);',input_kind:'code'},{title:'wait',operation:'wait',input:'{"cell_id":"fixture","yield_time_ms":100}',input_kind:'parameters'}]},{text:'The result is ready'}]}];
 await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();
 const calls=page.locator('#chat .tool-steps');await calls.waitFor();assert.match(await calls.innerText(),/2 calls/);await calls.locator('.steps-toggle').click();
 assert.match(await calls.innerText(),/Run code|Wait/);await calls.locator('.step-toggle').first().click();assert.match(await calls.getByRole('region',{name:'Recorded code',exact:true}).innerText(),/tools.check/);await evidence(page,'chat-code-and-wait');
 assert.equal(fixture.state.sends.length,0);
});
