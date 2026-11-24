const assert=require('node:assert/strict');
const {scenario}=require('./harness.cjs');
for(const [name,width,boardWidth,terminalOnly] of [
 ['AOP006 real maximized header wide',1440,0],
 ['AOP006 real maximized header medium',960,0],
 ['AOP006 real maximized header narrow board',1440,480],
 ['AOP006 real maximized header plain terminal',1440,0,true],
]) scenario(name,{viewport:{width,height:900},locale:'en-US'},async(page,fixture)=>{
 if(terminalOnly)fixture.state.agents[0].agent='';
 fixture.state.agents[0].task='Long header example · '+('Readable identity and separate actions · '.repeat(8));
 await page.goto(fixture.url+'/#code=TEST-1234');
 // The plain terminal branch has no agent status avatar/accessibility prefix.
 // Open the same exact fixture pane through the real sidebar in both branches.
 await page.locator('[data-tree-pane="%901"]').dblclick();
 const pane=page.locator('.tile'),head=pane.locator('.tile-head');
 const original=await pane.evaluate(e=>({width:e.style.width,height:e.style.height}));
 if(boardWidth)await page.locator('#board').evaluate((e,w)=>{e.style.maxWidth=w+'px';},boardWidth);
 await head.locator('.tile-name').click();
 const geometry=await headerGeometry(head);
 assert.ok(geometry.avatar.right<=geometry.identity.x,'actual avatar and title wrapper overlap');
 assert.ok(geometry.identity.width>=180,'actual identity wrapper lost its flexible space');
 assert.ok(geometry.button.right<=geometry.avatar.x,'return control overlaps avatar');
 assert.ok(geometry.identity.right<=geometry.controls.x,'title wrapper overlaps window controls');
 assert.equal(geometry.idText,'%901');
 assertIdentity(geometry);
 assert.equal(await head.getByRole('button',{name:'Restore layout'}).count(),1);
 await head.getByRole('button',{name:'Restore layout'}).click();
 assert.equal(await page.locator('.tile.max').count(),0);
 assert.deepEqual(await pane.evaluate(e=>({width:e.style.width,height:e.style.height})),original);
 assert.equal(fixture.state.sends.length,0);
});

for(const [name,paneID,readOnly,hasTouch] of [
 ['AOP006 drag minimum desktop writable','%14',false,false],
 ['AOP006 drag minimum desktop read-only','%901',true,false],
 ['AOP006 drag minimum coarse pointer read-only','%901234',true,true],
]) scenario(name,{viewport:{width:1440,height:900},locale:'en-US',hasTouch},async(page,fixture)=>{
 const agent=fixture.state.agents[0];
 agent.pane_id=paneID;
 agent.task='Long conversation title · '+('Read the complete identity · '.repeat(8));
 fixture.state.share.input=!readOnly;
 await page.goto(fixture.url+'/#code=TEST-1234');
 await page.locator('[data-tree-pane="'+paneID+'"]').dblclick();
 const pane=page.locator('.tile'),head=pane.locator('.tile-head');
 const grip=await pane.locator('.tile-resize').boundingBox();
 const wide=await pane.boundingBox();
 const x=grip.x+grip.width/2,y=grip.y+grip.height/2;
 await page.mouse.move(x,y);
 await page.mouse.down();
 await page.mouse.move(x-wide.width+160,y,{steps:8});
 await page.mouse.up();
 assert.equal((await pane.boundingBox()).width,180,'uses the real resize floor');
 if(readOnly) await head.getByText('Read-only',{exact:true}).waitFor();
 const g=await headerGeometry(head);
 assert.equal(g.idText,paneID);
 assert.equal(g.coarse,hasTouch);
 assertIdentity(g);
 assert.equal(Boolean(g.cap),readOnly);
 if(g.cap) {
   assert.ok(g.cap.x>=g.head.x-.5 && g.cap.right<=g.head.right+.5,'read-only chip fits inside the header');
   assert.ok(!overlaps(g.cap,g.controls) && !overlaps(g.cap,g.id) && !overlaps(g.cap,g.title),
     'read-only chip stays clear of identity and window controls');
 }
 assert.ok(!overlaps(g.controls,g.id) && !overlaps(g.controls,g.title),'window controls leave the identity clear');
 assert.ok(g.buttons.every(b=>b.x>=g.head.x-.5 && b.right<=g.head.right+.5 && b.height>=(hasTouch?44:36)),
   'every action stays inside the header with its pointer-specific target');
 assert.equal(fixture.state.sends.length,0);
});

function overlaps(a,b) {
 return Math.min(a.right,b.right)>Math.max(a.x,b.x)+.5 &&
   Math.min(a.bottom,b.bottom)>Math.max(a.top,b.top)+.5;
}
function assertIdentity(g) {
 assert.ok(g.title.width>0,'some title stays visible');
 assert.ok(g.id.x>=g.title.right-.5 && g.id.top<g.title.bottom && g.title.top<g.id.bottom,
   'the title and pane ID share one line');
 assert.ok(g.idInk.width<=g.id.width+.5 && g.idInk.x>=g.identity.x-.5 && g.idInk.right<=g.identity.right+.5,
   'all pane ID glyphs fit in the identity column');
}
function headerGeometry(head) {
 return head.evaluate(e=>{
   const rect=r=>({x:r.x,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height});
   const box=s=>{const node=e.querySelector(s);return node?.getClientRects().length?rect(node.getBoundingClientRect()):null;};
   const id=e.querySelector('.tile-id'),range=document.createRange();
   range.selectNodeContents(id);
   return {
     head:rect(e.getBoundingClientRect()),button:box('.max-exit'),avatar:box('.speaker-avatar, .avatar'),
     identity:box('.tile-identity'),title:box('.tile-name'),controls:box('.tile-window-controls'),id:box('.tile-id'),
     idText:id.textContent,idInk:rect(range.getBoundingClientRect()),cap:box('.tile-identity .cap-ro'),
     coarse:matchMedia('(pointer: coarse)').matches,
     buttons:[...e.querySelectorAll('button')].filter(n=>n.getClientRects().length).map(n=>rect(n.getBoundingClientRect())),
   };
 });
}
