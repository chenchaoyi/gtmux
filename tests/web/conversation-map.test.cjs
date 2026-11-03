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
 if(width>=700){await page.mouse.move(box.x+20,box.y+box.height*.3);assert.match(await tip.innerText(),/Question 169/);await page.waitForFunction(()=>Array.from(document.querySelectorAll('.conversation-map-track i')).some(x=>parseFloat(getComputedStyle(x).width)>20));}
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
 await page.waitForFunction(()=>document.querySelector('#chat .cturn:last-child')===document.activeElement);
 assert.equal(await page.locator('#chat .cturn').last().evaluate(e=>e===document.activeElement),true);
 fixture.state.turns['%901'].at(-1).response='Updated while this turn has keyboard focus';
 await page.locator('#chat').getByText('Updated while this turn has keyboard focus',{exact:true}).waitFor();
 assert.equal(await page.locator('#chat .cturn').last().evaluate(e=>e===document.activeElement),true,'focused turn survives transcript refresh');
 const last=await page.locator('#chat .cturn').last().boundingBox(),reading=await scroll.boundingBox();assert.ok(last.y<reading.y+reading.height && last.y+last.height>reading.y);
 await track.focus();await track.press('Home');
 for(let i=0;i<19;i++)await track.press('ArrowDown');
 await tip.getByRole('button',{name:'Go to turn',exact:true}).click();
 const visible=page.locator('#chat .cturn').filter({hasText:'Question 170:'});
 const anchored=await visible.evaluate(e=>e.getBoundingClientRect().top);
 fixture.state.turns['%901'].shift();
 fixture.state.turns['%901'].push({session_id:'long-session',record_id:'roll',turn_ordinal:212,prompt:'Rolled history window',response:'Still reading an earlier turn'});
 await page.locator('#chat').getByText('Rolled history window',{exact:true}).waitFor();
 assert.ok(Math.abs(await visible.evaluate(e=>e.getBoundingClientRect().top)-anchored)<2,'rolling recent-history window preserves the visible turn');

 if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await track.press('Home');await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,`conversation-map-${width}.png`)});}
 assert.equal(fixture.state.sends.length,0);
});

for(const width of [320,800,1440])scenario(`Overview formatted preview and persistent bookmarks at ${width}`,{viewport:{width,height:900},hasTouch:width<700},async(page,fixture)=>{
 const rows=turns();
 const url='https://downloads.example.test/install?token='+('safe-long-link-'.repeat(25));
 rows[0].prompt='**[Open report]('+url+')**';
 rows[0].response='## Result\n\n- **Complete** review\n- `safe code`\n\n<script>window.previewInjected=true</script>';
 fixture.state.turns['%901']=rows;
 await page.goto(fixture.url+'/#code=TEST-1234');
 async function open(){
  if(width>=900){await page.locator('#tree').getByText('Investigate retries',{exact:true}).dblclick();await page.locator('.tile-modes [data-m="chat"]').click();await page.locator('.tile-btn[data-action="focus"]').click();}
  else await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
 }
 await open();
 const track=page.getByRole('slider',{name:'Conversation overview — loaded turns'}),tip=page.locator('#chat .conversation-map-preview');
 await track.focus();await track.press('Home');
 assert.equal(await tip.locator('strong a').getAttribute('href'),url,'full destination survives preview clipping');
 assert.equal(await tip.locator('.md-h').innerText(),'Result');
 assert.equal(await tip.locator('script').count(),0);
 assert.equal(await page.evaluate(()=>window.previewInjected),undefined);
 await tip.getByRole('button',{name:'Bookmark this turn',exact:true}).click();
 assert.equal(await tip.getByRole('button',{name:'Remove bookmark',exact:true}).getAttribute('aria-pressed'),'true');
 assert.equal(await track.locator('i.marked').count(),1);
 const color=await track.locator('i.marked').evaluate(e=>getComputedStyle(e).backgroundColor);
 assert.notEqual(color,await track.locator('i:not(.marked):not(.current)').first().evaluate(e=>getComputedStyle(e).backgroundColor));
 const anchor=page.locator('#chat .cturn').last();
 await page.waitForFunction(()=>document.fonts.status==='loaded');
 await anchor.evaluate(e=>e.scrollIntoView({block:'center'}));
 const before=await anchor.evaluate(e=>e.getBoundingClientRect().top);
 fixture.state.turns['%901'].unshift({session_id:'long-session',record_id:'earlier',turn_ordinal:150,prompt:'Earlier turn',response:'Earlier response'});
 await page.waitForFunction(()=>document.querySelectorAll('#chat .cturn').length===61);
 const anchorAfterRefresh=await anchor.evaluate(e=>e.getBoundingClientRect().top);
 await track.focus();await track.press('Home');await track.press('ArrowDown');
 assert.match(await tip.innerText(),/Turn 151/);
 assert.equal(await tip.getByRole('button',{name:'Remove bookmark',exact:true}).getAttribute('aria-pressed'),'true','prepending history keeps same bookmarked turn');
 assert.equal(await track.locator('i.marked').count(),1);
 assert.ok(Math.abs(await page.locator('#chat .cturn').last().evaluate(e=>e.getBoundingClientRect().top)-before)<2,'visible turn remains anchored: '+before+' => refresh '+anchorAfterRefresh+' => focus '+await anchor.evaluate(e=>e.getBoundingClientRect().top));
 const data=await page.evaluate(()=>Object.fromEntries(Object.entries(localStorage).filter(([k])=>k.startsWith('gtmux.bookmarks:'))));
 assert.equal(Object.keys(data).length,1);assert.ok(!JSON.stringify(data).includes('Open report'),'storage contains identity only');
 await page.reload();await open();
 await track.focus();await track.press('Home');await track.press('ArrowDown');
 assert.equal(await tip.getByRole('button',{name:'Remove bookmark',exact:true}).getAttribute('aria-pressed'),'true','bookmark survives reload');
 const bounds=await tip.boundingBox();assert.ok(bounds.x>=0&&bounds.x+bounds.width<=width,'preview fits viewport');
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth),width);
 if(process.env.GTMUX_WEB_EVIDENCE){await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE,{recursive:true});await page.screenshot({path:path.join(process.env.GTMUX_WEB_EVIDENCE,`conversation-bookmark-${width}.png`)});}
 await tip.getByRole('button',{name:'Remove bookmark',exact:true}).click();assert.equal(await track.locator('i.marked').count(),0);
 assert.equal(fixture.state.sends.length,0);
});

scenario('Overview bookmarks retain unsampled turns, isolate homes and handle cleared storage',{viewport:{width:900,height:900}},async(page,fixture)=>{
 await page.goto(fixture.url+'/#code=TEST-1234');
 await page.locator('#radar').waitFor({state:'attached'});
 await page.evaluate(()=>{
  const main=document.createElement('div');main.id='map-fixture';main.style.cssText='position:fixed;inset:50px;width:700px;height:600px;background:white;z-index:100';
  const scroll=document.createElement('div');scroll.style.cssText='height:600px;overflow:auto';main.appendChild(scroll);document.body.appendChild(main);
  const rows=Array.from({length:200},(_,i)=>({record_id:'r'+i,session_id:'same-session',turn_ordinal:150+i,prompt:'Prompt '+i,response:'Reply'}));
  rows.forEach(()=>{const article=document.createElement('div');article.className='cturn';article.style.height='100px';scroll.appendChild(article);});
  let dispose;window.mountMapFixture=home=>{if(dispose)dispose();dispose=window.GtmuxConversationMap.mount(main,scroll,rows,en=>en,null,{storageKey:'gtmux.bookmarks:synthetic',bookmarkIcon:'<svg aria-hidden="true"></svg>',context:{home_id:home}});};
  window.mountMapFixture('home-a');
 });
 const map=page.locator('#map-fixture'),track=map.getByRole('slider');
 await track.focus();await track.press('End');await track.press('ArrowUp');
 await map.getByRole('button',{name:'Bookmark this turn',exact:true}).click();
 assert.equal(await track.locator('i.marked[data-extra="true"]').count(),1,'a bookmarked unsampled turn gets its own visible tick');
 assert.equal(await track.locator('i').count(),121);
 await page.evaluate(()=>window.mountMapFixture('home-b'));
 assert.equal(await track.locator('i.marked').count(),0,'same session and record in another home is isolated');
 await page.evaluate(()=>window.mountMapFixture('home-a'));
 await track.locator('i.marked').waitFor();
 assert.equal(await track.locator('i.marked').count(),1);
 await page.evaluate(()=>{localStorage.removeItem('gtmux.bookmarks:synthetic');window.dispatchEvent(new StorageEvent('storage',{key:'gtmux.bookmarks:synthetic',newValue:null}));});
 assert.equal(await track.locator('i.marked').count(),0,'cross-tab storage removal clears the mark');
 await track.focus();await track.press('Home');
 await map.getByRole('button',{name:'Bookmark this turn',exact:true}).focus();
 await page.keyboard.press('Escape');assert.equal(await map.locator('.conversation-map-preview').isVisible(),false);
 assert.equal(fixture.state.sends.length,0);
});

scenario('Legacy clockless repeated prompts keep separate call disclosures',{viewport:{width:390,height:900}},async(page,fixture)=>{
 fixture.state.turns['%901']=[1,2].map(n=>({prompt:'Same legacy prompt',segments:[{steps:[{title:'exec_command',operation:'command',input_kind:'command',input:'echo legacy-'+n}]}]}));
 await page.goto(fixture.url+'/#code=TEST-1234');
 await page.locator('#radar').getByText('Investigate retries',{exact:true}).click();
 await page.locator('#mode [data-mode="chat"]').click();
 await page.locator('#chat .steps-toggle').first().click();
 assert.equal(await page.locator('#chat .steps-toggle').first().getAttribute('aria-expanded'),'true');
 assert.equal(await page.locator('#chat .steps-toggle').last().getAttribute('aria-expanded'),'false');
 assert.equal(fixture.state.sends.length,0);
});
