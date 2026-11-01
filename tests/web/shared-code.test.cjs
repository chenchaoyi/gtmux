const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');
const {openTools} = require('./tools.cjs');
const python = 'from pathlib import Path\na=Path("/example");b={"count":2};print(a,b)\n';
const source = 'await tools.exec_command({cmd:' + JSON.stringify("set -e\npython3 - <<'PY'\n" + python + 'PY\ngit status') + '});';
for(const width of [320,768,1440]) for(const colorScheme of ['light','dark']) {
  scenario(`Shared code and nested trace calls ${width} ${colorScheme}`, {viewport:{width,height:1000},colorScheme,locale:'en-US',hasTouch:width<900,permissions:['clipboard-read','clipboard-write']}, async(page,fixture)=>{
    fixture.state.turns['%901']=[{session_id:'synthetic',prompt:'Inspect scripts',time:'2026-11-01T07:00:00Z',response:'Done',segments:[
      {steps:Array.from({length:5},()=>({kind:'tool',title:'exec',operation:'code',input_kind:'code',input:source}))},
      {text:'Done'}]}];
    await page.addInitScript(()=>localStorage.setItem('gtmux.chatFontSize','20'));
    await page.goto(fixture.url+'/#code=TEST-1234');
    if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile [data-action="focus"]').click();}
    else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
    await page.locator('#mode [data-mode="chat"]').click();
    await page.locator('#chat .steps-toggle').click();
    assert.equal(await page.locator('#chat .code-panel').count(),0,'folded call details do not parse or mount code');
    await page.locator('#chat .step-toggle').first().click();
    const chat=page.locator('#chat .code-panel').first();
    assert.equal(await chat.locator('pre').evaluate(node=>getComputedStyle(node).fontSize),'19px');
    assert.match(await chat.locator('pre').innerText(),/git status/);
    assert.equal(await chat.locator('.code-language').innerText(),'Shell');
    await chat.getByRole('tab',{name:'Python · cmd 1'}).click();
    assert.match(await chat.locator('pre').innerText(),/a = Path/);
    assert.equal(await chat.getByRole('tab',{name:'Python · cmd 1'}).getAttribute('aria-selected'),'true');
    await chat.getByRole('button',{name:'Copy recorded input',exact:true}).click();
    assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),source);
    await chat.getByRole('button',{name:'Show original',exact:true}).click();
    fixture.state.turns['%901'][0].segments[1].text='A later progress record';
    await page.locator('#chat').getByText('A later progress record',{exact:true}).waitFor();
    assert.equal(await chat.getByRole('tab',{name:'Python · cmd 1'}).getAttribute('aria-selected'),'true');
    assert.equal(await chat.locator('pre').innerText(),python);
    await chat.getByRole('tab',{name:'Full call',exact:true}).click();
    assert.match(await chat.locator('pre').innerText(),/tools\.exec_command/);
    assert.equal(await chat.locator('pre').innerText(),source);
    await openTools(page,'trace');
    const trace=page.locator('.session-tool .trace-view');
    if(await trace.locator('.trace-calls').getAttribute('aria-expanded')!=='true')
      await trace.locator('.trace-calls').click();
    const children=trace.locator('.trace-call-child');
    assert.equal(await children.count(),5);
    const parent=await trace.locator('.trace-calls').boundingBox(), child=await children.first().boundingBox();
    assert.ok(child.x>=parent.x+24);
    assert.match(await children.first().getAttribute('aria-description'),/Call 1 of 5/);
    if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,`nested-calls-${width}-${colorScheme}.png`)});}
    await children.first().click();
    const preview=trace.locator('.code-panel');
    assert.equal(await preview.locator('pre').evaluate(node=>getComputedStyle(node).fontSize),'19px');
    await preview.getByRole('tab',{name:'Python · cmd 1'}).click();
    assert.match(await preview.locator('pre').innerText(),/b = \{"count": 2\}/);
    const colors=await preview.locator('pre span').evaluateAll(nodes=>new Set(nodes.map(n=>n.style.color)).size);
    assert.ok(colors>1);
    await preview.getByRole('tab',{name:'Full call',exact:true}).click();
    await preview.getByRole('button',{name:'Show original',exact:true}).click();
    assert.equal(await preview.locator('pre').innerText(),source);
    await trace.getByRole('tab',{name:'Raw content',exact:true}).click();
    assert.equal(await trace.locator('.trace-source').innerText(),source);
    assert.equal(await trace.locator('.code-parts [role="tab"]').count(),1);
    assert.equal(await trace.locator('.code-format').count(),0);
    await trace.getByRole('tab',{name:'Preview',exact:true}).click();
    await preview.getByRole('tab',{name:'Python · cmd 1'}).click();
    if(process.env.GTMUX_WEB_EVIDENCE)await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,`python-preview-${width}-${colorScheme}.png`)});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
    assert.equal(fixture.state.sends.length,0);
  });
}
