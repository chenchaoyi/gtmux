const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

const desktop = {viewport: {width: 1440, height: 950}, locale: 'en-US'};
const initial = {x: 128, y: 110, w: 428, h: 344};
const cursorFor = {n: 'ns-resize', s: 'ns-resize', e: 'ew-resize', w: 'ew-resize',
  ne: 'nesw-resize', sw: 'nesw-resize', nw: 'nwse-resize', se: 'nwse-resize'};

async function openTile(page, fixture) {
  fixture.state.turns['%901'] = [{prompt: 'Synthetic question', response: 'Synthetic response'}];
  await page.addInitScript(rect => {
    if (!localStorage.getItem('gtmux.board')) localStorage.setItem('gtmux.board', JSON.stringify({
      tiles: [{id: '%901', ...rect, mode: 'chat'}],
    }));
  }, initial);
  await page.goto(fixture.url + '/#code=TEST-1234');
  const tile = page.locator('.tile');
  await tile.getByText('Synthetic response', {exact: true}).waitFor();
  return tile;
}

function geometry(tile) {
  return tile.evaluate(el => ({x: el.offsetLeft, y: el.offsetTop, w: el.offsetWidth, h: el.offsetHeight}));
}

async function handlePoint(tile, edge) {
  const handle = tile.locator('.tile-resize-handle[data-edge="' + edge + '"]');
  assert.equal(await handle.evaluate(el => getComputedStyle(el).cursor), cursorFor[edge]);
  const box = await handle.boundingBox();
  return {x: Math.round(box.x + box.width / 2), y: Math.round(box.y + box.height / 2)};
}

async function drag(page, tile, edge, dx, dy) {
  const from = await handlePoint(tile, edge);
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.mouse.move(from.x + dx, from.y + dy, {steps: 5});
  await page.mouse.up();
}

scenario('WLI-003 The workbench panel resizes on eight edges and corners', desktop, async (page, fixture) => {
  const tile = await openTile(page, fixture);
  assert.deepEqual(await geometry(tile), initial);
  const input = tile.locator('.cx-input');
  await input.fill('Keep this draft');
  for (const edge of Object.keys(cursorFor)) {
    const before = await geometry(tile);
    const dx = edge.includes('w') ? -16 : 16;
    const dy = edge.includes('n') ? -14 : 14;
    await drag(page, tile, edge, dx, dy);
    const after = await geometry(tile);
    assert.deepEqual(after, {
      x: edge.includes('w') ? before.x - 16 : before.x,
      y: edge.includes('n') ? before.y - 14 : before.y,
      w: /[ew]/.test(edge) ? before.w + 16 : before.w,
      h: /[ns]/.test(edge) ? before.h + 14 : before.h,
    }, edge + ' should move only the dragged edges');
    assert.equal(await tile.evaluate(el => el.classList.contains('max')), false);
  }

  const beforeBoundary = await geometry(tile);
  await drag(page, tile, 'nw', -beforeBoundary.x - 16, -beforeBoundary.y - 16);
  const reached = await geometry(tile);
  assert.deepEqual(reached, {x: 0, y: 0, w: beforeBoundary.x + beforeBoundary.w, h: beforeBoundary.y + beforeBoundary.h});

  await drag(page, tile, 'se', 1200, 1000);
  const board = await page.locator('#board').evaluate(el => ({w: el.clientWidth, h: el.clientHeight}));
  assert.deepEqual(await geometry(tile), {x: 0, y: 0, ...board});
  await drag(page, tile, 'se', -board.w + 130, -board.h + 85);
  const small = {x: 0, y: 0, w: 180, h: 120};
  assert.deepEqual(await geometry(tile), small);
  await tile.locator('.tile-name').click();
  await tile.locator('.max-exit').waitFor();
  assert.ok(await tile.locator('.tile-resize').isHidden());
  assert.equal(await tile.locator('.tile-resize-handle:visible').count(), 0);
  await tile.getByRole('button', {name: 'Restore layout'}).click();
  assert.deepEqual(await geometry(tile), small);
  assert.equal(await input.inputValue(), 'Keep this draft');
  await page.reload();
  await tile.waitFor();
  assert.deepEqual(await geometry(tile), small);
  assert.equal(await input.inputValue(), 'Keep this draft');
  await page.locator('#wb-snap').click();
  await drag(page, tile, 'e', 33, 17);
  assert.deepEqual(await geometry(tile), {x: 0, y: 0, w: 220, h: 120});
  assert.equal(fixture.state.sends.length, 0);
});

scenario('WLI-004 A touch drags the top-left corner without moving the header', {...desktop, hasTouch: true}, async (page, fixture) => {
  const tile = await openTile(page, fixture), from = await handlePoint(tile, 'nw');
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Input.dispatchTouchEvent', {type: 'touchStart', touchPoints: [from]});
  await cdp.send('Input.dispatchTouchEvent', {type: 'touchMove', touchPoints: [{x: from.x - 36, y: from.y - 32}]});
  await cdp.send('Input.dispatchTouchEvent', {type: 'touchEnd', touchPoints: []});
  assert.deepEqual(await geometry(tile), {x: initial.x - 36, y: initial.y - 32, w: initial.w + 36, h: initial.h + 32});
  assert.equal(await tile.evaluate(el => el.classList.contains('max')), false);
  await tile.locator('.cx-keyboard').tap();
  const input = tile.locator('.cx-input');
  await input.tap();
  await input.fill('A draft after touch resize');
  assert.equal(await input.inputValue(), 'A draft after touch resize');
  assert.equal(fixture.state.sends.length, 0);
});
