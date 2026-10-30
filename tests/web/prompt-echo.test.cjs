const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const width of [390, 1440]) {
  scenario('Recorded Codex image retires only its new sent echo at ' + width,
    {viewport: {width, height: 950}, locale: 'en-US'}, async (page, fixture) => {
      fixture.state.agents[0].session_id = 'image-session';
      fixture.state.agents[0].home_id = 'image-home';
      const old = {session_id: 'image-session', prompt: '![Image #1](/tmp/synthetic.png)\n[Image #1]', response: 'Older image reply', time: '2026-10-30T06:00:00Z'};
      fixture.state.turns['%901'] = [old];
      await page.goto(fixture.url + '/#code=TEST-1234');
      if (width < 800) {
        await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
        await page.locator('#mode [data-mode="chat"]').click();
      } else {
        await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
        await page.locator('.tile-modes [data-m="chat"]').click();
      }
      const composer = page.locator(width < 800 ? '#chat-input .composer' : '.tile .composer');
      await page.getByText('Older image reply', {exact: true}).waitFor();
      await composer.locator('.cx-input').fill('/tmp/synthetic.png');
      await composer.locator('.cx-send').click();
      await page.locator('.pending-prompt-note').filter({hasText: /^Sent$/}).waitFor();
      assert.equal(await page.locator('.pending-prompt').count(), 1, 'the old image cannot confirm the repeated send');
      fixture.state.turns['%901'] = [old, {...old, response: 'New image reply', time: '2026-10-30T06:01:00Z'}];
      await page.getByText('New image reply', {exact: true}).waitFor();
      await page.locator('.pending-prompt').waitFor({state: 'detached'});
      assert.equal(fixture.state.sends.length, 1, 'reconciliation must not resend the prompt');
      assert.equal(fixture.state.sends[0].text, '/tmp/synthetic.png');
    });
}
