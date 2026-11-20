const assert = require('node:assert/strict');
const {scenario} = require('./artifact-harness.cjs');

scenario('LAP-010 switching pairing in another tab restores both profiles', {
  locale: 'en-US', viewport: {width: 390, height: 850}
}, async (page, fixture) => {
  fixture.state.turns['%901'] = Array.from({length: 32}, (_, i) => ({
    session_id: 'saved-profile', record_id: 'turn-' + i, turn_ordinal: i + 1,
    prompt: 'Question ' + i, response: 'Reply ' + i
  }));
  const other = await page.context().newPage();
  await other.goto(fixture.url + '/style.css');
  const switchIdentity = identity => other.evaluate(value => {
    if (value) localStorage.setItem('gtmux.token', value);
    else localStorage.removeItem('gtmux.token');
  }, identity);
  const input = page.locator('#chat-input .cx-input');
  const track = page.getByRole('slider', {name: 'Conversation overview — loaded turns'});
  const tip = page.locator('#chat .conversation-map-preview');
  async function openConversation() {
    await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
    await page.locator('#mode [data-mode="chat"]').click();
    await input.waitFor();
    await track.focus();
    await track.press('Home');
  }
  async function checkDraftAndBookmark(draft, turn) {
    await openConversation();
    assert.equal(await input.inputValue(), draft);
    if (turn === 2) await track.press('ArrowDown');
    assert.equal(await tip.getByRole('button', {name: 'Remove bookmark'}).getAttribute('aria-pressed'), 'true');
    assert.equal(await track.locator('i.marked').count(), 1);
  }

  try {
    await switchIdentity('fixture-one');
    await page.goto(fixture.url);
    await openConversation();
    await tip.getByRole('button', {name: 'Bookmark this turn'}).click();
    await input.fill('First saved draft');

    await switchIdentity('fixture-two');
    await openConversation();
    assert.equal(await input.inputValue(), '');
    assert.equal(await track.locator('i.marked').count(), 0);
    await track.press('ArrowDown');
    await tip.getByRole('button', {name: 'Bookmark this turn'}).click();
    await input.fill('Second saved draft');

    await switchIdentity('fixture-one');
    await checkDraftAndBookmark('First saved draft', 1);
    await switchIdentity(null);
    await page.locator('#gate-code-input').waitFor();
    await page.reload();
    await page.locator('#gate-code-input').waitFor();
    await switchIdentity('fixture-two');
    await checkDraftAndBookmark('Second saved draft', 2);
    await page.reload();
    await checkDraftAndBookmark('Second saved draft', 2);
    await switchIdentity('fixture-one');
    await checkDraftAndBookmark('First saved draft', 1);
    assert.equal(fixture.state.sends.length, 0);
  } finally {
    await other.close();
  }
});
