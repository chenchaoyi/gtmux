const {test,after} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {chromium} = require('playwright');
const {start} = require('./fixture.cjs');
const {browserPool} = require('./browser-pool.cjs');
const launch=()=>chromium.launch({headless:true,executablePath:process.env.GTMUX_TEST_CHROMIUM || undefined});
const pool=browserPool(launch);
let freshLaunches=0;
after(()=>pool.close());

function scenario(name, options, run) {
  test(name, {timeout: 45000}, async () => {
    const started=performance.now();
    const timings={};
    let fixture, browser, context, page;
    const errors = [];
    try {
      fixture=await start();
      const boot=performance.now();
      browser = process.env.GTMUX_WEB_FRESH_BROWSER==='1' ? (freshLaunches++,await launch()) : await pool.get();
      timings.browser_ms=performance.now()-boot;
      context = await browser.newContext(options);
      context.setDefaultTimeout(6000);
      await context.tracing.start({screenshots: true, snapshots: true});
      page = await context.newPage();
      page.on('pageerror', error => errors.push(error.stack || error.message));
      page.on('console', message => {
        // Invalid enrollment and deliberate revocation produce expected HTTP 401 console messages.
        const status = /status of (\d+)/.exec(message.text());
        const expected = status && (status[1] === '401' || fixture.state.expectedHttpErrors.includes(Number(status[1])));
        if (['error', 'warning'].includes(message.type()) && !expected) errors.push(message.type() + ': ' + message.text());
      });
      const action=performance.now();
      await run(page, fixture);
      assert.match(await page.title(), / · gtmux$/);
      assert.equal(errors.length, 0, errors.join('\n'));
      timings.action_ms=performance.now()-action;
    } catch (error) {
      const output = path.join(process.env.GTMUX_WEB_RESULTS || path.join(__dirname, 'results'), name.replace(/[^a-z0-9]+/gi, '-'));
      await fs.mkdir(output, {recursive: true});
      if (page) await page.screenshot({path: path.join(output, 'failure.png'), fullPage: true}).catch(() => {});
      if (context) await context.tracing.stop({path: path.join(output, 'trace.zip')}).catch(() => {});
      await fs.writeFile(path.join(output, 'failure.txt'), String(error.stack) + '\nPage errors:\n' + errors.join('\n'));
      throw error;
    } finally {
      const cleanup=performance.now();
      try {if(context)await context.close();}
      finally {
        try {if(browser && process.env.GTMUX_WEB_FRESH_BROWSER==='1')await browser.close();}
        finally {await fixture?.close();}
      }
      timings.cleanup_ms=performance.now()-cleanup;
      if(process.env.GTMUX_WEB_TIMING) {
        await fs.mkdir(process.env.GTMUX_WEB_TIMING,{recursive:true});
        await fs.appendFile(path.join(process.env.GTMUX_WEB_TIMING,'worker-'+process.pid+'.jsonl'),JSON.stringify({name,...timings,total_ms:performance.now()-started,browser_launches:pool.launches+freshLaunches})+'\n');
      }
    }
  });
}

module.exports = {scenario};
