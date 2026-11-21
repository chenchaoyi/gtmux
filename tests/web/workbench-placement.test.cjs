const assert = require('node:assert/strict');
const {scenario} = require('./harness.cjs');

function tile(page, title) {
  return page.locator('.tile').filter({has: page.locator('.tile-name').filter({hasText: title})});
}

function geometry(panel) {
  return panel.evaluate(node => ({
    x: node.offsetLeft, y: node.offsetTop, w: node.offsetWidth, h: node.offsetHeight,
  }));
}

for (const [width, height] of [[1280, 900], [1440, 900], [1024, 1080]]) {
  scenario('WWP001 Opens two actionable workbench tiles at ' + width,
    {viewport: {width, height}, locale: 'en-US'}, async (page, fixture) => {
      fixture.state.turns['%901'] = [{prompt: 'Alpha question', response: 'Alpha history'}];
      fixture.state.turns['%902'] = [{prompt: 'Beta question', response: 'Beta history'}];
      await page.goto(fixture.url + '/#code=TEST-1234');
      const rail = page.locator('#tree');
      await rail.getByText('Investigate retries', {exact: true}).dblclick();
      const alpha = tile(page, 'Investigate retries');
      const initial = await geometry(alpha);
      await rail.getByText('Review notes', {exact: true}).dblclick();
      const beta = tile(page, 'Review notes');
      const next = await geometry(beta);
      assert.deepEqual(await geometry(alpha), initial);
      const separate = width === 1024 ? next.y >= initial.y + initial.h : next.x >= initial.x + initial.w;
      assert.ok(separate, 'new tile covers the earlier panel');
      const board = await page.locator('#board').evaluate(node => ({
        w: node.clientWidth, h: node.clientHeight,
      }));
      assert.ok(next.x + next.w <= board.w && next.y + next.h <= board.h, 'new tile extends past the workbench');

      await alpha.getByRole('button', {name: 'Chat', exact: true}).click();
      await beta.getByRole('button', {name: 'Chat', exact: true}).click();
      await alpha.getByText('Alpha history', {exact: true}).waitFor();
      await beta.getByText('Beta history', {exact: true}).waitFor();
      const input = panel => panel.getByRole('textbox', {name: 'Message or command'});
      await input(alpha).fill('Synthetic Alpha');
      await input(beta).fill('Synthetic Beta draft');
      await alpha.getByRole('button', {name: 'Terminal', exact: true}).click();
      await alpha.getByRole('button', {name: 'Chat', exact: true}).click();
      assert.equal(await input(alpha).inputValue(), 'Synthetic Alpha');
      const [receipt] = await Promise.all([
        page.waitForResponse(response => new URL(response.url()).pathname.endsWith('/api/send')),
        alpha.getByRole('button', {name: 'Send', exact: true}).click(),
      ]);
      assert.equal(receipt.status(), 200);
      assert.equal(await input(beta).inputValue(), 'Synthetic Beta draft');
      assert.equal(fixture.state.sends.length, 1);
      assert.equal(fixture.state.sends[0].id, '%901');
      assert.match(JSON.stringify(fixture.state.sends[0]), /Synthetic Alpha/);
    });
}

scenario('WWP002 Preserves saved freeform geometry while placing a new tile',
  {viewport: {width: 1440, height: 900}, locale: 'en-US'}, async (page, fixture) => {
    const first = {x: 18, y: 20, w: 460, h: 430};
    const second = {x: 36, y: 55, w: 420, h: 360};
    await page.addInitScript(tiles => {
      localStorage.setItem('gtmux.board', JSON.stringify({tiles}));
    }, [{id: '%901', ...first}, {id: '%902', ...second}]);
    await page.goto(fixture.url + '/#code=TEST-1234');
    const alpha = tile(page, 'Investigate retries');
    const beta = tile(page, 'Review notes');
    assert.deepEqual(await geometry(alpha), first);
    assert.deepEqual(await geometry(beta), second);

    await page.locator('#tree').getByText('Gtmux HQ', {exact: true}).dblclick();
    assert.ok((await geometry(tile(page, 'Gtmux HQ'))).x >= first.x + first.w);
    assert.deepEqual(await geometry(alpha), first);
    assert.deepEqual(await geometry(beta), second);
    assert.equal(fixture.state.sends.length, 0);
  });
