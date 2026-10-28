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
      page.on('console', message => {
        // Invalid enrollment and deliberate revocation produce expected HTTP 401 console messages.
        const status = /status of (\d+)/.exec(message.text());
        const expected = status && (status[1] === '401' || fixture.state.expectedHttpErrors.includes(Number(status[1])));
        if (['error', 'warning'].includes(message.type()) && !expected) errors.push(message.type() + ': ' + message.text());
      });
      await run(page, fixture);
      assert.match(await page.title(), / · gtmux$/);
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

module.exports = {scenario};
