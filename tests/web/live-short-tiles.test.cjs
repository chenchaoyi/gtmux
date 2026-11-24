const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

const desktop = {viewport: {width: 1440, height: 950}, locale: 'en-US'};
const capture = Array.from({length: 80}, (_, i) => 'Current terminal line ' + (i + 1)).join('\n');

async function openTile(page, fixture) {
  fixture.state.screens['%901'] = capture;
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  const tile = page.locator('.tile');
  await tile.locator('.tile-modes [data-m="chat"]').click();
  await tile.locator('.chat-live').waitFor();
  assert.equal(await tile.evaluate(el => el.classList.contains('max')), false);
  return tile;
}

async function resizeTile(page, tile, height) {
  const corner = await tile.locator('.tile-resize').boundingBox();
  const before = await tile.boundingBox();
  await page.mouse.move(corner.x + corner.width / 2, corner.y + corner.height / 2);
  await page.mouse.down();
  await page.mouse.move(corner.x + corner.width / 2, corner.y + corner.height / 2 + height - before.height, {steps: 8});
  await page.mouse.up();
  assert.equal((await tile.boundingBox()).height, height);
}

async function assertOutput(panel, rows) {
  const output = panel.locator('.live-output');
  await output.filter({hasText: 'Current terminal line 80'}).waitFor();
  assert.equal(await output.innerText(), capture);
  const height = await output.evaluate(el => {
    const r = el.getBoundingClientRect();
    const clipping = el.closest('.tile').getBoundingClientRect();
    const visible = Math.min(r.bottom, clipping.bottom) - Math.max(r.top, clipping.top);
    const style = getComputedStyle(el);
    return (visible - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom)) / parseFloat(style.lineHeight);
  });
  assert.ok(height >= rows, 'visible terminal rows: ' + height);
}

scenario('WLC-001 Live is readable and draggable on ordinary short workbench cards', desktop, async (page, fixture) => {
  fixture.state.turns['%901'] = Array.from({length: 12}, (_, i) => ({
    time: new Date(1750000000000 + i * 60000).toISOString(), prompt: 'Question ' + i, response: 'Answer ' + i,
  }));
  const tile = await openTile(page, fixture);
  const panel = tile.locator('.chat-live'), summary = panel.locator('summary');
  const chat = tile.locator('.tile-chat'), composer = tile.locator('.composer'), input = composer.locator('.cx-input');
  await input.fill('Draft on short tile');
  await resizeTile(page, tile, 200);
  await chat.evaluate(el => { el.scrollTop = 45; });
  const chatBox = await chat.boundingBox(), inputBox = await composer.boundingBox();
  const anchor = await chat.evaluate(el => el.scrollTop);
  await summary.click();
  await assertOutput(panel, 1);

  const start = await panel.boundingBox();
  const grip = await panel.getByRole('separator', {name: 'Live terminal height'}).boundingBox();
  await page.mouse.move(grip.x + grip.width / 2, grip.y + grip.height / 2);
  await page.mouse.down();
  await page.mouse.move(grip.x + grip.width / 2, grip.y + grip.height / 2 - 65, {steps: 8});
  await page.mouse.up();
  const expanded = await panel.boundingBox(), frame = await tile.boundingBox();
  assert.ok(expanded.height > start.height + 19);
  assert.ok(expanded.y >= frame.y);
  assert.ok(Math.abs(expanded.y + expanded.height - inputBox.y) <= 1);
  await assertOutput(panel, 3);
  assert.deepEqual(await chat.boundingBox(), chatBox);
  assert.deepEqual(await composer.boundingBox(), inputBox);
  assert.equal(await chat.evaluate(el => el.scrollTop), anchor);
  await input.click();
  assert.equal(await input.inputValue(), 'Draft on short tile');
  assert.equal(await tile.evaluate(el => el.classList.contains('max')), false);

  await summary.click();
  await resizeTile(page, tile, 120);
  await summary.click();
  await assertOutput(panel, 1);
  await panel.locator('.live-output').hover();
  await page.mouse.wheel(0, -180);
  const resume = panel.getByRole('button', {name: 'Return to live'});
  await resume.waitFor();
  await assertOutput(panel, 1);
  await resume.click();
  assert.equal(await resume.isVisible(), false);
  await summary.click();
  assert.equal(await panel.evaluate(el => el.open), false);
  await tile.getByRole('button', {name: 'Pane actions', exact: true}).click();
  await page.getByRole('dialog', {name: 'Pane actions', exact: true}).waitFor();
  await page.keyboard.press('Escape');
  assert.equal(await input.inputValue(), 'Draft on short tile');
  assert.equal(fixture.state.sends.length, 0);
});

scenario('WLC-002 Short workbench Live identifies its current pane and preserves read-only history', desktop, async (page, fixture) => {
  fixture.state.agents[0].observed_conversation = {session_id: 'last-known', home_id: 'test', state: 'last-known'};
  await page.route('**/api/transcript?*', async route => {
    const response = await route.fetch();
    await route.fulfill({response, headers: {...response.headers(), 'X-Gtmux-Conversation-State': 'last-known'}});
  });
  const tile = await openTile(page, fixture);
  await tile.locator('.tile-ro').waitFor();
  await resizeTile(page, tile, 120);
  const panel = tile.locator('.chat-live');
  await panel.locator('summary').click();
  const source = panel.locator('.live-source');
  assert.equal(await source.isVisible(), true);
  assert.match(await source.innerText(), /Current pane %901.*History association unconfirmed/);
  await assertOutput(panel, 1);
  assert.equal(await tile.locator('.composer').isVisible(), false);
  await panel.locator('summary').click();
  assert.equal(await panel.evaluate(el => el.open), false);
  assert.equal(fixture.state.sends.length, 0);
});
