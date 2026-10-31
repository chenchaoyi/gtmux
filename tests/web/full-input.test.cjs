const assert=require('node:assert/strict');
const {scenario}=require('./harness.cjs');
const {openTools,closeTools}=require('./tools.cjs');
for(const width of [390,1440])scenario('Full tool input and compact formatting at '+width,{viewport:{width,height:900},locale:'en-US'},async(page,fixture)=>{
 const raw='const result={text:"'+ 'retained-value-'.repeat(800)+'",count:2};text(result);';
 const step={kind:'tool',title:'exec',input:'const…',detail:'Preview only',input_kind:'code',operation:'code',details_truncated:true,input_ref:'full-ref'};
 fixture.state.agents[0].session_id='fixture-session';
 fixture.state.turns['%901']=[{session_id:'fixture-session',prompt:'Inspect full code',response:'Done',segments:[{text:'Done',steps:[step]}]}];
 fixture.state.toolInputs={'full-ref':{...step,input:raw,details_truncated:false}};
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width<700){await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();await page.locator('#mode [data-mode="chat"]').click();}
 else{await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();}
 const chat=page.locator(width<700?'#chat':'.tile');
 await chat.locator('.steps-toggle').click();
 assert.equal(fixture.state.toolReads||0,0);
 await chat.locator('.step-toggle').click();
 await page.waitForFunction(()=>[...document.querySelectorAll('.step-input')].some(n=>n.textContent.includes('text(result);')));
 const input=chat.locator('.step-input');
 assert.ok((await input.innerText()).includes('retained-value-'.repeat(800)));
 assert.ok((await input.innerText()).includes('\n'));
 await chat.getByRole('button',{name:'Show original',exact:true}).click();
 assert.equal(await input.innerText(),raw);
 const pos=await input.evaluate(n=>{n.scrollTop=40;return n.scrollTop});
 await page.waitForTimeout(1200);assert.equal(await input.evaluate(n=>n.scrollTop),pos);
 await openTools(page,'trace',width>=700);
 await page.locator('.trace-event.tool').click();
 await page.waitForFunction(()=>[...document.querySelectorAll('.trace-source')].some(n=>n.textContent.includes('text(result);')));
 await page.locator('.trace-source-mode').click();
 assert.equal(await page.locator('.trace-source').innerText(),raw);
 assert.equal(fixture.state.toolReads,1);
 await page.screenshot({path:(process.env.GTMUX_WEB_RESULTS||'/tmp')+'/full-input-'+width+'.png'});
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
 await closeTools(page);
});
for(const width of [390,1440])scenario('Diff file cards and independent hunks at '+width,{viewport:{width,height:900},locale:'en-US'},async(page,fixture)=>{
 fixture.state.diff='diff --git a/internal/review.go b/internal/review.go\n--- a/internal/review.go\n+++ b/internal/review.go\n@@ -1,2 +1,2 @@\n package server\n-func old() {}\n+func updated() { return }\n@@ -12 +12 @@\n-old\n+last\ndiff --git a/docs/report.md b/docs/report.md\n--- a/docs/report.md\n+++ b/docs/report.md\n@@ -1 +1 @@\n-before\n+after';
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width<700){await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();}
 else{await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();}
 await openTools(page,'diff',width>=700);
 await page.locator('.diff-file').first().waitFor();
 assert.equal(await page.locator('.diff-file').count(),2);
 assert.match(await page.locator('.diff-summary').innerText(),/2 files/);
 await page.locator('.diff-file').first().locator('.diff-hunk').first().locator('.diff-code').last().waitFor();
 assert.equal(await page.locator('.diff-file').nth(1).locator('.diff-code').count(),0);
 await page.locator('.diff-file').nth(1).locator('.diff-hunk-heading').click();
 assert.equal(await page.locator('.diff-file').nth(1).locator('.diff-line.add .diff-code').innerText(),'after');
 const header=page.locator('.diff-file-heading').first();await header.click();assert.equal(await page.locator('.diff-file').first().evaluate(n=>n.open),false);await header.click();
 await page.locator('.diff-wrap').click();assert.equal(await page.locator('.diff-wrap').getAttribute('aria-pressed'),'false');await page.locator('.diff-wrap').click();
 assert.ok(await page.locator('.diff-code span').count()>0);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
 await page.screenshot({path:(process.env.GTMUX_WEB_RESULTS||'/tmp')+'/diff-'+width+'.png'});
});
