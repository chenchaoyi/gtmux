const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');
const {openTools} = require('./tools.cjs');

const contextText = '<system-reminder>Use the fixture workspace.\nKeep the original indentation.</system-reminder>';
function turn(prompt, time) {
  return {session_id:'fold-current',agent:'Codex',prompt,time,segments:[
    {context:{title:'Environment reminder',text:contextText,source:'codex/response_item.message.user',time:'2026-10-30T10:01:01Z'}},
    {text:'Reading the fixture',steps:[{title:'exec_command',input:'hidden-first-command',operation:'command'},{title:'wait',input:'hidden-second-command',operation:'wait'}]},
    {text:'Checking results',steps:[{title:'exec_command',input:'hidden-final-command',operation:'command'}]},
    {text:'Finished checking the fixture'},
  ]};
}
async function shot(page,name) {
  if (!process.env.GTMUX_WEB_EVIDENCE) return;
  await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});
  await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,name+'.png')});
}
async function traceOpen(page,fixture,width) {
  await page.goto(fixture.url+'/#code=TEST-1234');
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile [data-action="focus"]').click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();await openTools(page);
  const trace=page.locator('.session-tool .trace-view');await trace.locator('.trace-event').first().waitFor();return trace;
}
for(const [width,locale] of [[320,'zh-CN'],[390,'en-US'],[1440,'zh-CN']]) scenario(`Trace turn and call folds stay independent, survive refresh, and reveal context detail at ${width}`,{viewport:{width,height:900},locale,hasTouch:width<700},async(page,fixture)=>{
  const originals=[turn('First request','2026-10-30T10:00:00Z'),turn('Second request','2026-10-30T10:10:00Z')];fixture.state.turns['%901']=originals;
  const reads=[];page.on('request', request=>{const url=new URL(request.url());if(url.pathname.endsWith('/api/transcript'))reads.push(url);});
  const trace=await traceOpen(page,fixture,width);assert.ok(reads.length>0&&reads.every(url=>url.searchParams.get('context')==='1'),'compatible browser readers explicitly opt into context');
  assert.equal(await trace.locator('.trace-event.tool').count(),6);
  await trace.locator('.trace-fold-calls').click();assert.equal(await trace.locator('.trace-event.tool').count(),0);
  const batches=trace.locator('.trace-calls');assert.equal(await batches.count(),4);
  assert.equal(await batches.filter({hasText:/2 (个工具调用|tool calls)/}).count(),2);
  await batches.first().click();assert.equal(await trace.locator('.trace-event.tool').count(),2);assert.equal(await trace.locator('.trace-calls[aria-expanded="true"]').count(),1);
  await trace.locator('.trace-fold-turns').click();assert.equal(await trace.locator('.trace-event').count(),0);assert.equal(await trace.locator('.trace-turn').count(),2);
  await trace.locator('.trace-fold-calls').click();assert.equal(await trace.locator('.trace-event').count(),0,'expanding calls must not open turns');
  await trace.locator('.trace-fold-turns').click();assert.equal(await trace.locator('.trace-event.tool').count(),6);
  await trace.locator('.trace-fold-calls').click();assert.equal(await trace.locator('.trace-event.tool').count(),0);
  await trace.locator('.trace-list').evaluate(node=>{node.scrollTop=0;});
  const anchor=await trace.locator('.trace-list').evaluate(node=>{const top=node.getBoundingClientRect().top;const row=[...node.querySelectorAll('[data-row-key]')].find(n=>n.getBoundingClientRect().bottom>top+4);return {key:row.dataset.rowKey,y:row.getBoundingClientRect().top};});
  fixture.state.turns['%901']=[...originals,turn('Third request','2026-10-30T10:20:00Z')];await trace.locator('.trace-turn').nth(2).waitFor();
  assert.equal(await trace.locator('.trace-event.tool').count(),0,'new call runs inherit the collapsed choice');
  assert.ok(Math.abs((await trace.locator('[data-row-key="'+anchor.key+'"]').boundingBox()).y-anchor.y)<3,'polling preserves the visible row');
  await trace.locator('.trace-search').fill('hidden-second-command');assert.equal(await trace.locator('.trace-event.tool').count(),3);assert.equal(await trace.locator('.trace-fold-calls').isDisabled(),true);
  await trace.locator('.trace-search').fill('');assert.equal(await trace.locator('.trace-event.tool').count(),0,'clearing search restores disclosures');
  if(width>=700){
    await trace.locator('.trace-calls').first().click();
    await trace.locator('.trace-event.tool').first().click();
    await trace.locator('.trace-calls').first().click();
    await trace.locator('.trace-close').click();
    assert.equal(await trace.locator('.trace-calls').first().evaluate(node=>node===document.activeElement),true,'closing a hidden record returns focus to its folded call group');
  }
  await shot(page,'trace-folded-'+width);
  await trace.locator('.trace-filter').selectOption('context');await trace.locator('.trace-event.context').first().click();
  assert.equal(await trace.getByRole('tab').count(),4);
  await trace.locator('.trace-overview-mode').click();assert.match(await trace.locator('.trace-facts').innerText(),/上下文|Context/);
  await trace.locator('.trace-source-mode').click();assert.equal(await trace.locator('.trace-source').innerText(),contextText);
  await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.traceCopied=text;}}}));
  await trace.locator('.trace-origin-mode').click();const origin=JSON.parse(await trace.locator('.trace-origin').innerText());assert.equal(origin.kind,'context');assert.equal(origin.record_source,'codex/response_item.message.user');assert.equal(origin.recorded_at,'2026-10-30T10:01:01Z');assert.equal('token_count' in origin,false);assert.equal('request_id' in origin,false);
  await trace.locator('.trace-detail-actions button').first().click();assert.deepEqual(JSON.parse(await page.evaluate(()=>window.traceCopied)),origin);
  await shot(page,'trace-source-'+width);
  await trace.locator('.trace-origin-mode').focus();await page.keyboard.press('Home');assert.equal(await trace.locator('.trace-overview-mode').getAttribute('aria-selected'),'true');
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);assert.equal(fixture.state.sends.length,0);
});

for(const width of [320,390,1440]) scenario(`Settings categories and theme icons remain clear and keyboard accessible at ${width}`,{viewport:{width,height:900},locale:'en-US',hasTouch:width<700},async(page,fixture)=>{
  await page.goto(fixture.url+'/#code=TEST-1234');const gear=width>=900?page.locator('#wb-gear'):page.locator('#gear');await gear.click();
  const panel=page.locator('#settings'),nav=panel.locator('nav');assert.equal(await nav.getByRole('button').count(),3);assert.equal(await panel.locator('.appearance-icon svg').count(),3);
  await shot(page,'settings-new-'+width);
  await nav.locator('[data-settings-section="settings-terminal-section"]').click();assert.equal(await panel.locator('#font-sel').isVisible(),true);await page.waitForFunction(()=>document.querySelector('#settings-nav-terminal').getAttribute('aria-current')==='location');
  await nav.locator('[data-settings-section="settings-appearance-section"]').click();await panel.locator('#appearance-system-radio').check();
  await page.emulateMedia({colorScheme:'dark'});await page.waitForFunction(()=>document.documentElement.dataset.theme==='dark');await shot(page,'settings-dark-'+width);
  await panel.locator('#appearance-system-radio').press('ArrowLeft');assert.equal(await panel.locator('#appearance-dark-radio').isChecked(),true);
  await page.emulateMedia({colorScheme:'light'});assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');
  await panel.locator('#appearance-system-radio').check();await page.waitForFunction(()=>document.documentElement.dataset.theme==='light');
  for(const node of await nav.getByRole('button').all()){const box=await node.boundingBox();assert.ok(box.width>=44&&box.height>=44);}
  const box=await panel.boundingBox();assert.ok(box.x>=0&&box.x+box.width<=width&&box.height<=480);
  await panel.press('Escape');assert.equal(await panel.isVisible(),false);assert.equal(await gear.evaluate(node=>node===document.activeElement),true);assert.equal(fixture.state.sends.length,0);
});
