const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const [name, readOnly] of [
  ['WHS-007 diagonal minimum writable', false],
  ['WHS-007 diagonal minimum read-only touch', true],
]) scenario(name, {viewport:{width:1440,height:900},locale:'en-US',hasTouch:readOnly}, async (page, fixture) => {
  const agent = fixture.state.agents[0];
  agent.pane_id = '%901234';
  agent.task = 'A long title for a conversation reduced to its minimum size';
  fixture.state.share.input = !readOnly;
  await page.goto(fixture.url+'/#code=TEST-1234');
  await page.locator('[data-tree-pane="%901234"]').dblclick();
  const tile = page.locator('.tile'), head = tile.locator('.tile-head');
  await dragTile(page, tile, 160, 104);
  const box = await tile.boundingBox();
  assert.equal(box.width, 180);
  assert.equal(box.height, 120);
  const opener = head.getByRole('button', {name:'Pane actions',exact:true});
  if (readOnly) await head.getByText('Read-only',{exact:true}).waitFor();
  await opener.waitFor();
  const geometry = await tile.evaluate(el => {
    const id = el.querySelector('.tile-id'), range = document.createRange();
    range.selectNodeContents(id);
    const bounds = el.getBoundingClientRect();
    const fits = box => box.left >= bounds.left && box.right <= bounds.right && box.top >= bounds.top && box.bottom <= bounds.bottom;
    const opener = el.querySelector('.tile-actions-menu'), target = opener.getBoundingClientRect();
    const badge = el.querySelector('.tile-identity .cap-ro:not([hidden])');
    const at = document.elementFromPoint(target.x + target.width / 2, target.y + target.height / 2);
    return {
      id: id.textContent, idFits: fits(range.getBoundingClientRect()),
      capFits: badge && fits(badge.getBoundingClientRect()),
      hit: opener.contains(at), targetFits: fits(target),
      targetWidth:target.width, targetHeight:target.height,
    };
  });
  assert.equal(geometry.id,'%901234');
  assert.ok(geometry.idFits && geometry.hit && geometry.targetFits);
  assert.equal(Boolean(geometry.capFits), readOnly);
  const minimum = readOnly ? 44 : 36;
  assert.ok(geometry.targetWidth >= minimum && geometry.targetHeight >= minimum);
  assert.equal(await head.getByRole('button', {name:'Trace', exact:true}).count(),0);
  await opener.click();
  const dialog = page.getByRole('dialog', {name:'Pane actions',exact:true});
  await page.keyboard.press('Escape');
  await dialog.waitFor({state:'detached'});
  assert.ok(await opener.evaluate(e=>e===document.activeElement),'Escape restores focus to the visible opener');

  const choose = async label => {
    await opener.click();
    const control = dialog.getByRole('button',{name:label,exact:true});
    const {height} = await control.boundingBox();
    assert.ok(height >= minimum);
    await control.click();
    await dialog.waitFor({state:'detached'});
  };
  await choose('Terminal');
  await tile.locator('.xt').waitFor({state:'attached'});
  await choose('Chat');
  await tile.locator('.tile-chat').waitFor({state:'attached'});
  for (const label of ['Trace','Diff']) {
    await choose(label);
    const tool=page.locator('dialog.session-tool');
    await tool.waitFor();
    await tool.locator('.dialog-close').click();
    await tool.waitFor({state:'detached'});
  }
  await choose('Open conversation');
  await page.locator('#chat').waitFor({state:'visible'});
  await page.locator('#back').click();
  await opener.waitFor();

  if (readOnly) {
    await choose('Close panel');
    await tile.waitFor({state:'detached'});
  } else {
    await dragTile(page,tile,180,480);
    await opener.waitFor({state:'hidden'});
    await head.getByRole('button',{name:'Trace',exact:true}).waitFor();
    assert.equal((await tile.boundingBox()).width,180);
  }
  assert.equal(fixture.state.sends.length,0);
});

async function dragTile(page,tile,width,height) {
  const grip=await tile.locator('.tile-resize').boundingBox(), size=await tile.boundingBox();
  const x=grip.x+grip.width/2, y=grip.y+grip.height/2;
  await page.mouse.move(x,y);
  await page.mouse.down();
  await page.mouse.move(x-size.width+width,y-size.height+height,{steps:8});
  await page.mouse.up();
}
