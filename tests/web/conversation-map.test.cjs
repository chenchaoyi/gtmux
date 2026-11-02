const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {scenario}=require('./harness.cjs');
const turns=()=>Array.from({length:60},(_,i)=>({session_id:'long-session',record_id:'record-'+i,turn_ordinal:151+i,agent:'Codex',prompt:`Question ${151+i}: review navigation`,response:(`Finding ${151+i}: preserve the reader's position.\n\n`).repeat(5)}));
for(const width of [390,1440])scenario(`Long chat overview lens and navigation at ${width}`,{viewport:{width,height:900},hasTouch:width<700},async(page,fixture)=>{
 fixture.state.turns['%901']=turns();
 await page.goto(fixture.url+'/#code=TEST-1234');
 if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();await page.locator('.tile-btn[data-action="focus"]').click();}
 else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
 await page.locator('#mode [data-mode="chat"]').click();
 const track=page.getByRole('slider',{name:'Conversation overview — loaded turns'}),tip=page.locator('#chat .conversation-map-preview'),scroll=page.locator('.conv-scroll');
 await track.waitFor();await track.focus();await track.press('Home');
 assert.match(await tip.innerText(),/Turn 151/);
 await track.press('ArrowDown');assert.match(await tip.innerText(),/Question 152/);
 const box=await track.boundingBox();
 if(width>=700){await page.mouse.move(box.x+20,box.y+box.height*.3);assert.match(await tip.innerText(),/Question 169/);assert.ok(await track.locator('i').evaluateAll(m=>m.some(x=>parseFloat(getComputedStyle(x).width)>20)));}
 else {await track.tap({position:{x:20,y:box.height*.3}});assert.match(await tip.innerText(),/Question 169/);}
 const previous=await scroll.evaluate(e=>e.scrollTop);
 await tip.getByRole('button',{name:'Go to turn',exact:true}).click();
 assert.ok(await scroll.evaluate(e=>e.scrollTop)<previous);
 const top=await scroll.evaluate(e=>e.scrollTop);
 fixture.state.turns['%901'].push({session_id:'long-session',record_id:'new',turn_ordinal:211,prompt:'New question',response:'New response'});
 await page.waitForFunction(()=>document.querySelectorAll('#chat .cturn').length===61);
 assert.ok(Math.abs(await scroll.evaluate(e=>e.scrollTop)-top)<2,'incoming turn must preserve reading position');
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
 await track.focus();await track.press('End');await track.press('Enter');
 assert.equal(await page.locator('#chat .cturn').last().evaluate(e=>e===document.activeElement),true);
 const last=await page.locator('#chat .cturn').last().boundingBox(),reading=await scroll.boundingBox();assert.ok(last.y<reading.y+reading.height && last.y+last.height>reading.y);
 if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await track.press('Home');await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,`conversation-map-${width}.png`)});}
 assert.equal(fixture.state.sends.length,0);
});
