const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

scenario('WLI-008 Equivalent ANSI and RGB styles do not report new output',
  {viewport: {width: 1440, height: 950}, locale: 'en-US'}, async (page, fixture) => {
    const palette = Array(16).fill('#888888');
    palette[1] = '#c44055';
    await page.route('**/api/theme', route => route.fulfill({json: {palette}}));
    const render = run => Array.from({length: 95}, (_, i) => i === 90 ? run : 'Terminal row ' + i).join('\n');
    fixture.state.screens['%901'] = render('\x1b[31mSample output\x1b[0m');
    await page.goto(fixture.url + '/#code=TEST-1234');
    await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
    await page.locator('.tile .tile-modes [data-m="chat"]').click();
    const panel = page.locator('.tile .chat-live'), output = panel.locator('.live-output');
    const status = panel.getByText('New output', {exact: true});
    await panel.locator('summary').click();
    const sample = output.getByText('Sample output', {exact: true});
    await sample.waitFor();
    await page.waitForFunction(() => getComputedStyle(document.documentElement).getPropertyValue('--terminal-ansi-1').trim() === '#c44055');
    assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(196, 64, 85)');
    await sample.evaluate(el => { window.equivalentLiveNode = el; });

    async function nextCapture(run) {
      const screen = render(run);
      const delivered = page.waitForResponse(async r => {
        if (!r.ok() || !new URL(r.url()).pathname.endsWith('/api/pane')) return false;
        return (await r.json()).text === screen;
      });
      fixture.state.screens['%901'] = screen;
      await delivered;
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    }
    await nextCapture('\x1b[38;2;196;64;85mSample output\x1b[0m');
    assert.equal(await sample.evaluate(el => window.equivalentLiveNode === el), true);

    await output.hover();
    await page.mouse.wheel(0, -420);
    const resume = panel.getByRole('button', {name: 'Return to live'});
    await resume.waitFor();
    const top = await output.evaluate(el => el.scrollTop);
    await nextCapture('\x1b[38;5;1mSample\x1b[38:2::196:64:85m output\x1b[0m');
    assert.equal(await status.isVisible(), false);
    assert.equal(await sample.evaluate(el => window.equivalentLiveNode === el), true);
    assert.equal(await output.evaluate(el => el.scrollTop), top);
    await nextCapture('\x1b[38;2;20;162;110mSample output\x1b[0m');
    await status.waitFor();
    assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(196, 64, 85)');
    await resume.click();
    assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(20, 162, 110)');
    assert.equal(fixture.state.sends.length, 0);
  });
