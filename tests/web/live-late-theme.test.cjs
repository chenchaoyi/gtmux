const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const frozen of [true, false]) {
  scenario('WLI-009 Late host theme reconciles the ' + (frozen ? 'frozen' : 'following') + ' Live frame',
    {viewport: {width: 1440, height: 950}, locale: 'en-US', colorScheme: 'dark'}, async (page, fixture) => {
      const palette = Array(16).fill('#888888');
      palette[1] = '#c44055';
      let releaseTheme;
      const delayedTheme = new Promise(resolve => { releaseTheme = resolve; });
      await page.route('**/api/theme', async route => {
        await delayedTheme;
        await route.fulfill({json: {palette}});
      });
      const screen = code => Array.from({length: 95}, (_, i) => i === 90 ?
        '\x1b[' + code + 'mSample output\x1b[0m' : 'Terminal row ' + i).join('\n');
      const rgb = screen('38;2;204;0;0'), ansi = screen('31');
      fixture.state.screens['%901'] = rgb;
      async function receive(raw) {
        const delivered = page.waitForResponse(async r => {
          if (!r.ok() || !new URL(r.url()).pathname.endsWith('/api/pane')) return false;
          return (await r.json()).text === raw;
        });
        fixture.state.screens['%901'] = raw;
        await delivered;
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      }
      try {
        await page.goto(fixture.url + '/#code=TEST-1234');
        await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
        await page.locator('.tile .tile-modes [data-m="chat"]').click();
        const panel = page.locator('.tile .chat-live'), output = panel.locator('.live-output');
        const badge = panel.getByText('New output', {exact: true});
        await panel.locator('summary').click();
        const sample = output.getByText('Sample output', {exact: true});
        await sample.waitFor();
        assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(204, 0, 0)');
        await sample.evaluate(el => { window.lateThemeLiveNode = el; });
        await receive(ansi);
        assert.equal(await sample.evaluate(el => window.lateThemeLiveNode === el), true);

        let readingTop;
        if (frozen) {
          await output.hover();
          await page.mouse.wheel(0, -380);
          await panel.getByRole('button', {name: 'Return to live'}).waitFor();
          readingTop = await output.evaluate(el => el.scrollTop);
          await sample.evaluate(el => {
            const range = document.createRange();
            range.selectNodeContents(el);
            const selection = window.getSelection();
            selection.removeAllRanges();
            selection.addRange(range);
          });
          assert.equal(await page.evaluate(() => window.getSelection().toString()), 'Sample output');
        } else {
          await page.evaluate(() => {
            const observer = new MutationObserver(() => {
              if (document.documentElement.style.getPropertyValue('--terminal-ansi-1') !== '#c44055') return;
              const span = [...document.querySelectorAll('.live-output span')].find(el => el.textContent === 'Sample output');
              window.liveColorAtThemeUpdate = getComputedStyle(span).color;
              observer.disconnect();
            });
            observer.observe(document.documentElement, {attributes: true, attributeFilter: ['style']});
          });
        }
        releaseTheme();
        await page.waitForFunction(() => getComputedStyle(document.documentElement).getPropertyValue('--terminal-ansi-1').trim() === '#c44055');
        if (frozen) {
          await receive(rgb);
          assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(204, 0, 0)');
          assert.equal(await sample.evaluate(el => window.lateThemeLiveNode === el), true);
          assert.equal(await page.evaluate(() => window.getSelection().toString()), 'Sample output');
          assert.equal(await output.evaluate(el => el.scrollTop), readingTop);
          assert.equal(await badge.isVisible(), false);
        } else {
          await page.waitForFunction(() => window.liveColorAtThemeUpdate !== undefined);
          assert.equal(await page.evaluate(() => window.liveColorAtThemeUpdate), 'rgb(196, 64, 85)');
          assert.equal(await sample.evaluate(el => getComputedStyle(el).color), 'rgb(196, 64, 85)');
        }
        assert.equal(fixture.state.sends.length, 0);
      } finally {
        releaseTheme();
      }
    });
}
