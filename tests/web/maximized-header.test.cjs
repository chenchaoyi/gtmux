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
 const geometry=await head.evaluate(e=>{const box=s=>{const r=e.querySelector(s).getBoundingClientRect();return {x:r.x,right:r.right,width:r.width,height:r.height};};return {button:box('.max-exit'),avatar:box('.speaker-avatar, .avatar'),identity:box('.tile-identity'),title:box('.tile-name'),controls:box('.tile-window-controls')};});
 assert.ok(geometry.avatar.right<=geometry.identity.x,'actual avatar and title wrapper overlap');
 assert.ok(geometry.identity.width>=180,'actual identity wrapper lost its flexible space');
 assert.ok(geometry.button.right<=geometry.avatar.x,'return control overlaps avatar');
 assert.ok(geometry.identity.right<=geometry.controls.x,'title wrapper overlaps window controls');
 assert.equal(geometry.title.width,geometry.identity.width);
 assert.ok(await head.locator('.tile-meta').isVisible());
 assert.equal(await head.getByRole('button',{name:'Restore layout'}).count(),1);
 await head.getByRole('button',{name:'Restore layout'}).click();
 assert.equal(await page.locator('.tile.max').count(),0);
 assert.deepEqual(await pane.evaluate(e=>({width:e.style.width,height:e.style.height})),original);
 assert.equal(fixture.state.sends.length,0);
});
