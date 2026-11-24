const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

function frame(tile) {
  return tile.evaluate(el => ({x: el.offsetLeft, y: el.offsetTop, w: el.offsetWidth, h: el.offsetHeight}));
}

scenario('WLI-006 Resizing recovers a clipped saved workbench tile', {
  viewport: {width: 1460, height: 960}, locale: 'en-US',
}, async (page, fixture) => {
  const initial = {x: 760, y: 620, w: 390, h: 252};
  fixture.state.turns['%901'] = [{prompt: 'Synthetic question', response: 'Synthetic response'}];
  await page.addInitScript(rect => {
    if (!localStorage.getItem('gtmux.board')) localStorage.setItem('gtmux.board', JSON.stringify({
      snap: true, tiles: [{id: '%901', ...rect, mode: 'chat'}],
    }));
  }, initial);
  await page.goto(fixture.url + '/#code=TEST-1234');
  const tile = page.locator('.tile');
  await tile.getByText('Synthetic response', {exact: true}).waitFor();
  assert.deepEqual(await frame(tile), initial);
  const input = tile.locator('.cx-input');
  await input.fill('Draft on clipped tile');

  await page.setViewportSize({width: 1024, height: 745});
  await page.waitForFunction(rect => {
    const board = document.getElementById('board');
    return rect.x < board.clientWidth && rect.x + rect.w > board.clientWidth &&
      rect.y < board.clientHeight && rect.y + rect.h > board.clientHeight;
  }, initial);
  const board = await page.locator('#board').evaluate(el => ({w: el.clientWidth, h: el.clientHeight}));
  assert.ok(initial.x < board.w && initial.x + initial.w > board.w);
  assert.ok(initial.y < board.h && initial.y + initial.h > board.h);
  const handle = await tile.locator('[data-edge="nw"]').boundingBox();
  const x = Math.round(handle.x + handle.width / 2), y = Math.round(handle.y + handle.height / 2);
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + Math.min(14, board.w - initial.x - 10), y + 14, {steps: 5});
  await page.mouse.up();
  const result = await frame(tile);
  assert.ok(result.x >= 0 && result.x + result.w <= board.w, 'the whole tile fits horizontally');
  assert.ok(result.y >= 0 && result.y + result.h <= board.h, 'the whole tile fits vertically');
  assert.ok(result.w >= 180 && result.h >= 120);
  assert.equal(result.x % 20, 0);
  assert.equal(result.y % 20, 0);
  assert.equal(await input.inputValue(), 'Draft on clipped tile');
  await page.reload();
  await tile.waitFor();
  assert.deepEqual(await frame(tile), result);
  assert.equal(await input.inputValue(), 'Draft on clipped tile');
  assert.equal(fixture.state.sends.length, 0);
});
