const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const width of [390, 1440]) {
  scenario('LRB-001 Binding preserves the same Live reader at ' + width,
    {viewport: {width, height: 900}, locale: 'en-US'}, async (page, fixture) => {
      const agent = fixture.state.agents[0];
      const sessionID = 'same-synthetic-source';
      agent.observed_conversation = {home_id: 'fixture-home', session_id: sessionID, state: 'recovered'};
      fixture.state.turns['%901'] = [{session_id: sessionID, prompt: 'Same source', response: 'Still here'}];
      fixture.state.screens['%901'] = Array.from({length: 140}, (_, i) => 'Same output ' + i).join('\n');
      await page.goto(fixture.url + '/#code=TEST-1234');
      const tile = width > 900;
      if (tile) {
        await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
        await page.locator('.tile-modes [data-m="chat"]').click();
        await page.locator('.tile-name').click();
      } else {
        await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
        await page.locator('#mode [data-mode="chat"]').click();
      }
      const selector = (tile ? '.tile' : '#chat') + ' .chat-live';
      const panel = page.locator(selector);
      const output = panel.locator('.live-output');
      await panel.locator('summary').click();
      await output.filter({hasText: 'Same output 139'}).waitFor();
      await panel.getByRole('separator').press('ArrowUp');
      await output.hover();
      await page.mouse.wheel(0, -275);
      await panel.getByRole('button', {name: 'Return to live'}).waitFor();
      const before = await panel.evaluate(el => {
        window.originalPanel = el;
        return {open: el.open, height: el.clientHeight, top: el.output.scrollTop, same: true};
      });
      const confirmed = page.waitForResponse(async response =>
        new URL(response.url()).pathname.endsWith('/api/agents') && response.ok() &&
        (await response.json()).some(row => row.pane_id === '%901' && !row.observed_conversation));
      delete agent.observed_conversation;
      fixture.state.turns['%901'].push({session_id: sessionID, prompt: 'Same confirmed source', response: 'Additional reply'});
      await confirmed;
      await page.getByText('Additional reply', {exact: true}).first().waitFor();
      await page.waitForFunction(sel => document.querySelector(sel)?.source?.hidden === true, selector);
      const after = await panel.evaluate(el => ({open: el.open, height: el.clientHeight, top: el.output.scrollTop, same: el === window.originalPanel}));
      assert.deepEqual(after, before);
      fixture.state.turns['%901'] = [{session_id: 'another-source', prompt: 'New source', response: 'New reply'}];
      await page.getByText('New reply', {exact: true}).first().waitFor();
      assert.equal(await panel.evaluate(el => el.open), false);
      assert.equal(await panel.evaluate(el => el === window.originalPanel), false);
    });
}
