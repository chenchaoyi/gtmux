const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const width of [390, 1440]) {
  for (const edge of ['verified empty replacement', 'unassociated but pane readable']) {
    scenario((edge.startsWith('unassociated') ? 'LRE-002 ' : 'LRE-001 ') + edge + ' at ' + width,
      {viewport: {width, height: 900}, locale: 'en-US'}, async (page, fixture) => {
        const tile = width > 900;
        const path = (tile ? '.tile' : '#chat') + ' .chat-live';
        const panel = page.locator(path);
        const output = panel.locator('.live-output');
        const agent = fixture.state.agents[0];
        const first = 'original-session';
        const unassociated = edge.startsWith('unassociated');
        if (!unassociated) agent.agent = 'Claude Code';
        if (unassociated) agent.observed_conversation = {home_id: 'fixture', session_id: first, state: 'last-known'};
        fixture.state.turns['%901'] = [{session_id: first, prompt: 'Old prompt', response: 'Existing reply'}];
        fixture.state.screens['%901'] = Array.from({length: 140}, (_, i) => 'OLD FRAME ' + i).join('\n');

        let phase = 'original';
        let subsequentReads = 0;
        await page.route('**/api/transcript?**', async route => {
          if (phase === 'original') return route.continue();
          if (phase === 'empty') return route.fulfill({status: 200, json: []});
          subsequentReads++;
          if (unassociated) return route.fulfill({status: 409, json: {code: 'conversation-unassociated'}});
          return route.fulfill({status: 200, headers: {'X-Gtmux-Conversation-State': 'verified',
            'X-Gtmux-Session-Reset': 'clear', 'X-Gtmux-Session-Reset-At': '1700000000'}, json: []});
        });

        await page.goto(fixture.url + '/#code=TEST-1234');
        if (tile) {
          await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
          await page.locator('.tile-modes [data-m="chat"]').click();
          await page.locator('.tile-name').click();
        } else {
          await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
          await page.locator('#mode [data-mode="chat"]').click();
        }
        await panel.locator('summary').click();
        await output.filter({hasText: 'OLD FRAME 139'}).waitFor();
        await panel.getByRole('separator').press('ArrowUp');
        await output.hover();
        await page.mouse.wheel(0, -275);
        await panel.getByRole('button', {name: 'Return to live'}).waitFor();
        await page.getByText('Existing reply', {exact: true}).first().waitFor();
        if (!unassociated) {
          const empty = page.waitForResponse(r => new URL(r.url()).pathname.endsWith('/api/transcript') && r.ok());
          phase = 'empty';
          await empty;
          await page.getByText('Existing reply', {exact: true}).first().waitFor({state: 'hidden'});
          assert.equal(await panel.evaluate(el => el.open), true);
          await output.filter({hasText: 'OLD FRAME 139'}).waitFor();
        } else fixture.state.expectedHttpErrors.push(409);
        phase = 'changed';
        delete agent.observed_conversation;
        fixture.state.screens['%901'] = 'NEWLY AUTHORIZED CURRENT PANE';
        if (unassociated) await page.getByText('Conversation association unavailable.', {exact: true}).first().waitFor();
        await page.waitForFunction(sel => {
          const panel = document.querySelector(sel);
          return panel && !panel.hidden && !panel.open && !panel.output.textContent.includes('OLD FRAME');
        }, path);
        await panel.locator('summary').click();
        await output.filter({hasText: 'NEWLY AUTHORIZED CURRENT PANE'}).waitFor();
        subsequentReads = 0;
        await expectTwoReads();
        assert.equal(await panel.evaluate(el => el.open), true);
        assert.equal(await output.textContent(), 'NEWLY AUTHORIZED CURRENT PANE');
        async function expectTwoReads() {
          const finish = Date.now() + 6500;
          while (subsequentReads < 2 && Date.now() < finish) await new Promise(resolve => setTimeout(resolve, 100));
          assert.ok(subsequentReads >= 2);
        }
      });
  }
}
