const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

for (const width of [390, 1440]) {
  scenario('LRP-001 Live survives conversation and status redraws at ' + width,
    {viewport: {width, height: 900}, locale: 'en-US'}, async (page, fixture) => {
      fixture.state.turns['%901'] = [{prompt: 'Start', response: 'Ready'}];
      const agent = fixture.state.agents[0];
      agent.observed_conversation = {home_id: 'fixture', session_id: 'one', state: 'last-known'};
      fixture.state.screens['%901'] = Array.from({length: 130}, (_, i) => 'Captured line ' + i).join('\n');
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
      const host = page.locator(tile ? '.tile' : '#chat');
      const panel = host.locator('.chat-live'), output = panel.locator('.live-output');
      await panel.locator('summary').click();
      await output.filter({hasText: 'Captured line 129'}).waitFor();
      await panel.getByRole('separator').press('ArrowUp');
      await output.hover();
      await page.mouse.wheel(0, -240);
      await panel.getByRole('button', {name: 'Return to live'}).waitFor();
      const original = await panel.evaluate(el => {
        window.originalLivePanel = el;
        return [el.clientHeight, el.output.scrollTop];
      });
      fixture.state.turns['%901'].push({time: new Date().toISOString(), prompt: 'Transcript refresh', response: 'Recorded reply'});
      await host.getByText('Recorded reply', {exact: true}).waitFor();
      const refreshed = page.waitForResponse(async response =>
        new URL(response.url()).pathname.endsWith('/api/agents') && response.ok() &&
        (await response.json()).some(row => row.observed_conversation?.state === 'recovered'));
      agent.observed_conversation.state = 'recovered';
      agent.since++;
      await refreshed;
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      assert.deepEqual(await panel.evaluate(el => [el === window.originalLivePanel, el.open, el.clientHeight, el.output.scrollTop]), [true, true, ...original]);
      agent.status = 'waiting';
      await panel.locator('summary strong').filter({hasText: 'needs attention'}).waitFor();
      assert.deepEqual(await panel.evaluate(el => [el === window.originalLivePanel, el.open, el.clientHeight, el.output.scrollTop]), [true, true, ...original]);
      await panel.getByRole('button', {name: 'Return to live'}).waitFor();
      if (process.env.GTMUX_WEB_EVIDENCE) await page.screenshot({path: process.env.GTMUX_WEB_EVIDENCE + '/live-refresh-' + width + '.png'});
      agent.status = 'idle';
      await panel.locator('summary strong').filter({hasText: 'idle'}).waitFor();
      await panel.locator('summary').click();
      await panel.waitFor({state: 'detached'});
      agent.status = 'working';
      await panel.waitFor();
      assert.equal(await panel.evaluate(el => el.open), false);
      await panel.locator('summary').click();
      agent.observed_conversation.session_id = 'two';
      await page.waitForFunction(selector => document.querySelector(selector)?.open === false,
        (tile ? '.tile' : '#chat') + ' .chat-live');
      assert.equal(fixture.state.sends.length, 0);
    });
}
