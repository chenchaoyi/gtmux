const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');
const {openTools, closeTools} = require('./tools.cjs');
const record = n => ({session_id:'history-session',record_id:'record-'+n,turn_ordinal:n,agent:'Codex',prompt:'Question '+n,response:('Answer '+n+' — readable context. ').repeat(12)});
async function enroll(page, fixture) { await page.goto(fixture.url+'/#code=TEST-1234'); }
async function focus(page, width) {
  if(width>=900) { await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile [data-action="focus"]').click(); }
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
}
for(const width of [390,1440]) for(const colorScheme of ['light','dark']) scenario(`Client reading consistency ${width} ${colorScheme}`,{viewport:{width,height:950},locale:'en-US',colorScheme,hasTouch:width<900},async(page,fixture)=>{
  fixture.state.turns['%901']=[{...record(151),segments:[{steps:[{title:'exec',operation:'code',input_kind:'code',input:'const result={ok:true,count:2};'}]},{text:'Ready'}]}];
  fixture.state.screens['%901']='Working on the current task\n'+('long terminal column '.repeat(40));
  await enroll(page,fixture);await focus(page,width);
  const live=page.locator('#chat .chat-live');await live.locator('summary').click();await live.locator('.live-output').getByText(/Working on the current task/).waitFor();
  const style=await live.evaluate(n=>({bg:getComputedStyle(n).backgroundColor,fg:getComputedStyle(n.querySelector('pre')).color,white:getComputedStyle(n.querySelector('pre')).whiteSpace,padding:parseFloat(getComputedStyle(n.querySelector('pre')).paddingBottom)}));
  assert.equal(style.bg,'rgb(23, 23, 26)');assert.equal(style.fg,'rgb(214, 214, 218)');assert.equal(style.white,'pre');assert.ok(style.padding>=14);
  await page.locator('#chat .steps-toggle').click();await page.locator('#chat .step-toggle').click();
  const format=page.locator('#chat .code-format');assert.equal(await format.evaluate(n=>getComputedStyle(n).borderTopWidth),'0px');assert.ok((await format.boundingBox()).width>=36);
  assert.equal(await format.getAttribute('aria-pressed'),'true');await format.click();assert.equal(await format.getAttribute('aria-pressed'),'false');
  await openTools(page);const row=page.locator('.trace-turn').first();await row.waitFor();assert.match(await row.innerText(),/Turn 151/);assert.doesNotMatch(await row.innerText(),/Codex/);
  await closeTools(page);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
  if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,`client-consistency-${width}-${colorScheme}.png`)});}
});
for(const view of ['focus','tile','external']) scenario(`Chat earlier windows preserve reading and refresh in ${view}`,{viewport:{width:view==='focus'?390:1440,height:950},locale:'en-US'},async(page,fixture)=>{
  const native={source:'native',agent:'DeepSeek Harness',task:'External history',session_id:'history-session',home_id:'home-history',can_read_conversation:true,status:'idle',current_client:'unconfirmed'};
  let tail=Array.from({length:8},(_,i)=>record(151+i));tail[0].history_before='before-151';
  fixture.state.expectedHttpErrors.push(503,409);
  let fail=false, changed=false;
  if(view==='external') fixture.state.agents=[native];
  fixture.state.turns['%901']=tail;
  await page.route(view==='external'?'**/api/session/transcript?*':'**/api/transcript?*',async route=>{
    const before=new URL(route.request().url()).searchParams.get('before');
    if(!before) return route.fulfill({json:tail});
    if(changed) return route.fulfill({status:409,json:{error:'source changed'}});
    if(fail) return route.fulfill({status:503,json:{error:'synthetic retry'}});
    const end=Number(before.split('-')[1])-1;const rows=Array.from({length:8},(_,i)=>record(end-7+i));rows[0].history_before='before-'+(end-7);
    return route.fulfill({status:200,headers:{'X-Gtmux-History-Page':'1'},json:rows});
  });
  await enroll(page,fixture);
  if(view==='focus')await focus(page,390);
  else if(view==='tile'){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();}
  else await page.locator('#tree').getByText('External history',{exact:true}).dblclick();
  const root=page.locator(view==='focus'?'#chat':'.tile-body');const scroll=root.locator(view==='focus'?'.conv-scroll':'.tile-chat');
  await root.getByText('Question 151',{exact:true}).waitFor();await scroll.evaluate(n=>{n.scrollTop=0;});
  const anchor=root.locator('.cturn').first(), y=await anchor.evaluate(n=>n.getBoundingClientRect().top);
  await root.locator('.chat-history-earlier').click();await root.getByText('Question 143',{exact:true}).waitFor();
  assert.ok(Math.abs(await root.locator('.cturn').filter({hasText:'Question 151'}).evaluate(n=>n.getBoundingClientRect().top)-y)<3,'prepend keeps the visible turn in place');
  tail=Array.from({length:8},(_,i)=>record(152+i));tail[0].history_before='before-152';
  await root.getByText('Question 159',{exact:true}).waitFor();assert.equal(await root.getByText('Question 143',{exact:true}).count(),1);
  tail=Array.from({length:8},(_,i)=>record(153+i));tail[0].history_before='before-153';
  await root.getByText('Question 160',{exact:true}).waitFor();assert.equal(await root.getByText('Question 143',{exact:true}).count(),1,'successive rolling windows retain both old and newly received turns');
  fail=true;await root.locator('.chat-history-earlier').click();await root.getByText('Could not load earlier messages.',{exact:true}).waitFor();assert.equal(await root.getByText('Question 143',{exact:true}).count(),1);
  fail=false;await root.locator('.chat-history-earlier').click();await root.getByText('Question 135',{exact:true}).waitFor();
  changed=true;await root.locator('.chat-history-earlier').click();await root.getByText('Load latest messages',{exact:true}).waitFor();
  await root.getByText('Load latest messages',{exact:true}).click();assert.equal(await root.getByText('Question 135',{exact:true}).count(),0);assert.equal(await root.getByText('Question 159',{exact:true}).count(),1);
  assert.equal(fixture.state.sends.length,0);
});
scenario('External board tiles stay scoped, resize, restore and revoke without pane input',{viewport:{width:1440,height:950},locale:'en-US'},async(page,fixture)=>{
  const native=home=>({source:'native',agent:'DeepSeek Harness',task:home,session_id:'same-session',home_id:home,can_read_conversation:true,status:'idle',current_client:'unconfirmed'});
  fixture.state.agents.push(native('home-a'),native('home-b'));
  fixture.state.externalSessions={'same-session\u0000home-a':[{prompt:'Home A',response:'Reply A'}],'same-session\u0000home-b':[{prompt:'Home B',response:'Reply B'}]};
  await page.route('**/api/session/transcript?*',async route=>{const u=new URL(route.request().url()),home=u.searchParams.get('home_id');return route.fulfill({headers:{'X-Gtmux-Turns-Dropped':home==='home-a'?'7':'11'},json:fixture.state.externalSessions['same-session\u0000'+home]});});
  const requests=[];page.on('request',r=>requests.push(new URL(r.url()).pathname));
  await enroll(page,fixture);
  const a=page.locator('#tree').getByText('home-a',{exact:true});await a.click();assert.equal(await page.locator('#workbench').isVisible(),true);assert.equal(await page.locator('.tile').count(),0);
  await a.dblclick();await page.locator('.tile').getByText('Reply A',{exact:true}).waitFor();
  const firstTile=page.locator('.tile'),before=await firstTile.boundingBox(),handle=await firstTile.locator('.tile-resize').boundingBox();await page.mouse.move(handle.x+3,handle.y+3);await page.mouse.down();await page.mouse.move(handle.x+93,handle.y+53);await page.mouse.up();assert.ok((await firstTile.boundingBox()).width>before.width+50);
  await page.locator('#tree').getByText('home-b',{exact:true}).dblclick();await page.locator('.tile').getByText('Reply B',{exact:true}).waitFor();
  await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();
  assert.equal(await page.locator('.tile').count(),3);
  const ext=page.locator('.tile').filter({hasText:'Reply A'});assert.equal(await ext.locator('[data-m="term"]').count(),0);assert.equal(await ext.locator('[data-tool="diff"]').count(),0);assert.equal(await ext.locator('.cx-input').count(),0);
  await a.dblclick();assert.match(await ext.locator('.chat-source-note').innerText(),/7/);
  await ext.locator('[data-tool="trace"]').click();await page.locator('.trace-history-bar').getByText('7 earlier turns',{exact:true}).waitFor();await closeTools(page);
  await a.dblclick();await ext.locator('[data-action="focus"]').click();await page.locator('#chat').getByText('Reply A',{exact:true}).waitFor();await page.locator('#back').click();await ext.getByText('Reply A',{exact:true}).waitFor();
  await page.reload();await page.locator('.tile').getByText('Reply A',{exact:true}).waitFor();assert.equal(await page.locator('.tile').count(),3);
  fixture.state.agents=fixture.state.agents.filter(a=>a.home_id!=='home-a');await page.waitForFunction(()=>document.querySelectorAll('.tile').length===2);assert.equal(await page.getByText('Reply A',{exact:true}).count(),0);
  assert.ok(!requests.some(p=>p.endsWith('/api/send')));assert.equal(fixture.state.sends.length,0);
});
scenario('Readable external conversations can be dragged into a board panel',{viewport:{width:1440,height:950},locale:'en-US'},async(page,fixture)=>{
 fixture.state.agents=[{source:'native',agent:'DeepSeek Harness',task:'Drag this conversation',session_id:'drag-session',home_id:'drag-home',can_read_conversation:true,status:'idle',current_client:'unconfirmed'}];
 fixture.state.externalSessions={'drag-session\u0000drag-home':[{prompt:'Dragged question',response:'Dragged reply'}]};
 await enroll(page,fixture);await page.locator('.tree-row.native').dragTo(page.locator('#board'),{targetPosition:{x:100,y:100}});
 await page.locator('.tile').getByText('Dragged reply',{exact:true}).waitFor();assert.equal(await page.locator('.tile').count(),1);assert.equal(await page.locator('.tile .cx-input').count(),0);
});
scenario('A late Chat history page cannot enter another conversation',{viewport:{width:390,height:950},locale:'en-US'},async(page,fixture)=>{
 fixture.state.agents=fixture.state.agents.slice(0,2);fixture.state.agents[0].session_id='history-session';fixture.state.turns['%901']=[{...record(151),history_before:'before-151'}];
 fixture.state.turns['%902']=[{session_id:'another-session',record_id:'other',prompt:'Other conversation',response:'Other reply'}];
 let release,started;const hold=new Promise(r=>{release=r;}),pending=new Promise(r=>{started=r;});
 await page.route('**/api/transcript?*',async route=>{const u=new URL(route.request().url());if(u.searchParams.has('before')){started();await hold;await route.fulfill({headers:{'X-Gtmux-History-Page':'1'},json:[record(150)]}).catch(()=>{});}else await route.continue();});
 await enroll(page,fixture);await focus(page,390);await page.locator('#chat .chat-history-earlier').click();await pending;
 await page.locator('#back').click();await page.locator('#radar').getByText('Review notes',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();await page.locator('#chat').getByText('Other reply',{exact:true}).waitFor();release();
 await page.waitForTimeout(150);assert.doesNotMatch(await page.locator('#chat').innerText(),/Question 150|Answer 150/);assert.equal(await page.locator('#chat .chat-history').count(),0);
});

require('node:test')('Diff glyph geometry is shared across Web and both App action catalogs',async()=>{
 const mobile=await fs.readFile(path.join(__dirname,'../../mobileapp/src/ui/SettingsIcons.tsx'),'utf8');
 const web=await fs.readFile(path.join(__dirname,'../../internal/server/web/app.js'),'utf8');
 const actions=await fs.readFile(path.join(__dirname,'../../mobileapp/src/ui/ActionIcon.tsx'),'utf8');
 const geometry=/case 'diff':[\s\S]*?<Path d="([^"]+)"/.exec(mobile)[1];
 assert.ok(web.includes(geometry));assert.match(actions,/if \(name === 'diff'\) return <SIcon name="diff"/);
});
