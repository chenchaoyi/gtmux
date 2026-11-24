// The app renders a real, read-only capture against a synthetic loopback API.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');

async function openTile(page, fixture) {
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  const tile = page.locator('.tile');
  await tile.locator('.tile-modes [data-m="chat"]').click();
  await tile.locator('.tile-name').click();
  await tile.waitFor();
  return tile;
}

async function readBounds(locator) {
  return locator.evaluate(e => {
    const r = e.getBoundingClientRect();
    return {top: r.top, bottom: r.bottom, height: r.height};
  });
}

async function gripPoint(grip) {
  const box = await grip.boundingBox();
  return {x: Math.floor(box.x + box.width / 2), y: Math.floor(box.y + box.height / 2)};
}

function assertNear(actual, expected, label) {
  assert.ok(Math.abs(actual - expected) <= 1, label + ': ' + actual + ' / ' + expected);
}

async function screenshot(page, name) {
  const dir = process.env.GTMUX_WEB_EVIDENCE;
  if (!dir) return;
  await fs.mkdir(dir, {recursive: true});
  await page.screenshot({path: path.join(dir, name), animations: 'disabled'});
}

scenario('WLV-001 Desktop Live drag overlays the chat and survives transcript refresh', {viewport: {width: 1440, height: 950}, locale: 'en-US', colorScheme: 'light'}, async (page, fixture) => {
  fixture.state.turns['%901'] = Array.from({length: 14}, (_, i) => ({
    time: new Date(1750000000000 + i * 60000).toISOString(),
    prompt: 'Investigate case ' + (i + 1), response: 'Synthetic report for case ' + (i + 1),
  }));
  fixture.state.screens['%901'] = Array.from({length: 120}, (_, i) => 'Service check ' + (i + 1) + '  ✓').join('\n');
  const tile = await openTile(page, fixture);
  const chat = tile.locator('.tile-chat'), input = tile.locator('.cx-input'), panel = tile.locator('.chat-live');
  await chat.getByText('Investigate case 14', {exact: true}).waitFor();
  await input.fill('Keep the draft across resize');
  await chat.evaluate(e => { e.scrollTop = 100; });
  const chatBefore = await readBounds(chat), inputBefore = await readBounds(input);
  const originalScroll = await chat.evaluate(e => e.scrollTop);
  await panel.locator('summary').click();
  await panel.locator('.live-output').filter({hasText: 'Service check 120'}).waitFor();
  const before = await readBounds(panel);
  const grip = panel.getByRole('separator', {name: 'Live terminal height'});
  const point = await gripPoint(grip);
  await page.mouse.move(point.x, point.y);
  await page.mouse.down();
  await page.mouse.move(point.x, point.y - 85, {steps: 5});
  const halfway = await readBounds(panel);
  assert.ok(halfway.top < before.top - 60, 'top edge follows the pointer upward');

  fixture.state.turns['%901'].push({
    time: new Date(1750001200000).toISOString(), prompt: 'New transcript during drag', response: 'Synthetic tail',
  });
  await chat.getByText('New transcript during drag', {exact: true}).waitFor();
  await page.mouse.move(point.x, point.y - 155, {steps: 5});
  await page.mouse.up();
  const expanded = await readBounds(panel);
  assert.ok(expanded.top < halfway.top - 45, 'transcript updates keep the captured pointer active');
  assertNear(expanded.bottom, before.bottom, 'panel bottom stays anchored');
  assert.deepEqual(await readBounds(chat), chatBefore);
  assert.deepEqual(await readBounds(input), inputBefore);
  assert.equal(await chat.evaluate(e => e.scrollTop), originalScroll);
  assert.equal(await input.inputValue(), 'Keep the draft across resize');
  await screenshot(page, 'web-live-workbench-expanded.png');

  const shrink = await gripPoint(grip);
  await page.mouse.move(shrink.x, shrink.y);
  await page.mouse.down();
  await page.mouse.move(shrink.x, shrink.y + 82, {steps: 5});
  await page.mouse.up();
  const smaller = await readBounds(panel);
  assert.ok(smaller.height < expanded.height - 65, 'downward drag shrinks the panel');
  await panel.locator('summary').click();
  await panel.locator('summary').click();
  await panel.locator('.live-output').waitFor();
  assertNear((await readBounds(panel)).height, smaller.height, 'reopening restores the selected height');
  assert.equal(fixture.state.sends.length, 0);
});

scenario('WLV-002 Web Live freezes the readable capture until returning to live', {viewport: {width: 1320, height: 900}, locale: 'en-US', colorScheme: 'dark'}, async (page, fixture) => {
  const rows = Array.from({length: 2050}, (_, i) => 'Terminal row ' + i);
  rows[1800] += '    ' + 'wide output '.repeat(45);
  fixture.state.screens['%901'] = rows.join('\n') + '\n\n';
  const tile = await openTile(page, fixture);
  const panel = tile.locator('.chat-live'), output = panel.locator('.live-output');
  await panel.locator('summary').click();
  await output.filter({hasText: 'Terminal row 2049'}).waitFor();
  await page.waitForFunction(() => {
    const e = document.querySelector('.live-output');
    return e && e.scrollHeight - e.scrollTop - e.clientHeight < 3;
  });
  const first = (await output.innerText()).split('\n');
  assert.equal(first.length, 2000);
  assert.equal(first[0], 'Terminal row 50');
  assert.ok(await output.evaluate(e => e.scrollWidth > e.clientWidth), 'wide terminal rows remain horizontally scrollable');
  await output.hover();
  await page.mouse.wheel(0, -670);
  await panel.getByRole('button', {name: 'Return to live'}).waitFor();
  const readingTop = await output.evaluate(e => e.scrollTop);
  rows.push('New output · ready to inspect');
  fixture.state.screens['%901'] = rows.join('\n');
  await panel.getByText('New output', {exact: true}).waitFor();
  assert.equal(await output.evaluate(e => e.scrollTop), readingTop);
  assert.doesNotMatch(await output.innerText(), /ready to inspect/);
  await panel.getByRole('button', {name: 'Return to live'}).click();
  await output.filter({hasText: 'ready to inspect'}).waitFor();
  assert.ok(await output.evaluate(e => e.scrollHeight - e.scrollTop - e.clientHeight < 3));
  assert.equal(fixture.state.sends.length, 0);
});

scenario('WLV-003 Narrow focused Live supports touch, keyboard and a shorter viewport', {
  viewport: {width: 430, height: 900}, locale: 'zh-CN', colorScheme: 'dark', hasTouch: true, isMobile: true,
}, async (page, fixture) => {
  fixture.state.screens['%901'] = Array.from({length: 96}, (_, i) => '同步检查 ' + (i + 1)).join('\n');
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries', {exact: true}).tap();
  await page.locator('#mode [data-mode="chat"]').tap();
  const panel = page.locator('#chat .chat-live'), input = page.locator('#chat-input .cx-input');
  const summary = panel.locator('summary'), grip = panel.getByRole('separator', {name: '实时终端高度'});
  await page.locator('#chat-input .cx-keyboard').tap();
  await input.fill('保留这条手写草稿');
  const chatBefore = await readBounds(page.locator('#chat .conv-scroll'));
  await summary.tap();
  await panel.locator('.live-output').filter({hasText: '同步检查 96'}).waitFor();
  assert.ok((await grip.boundingBox()).height >= 44);
  const before = await readBounds(panel);
  const point = await gripPoint(grip);
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Input.dispatchTouchEvent', {type: 'touchStart', touchPoints: [point]});
  for (const distance of [40, 80, 120]) {
    await cdp.send('Input.dispatchTouchEvent', {type: 'touchMove', touchPoints: [{x: point.x, y: point.y - distance}]});
  }
  await cdp.send('Input.dispatchTouchEvent', {type: 'touchEnd', touchPoints: []});
  await cdp.detach();
  const expanded = await readBounds(panel);
  assert.ok(expanded.height > before.height + 88);
  assertNear(expanded.bottom, before.bottom, 'touch drag keeps the bottom anchored');
  assert.deepEqual(await readBounds(page.locator('#chat .conv-scroll')), chatBefore);
  await screenshot(page, 'web-live-narrow-expanded.png');

  await grip.press('Home');
  const minimum = (await readBounds(panel)).height;
  await grip.press('ArrowUp');
  assert.ok((await readBounds(panel)).height > minimum + 50);
  await grip.press('End');
  const chosen = (await readBounds(panel)).height;
  await page.setViewportSize({width: 430, height: 390});
  await page.waitForFunction(height => {
    const panel = document.querySelector('#chat .chat-live');
    return panel.getBoundingClientRect().height < height - 120;
  }, chosen, {timeout: 5000});
  assertNear((await readBounds(panel)).bottom, (await readBounds(page.locator('#chat'))).bottom, 'short viewport stays inside chat');
  await input.waitFor();
  await page.locator('#back').waitFor();
  await page.setViewportSize({width: 430, height: 900});
  await page.waitForFunction(height => {
    const panel = document.querySelector('#chat .chat-live');
    return Math.abs(panel.getBoundingClientRect().height - height) <= 2;
  }, chosen, {timeout: 5000});
  assertNear((await readBounds(panel)).height, chosen, 'restoring the viewport restores the chosen height');
  await grip.press('Escape');
  assert.equal(await panel.evaluate(e => e.open), false);
  assert.ok(await summary.evaluate(e => e === document.activeElement));
  assert.equal(await input.inputValue(), '保留这条手写草稿');
  assert.equal(fixture.state.sends.length, 0);
});
