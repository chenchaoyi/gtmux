// These tests exercise actual DOM/CSS/xterm with a fixture API. They do not
// replace Go HTTP contract tests, Safari or real-device accessibility acceptance.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {chromium} = require('playwright');
const {start} = require('./fixture.cjs');

function scenario(name, options, run) {
  test(name, {timeout: 45000}, async () => {
    const fixture = await start();
    let browser, context, page;
    const errors = [];
    try {
      browser = await chromium.launch({headless: true, executablePath: process.env.GTMUX_TEST_CHROMIUM || undefined});
      context = await browser.newContext(options);
      context.setDefaultTimeout(6000);
      await context.tracing.start({screenshots: true, snapshots: true});
      page = await context.newPage();
      page.on('pageerror', error => errors.push(error.message));
      await run(page, fixture);
      assert.equal(await page.title(), 'gtmux');
      assert.equal(errors.length, 0, errors.join('\n'));
    } catch (error) {
      const output = path.join(__dirname, 'results', name.replace(/[^a-z0-9]+/gi, '-'));
      await fs.mkdir(output, {recursive: true});
      if (page) await page.screenshot({path: path.join(output, 'failure.png'), fullPage: true}).catch(() => {});
      if (context) await context.tracing.stop({path: path.join(output, 'trace.zip')}).catch(() => {});
      await fs.writeFile(path.join(output, 'failure.txt'), String(error.stack) + '\nPage errors:\n' + errors.join('\n'));
      throw error;
    } finally {
      if (browser) await browser.close();
      await fixture.close();
    }
  });
}

scenario('Desktop enrollment, sidebar, restore and live chat', {viewport: {width: 1280, height: 850}, locale: 'en-US'}, async (page, fixture) => {
  await page.goto(fixture.url); // Direct path without trailing slash.
  assert.equal(await page.locator('#gate').isVisible(), true);
  await page.locator('#gate-code-input').fill('WRONG-1234');
  await page.locator('#gate-code-go').click();
  await page.locator('#gate-code-why').waitFor();
  assert.equal(await page.locator('#gate-code-go').isEnabled(), true);
  await page.locator('#gate-code-input').fill('TEST-1234');
  await page.locator('#gate-code-go').click();
  await page.locator('.tree-row').first().waitFor();
  const native = page.locator('.tree-row.native');
  assert.match(await native.innerText(), /Status only/);
  await native.dblclick();
  assert.equal(await page.locator('.tile').count(), 0);
  await page.locator('#rail-collapse').click();
  await page.locator('#rail-tab').click();
  await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
  await page.locator('.tile .cx-input').waitFor();
  await page.locator('.tile-modes [data-m="chat"]').click();
  await page.getByText('Initial finding', {exact: true}).waitFor();
  const updated = fixture.turns();
  updated[0].segments[0].text = 'Updated live segment';
  fixture.state.turns['%901'] = updated;
  await page.getByText('Updated live segment', {exact: true}).waitFor();
  await page.locator('.tile-modes [data-m="diff"]').click();
  await page.locator('.tile-diff .add').waitFor();
  assert.equal(await page.locator('.tile-diff .add').innerText(), '+new\n');
  await page.locator('.tile-modes [data-m="term"]').click();
  await page.locator('.tile .xterm-rows').waitFor();
  await page.locator('.tile-name').click();
  for (let i = 0; i < 60; i++) await page.locator('.tile-body').click({position: {x: 100, y: 60}});
  await page.locator('#max-exit').click();
  assert.equal(await page.locator('#board').evaluate(e => e.classList.contains('maximized')), false);
});

scenario('Narrow Chinese read-only share and shell layout', {viewport: {width: 390, height: 844}, locale: 'zh-CN'}, async (page, fixture) => {
  fixture.state.share = {all: false, input: false, panes: [], view_panes: ['%901', '%902', '%903']};
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('.row:not(.native)').first().waitFor();
  assert.equal(new URL(page.url()).hash, '');
  await page.locator('#radar').getByText('Review notes', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.locator('.appr-card').waitFor();
  assert.match(await page.locator('.appr-hint').innerText(), /只读/);
  await page.locator('#mode [data-mode="term"]').click();
  assert.equal(await page.locator('#pane-input').isVisible(), false);
  await page.locator('#back').click();
  await page.locator('#panes-btn').click();
  await page.locator('#panes-search').fill('%904');
  await page.locator('.pb-row').click();
  for (const width of [320, 390, 799]) {
    await page.setViewportSize({width, height: 844});
    await page.waitForFunction(w => document.documentElement.scrollWidth === w, width);
    assert.equal(await page.locator('#mode').isVisible(), false);
  }
  assert.equal(fixture.state.sends.length, 0);
});

scenario('Idle rows never date an unknown completion from window activity', {viewport: {width: 390, height: 844}, locale: 'en-US'}, async (page, fixture) => {
  const now = Math.floor(Date.now() / 1000);
  await page.route('**/api/agents', route => route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify([
    {pane_id: '%901', agent: 'Codex', task: 'Unknown completion', status: 'idle', since: 0, activity_at: now, source: 'tmux'},
    {pane_id: '%902', agent: 'Codex', task: 'Known completion', status: 'idle', since: now - 86400, activity_at: now, source: 'tmux'},
    {pane_id: '%903', agent: 'Codex', task: 'Working fallback', status: 'working', activity_at: now, source: 'tmux'},
  ])}));
  await page.goto(fixture.url + '/#code=TEST-1234');
  const unknown = page.locator('#radar .row').filter({hasText: 'Unknown completion'});
  await unknown.waitFor();
  assert.equal(await unknown.locator('.time').count(), 0);
  assert.equal(await page.locator('#radar .row').filter({hasText: 'Known completion'}).locator('.time').innerText(), '1d');
  assert.match(await page.locator('#radar .row').filter({hasText: 'Working fallback'}).locator('.time').innerText(), /^\d+s$/);
});

scenario('Revocation clears private content and permits re-enrollment', {viewport: {width: 1280, height: 850}, locale: 'en-US'}, async (page, fixture) => {
  await page.goto(fixture.url + '/#code=TEST-1234');
  for (let i = 0; i < 2; i++) {
    await page.locator('.tree-row').first().waitFor();
    await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
    await page.locator('.tile').waitFor();
    fixture.state.deny = true;
    await page.locator('#gate').waitFor();
    assert.equal(await page.locator('.tile').count(), 0);
    assert.equal(await page.locator('#identity').isVisible(), false);
    assert.equal(await page.locator('#gate-code-go').isEnabled(), true);
    fixture.state.deny = false;
    await page.locator('#gate-code-input').fill('TEST-1234');
    await page.locator('#gate-code-go').click();
    await page.locator('#workbench').waitFor();
  }
});

scenario('Slow chat reads stay single and render when they finish', {viewport: {width: 390, height: 844}, locale: 'en-US'}, async (page, fixture) => {
  let release;
  const held = new Promise(resolve => {release = resolve;});
  let calls = 0;
  await page.route('**/api/transcript?*', async route => {
    calls++;
    await held;
    await route.fulfill({status: 200, contentType: 'application/json', body: JSON.stringify([{prompt: 'Synthetic slow read', response: 'Delayed current reply'}])});
  });
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.waitForTimeout(3500);
  assert.equal(calls, 1, 'a pending transcript must not start another request');
  release();
  await page.getByText('Delayed current reply', {exact: true}).waitFor();
  assert.equal(await page.locator('#chat').isVisible(), true);
  if (process.env.GTMUX_WEB_EVIDENCE) {
    await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
    await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, 'web-slow-chat.png')});
  }
});
