const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const pause of ['scroll', 'selection']) {
  scenario('WLI-010 Paused ANSI output keeps its palette after ' + pause,
    {viewport: {width: 1440, height: 950}, locale: 'en-US', colorScheme: 'dark'}, async (page, fixture) => {
      const palette = Array(16).fill('#888888');
      palette[1] = '#c44055';
      let releaseTheme, allowReads;
      const theme = new Promise(resolve => { releaseTheme = resolve; });
      await page.route('**/api/theme', async route => {
        await theme;
        await route.fulfill({json: {palette, foreground: '#ddeeff', background: '#223344'}});
      });
      const screen = code => Array.from({length: 95}, (_, i) => i === 90 ?
        '\x1b[' + code + 'mColored output\x1b[0m' : 'Terminal row ' + i).join('\n');
      const ansi = screen('31'), equivalent = screen('38;2;204;0;0'), changed = screen('38;2;20;170;55');
      fixture.state.screens['%901'] = ansi;
      async function receive(raw) {
        const response = page.waitForResponse(async r => {
          if (!r.ok() || !new URL(r.url()).pathname.endsWith('/api/pane')) return false;
          return (await r.json()).text === raw;
        });
        fixture.state.screens['%901'] = raw;
        await response;
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      }
      try {
        await page.goto(fixture.url + '/#code=TEST-1234');
        await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
        await page.locator('.tile .tile-modes [data-m="chat"]').click();
        const panel = page.locator('.tile .chat-live'), output = panel.locator('.live-output');
        const badge = panel.getByText('New output', {exact: true}), resume = panel.getByRole('button', {name: 'Return to live'});
        await panel.locator('summary').click();
        const sample = output.getByText('Colored output', {exact: true});
        await sample.evaluate(el => { window.originalLiveNode = el; });
        await receive(equivalent);
        assert.equal(await sample.evaluate(el => window.originalLiveNode === el), true);
        if (pause === 'scroll') {
          await output.hover();
          await page.mouse.wheel(0, -380);
          await resume.waitFor();
        } else {
          const held = new Promise(resolve => { allowReads = resolve; });
          await page.route('**/api/pane?**', async route => {
            await held;
            await route.continue();
          });
        }
        await sample.evaluate(el => {
          const range = document.createRange();
          range.selectNodeContents(el);
          const selection = window.getSelection();
          selection.removeAllRanges();
          selection.addRange(range);
        });
        const readingTop = await output.evaluate(el => el.scrollTop);
        if (pause === 'selection') assert.equal(await resume.isVisible(), false);
        releaseTheme();
        await page.waitForFunction(() => document.documentElement.style.getPropertyValue('--terminal-ansi-1') === '#c44055');
        await resume.waitFor();
        assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(204, 0, 0)');
        assert.deepEqual(await output.evaluate(el => {
          const style = getComputedStyle(el);
          return [style.color, style.backgroundColor];
        }), ['rgb(214, 214, 218)', 'rgb(23, 23, 26)']);
        assert.equal(await sample.evaluate(el => window.originalLiveNode === el), true);
        assert.equal(await page.evaluate(() => window.getSelection().toString()), 'Colored output');
        assert.equal(await output.evaluate(el => el.scrollTop), readingTop);
        assert.equal(await badge.isVisible(), false);
        if (allowReads) allowReads();
        await receive(equivalent);
        assert.equal(await badge.isVisible(), false);
        await receive(changed);
        await badge.waitFor();
        assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(204, 0, 0)');
        await resume.click();
        assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(20, 170, 55)');
        assert.deepEqual(await output.evaluate(el => {
          const style = getComputedStyle(el);
          return [style.color, style.backgroundColor];
        }), ['rgb(221, 238, 255)', 'rgb(34, 51, 68)']);
        assert.equal(fixture.state.sends.length, 0);
      } finally {
        if (allowReads) allowReads();
        releaseTheme();
      }
    });
}
