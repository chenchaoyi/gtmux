const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const status of [403, 404]) {
  scenario('WLI-011 Access loss clears old Live content after ' + status,
    {viewport: {width: 1440, height: 950}, locale: 'en-US'}, async (page, fixture) => {
      const oldOutput = 'Output from the previously readable pane';
      const rows = Array.from({length: 95}, (_, i) => '\x1b[31m' + (i === 91 ? oldOutput : 'Row ' + i) + '\x1b[0m');
      fixture.state.screens['%901'] = rows.join('\n');
      fixture.state.expectedHttpErrors.push(status);
      await page.goto(fixture.url + '/#code=TEST-1234');
      await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
      await page.locator('.tile .tile-modes [data-m="chat"]').click();
      const panel = page.locator('.tile .chat-live'), output = panel.locator('.live-output');
      const resume = panel.getByRole('button', {name: 'Return to live'});
      await panel.locator('summary').click();
      const sample = output.getByText(oldOutput, {exact: true});
      await sample.waitFor();
      if (status === 404) {
        await output.hover();
        await page.mouse.wheel(0, -380);
        await resume.waitFor();
      }
      let release, held;
      const answer = new Promise(resolve => { release = resolve; });
      const requestHeld = new Promise(resolve => { held = resolve; });
      await page.route('**/api/pane?**', async route => {
        held();
        await answer;
        await route.fulfill({status});
      });
      try {
        await requestHeld;
        await sample.evaluate(el => {
          const range = document.createRange();
          range.selectNodeContents(el);
          const selection = window.getSelection();
          selection.removeAllRanges();
          selection.addRange(range);
        });
        assert.equal(await page.evaluate(() => window.getSelection().toString()), oldOutput);
        assert.equal(await resume.isVisible(), status === 404);
        release();
        await output.getByText('Live output is unavailable.', {exact: true}).waitFor();
        assert.equal(await sample.count(), 0);
        assert.equal(await page.evaluate(() => window.getSelection().toString()), '');
        assert.equal(await resume.isVisible(), false);
        assert.equal(await output.evaluate(el => el.style.getPropertyValue('--terminal-ansi-1')), '');
        await panel.getByText('Retrying automatically…', {exact: true}).waitFor();
        assert.equal(fixture.state.sends.length, 0);
      } finally {
        release();
      }
    });
}
