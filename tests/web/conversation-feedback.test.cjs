const assert=require('node:assert/strict');
const {scenario}=require('./harness.cjs');
const path=require('node:path');
for(const width of [390,1440])scenario('CRX-003 visible conversation reload '+width,{viewport:{width,height:950},locale:'en-US'},async(page,fixture)=>{
 fixture.state.expectedHttpErrors.push(500);
 const prompt='synthetic read feedback';fixture.state.turns['%901']=[{prompt,response:'kept answer',session_id:'s',record_id:'r1',turn_ordinal:1}];
 let mode='initial',requests=0,release;const held=new Promise(r=>{release=r;});
 await page.route('**/api/transcript?*',async route=>{
  if(new URL(route.request().url()).searchParams.get('id')!=='%901')return route.continue();
  requests++;
  if(mode==='initial')return route.continue();
  if(mode==='failed')return route.fulfill({status:500,contentType:'application/json',body:'{}'});
  await held;await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify([{prompt,response:'fresh answer',session_id:'s',record_id:'r1',turn_ordinal:1}])}).catch(()=>{});
 });
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width<800){await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();}
 else {await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();}
 await page.getByText('kept answer',{exact:true}).waitFor();mode='failed';
 const button=page.getByRole('button',{name:'Reload conversation',exact:true});await button.waitFor();
 assert.equal(await page.getByText('kept answer',{exact:true}).count(),1);
 const box=await button.boundingBox();assert.ok(box.height>=44);
 mode='held';const before=requests;await button.click();
 const busy=page.getByRole('button',{name:'Reloading…',exact:true});await busy.waitFor();assert.equal(await busy.isDisabled(),true);assert.equal(await busy.getAttribute('aria-busy'),'true');
 await page.waitForFunction(()=>document.querySelector('.conversation-reload')?.disabled);
 assert.ok(requests>before);assert.equal(await page.getByText('kept answer',{exact:true}).count(),1);
 if(process.env.GTMUX_WEB_RESULTS)await page.screenshot({path:path.join(process.env.GTMUX_WEB_RESULTS,'reload-'+width+'.png')});
 release();await page.getByText('fresh answer',{exact:true}).waitFor();assert.equal(await page.locator('.chat-read-error').count(),0);assert.equal(fixture.state.sends.length,0);
});

scenario('CRX-006 empty conversation uses an actionable short message',{viewport:{width:390,height:950},locale:'en-US'},async(page,fixture)=>{
 fixture.state.turns['%901']=[];
 await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();
 await page.getByText('No messages yet. Start a conversation.',{exact:true}).waitFor();
 assert.doesNotMatch(await page.locator('#chat').innerText(),/session log|History comes/);
 await page.getByRole('button',{name:'View Terminal',exact:true}).click();
 assert.equal(await page.locator('#mode [data-mode="term"]').getAttribute('class'),'on');
 assert.equal(fixture.state.sends.length,0);
});
