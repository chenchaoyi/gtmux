const assert = require('node:assert/strict');
const { scenario } = require('./harness.cjs');
scenario(
  'shared browser first case changes its own context and server',
  {},
  async (page, fixture) => {
    await page.goto(fixture.url);
    await page.evaluate(() =>
      localStorage.setItem('isolation-canary', 'first-case'),
    );
    await page
      .context()
      .addCookies([
        {
          name: 'port-independent-canary',
          value: 'first-case',
          domain: '127.0.0.1',
          path: '/',
        },
      ]);
    fixture.state.sends.push({ text: 'only for the first server' });
  },
);
scenario(
  'shared browser next case has fresh storage and fixture',
  {},
  async (page, fixture) => {
    await page.goto(fixture.url);
    assert.equal(
      await page.evaluate(() => localStorage.getItem('isolation-canary')),
      null,
    );
    assert.deepEqual(await page.context().cookies(fixture.url), []);
    assert.deepEqual(fixture.state.sends, []);
  },
);
