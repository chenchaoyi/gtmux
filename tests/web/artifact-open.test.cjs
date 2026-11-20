const {test, after} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {chromium} = require('playwright');
const {start} = require('./fixture.cjs');
const {browserPool} = require('./browser-pool.cjs');
const pool = browserPool(() => chromium.launch({headless: true, executablePath: process.env.GTMUX_TEST_CHROMIUM || undefined}));
after(() => pool.close());
const id = 'b'.repeat(32), ref = 'gtmux-file:' + id, session = 'original-session';
const image = require('node:fs').readFileSync(path.resolve(__dirname, '../../assets/agent-icons/codex.png')).toString('base64');
const html = '<style>p{color:#223344}</style><h1>Snapshot report</h1><details><summary>Details</summary><p>Preserved detail</p></details><img src="data:image/png;base64,' + image + '"><a href="https://example.test/">Reference</a>';
function scenario(name, options, run) {
  test((name.startsWith('LAP-006') ? '' : 'LAP-004 ') + name, {timeout: 45000}, async () => {
    const fixture = await start(), browser = await pool.get(), context = await browser.newContext(options);
    const page = await context.newPage(), errors = [], outside = [];
    context.setDefaultTimeout(7000);
    fixture.state.requireAuth = 'synthetic-token';
    fixture.state.documents = {[ref]: {id, session, home: '', kind: 'html', media: 'text/html', body: html}};
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
    if (!name.startsWith('first pairing')) await page.addInitScript(() => {
      if (window.top === window && !localStorage.getItem('gtmux.token')) localStorage.setItem('gtmux.token', 'synthetic-token');
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
function entry(fixture, extra = '') {
  return fixture.url + '/#' + new URLSearchParams({document: id, session, home: '', ...extra}).toString();
}
for (const width of [390, 1440]) scenario('static HTML and immutable identity ' + width, {viewport: {width, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  const scopes = [];
  page.on('request', r => {if (r.url().endsWith('/api/artifacts/resolve')) scopes.push(r.postDataJSON());});
  await page.goto(entry(fixture));
  const frame = page.frameLocator('.conversation-document iframe');
  await frame.getByRole('heading', {name: 'Snapshot report'}).waitFor();
  assert.equal(new URL(page.url()).hash, '');
  assert.deepEqual(scopes, [{reference: ref, session_id: session, home_id: ''}]);
  assert.equal(await page.locator('.document-html').getAttribute('sandbox'), '');
  assert.equal(await frame.locator('script,iframe,meta[http-equiv="refresh"],a[href]').count(), 0);
  assert.equal(await frame.locator('img').count(), 1);
  await frame.locator('img').evaluate(i => i.decode());
  // Activate the native disclosure label/marker, rather than the row's blank center.
  await frame.locator('summary').click({position: {x: 20, y: 8}});
  await frame.getByText('Preserved detail', {exact: true}).waitFor({state: 'visible'});
  assert.equal(await frame.locator('details').getAttribute('open'), '');
  assert.equal(await frame.locator('p').evaluate(p => getComputedStyle(p).color), 'rgb(34, 51, 68)');
  const directory = process.env.GTMUX_WEB_RESULTS;
  if (directory) {await fs.mkdir(directory, {recursive: true}); await page.screenshot({path: path.join(directory, 'local-html-' + width + '.png')});}
  assert.equal(fixture.state.sends.length, 0);
});
scenario('restricted owner remains forbidden', {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  fixture.state.share.all = false; let reads = 0;
  page.on('request', r => {if (r.url().includes('/api/artifacts/')) reads++;});
  await page.goto(entry(fixture));
  await page.locator('.conversation-document .document-error').filter({hasText: 'This connection cannot view files.'}).waitFor();
  assert.equal(reads, 0); assert.equal(await page.locator('.document-html').count(), 0);
});
scenario('wrong home stays unavailable', {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  await page.goto(entry(fixture, {home: 'other-home'}));
  await page.locator('.conversation-document .document-error').filter({hasText: /conversation|available/i}).waitFor();
  assert.equal(await page.locator('.document-html').count(), 0);
});
scenario('revoked credentials retire the delayed document', {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  let release; const held = new Promise(resolve => {release = resolve;});
  const seen = page.waitForRequest('**/api/artifacts/content?*');
  await page.route('**/api/artifacts/content?*', async route => {const response = await route.fetch(); await held; await route.fulfill({response}).catch(() => {});});
  try {
    await page.goto(entry(fixture)); await seen; fixture.state.deny = true;
    await page.locator('#gate').waitFor({state: 'visible'}); release();
    assert.equal(await page.locator('.conversation-document').count(), 0);
    assert.equal(await page.locator('.document-html').count(), 0);
  } finally {release();}
});
scenario('corrupt bytes cannot enter HTML renderer', {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  const changed = Buffer.from(html.replace('Snapshot report', 'Modified report'));
  assert.equal(changed.length, Buffer.byteLength(html));
  await page.route('**/api/artifacts/content?*', route => route.fulfill({status: 200, body: changed}));
  await page.goto(entry(fixture)); await page.locator('.conversation-document .document-error').waitFor();
  assert.equal(await page.locator('.document-html').count(), 0);
});

scenario('existing credentials retain original scope without enrollment', {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  fixture.state.requireAuth = 'existing-owner';
  await page.addInitScript(() => {if (window.top === window) localStorage.setItem('gtmux.token', 'existing-owner');});
  const reads = [], enrollments = [];
  page.on('request', r => {
    if (r.url().includes('/api/artifacts/')) reads.push(r);
    if (r.url().includes('/api/enroll')) enrollments.push(r);
  });
  await page.goto(fixture.url + '/#document=' + id + '&session=' + session);
  await page.locator('.document-html').waitFor();
  assert.equal(new URL(page.url()).hash, '');
  assert.equal(enrollments.length, 0);
  assert.equal(reads.length, 2);
  assert.equal(reads.every(r => r.headers().authorization === 'Bearer existing-owner'), true);
  assert.deepEqual(reads[0].postDataJSON(), {reference: ref, session_id: session, home_id: ''});
});
scenario('LAP-006 PDF waits for a user action and releases its blob on close', {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  fixture.state.documents[ref] = {id, session, home: '', kind: 'pdf', media: 'application/pdf', body: '%PDF-1.4\nsynthetic fixture bytes'};
  await page.addInitScript(() => {
    window.__revokedURLs = [];
    const original = URL.revokeObjectURL;
    URL.revokeObjectURL = function (url) {window.__revokedURLs.push(url); return original.call(URL, url);};
  });
  await page.goto(entry(fixture));
  const action = page.getByRole('link', {name: 'Preview PDF in browser', exact: true});
  await action.waitFor();
  const blob = await action.getAttribute('href');
  assert.match(blob, /^blob:http:\/\/127\.0\.0\.1:/);
  assert.equal(await action.getAttribute('target'), '_blank');
  assert.equal(await action.getAttribute('rel'), 'noopener noreferrer');
  assert.equal(page.context().pages().length, 1);
  assert.equal(await page.locator('.document-html').count(), 0);
  await page.getByRole('button', {name: 'Close', exact: true}).click();
  await page.locator('.conversation-document').waitFor({state: 'detached'});
  assert.deepEqual(await page.evaluate(() => window.__revokedURLs), [blob]);
});
scenario('LAP-006 invalid PDF never exposes the preview action', {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
  fixture.state.documents[ref] = {id, session, home: '', kind: 'pdf', media: 'application/pdf', body: 'invalid pdf fixture bytes'};
  await page.goto(entry(fixture));
  await page.locator('.conversation-document .document-error').filter({hasText: 'This format cannot be previewed in the browser.'}).waitFor();
  assert.equal(await page.locator('.document-pdf').count(), 0);
  assert.equal(page.context().pages().length, 1);
});

scenario('first pairing resumes the requested snapshot', {locale: 'en-US'}, async (page, fixture) => {
  const reads = [];
  page.on('request', r => {if (r.url().includes('/api/artifacts/')) reads.push(r);});
  await page.goto(entry(fixture));
  await page.locator('#gate-code-input').waitFor();
  assert.equal(reads.length, 0);
  await page.locator('#gate-code-input').fill('TEST-1234');
  await page.locator('#gate-code-go').click();
  await page.frameLocator('.document-html').getByText('Snapshot report').waitFor();
  assert.equal(reads.length, 2);
});

scenario('local image uses verified parent and layout stays resource-free', {locale: 'en-US'}, async (page, fixture) => {
  const src = 'assets/chart.png?revision=1';
  fixture.state.documents[ref].body = '<style>.chart{padding:4px;background-image:url(chart.png)}</style>' +
    '<div class="chart" style="margin:3px;background:linear-gradient(red,blue)"><img alt="chart" src="' + src + '"></div>';
  fixture.state.documents[src] = {id: 'c'.repeat(32), session, home:'', kind:'image', media:'image/png', body:Buffer.from(image, 'base64')};
  const scopes = [];
  page.on('request', r => {if (r.url().endsWith('/api/artifacts/resolve')) scopes.push(r.postDataJSON());});
  await page.goto(entry(fixture));
  const frame = page.frameLocator('.document-html');
  await frame.getByAltText('chart').evaluate(i => i.decode());
  assert.deepEqual(scopes, [{reference:ref, session_id:session, home_id:''}, {reference:src, session_id:session, home_id:'', parent_id:id}]);
  assert.deepEqual(await frame.locator('.chart').evaluate(e => {
    const css = getComputedStyle(e); return [css.paddingTop, css.marginTop, css.backgroundImage];
  }), ['4px', '3px', 'none']);
});
scenario('different registered ID is refused before reading bytes', {locale: 'en-US'}, async (page, fixture) => {
  fixture.state.documents[ref].id = 'c'.repeat(32);
  let reads = 0;
  page.on('request', r => {if (r.url().includes('/api/artifacts/content')) reads++;});
  await page.goto(entry(fixture));
  await page.locator('.document-error').waitFor();
  assert.equal(reads, 0);
});

scenario('short document body stays unavailable', {locale: 'en-US'}, async (page, fixture) => {
  await page.route('**/api/artifacts/content?*', route => route.fulfill({status:200, body:html.slice(0, -1)}));
  await page.goto(entry(fixture));
  await page.locator('.document-error').waitFor();
  assert.equal(await page.locator('.document-html').count(), 0);
});
scenario('inline raster type and count are checked before display', {locale: 'en-US'}, async (page, fixture) => {
  const png = '<img src="data:image/png;base64,' + image + '">';
  fixture.state.documents[ref].body = '<p>Figures</p>' + png.repeat(33);
  await page.goto(entry(fixture));
  const frame = page.frameLocator('.document-html');
  await frame.getByText('Figures').waitFor();
  assert.equal(await frame.locator('img').count(), 32);
  fixture.state.documents[ref].body = '<p>Format check</p><img src="data:image/jpeg;base64,' + image + '">' + png;
  await page.goto(entry(fixture));
  await frame.getByText('Format check').waitFor();
  assert.equal(await frame.locator('img').count(), 1);
});
