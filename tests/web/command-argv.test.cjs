const assert=require('node:assert/strict');
const {scenario}=require('./harness.cjs');
const {openTools}=require('./tools.cjs');
for(const width of [390,1440])scenario('ARGV-003 Trace argv scripts share typed code reading '+width,{viewport:{width,height:950},locale:'en-US'},async(page,fixture)=>{
 const py='import json\nx={"n":1};print(x)\n';
 const source=JSON.stringify(['/bin/bash','-c',"python3 - <<'PY'\n"+py+'PY\necho done']).replace(/</g,'\\u003c');
 fixture.state.agents=[{pane_id:'%901',agent:'Codex',task:'Argv reading',session:'QA',status:'idle',source:'tmux'}];
 fixture.state.turns['%901']=[{session_id:'synthetic-argv',agent:'Codex',prompt:'Inspect command',segments:[{steps:[{title:'Run command',input:source,input_kind:'command',operation:'command'}]}]}];
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width>=900){await page.locator('#tree').getByText('Argv reading',{exact:true}).dblclick();await page.locator('.tile [data-action="focus"]').click();}
 else await page.locator('#radar').getByText('Argv reading',{exact:true}).click();
 await page.locator('#mode [data-mode="chat"]').click();await openTools(page);
 const trace=page.locator('.trace-view');await trace.locator('.trace-event.tool').click();
 const inspector=trace.locator('.trace-inspector');
 await inspector.getByRole('tab',{name:/^Python/}).click();
 assert.match(await inspector.locator('pre.trace-source').innerText(),/x = \{"n": 1\}\nprint\(x\)/);
 assert.ok(await inspector.locator('pre.trace-source span').count()>0,'Actual syntax tokens render');
 if(process.env.GTMUX_WEB_EVIDENCE)await page.screenshot({path:process.env.GTMUX_WEB_EVIDENCE+'/argv-'+width+'.png'});
 await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.argvCopy=text;}}}));
 await inspector.getByRole('button',{name:'Copy',exact:true}).click();
 assert.equal(await page.evaluate(()=>window.argvCopy),source);
 await inspector.getByRole('tab',{name:'Full call',exact:true}).click();
 assert.deepEqual(JSON.parse(await inspector.locator('pre.trace-source').innerText()),JSON.parse(source));
 await inspector.getByRole('tab',{name:'Raw content',exact:true}).click();
 assert.equal(await inspector.locator('pre.trace-source').innerText(),source);
 assert.equal(await inspector.getByRole('tab',{name:/^Python/}).count(),0);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
 assert.equal(fixture.state.sends.length,0);
});
