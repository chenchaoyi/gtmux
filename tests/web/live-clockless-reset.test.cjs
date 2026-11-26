const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const width of [390, 1440]) {
  for (const withID of [false, true]) scenario('LRG-002 clockless reset at ' + width + ' with generation ' + withID,
    {viewport: {width, height: 900}, locale: 'en-US'}, async (page, fixture) => {
      const tile = width > 900;
      const root = (tile ? '.tile' : '#chat') + ' .chat-live';
      const panel = page.locator(root);
      const output = panel.locator('.live-output');
      fixture.state.agents[0].agent = 'Claude Code';
      fixture.state.screens['%901'] = Array.from({length: 140}, (_, n) => 'Earlier reset frame ' + n).join('\n');
      let generation = 1;
      await page.route('**/api/transcript?**', async route => {
        return route.fulfill({status: 200, headers: {
          'X-Gtmux-Conversation-State': 'verified', 'X-Gtmux-Session-Reset': 'clear',
          ...(withID ? {'X-Gtmux-Session-Reset-ID': String(generation)} : {}),
        }, json: generation > 1 ? [] : [{session_id: 'first-cleared-session', prompt: 'First cleared session', response: 'Old cleared reply'}]});
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
      await page.getByText('Old cleared reply', {exact: true}).first().waitFor();
      await panel.locator('summary').click();
      await output.filter({hasText: 'Earlier reset frame 139'}).waitFor();
      await panel.getByRole('separator').press('ArrowUp');
      await output.hover();
      await page.mouse.wheel(0, -275);
      await panel.getByRole('button', {name: 'Return to live'}).waitFor();

      generation = 2;
      fixture.state.screens['%901'] = 'Second session current pane';
      await page.getByText('Old cleared reply', {exact: true}).first().waitFor({state: 'hidden'});
      await page.waitForFunction(sel => document.querySelector(sel)?.open === false, root);
      assert.equal((await output.textContent()).includes('Earlier reset frame'), false);
      await panel.locator('summary').click();
      await output.filter({hasText: 'Second session current pane'}).waitFor();
      const next = page.waitForResponse(response => new URL(response.url()).pathname.endsWith('/api/transcript'));
      await next;
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      assert.equal(await panel.evaluate(el => el.open), true);
      if (withID) {
        generation = 3;
        fixture.state.screens['%901'] = 'Third session current pane';
        await page.waitForFunction(sel => document.querySelector(sel)?.open === false, root);
        assert.equal((await output.textContent()).includes('Second session current pane'), false);
      }
    });
}
