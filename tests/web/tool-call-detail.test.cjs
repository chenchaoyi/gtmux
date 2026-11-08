const assert=require('node:assert/strict');
const fs=require('node:fs/promises');const path=require('node:path');
const {scenario}=require('./harness.cjs');const {openTools}=require('./tools.cjs');
const step={kind:'tool',title:'github.fetch_commit_workflow_runs',operation:'tool',input_ref:'native-ref',call:{provider:'GitHub',name:'github.fetch_commit_workflow_runs',status:'completed',duration_ms:42,has_arguments:true,has_result:true,ref:'native-ref'}};
const full={...step,input:'',call:{...step.call,loaded:true,arguments:JSON.stringify({repo_full_name:'example/project',commit_sha:'abc123'}),result:JSON.stringify({structuredContent:{workflow_runs:[{conclusion:'success'}]}})}};
for(const width of [390,768,1440])scenario('Typed query details in Chat and Trace '+width,{viewport:{width,height:920},locale:'en-US',colorScheme:width===390?'dark':'light'},async(page,fixture)=>{
 fixture.state.agents[0].session_id='fixture-session';fixture.state.toolInputs={'native-ref':full};
 fixture.state.turns['%901']=[{session_id:'fixture-session',prompt:'Inspect runs',response:'Done',segments:[{steps:[step]}]}];
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width<900){await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();}
 else{await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();}
 const chat=page.locator(width<900?'#chat':'.tile');await chat.locator('.steps-toggle').click();assert.equal(fixture.state.toolReads||0,0);await chat.locator('.step-toggle').click();await chat.getByText('example/project',{exact:true}).waitFor();assert.equal(fixture.state.toolReads,1);assert.match(await chat.locator('.tool-call-details').innerText(),/success/);
 await openTools(page,'trace',width>=900);await page.locator('.trace-event.tool').click();const trace=page.locator('.session-tool .trace-view');await trace.locator('[data-call-section="arguments"]').waitFor();assert.match(await trace.locator('.trace-detail-title').innerText(),/GitHub/);assert.match(await trace.locator('[data-call-section="result"]').innerText(),/success/);assert.equal(await trace.getByText('Only a call summary was recorded.',{exact:true}).count(),0);
 if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,'tool-preview-'+width+'.png')});}
 await trace.locator('.trace-source-mode').click();assert.match(await trace.locator('[data-call-section="raw"]').innerText(),/commit_sha/);assert.match(await trace.locator('[data-call-section="raw"]').innerText(),/structuredContent/);assert.equal(fixture.state.toolReads,1);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
 if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,'tool-call-'+width+'.png')});}
});

scenario('Recorded query failure retries and large results page without losing full copy',{viewport:{width:390,height:920},locale:'en-US'},async(page,fixture)=>{
 fixture.state.expectedHttpErrors.push(404);fixture.state.agents[0].session_id='fixture-session';fixture.state.turns['%901']=[{session_id:'fixture-session',prompt:'Read result',response:'Done',segments:[{steps:[step]}]}];
 await page.goto(fixture.url+'/#code=TEST-1234');await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await openTools(page);await page.locator('.trace-event.tool').click();
 const warning=page.locator('.trace-truncated');await warning.getByText('Could not load full input. Tap to retry.',{exact:true}).waitFor();
 const result='first line\n'+'result segment '.repeat(3000)+'LAST';fixture.state.toolInputs={'native-ref':{...full,call:{...full.call,result:JSON.stringify(result)}}};await warning.click();
 const section=page.locator('[data-call-section="result"]');await section.getByRole('button',{name:'Next page',exact:true}).waitFor();
 await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async value=>{window.callCopy=value;}}}));await section.getByRole('button',{name:'Copy full Result',exact:true}).click();assert.equal(await page.evaluate(()=>window.callCopy),result);
 while(await section.getByRole('button',{name:'Next page',exact:true}).isEnabled())await section.getByRole('button',{name:'Next page',exact:true}).click();assert.match(await section.innerText(),/LAST/);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
});
