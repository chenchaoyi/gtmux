const assert=require('node:assert/strict');
const {createHash}=require('node:crypto');
const contract=require('../contracts/client-behavior.json');
const {scenario}=require('./harness.cjs');
const {openTools,closeTools}=require('./tools.cjs');
async function open(page,fixture,width){
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width>=900){await page.locator('#tree').getByText('Contract review',{exact:true}).dblclick();await page.locator('.tile [data-action="focus"]').click();}
 else await page.locator('#radar').getByText('Contract review',{exact:true}).click();
 await page.locator('#mode [data-mode="chat"]').click();
}
for(const width of [390,1440]){
 for(const agent of contract.agents)scenario(`CP-001 shared rendered Trace contract ${agent} ${width}`,{viewport:{width,height:950},locale:'en-US'},async(page,fixture)=>{
  fixture.state.agents[0]={...fixture.state.agents[0],agent,task:'Contract review',session_id:'contract-session',status:'idle'};
  fixture.state.turns['%901']=[{...contract.trace.turn,agent}];
  await open(page,fixture,width);await openTools(page,'trace',false,false);
  const trace=page.locator('.trace-view');await trace.locator('.trace-turn').waitFor();
  assert.equal(await trace.locator('.trace-count').innerText(),contract.trace.count);
  assert.equal(await trace.locator('.trace-group-count').innerText(),contract.trace.summary);
  assert.equal(await trace.locator('.trace-event').count(),0);
  assert.match(await trace.locator('.trace-turn').innerText(),/Turn 151/);
  await trace.locator('.trace-turn').click();await trace.locator('.trace-event.tool').click();
  assert.equal(await trace.locator('.trace-detail-title').innerText(),contract.trace.toolLabel);
  assert.equal(await trace.locator('.trace-source').innerText(),contract.trace.input);
  await trace.locator('.trace-detail-head .trace-close').click();await trace.locator('.trace-event.assistant').click();
  assert.equal(await trace.locator('.trace-detail-title').innerText(),contract.trace.replyLabel);
  await closeTools(page);assert.equal(fixture.state.sends.length,0);
 });
 scenario(`CP-002 CP-003 CP-004 shared HQ and document contract ${width}`,{viewport:{width,height:950},locale:'en-US'},async(page,fixture)=>{
  fixture.state.agents[0]={...fixture.state.agents[0],task:'Contract review',session_id:'contract-session',status:'idle'};
  fixture.state.turns['%901']=[{...contract.hq,session_id:'contract-session'},{prompt:'Open report',response:`[Contract report](${contract.document.reference})`,session_id:'contract-session'}];
  const bytes=Buffer.from(contract.document.source), resolves=[];
  await page.route('**/api/artifacts/resolve',async route=>{
   resolves.push(route.request().postDataJSON());return route.fulfill({json:{id:'a'.repeat(32),sha256:createHash('sha256').update(bytes).digest('hex'),bytes:bytes.length,name:contract.document.name,kind:'markdown',media_type:'text/markdown',session_id:'contract-session',source:contract.document.reference,created_at:100}});
  });
  await page.route('**/api/artifacts/content?*',route=>route.fulfill({status:200,body:bytes,contentType:'text/markdown'}));
  await open(page,fixture,width);
  const hq=page.locator('#chat .urow').first();
  assert.match(await hq.innerText(),/keep the current access policy/);
  assert.equal(await hq.getByText('Show all',{exact:true}).count(),0);
  await hq.locator('.speaker-avatar').click();await page.locator('.participant-profile').waitFor();assert.match(await page.locator('.participant-profile').innerText(),/HQ/);
  await page.locator('.participant-profile').getByRole('button',{name:'Close',exact:true}).click();
  await page.locator('#chat .conversation-file').click();const doc=page.locator('.conversation-document');
  await doc.getByText(contract.document.heading,{exact:true}).waitFor();
  assert.equal(await doc.locator('strong').innerText(),contract.document.emphasis);
  assert.equal(await doc.locator('.code-panel pre').innerText(),contract.document.code);
  assert.equal(await doc.locator('a[href^="javascript:"]').count(),0);
  assert.deepEqual(resolves,[{reference:contract.document.reference,session_id:'contract-session',home_id:''}]);
  if(process.env.GTMUX_WEB_EVIDENCE){const fs=require('node:fs/promises'),path=require('node:path');await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,'hq-document-contract-'+width+'.png')});}
  fixture.state.deny=true;await page.locator('#gate').waitFor();assert.equal(await page.locator('.conversation-document').count(),0);
  assert.equal(fixture.state.sends.length,0);
 });
}
