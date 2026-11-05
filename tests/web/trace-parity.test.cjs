// Reproduce a late, tool-free coordination tail without mistaking it for the session.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');
const {openTools} = require('./tools.cjs');
const traceModel = require('../../internal/server/web/trace-model.js');

for (const [width, locale] of [[390,'en-US'],[834,'en-US'],[1440,'en-US'],[1440,'zh-CN']]) {
  scenario('Trace App parity for original turns 253–256 at '+width+' '+locale, {viewport:{width,height:950},locale,hasTouch:width<900}, async (page, fixture) => {
    const zh = locale === 'zh-CN';
    const session = 'synthetic-private-session-id';
    const turns = Array.from({length:4}, (_,i) => ({session_id:session,agent:'Codex',record_id:'turn-'+(253+i),turn_ordinal:253+i,prompt:'Coordination message '+(253+i),response:'Recorded reply '+(253+i)}));
    turns[0].history_before='before-253';
    const older = {session_id:session,agent:'Codex',record_id:'turn-252',turn_ordinal:252,prompt:'Inspect files',segments:[
      {context:{title:'Workspace instructions',text:'Recorded workspace instruction',source:'synthetic/context'}},
      {steps:[{title:'Read',operation:'read',input:'{"path":"sample.ts"}'},{title:'Edit',operation:'edit',input:'{"path":"sample.ts","text":""}'},{title:'shell',operation:'shell',input:'printf done'}]},
      {text:'Inspection complete'},
    ]};
    fixture.state.agents=[{...fixture.state.agents[0],task:'Trace parity QA',status:'idle'}];
    fixture.state.turns['%901']=turns;
    const reads=[];
    await page.route('**/api/transcript?*',async route=>{
      const url=new URL(route.request().url());reads.push(url);
      if(!url.searchParams.has('before')) return route.continue();
      assert.equal(url.searchParams.get('context'),'1');
      return route.fulfill({status:200,headers:{'Content-Type':'application/json','X-Gtmux-History-Page':'1'},body:JSON.stringify([older])});
    });
    await page.goto(fixture.url+'/#code=TEST-1234');
    if(width>=900) {
      await page.locator('#tree').getByText('Trace parity QA',{exact:true}).dblclick();
      await page.locator('.tile-modes [data-m="chat"]').click();
      await page.locator('.tile-btn[data-action="focus"]').click();
    } else await page.locator('#radar').getByText('Trace parity QA',{exact:true}).click();
    await page.locator('#mode [data-mode="chat"]').click();
    await openTools(page,'trace',false,false);
    const trace=page.locator('.session-tool .trace-view');
    await trace.locator('.trace-turn').last().waitFor();
    assert.equal(await trace.locator('.trace-turn').count(),4);
    assert.equal(await trace.locator('.trace-event').count(),0,'turns initially folded like App');
    assert.equal(await trace.locator('.trace-inspector').isVisible(),false,'opening Trace keeps the list full-width until a record is selected');
    assert.equal(await trace.locator('.trace-count').innerText(),traceModel.traceWindowLabel(traceModel.build(turns).events,zh?'zh':'en'));
    assert.equal(await trace.locator('.trace-count').innerText(),zh?'4 轮 · 8 条记录':'4 turns · 8 records');
    assert.equal(await trace.locator('.trace-count').getAttribute('title'),zh?'统计仅包含已加载记录，可加载更早记录查看更多。':'Counts describe loaded records only. Load earlier records to see more.');
    for(const stat of await trace.locator('.trace-group-count').allTextContents()) assert.equal(stat,zh?'0 次调用 · 2 条消息':'0 calls · 2 messages');
    assert.match(await trace.locator('.trace-progress').innerText(),/256/);
    assert.match(await trace.locator('.trace-progress').innerText(),/Trace parity QA/);
    assert.doesNotMatch(await trace.locator('.trace-meta').innerText(),/synthetic-private-session-id/);
    if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,'trace-parity-initial-'+width+'-'+locale+'.png')});}
    await trace.locator('.trace-history-earlier').click();
    await trace.locator('.trace-turn').filter({hasText:'252'}).waitFor();
    const first=trace.locator('.trace-turn').first();
    assert.equal(await first.locator('.trace-group-count').innerText(),zh?'3 次调用 · 2 条消息 · 1 条上下文':'3 calls · 2 messages · 1 context record');
    await first.click();
    assert.equal(await trace.locator('.trace-calls').count(),1);
    assert.equal(await trace.locator('.trace-event.tool').count(),0,'3-call batch defaults folded like App');
    await trace.locator('.trace-calls').click();
    assert.equal(await trace.locator('.trace-event.tool').count(),3);
    await trace.locator('.trace-event.tool').first().click();
    assert.equal(await trace.locator('.trace-detail-title').innerText(),zh?'读取文件':'Read file');
    assert.equal(await trace.locator('.trace-meta').innerText(),'Codex · '+(zh?'第 252 轮':'Turn 252'));
    await trace.locator('.trace-origin-mode').click();
    assert.equal(JSON.parse(await trace.locator('.trace-origin').innerText()).session_id,session,'full identity remains in Source');
    await trace.locator('.trace-detail-head .trace-close').click();
    await trace.locator('.trace-event.assistant').click();
    assert.equal(await trace.locator('.trace-detail-title').innerText(),zh?'Agent 回复':'Agent reply');
    assert.equal(await trace.locator('.trace-preview-mode').getAttribute('aria-selected'),'true','selecting a record starts with Preview like App');
    assert.equal(await trace.locator('.trace-prose').innerText(),'Inspection complete');
    if(width<700){
      assert.equal(await trace.locator('.trace-history-bar').isVisible(),false,'compact details keep the list history controls out of the reading area');
      assert.equal(await trace.locator('.trace-detail-head').evaluate(n=>n.firstElementChild.classList.contains('trace-close')),true,'Back precedes the speaker avatar like App');
    }
    if(width===1440&&!zh){
      await trace.locator('.trace-detail-identity button').click();
      await page.locator('.participant-profile').waitFor();
      assert.match(await page.locator('.participant-profile').innerText(),/Codex/);
      await page.locator('.participant-profile .dialog-close').click();
    }
    assert.ok(reads.length&&reads.every(url=>url.searchParams.get('context')==='1'));
    assert.equal(fixture.state.sends.length,0);
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
    if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,'trace-parity-'+width+'-'+locale+'.png')});}
  });
}
