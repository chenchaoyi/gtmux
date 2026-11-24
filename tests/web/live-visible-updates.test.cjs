const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

scenario('WLI-007 Live only announces visible changes and does announce an empty capture',
  {viewport: {width: 1440, height: 950}, locale: 'en-US'}, async (page, fixture) => {
    const rows = Array.from({length: 2050}, (_, i) => 'Terminal output ' + i);
    const colored = '\x1b[31mCurrent readable output\x1b[0m';
    rows[2048] = colored;
    fixture.state.screens['%901'] = rows.join('\n');
    await page.goto(fixture.url + '/#code=TEST-1234');
    await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
    await page.locator('.tile .tile-modes [data-m="chat"]').click();
    const panel = page.locator('.tile .chat-live'), output = panel.locator('.live-output');
    const status = panel.getByText('New output', {exact: true});
    await panel.locator('summary').click();
    const styled = output.getByText('Current readable output', {exact: true});
    await styled.waitFor();
    await styled.evaluate(el => { window.testLiveNode = el; });

    async function nextCapture(screen) {
      const received = page.waitForResponse(async r => {
        if (!r.ok() || !new URL(r.url()).pathname.endsWith('/api/pane')) return false;
        const capture = await r.json();
        return capture.text === screen;
      });
      fixture.state.screens['%901'] = screen;
      await received;
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    }
    rows[0] = 'Different discarded text';
    rows[2048] = '\x1b]2;different invisible title\x07' + colored;
    await nextCapture(rows.join('\n'));
    assert.equal(await styled.evaluate(el => window.testLiveNode === el), true);
    await output.hover();
    await page.mouse.wheel(0, -420);
    const resume = panel.getByRole('button', {name: 'Return to live'});
    await resume.waitFor();
    const top = await output.evaluate(el => el.scrollTop);
    rows[0] = 'Another discarded row';
    rows[2048] = '\x1b[2K' + colored;
    await nextCapture(rows.join('\n'));
    assert.equal(await status.isVisible(), false);
    assert.equal(await styled.evaluate(el => window.testLiveNode === el), true);
    assert.equal(await output.evaluate(el => el.scrollTop), top);
    await nextCapture('  \n\x1b[0m');
    await status.waitFor();
    assert.equal(await output.evaluate(el => el.scrollTop), top);
    assert.equal(await styled.evaluate(el => window.testLiveNode === el), true);
    await resume.click();
    await output.getByText('Waiting for output…', {exact: true}).waitFor();
    assert.equal(await status.isVisible(), false);
    assert.equal(fixture.state.sends.length, 0);
  });
