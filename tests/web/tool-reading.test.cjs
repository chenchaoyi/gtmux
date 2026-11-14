const assert=require('node:assert/strict');
const path=require('node:path');
const {scenario}=require('./harness.cjs');
const {openTools}=require('./tools.cjs');
for(const width of [390,1440])scenario('TSV-003 compact recorded command and readable result '+width,{viewport:{width,height:950},locale:'en-US',colorScheme:'dark'},async(page,fixture)=>{
 const command="printf 'synthetic output'";const source=JSON.stringify(['/bin/bash','-c',command]);
 const result=JSON.stringify({content:[{type:'text',text:'[32mReady[0m\n\x1b[2mPreserved output\x1b[0m'}]});
 const step={kind:'tool',title:'exec',operation:'command',input_kind:'arguments',input:source,workdir:'file:///tmp/public%20fixture',call:{name:'exec',loaded:true,has_arguments:true,arguments:source,has_result:true,result}};
 fixture.state.agents[0].session_id='fixture-session';fixture.state.turns['%901']=[{session_id:'fixture-session',prompt:'Synthetic tool detail',response:'Recorded',segments:[{steps:[step]}]}];
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width<900){await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();}
 else {await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();}
 const chat=page.locator(width<900?'#chat':'.tile');await chat.locator('.steps-toggle').click();await chat.locator('.step-toggle').click();
 const input=chat.locator('[data-call-section="input"]');await input.getByRole('button',{name:'View original arguments',exact:true}).waitFor();
 assert.equal(await input.locator('pre').innerText(),command);assert.equal(await input.locator('.code-call-outline').count(),0);
 const output=chat.locator('[data-call-section="result-0"]');assert.equal(await output.locator('pre').innerText(),'Ready\nPreserved output');
 assert.match(await chat.locator('.step-meta').innerText(),/\/tmp\/public fixture/);assert.doesNotMatch(await chat.locator('.step-meta').innerText(),/file:\/\/\//);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
 if(process.env.GTMUX_WEB_RESULTS)await page.screenshot({path:path.join(process.env.GTMUX_WEB_RESULTS,'tool-reading-'+width+'.png')});
 await input.getByRole('button',{name:'View original arguments',exact:true}).click();assert.deepEqual(JSON.parse(await input.locator('pre').innerText()),JSON.parse(source));
 await openTools(page,'trace',width>=900);await page.locator('.trace-event.tool').click();
 const trace=page.locator('.session-tool .trace-view');await trace.locator('[data-call-section="input"]').waitFor();
 assert.equal(await trace.locator('[data-call-section="input"] pre').innerText(),command);
 await trace.locator('.trace-source-mode').click();const raw=trace.locator('[data-call-section="raw"] pre');await raw.waitFor();assert.match(await raw.innerText(),/32mReady/);
 assert.equal(fixture.state.sends.length,0);
});
