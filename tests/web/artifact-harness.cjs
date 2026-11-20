const {test, after} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {chromium} = require('playwright');
const {start} = require('./fixture.cjs');
const {browserPool} = require('./browser-pool.cjs');
const pool = browserPool(() => chromium.launch({headless: true, executablePath: process.env.GTMUX_TEST_CHROMIUM || undefined}));
after(() => pool.close());
function scenario(name, options, run) {
  test(name, {timeout: 45000}, async () => {
    const fixture = await start(), browser = await pool.get(), context = await browser.newContext(options);
    const page = await context.newPage(), errors = [], outside = [];
    context.setDefaultTimeout(7000);
    page.on('pageerror', e => errors.push(e.message));
    page.on('console', m => {
      if (!['warning', 'error'].includes(m.type())) return;
      const text = m.text();
      // Automation probes in this opaque, script-disabled frame are refused.
      if (/^Blocked script execution in 'about:srcdoc'.*'allow-scripts' permission is not set/.test(text)) return;
      if (/status of (401|403|404)/.test(text)) return;
      errors.push(text);
    });
    page.on('request', r => {if (/^https?:/.test(r.url()) && !r.url().startsWith(new URL(fixture.url).origin + '/')) outside.push(r.url());});
    await page.route('**/api/artifacts/resolve', async route => {
      const response = await route.fetch();
      if (response.status() !== 200) { await route.fulfill({response}); return; }
      // The generic fixture uses the URI as source; a registered production
      // artifact has an absolute original path and its real display name.
      const metadata = await response.json();
      const name = metadata.kind === 'image' ? 'chart.png' : 'report.' + (metadata.kind === 'pdf' ? 'pdf' : 'html');
      await route.fulfill({response, json: {...metadata, name, source: '/synthetic/' + name}});
    });
    try {
      await run(page, fixture);
      assert.match(await page.title(), / · gtmux$/); assert.deepEqual(errors, []); assert.deepEqual(outside, []);
    } catch (error) {
      const directory = process.env.GTMUX_WEB_RESULTS || path.join(__dirname, 'results');
      await fs.mkdir(directory, {recursive: true});
      await page.screenshot({path: path.join(directory, 'local-artifact-' + name.replace(/[^a-z0-9]+/gi, '-') + '.png')}).catch(() => {});
      await fs.writeFile(path.join(directory, 'local-artifact-' + name.replace(/[^a-z0-9]+/gi, '-') + '.failure.json'), JSON.stringify({error: String(error), errors, outside}, null, 2));
      throw error;
    } finally {await context.close(); await fixture.close();}
  });
}
module.exports = {scenario};
