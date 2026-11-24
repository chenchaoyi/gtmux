const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');

const desktop = {viewport: {width: 1440, height: 950}, locale: 'en-US', colorScheme: 'dark'};
const narrow = {viewport: {width: 430, height: 875}, locale: 'en-US', colorScheme: 'light', hasTouch: true, isMobile: true};
const palette = [
  '#222222', '#c44055', '#009145', '#977500', '#3760a8', '#915186', '#207782', '#cccccc',
  '#667788', '#ed7185', '#29c865', '#ae9300', '#6588d0', '#aa6fbc', '#388fa4', '#dddddd',
];

async function openLive(page, fixture, workbench) {
  await page.route('**/api/theme', route => route.fulfill({json: {
    background: '#f4f3ef', foreground: '#302f35', palette,
  }}));
  await page.goto(fixture.url + '/#code=TEST-1234');
  let panel;
  if (workbench) {
    await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
    await page.locator('.tile .tile-modes [data-m="chat"]').click();
    panel = page.locator('.tile .chat-live');
  } else {
    await page.locator('#radar').getByText('Investigate retries', {exact: true}).tap();
    await page.locator('#mode [data-mode="chat"]').tap();
    panel = page.locator('#chat .chat-live');
  }
  await panel.locator('summary').click();
  return panel;
}

function styleOf(output, text, key = 'color') {
  return output.getByText(text, {exact: true}).evaluate((el, property) => getComputedStyle(el)[property], key);
}

for (const [label, options, workbench] of [['workbench', desktop, true], ['focused chat', narrow, false]]) {
  scenario('WLI-001 Live renders the terminal palette in ' + label, options, async (page, fixture) => {
    const lines = Array.from({length: 2050}, (_, i) => 'Stable terminal row ' + i);
    lines[0] = '\x1b[31mOutside retained rows';
    lines[50] = 'Kept red\x1b[39m';
    lines[2038] = '\x1b[30mPalette zero\x1b[0m';
    lines[2039] = '\x1b[1;31mBasic bold red\x1b[0m';
    lines[2040] = '\x1b[91mBright red\x1b[0m';
    lines[2041] = '\x1b[38;5;202mExtended palette\x1b[0m';
    lines[2042] = '\x1b[38:2::12:34:56mRGB foreground\x1b[0m';
    lines[2043] = '\x1b[97;48;2;20;40;60mRGB background\x1b[39;49m';
    lines[2044] = '\x1b[1;4mBold and underlined\x1b[22;24m';
    lines[2045] = '\x1b[7mInverted defaults\x1b[27m';
    lines[2046] = 'After resets';
    lines[2047] = '\x1b]8;;https://example.com\x07Open literally\x1b]8;;\x07';
    lines[2048] = '<img src=x onerror=window.badMarkup=true>';
    lines[2049] = 'Still plain\x1b[2K';
    fixture.state.screens['%901'] = lines.join('\n') + '\n\n';
    const panel = await openLive(page, fixture, workbench);
    const output = panel.locator('.live-output');
    await output.filter({hasText: 'Still plain'}).waitFor();
    await page.waitForFunction(() => getComputedStyle(document.documentElement).getPropertyValue('--terminal-ansi-1').trim() === '#c44055');
    assert.equal((await output.textContent()).split('\n').length, 2000);
    assert.match(await output.textContent(), /^Kept red\n/);
    assert.equal(await styleOf(output, 'Kept red'), 'rgb(196, 64, 85)');
    assert.equal(await styleOf(output, 'Palette zero'), 'rgb(34, 34, 34)');
    assert.equal(await styleOf(output, 'Basic bold red'), 'rgb(237, 113, 133)');
    assert.equal(await styleOf(output, 'Bright red'), 'rgb(237, 113, 133)');
    assert.equal(await styleOf(output, 'Extended palette'), 'rgb(255, 95, 0)');
    assert.equal(await styleOf(output, 'RGB foreground'), 'rgb(12, 34, 56)');
    assert.equal(await styleOf(output, 'RGB background', 'backgroundColor'), 'rgb(20, 40, 60)');
    assert.equal(await styleOf(output, 'Bold and underlined', 'fontWeight'), '700');
    assert.equal(await styleOf(output, 'Bold and underlined', 'textDecorationLine'), 'underline');
    assert.equal(await styleOf(output, 'Inverted defaults'), 'rgb(244, 243, 239)');
    assert.equal(await styleOf(output, 'Inverted defaults', 'backgroundColor'), 'rgb(48, 47, 53)');
    assert.equal(await output.evaluate(el => getComputedStyle(el).color), 'rgb(48, 47, 53)');
    assert.equal(await output.evaluate(el => getComputedStyle(el).backgroundColor), 'rgb(244, 243, 239)');
    assert.equal(await output.locator('a, img, script').count(), 0);
    assert.match(await output.textContent(), /Open literally\n<img src=x onerror=window\.badMarkup=true>\nStill plain$/);
    assert.doesNotMatch(await output.textContent(), /[\x07\x1b]/);
    assert.equal(await page.locator('html').getAttribute('data-theme'), options.colorScheme);
    if (process.env.GTMUX_WEB_EVIDENCE) {
      await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
      await panel.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'live-colors-' + (workbench ? 'workbench' : 'focused') + '.png')});
    }
    assert.equal(fixture.state.sends.length, 0);
  });
}

scenario('WLI-002 Frozen Live preserves colors until returning to live', desktop, async (page, fixture) => {
  const screen = color => Array.from({length: 120}, (_, i) => i === 100 ?
    '\x1b[' + color + 'mChanging state\x1b[0m' : 'Output sample ' + i).join('\n');
  fixture.state.screens['%901'] = screen(31);
  const panel = await openLive(page, fixture, true), output = panel.locator('.live-output');
  await output.filter({hasText: 'Changing state'}).waitFor();
  await page.waitForFunction(() => getComputedStyle(document.documentElement).getPropertyValue('--terminal-ansi-1').trim() === '#c44055');
  const red = await styleOf(output, 'Changing state');
  assert.equal(red, 'rgb(196, 64, 85)');
  await output.hover();
  await page.mouse.wheel(0, -540);
  const resume = panel.getByRole('button', {name: 'Return to live'});
  await resume.waitFor();
  const scroll = await output.evaluate(el => el.scrollTop);
  const before = await output.textContent();
  fixture.state.screens['%901'] = screen(32);
  await panel.getByText('New output', {exact: true}).waitFor();
  assert.equal(await output.textContent(), before);
  assert.equal(await styleOf(output, 'Changing state'), red);
  assert.equal(await output.evaluate(el => el.scrollTop), scroll);
  await resume.click();
  assert.equal(await styleOf(output, 'Changing state'), 'rgb(0, 145, 69)');
  assert.equal(await output.textContent(), before);
  assert.ok(await output.evaluate(el => el.scrollHeight - el.clientHeight - el.scrollTop < 3));
  assert.equal(fixture.state.sends.length, 0);
});
