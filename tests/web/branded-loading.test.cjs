const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const {scenario} = require('./harness.cjs');

async function openChat(page, fixture, width) {
  await page.goto(fixture.url + '/#code=TEST-1234');
  if (width < 800) {
    await page.locator('#radar').getByText('Investigate retries', {exact: true}).click();
    await page.locator('#mode [data-mode="chat"]').click();
  } else {
    await page.locator('#tree').getByText('Investigate retries', {exact: true}).dblclick();
    await page.locator('.tile-modes [data-m="chat"]').click();
  }
}

async function seek(loader, milliseconds) {
  return loader.evaluate((element, time) => {
    const animations = element.getAnimations({subtree: true}).filter(a => a.animationName === 'brandload-focus');
    const running = animations.map(a => a.playState);
    for (const animation of animations) { animation.pause(); animation.currentTime = time; }
    const mark = element.querySelector('.brandload-mark');
    const rect = mark.getBoundingClientRect();
    return {running, duration: animations.map(a => a.effect.getTiming().duration),
      panes: [...mark.children].map(pane => {
        const bounds = pane.getBoundingClientRect();
        return {x: (bounds.x - rect.x) / rect.width, y: (bounds.y - rect.y) / rect.height,
          width: bounds.width / rect.width, height: bounds.height / rect.height,
          radius: parseFloat(getComputedStyle(pane).borderRadius) / rect.width,
          opacity: Number(getComputedStyle(pane, '::after').opacity),
          color: getComputedStyle(pane, '::after').backgroundColor};
      })};
  }, milliseconds);
}

async function screenshot(page, name) {
  const directory = process.env.GTMUX_WEB_RESULTS || path.join(__dirname, 'results');
  await fs.mkdir(directory, {recursive: true});
  await page.screenshot({path: path.join(directory, 'brand-loading-' + name + '.png')});
}

for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
  scenario('WBL-001 moving brand focus ' + width + ' ' + theme,
    {viewport: {width, height: 950}, locale: 'en-US', reducedMotion: 'no-preference'}, async (page, fixture) => {
      let release;
      const held = new Promise(resolve => { release = resolve; });
      await page.route('**/api/transcript?*', async route => {
        await held;
        await route.continue().catch(() => {});
      });
      await page.addInitScript(value => localStorage.setItem('gtmux.appearance', value), theme);
      try {
        await openChat(page, fixture, width);
        const loader = page.locator('.brandload:visible').last();
        await loader.waitFor();
        assert.equal(await page.locator('html').getAttribute('data-theme'), theme);
        assert.equal(await loader.locator('.brandload-mark').getAttribute('aria-hidden'), 'true');
        const geometry = [[.08, .08, .4, .4], [.52, .08, .4, .4], [.08, .52, .84, .4]];
        for (const [time, active] of [[0, 1], [600, 2], [1200, 0]]) {
          const frame = await seek(loader, time);
          assert.equal(frame.panes.length, 3);
          assert.deepEqual(frame.duration, [1800, 1800, 1800]);
          if (time === 0) assert.deepEqual(frame.running, ['running', 'running', 'running']);
          frame.panes.forEach((pane, index) => {
            for (const [key, expected] of Object.entries({x: geometry[index][0], y: geometry[index][1],
              width: geometry[index][2], height: geometry[index][3], radius: .09})) {
              assert.ok(Math.abs(pane[key] - expected) < .002, key + ' changed');
            }
            assert.ok(Math.abs(pane.opacity - (index === active ? 1 : 0)) < .01);
            assert.equal(pane.color, 'rgb(6, 182, 212)');
          });
          await screenshot(page, width + '-' + theme + '-' + time);
        }
        release();
        await loader.waitFor({state: 'detached'});
        await page.getByText('Initial finding', {exact: true}).waitFor();
        assert.equal(fixture.state.sends.length, 0);
      } finally { release(); }
    });
}

scenario('WBL-002 live reduced motion preserves one loading status',
  {viewport: {width: 390, height: 950}, locale: 'en-US', reducedMotion: 'no-preference'}, async (page, fixture) => {
    let release;
    const held = new Promise(resolve => { release = resolve; });
    await page.route('**/api/transcript?*', async route => { await held; await route.continue().catch(() => {}); });
    try {
      await openChat(page, fixture, 390);
      const loader = page.locator('#chat-load');
      await loader.waitFor();
      assert.equal(await loader.getAttribute('role'), 'status');
      assert.equal(await loader.locator('.brandload-label').count(), 1);
      assert.equal(await loader.locator('.brandload-mark').getAttribute('aria-hidden'), 'true');
      for (const reducedMotion of ['reduce', 'no-preference', 'reduce']) {
        await page.emulateMedia({reducedMotion});
        await page.waitForFunction(reduced => {
          const loader = document.querySelector('#chat-load');
          const animations = loader.getAnimations({subtree: true}).filter(a => a.animationName === 'brandload-focus');
          return reduced ? animations.length === 0 : animations.length === 3;
        }, reducedMotion === 'reduce');
        if (reducedMotion === 'reduce') {
          const frame = await seek(loader, 0);
          assert.deepEqual(frame.panes.map(p => p.opacity), [0, 1, 0]);
        }
      }
      await screenshot(page, '390-reduced');
      release();
      await loader.waitFor({state: 'detached'});
      assert.equal(await page.evaluate(() => document.getAnimations().filter(a => a.animationName === 'brandload-focus').length), 0);
      assert.equal(fixture.state.sends.length, 0);
    } finally { release(); }
  });

scenario('WBL-003 retired Chat read cannot restore a loader in Terminal',
  {viewport: {width: 390, height: 950}, locale: 'en-US'}, async (page, fixture) => {
    let release;
    const held = new Promise(resolve => { release = resolve; });
    let complete;
    const settled = new Promise(resolve => { complete = resolve; });
    await page.route('**/api/transcript?*', async route => {
      await held;
      await route.fulfill({status: 200, contentType: 'application/json',
        body: JSON.stringify([{prompt: 'retired synthetic', response: 'late retired response'}])}).catch(() => {});
      complete();
    });
    try {
      await openChat(page, fixture, 390);
      await page.locator('#chat-load').waitFor();
      await page.locator('#mode [data-mode="term"]').click();
      await page.locator('#pane-load').waitFor({state: 'detached'});
      release();
      await page.waitForFunction(() => !document.querySelector('#pane-load'));
      await settled;
      assert.equal(await page.locator('#mode [data-mode="term"]').getAttribute('class'), 'on');
      assert.equal(await page.locator('.brandload:visible').count(), 0);
      assert.equal(await page.getByText('late retired response', {exact: true}).count(), 0);
      assert.equal(fixture.state.sends.length, 0);
    } finally { release(); }
  });
