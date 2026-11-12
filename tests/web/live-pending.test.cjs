const assert=require('node:assert/strict');
const {scenario}=require('./harness.cjs');

for(const width of [390,1440])scenario(`LPV003 pending history labels its live pane at ${width}`,{viewport:{width,height:900},locale:'en-US',hasTouch:width<900,colorScheme:width<900?'dark':'light'},async(page,fixture)=>{
  const agent=fixture.state.agents[0];agent.observed_conversation={session_id:'recorded-history',home_id:'fixture-home',state:'last-known'};
  fixture.state.screens['%901']=Array.from({length:20},(_,i)=>'current-pane-line-'+i).join('\n');
  await page.route('**/api/transcript?*',async route=>{const response=await route.fetch();await route.fulfill({response,headers:{...response.headers(),'X-Gtmux-Conversation-State':'last-known'}});});
  await page.goto(fixture.url+'/#code=TEST-1234');
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();await page.locator('.tile [data-action="focus"]').click();}
  else{await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();}
  const live=page.locator('#chat .chat-live');await live.waitFor();
  assert.equal(await live.getAttribute('open'),null);
  await live.locator('summary').click();
  await page.waitForFunction(()=>document.querySelector('#chat .live-output')?.textContent.includes('current-pane-line-19'));
  assert.match(await live.locator('.live-source').innerText(),/Current pane %901.*association unconfirmed/);
  const output=await live.locator('.live-output').innerText();assert.match(output,/current-pane-line-19/);assert.doesNotMatch(output,/current-pane-line-5(?:\n|$)/);
  assert.equal(await page.locator('#chat-input .composer').isVisible(),false);
  assert.equal(fixture.state.sends.length,0);
  if(process.env.GTMUX_WEB_EVIDENCE)await page.screenshot({path:require('node:path').join(process.env.GTMUX_WEB_EVIDENCE,'live-pending-'+width+'.png')});
  await live.locator('summary').click();assert.equal(await live.locator('.live-output').isVisible(),false);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
});
