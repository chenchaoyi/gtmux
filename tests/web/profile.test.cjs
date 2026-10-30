const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

const time='2026-10-28T14:20:00Z';
function setup(fixture) {
  fixture.state.agents=[{pane_id:'%901',session_id:'current-session',agent:'Codex',task:'Profile QA',session:'QA',status:'working',source:'tmux'}];
  fixture.state.turns['%901']=[{session_id:'old-session',time,agent:'Codex',prompt:'Earlier request',response:'Earlier reply'}];
  fixture.state.profiles={
    ['old-session@'+time]:{agent:'codex',name:'Codex',session_id:'old-session',historical:true,facts:[
      {key:'model',value:'old-recorded-model',source:'session_log',observed_at:1793197200000},
      {key:'provider',value:'openai',source:'session_log'},
      {key:'unknown',value:'Do not display unknown fields',source:'session_log'}]},
    'current-session@null':{agent:'codex',name:'Codex',session_id:'current-session',historical:false,facts:[{key:'model',value:'current-recorded-model',source:'verified_binding'}]},
  };
}
async function openChat(page,fixture) {
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('#radar').getByText('Profile QA',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.getByText('Earlier reply',{exact:true}).waitFor();
}

scenario('HQ roster avatars retain supervisor identity without a pane-browser tier', {viewport:{width:390,height:844},locale:'en-US'},async(page,fixture)=>{
  setup(fixture);fixture.state.agents[0].role='supervisor';
  fixture.state.agents.unshift({pane_id:'%902',session_id:'other-hq',agent:'Codex',task:'Other HQ',role:'supervisor',status:'idle',source:'tmux',project:'Other project'});
  await openChat(page,fixture);
  await page.locator('#chat .chat-state .speaker-avatar').click();
  const dialog=page.locator('.browser-dialog');
  await dialog.getByRole('heading',{name:'gtmux HQ',exact:true}).waitFor();
  assert.match(await dialog.innerText(),/gtmux coordinating agent/);
  assert.doesNotMatch(await dialog.innerText(),/Pane|Directory|Profile scope/);
  assert.doesNotMatch(await dialog.innerText(),/Other project/);
  await dialog.getByText('current-recorded-model',{exact:true}).waitFor();
  assert.equal(await dialog.getByRole('heading',{name:'gtmux HQ',exact:true}).count(),1);
});

scenario('Message profiles use recorded session/time instead of another same-agent live session', {viewport:{width:390,height:844},locale:'zh-CN',hasTouch:true},async(page,fixture)=>{
  setup(fixture);const requests=[];
  page.on('request',r=>{if(r.url().includes('/participant-profile'))requests.push(new URL(r.url()));});
  await openChat(page,fixture);
  await page.locator('#chat .arow > .speaker-avatar').click();
  const dialog=page.locator('.browser-dialog');
  await dialog.getByText('old-recorded-model',{exact:true}).waitFor();
  assert.equal(requests[0].searchParams.get('session_id'),'old-session');
  assert.equal(requests[0].searchParams.get('at'),time);
  assert.match(await dialog.innerText(),/这条消息的记录|OpenAI/);
  assert.doesNotMatch(await dialog.innerText(),/运行中|current-recorded-model|Do not display unknown fields/);
  await dialog.getByText('来源与记录时间',{exact:true}).click();
  assert.match(await dialog.locator('.profile-provenance:not(.profile-chain)').innerText(),/本会话日志/);
  await dialog.getByRole('button',{name:'关闭',exact:true}).click();
  await page.locator('#chat .chat-state .speaker-avatar').click();
  await page.locator('.browser-dialog').getByText('current-recorded-model',{exact:true}).waitFor();
  assert.equal(requests.at(-1).searchParams.has('at'),false);
});

scenario('Closing a profile retires a delayed response before a different profile opens', {viewport:{width:390,height:844},locale:'en-US'},async(page,fixture)=>{
  setup(fixture);let release,started;
  const held=new Promise(r=>{release=r;}),pending=new Promise(r=>{started=r;});
  await page.route('**/api/participant-profile?*',async route=>{
    if(new URL(route.request().url()).searchParams.get('at')){started();await held;await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(fixture.state.profiles['old-session@'+time])}).catch(()=>{});}
    else await route.continue();
  });
  await openChat(page,fixture);
  await page.locator('#chat .arow > .speaker-avatar').click();await pending;
  await page.locator('.browser-dialog').getByRole('button',{name:'Close',exact:true}).click();
  await page.locator('#chat .chat-state .speaker-avatar').click();
  await page.locator('.browser-dialog').getByText('current-recorded-model',{exact:true}).waitFor();
  release();await page.waitForTimeout(150);
  assert.doesNotMatch(await page.locator('.browser-dialog').innerText(),/old-recorded-model/);
  fixture.state.deny=true;
  await page.locator('#gate').waitFor();
  assert.equal(await page.locator('.browser-dialog').count(),0);
});

scenario('A profile without message identity or owner access does not request current facts', {viewport:{width:390,height:844},locale:'en-US'},async(page,fixture)=>{
  setup(fixture);fixture.state.turns['%901']=[{agent:'Codex',prompt:'Earlier request',response:'Earlier reply'}];
  const requests=[];page.on('request',r=>{if(r.url().includes('/participant-profile'))requests.push(r.url());});
  await openChat(page,fixture);
  await page.locator('#chat .arow > .speaker-avatar').click();
  await page.locator('.browser-dialog').getByText('No recorded profile is available for this message.',{exact:true}).waitFor();
  assert.equal(requests.length,0);
  await page.locator('.browser-dialog').getByRole('button',{name:'Close',exact:true}).click();
  fixture.state.share={all:false,input:false,panes:[],view_panes:['%901']};
  await page.reload();
  await openChat(page,fixture);
  await page.locator('#chat-input .composer').waitFor({state:'hidden'});
  await page.locator('#chat-ro').waitFor();
  await page.locator('#chat .chat-state .speaker-avatar').click();
  await page.locator('.browser-dialog').getByText('Recorded profiles are unavailable for this connection.',{exact:true}).waitFor();
  assert.equal(requests.length,0);
});

scenario('Agent profile grid and execution chain retain exact current conversation identity',{viewport:{width:390,height:844},locale:'en-US'},async(page,fixture)=>{
 setup(fixture);fixture.state.profiles['current-session@null'].execution_chain=[{name:'synthetic-client',path:'/fixture/client',arguments:['--mode','synthetic']}];
 await openChat(page,fixture);await page.locator('#chat .chat-state .speaker-avatar').click();const dialog=page.locator('.participant-profile');await dialog.getByText('current-recorded-model',{exact:true}).waitFor();
 assert.equal(await dialog.locator('.profile-field').count(),1);await dialog.locator('.profile-chain summary').click();assert.match(await dialog.locator('.profile-chain').innerText(),/synthetic-client|Recorded launch ancestry/);
 assert.doesNotMatch(await dialog.innerText(),/Directory|Pane|Browser access/);
 if(process.env.GTMUX_WEB_EVIDENCE) await page.screenshot({path:require('node:path').join(process.env.GTMUX_WEB_EVIDENCE,'profile-agent-recorded.png')});
});

scenario('Historical profile reads current ancestry only on expansion and only for the same conversation',{viewport:{width:390,height:844},locale:'en-US'},async(page,fixture)=>{
 setup(fixture);fixture.state.turns['%901'][0].session_id='current-session';fixture.state.profiles['current-session@'+time]={...fixture.state.profiles['old-session@'+time],session_id:'current-session'};
 fixture.state.profiles['current-session@null'].execution_chain=[{name:'verified-client',path:'/fixture/verified-client'}];
 const requests=[];page.on('request',r=>{if(r.url().includes('/participant-profile'))requests.push(new URL(r.url()));});
 await openChat(page,fixture);await page.locator('#chat .arow > .speaker-avatar').click();const dialog=page.locator('.participant-profile');await dialog.getByText('old-recorded-model',{exact:true}).waitFor();assert.equal(requests.length,1);
 await dialog.locator('.profile-chain summary').click();await dialog.getByText('verified-client',{exact:true}).waitFor();assert.equal(requests.length,2);assert.equal(requests[1].searchParams.get('session_id'),'current-session');assert.equal(requests[1].searchParams.has('at'),false);
 assert.match(await dialog.locator('.profile-chain').innerText(),/not the processes at this message/);assert.doesNotMatch(await dialog.innerText(),/current-recorded-model/);
});
