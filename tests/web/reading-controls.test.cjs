const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');
const {openTools, closeTools} = require('./tools.cjs');
async function session(page, fixture, width) {
  await page.goto(fixture.url + '/#code=TEST-1234');
  if (width >= 900) {
    await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
    await page.locator('.tile [data-action="focus"]').click();
  } else await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
}
async function shot(page, name) {
  if (!process.env.GTMUX_WEB_EVIDENCE) return;
  await fs.mkdir(process.env.GTMUX_WEB_EVIDENCE, {recursive: true});
  await page.screenshot({path: path.join(process.env.GTMUX_WEB_EVIDENCE, name + '.png')});
}
for (const width of [320, 390, 1440]) {
  scenario(`Diff preserves recorded layout, syntax and original copy at ${width}`, {viewport: {width, height: 900}, locale: 'en-US', hasTouch: width < 700, permissions: ['clipboard-read', 'clipboard-write']}, async(page, fixture) => {
    const raw = 'diff --git a/a.js b/a.js\n--- a/a.js\n+++ b/a.js\n@@ -9 +9 @@\n-const old=1;\n+const next={value:1};';
    fixture.state.diff = raw;
    await session(page, fixture, width); await openTools(page, 'diff');
    const diff = page.locator('.session-diff'), code = diff.locator('.diff-line.add .diff-code');
    await diff.locator('.diff-file > summary').click();
    await diff.locator('.diff-hunk > summary').click();
    assert.equal(await code.innerText(), 'const next={value:1};');
    assert.equal(await diff.locator('.diff-line.add .diff-line-number').count(), 1);
    assert.equal(await diff.locator('.diff-line.add .diff-line-number').innerText(), '9');
    const colors = await code.locator('span').evaluateAll(nodes => [...new Set(nodes.map(n => n.style.color))]);
    assert.ok(colors.length > 1);
    await diff.locator('.diff-wrap').click();
    assert.equal(await code.innerText(), 'const next={value:1};');
    assert.equal(await diff.locator('.diff-format').count(), 0);
    assert.equal(await code.innerText(), 'const next={value:1};');
    await diff.locator('.diff-wrap').click();
    assert.equal(await code.innerText(), 'const next={value:1};');
    await diff.getByRole('button', {name: 'Copy complete diff', exact: true}).click();
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), raw);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), width);
    await shot(page, 'diff-raw-wrapped-' + width);
    assert.equal(fixture.state.sends.length, 0);
  });
  scenario(`Trace defaults to folded turns and preserves manual disclosure at ${width}`, {viewport: {width, height: 900}, locale: 'en-US', hasTouch: width < 700}, async(page, fixture) => {
    const turn = n => ({session_id: 'synthetic', prompt: 'Request ' + n, response: 'Result ' + n, time: `2026-11-01T01:0${n}:00Z`});
    fixture.state.turns['%901'] = [turn(1), turn(2)];
    await session(page, fixture, width); await openTools(page, 'trace', false, false);
    const trace = page.locator('.session-tool .trace-view');
    assert.equal(await trace.locator('.trace-turn').count(), 2);
    assert.equal(await trace.locator('.trace-event').count(), 0);
    fixture.state.turns['%901'].push(turn(3));
    await trace.locator('.trace-turn').nth(2).waitFor();
    assert.equal(await trace.locator('.trace-event').count(), 0);
    await trace.locator('.trace-turn').first().click();
    assert.equal(await trace.locator('.trace-turn').first().getAttribute('aria-expanded'), 'true');
    fixture.state.turns['%901'].push(turn(4));
    await trace.locator('.trace-turn').nth(3).waitFor();
    assert.equal(await trace.locator('.trace-turn').first().getAttribute('aria-expanded'), 'true');
    assert.equal(await trace.locator('.trace-turn').last().getAttribute('aria-expanded'), 'false');
    await shot(page, 'trace-folded-default-' + width);
    await closeTools(page); assert.equal(fixture.state.sends.length, 0);
  });
}
