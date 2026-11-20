const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {scenario} = require('./artifact-harness.cjs');

const id = 'a'.repeat(32), reference = 'gtmux-file:' + id, session = 'fixture';
const options = {locale: 'en-US', viewport: {width: 390, height: 850}};
const image = fs.readFileSync(path.join(__dirname, '../../assets/agent-icons/codex.png'));

async function prepare(page, fixture, body) {
  fixture.state.documents = {[reference]: {id, session, home: '', kind: 'html', media: 'text/html', body}};
  await page.goto(fixture.url + '/style.css');
  await page.evaluate(() => localStorage.setItem('gtmux.token', 'fixture-owner'));
  return fixture.url + '/#document=' + id + '&session=' + session;
}

async function holdContent(page) {
  let release;
  const hold = new Promise(resolve => { release = resolve; });
  const seen = page.waitForRequest('**/api/artifacts/content?*');
  await page.route('**/api/artifacts/content?*', async route => {
    const response = await route.fetch();
    await hold;
    await route.fulfill({response}).catch(() => {});
  }, {times: 1});
  return {seen, release};
}

scenario('LAP-008 another tab switches to a guest and signs out', options, async (page, fixture) => {
  const address = await prepare(page, fixture, '<p>Owner report</p>');
  const checks = [], reads = [];
  await page.route('**/api/share', route => {
    const auth = route.request().headers().authorization;
    checks.push(auth);
    return route.fulfill({json: {all: auth === 'Bearer fixture-owner', panes: []}});
  });
  page.on('request', r => {if (r.url().includes('/api/artifacts/')) reads.push(r);});
  const other = await page.context().newPage();
  await other.goto(fixture.url + '/style.css');
  const pending = await holdContent(page);
  try {
    await page.goto(address); await pending.seen;
    await other.evaluate(() => localStorage.setItem('gtmux.token', 'fixture-guest'));
    await page.locator('.conversation-document').waitFor({state: 'detached'});
    pending.release();
    const previous = reads.length;
    await page.goto(address);
    await page.getByText('This connection cannot view files.').waitFor();
    assert.equal(checks.at(-1), 'Bearer fixture-guest');
    assert.equal(reads.length, previous);
    await other.evaluate(() => localStorage.clear());
    await page.locator('#gate-code-input').waitFor();
    await page.goto(address);
    await page.locator('#gate-code-input').waitFor();
    assert.equal(await page.locator('.conversation-document').count(), 0);
    assert.equal(reads.length, previous);
  } finally { pending.release(); await other.close(); }
});

scenario('LAP-008 resizing retires a pending reader', options, async (page, fixture) => {
  const address = await prepare(page, fixture, '<p>Report</p>');
  const pending = await holdContent(page);
  try {
    await page.goto(address); await pending.seen;
    await page.setViewportSize({width: 1440, height: 900});
    await page.locator('.conversation-document').waitFor({state:'detached'});
    pending.release();
    await page.goto(address);
    await page.locator('.document-html').waitFor();
  } finally { pending.release(); }
});

scenario('LAP-009 oversized related image leaves the rest of the report readable', options, async (page, fixture) => {
  const address = await prepare(page, fixture, '<p>Figures</p><img alt="large" src="large.png"><img alt="small" src="small.png">');
  const large = 'b'.repeat(32), small = 'c'.repeat(32);
  for (const [src, key] of [['large.png', large], ['small.png', small]]) {
    fixture.state.documents[src] = {id:key, session, home:'', kind:'image', media:'image/png', body:image};
  }
  await page.route('**/api/artifacts/resolve', async route => {
    if (route.request().postDataJSON().reference !== 'large.png') return route.fallback();
    const response = await route.fetch(), file = await response.json();
    await route.fulfill({response, json:{...file, source:'/synthetic/large.png', bytes:8 * 1024 * 1024 + 1}});
  });
  const bodies = [];
  page.on('request', r => {if (r.url().includes('/api/artifacts/content?')) bodies.push(new URL(r.url()).searchParams.get('id'));});
  await page.goto(address);
  const frame = page.frameLocator('.document-html');
  await frame.getByText('Figures').waitFor();
  await frame.getByAltText('small').evaluate(i => i.decode());
  assert.equal(await frame.locator('img').count(), 1);
  assert.deepEqual(bodies, [id, small]);
  fixture.state.expectedHttpErrors.push(403);
  await page.route('**/api/artifacts/resolve', route => route.request().postDataJSON().parent_id
    ? route.fulfill({status:403, json:{error:'forbidden'}}) : route.fallback());
  await page.goto(address);
  await page.getByText('This connection cannot view files.').waitFor();
  assert.equal(await page.locator('.document-html').count(), 0);
});
