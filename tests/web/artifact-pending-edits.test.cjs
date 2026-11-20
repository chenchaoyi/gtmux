const assert = require('node:assert/strict');
const {scenario} = require('./artifact-harness.cjs');

const options = {locale: 'en-US', viewport: {width: 390, height: 850}};

async function openChat(page) {
  await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
  await page.locator('#mode [data-mode="chat"]').click();
  await page.locator('#chat-input .cx-input').waitFor();
}

// Split the shared-storage update and its asynchronous notification so both
// legal orderings of the save callback can be checked deterministically.
for (const callback of ['timer', 'pagehide']) {
  scenario('LAP-012 pending draft survives an early ' + callback, options, async (page, fixture) => {
    await page.goto(fixture.url + '/#code=TEST-1234');
    await openChat(page);
    const input = page.locator('#chat-input .cx-input');
    await page.clock.install();
    await input.fill('Original profile edit');
    await page.evaluate(() => localStorage.setItem('gtmux.token', 'next-profile'));
    if (callback === 'timer') await page.clock.runFor(450);
    else await page.evaluate(() => window.dispatchEvent(new Event('pagehide')));
    const savedDrafts = () => page.evaluate(() => Object.entries(localStorage)
      .filter(([key]) => key.startsWith('gtmux.compose:')).map(([, value]) => JSON.parse(value).drafts));
    assert.equal((await savedDrafts()).some(drafts => drafts['%901']?.text === 'Original profile edit'), false);
    await page.evaluate(() => window.dispatchEvent(new StorageEvent('storage', {
      storageArea: localStorage, key: 'gtmux.token', oldValue: 'synthetic-token', newValue: 'next-profile'
    })));
    await openChat(page);
    assert.equal(await input.inputValue(), '');
    const profiles = await savedDrafts();
    assert.equal(profiles.filter(drafts => drafts['%901']?.text === 'Original profile edit').length, 1);
    await page.goto(fixture.url + '/#code=TEST-1234');
    await page.reload();
    await openChat(page);
    assert.equal(await input.inputValue(), 'Original profile edit');
  });
}

for (const denial of ['generation', 'writes']) {
  scenario('LAP-013 bookmarks remain usable without persistent ' + denial, options, async (page, fixture) => {
    await page.addInitScript(mode => {
      const write = Storage.prototype.setItem;
      Storage.prototype.setItem = function (key, value) {
        if (mode === 'writes' || key.startsWith('gtmux.compose-generation:')) {
          throw new DOMException('Synthetic storage quota', 'QuotaExceededError');
        }
        return write.call(this, key, value);
      };
    }, denial);
    fixture.state.turns['%901'] = Array.from({length: 32}, (_, i) => ({
      session_id: 'offline-marks', record_id: 'turn-' + i, turn_ordinal: i + 1,
      prompt: 'Question ' + i, response: 'Reply ' + i
    }));
    await page.goto(fixture.url + '/#code=TEST-1234');
    await openChat(page);
    const track = page.locator('#chat .conversation-map-track');
    const tip = page.locator('#chat .conversation-map-preview');
    await track.focus(); await track.press('Home');
    await tip.getByRole('button', {name: 'Bookmark this turn'}).click();
    assert.equal(await track.locator('i.marked').count(), 1);
    assert.match(await tip.innerText(), /this page only/);
    fixture.state.turns['%901'].push({session_id: 'offline-marks', record_id: 'last', turn_ordinal: 33, prompt: 'New turn', response: 'New reply'});
    await page.locator('#chat').getByText('New reply', {exact: true}).waitFor();
    await track.focus(); await track.press('Home');
    await tip.getByRole('button', {name: 'Remove bookmark'}).click();
    assert.equal(await track.locator('i.marked').count(), 0);
    assert.equal(await page.evaluate(() => Object.keys(localStorage).some(k => k.startsWith('gtmux.bookmarks:'))), false);
  });
}
