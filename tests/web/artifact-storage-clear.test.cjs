const assert = require('node:assert/strict');
const {scenario} = require('./artifact-harness.cjs');

const options = {locale: 'en-US', viewport: {width: 390, height: 850}};
async function openDraft(page, fixture) {
  await page.goto(fixture.url + '/#code=TEST-1234');
  await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  const input = page.locator('#chat-input .cx-input');
  await input.fill('Synthetic saved draft');
  await page.waitForFunction(() => Object.entries(localStorage).some(([key, text]) =>
    key.startsWith('gtmux.compose:') && text.includes('Synthetic saved draft')));
  return input;
}

scenario('LAP-011 clearing site data retires a scheduled save', options, async (page, fixture) => {
  const input = await openDraft(page, fixture);
  await page.clock.install();
  await input.fill('Synthetic pending draft');
  // A clear in this document does not send a storage event to this document.
  await page.evaluate(() => localStorage.clear());
  await page.clock.runFor(450);
  assert.equal(await page.evaluate(() => localStorage.length), 0);
  await page.goto(fixture.url + '/style.css');
  assert.equal(await page.evaluate(() => Object.keys(localStorage).filter(k => k.startsWith('gtmux.compose:')).length), 0);
  await page.goto(fixture.url);
  await page.locator('#gate-code-input').waitFor();
});

scenario('LAP-011 server revocation wins before the other tab retires', options, async (page, fixture) => {
  // Observe the page-hide writer at notification time, before the view listener.
  // The event continues to the app normally.
  await page.addInitScript(() => {
    window.addEventListener('storage', event => {
      if (event.key !== 'gtmux.token' || event.newValue !== null) return;
      const saved = () => Object.entries(localStorage).filter(([key]) => key.startsWith('gtmux.compose:'));
      const before = saved();
      const draft = document.querySelector('#chat-input .cx-input')?.value;
      window.dispatchEvent(new Event('pagehide'));
      window.saveObservation = {draft, before, after: saved()};
    });
  });
  const input = await openDraft(page, fixture);
  const other = await page.context().newPage();
  try {
    await input.fill('Synthetic pending draft');
    // Only the second page receives the fixture server's real HTTP refusal.
    await other.route('**/api/agents', route => route.fulfill({status: 401, json: {error: 'revoked'}}));
    await other.goto(fixture.url);
    await other.locator('#gate-code-input').waitFor();
    await page.waitForFunction(() => window.saveObservation);
    const {draft, before, after} = await page.evaluate(() => window.saveObservation);
    assert.equal(draft, 'Synthetic pending draft');
    assert.deepEqual(after, before);
    await page.locator('#gate-code-input').waitFor();
    assert.equal(await page.evaluate(() => Object.keys(localStorage).filter(k => k.startsWith('gtmux.compose:')).length), 0);
  } finally {
    await other.close();
  }
});
