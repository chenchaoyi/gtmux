import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags} from '../setup/app';
import {startFake, Fake} from '../fake-serve/server';

let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
  await fake.world.seedDemo('en');
});
afterAll(async () => { await fake?.close(); });

it('opens a knowledge search result on the first tap while the keyboard is visible', async () => {
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_RESET_SERVERS: '1',
    GTMUX_DEBUG_NO_PUSH: '1',
    GTMUX_DEBUG_LANG: 'en',
  });
  const hq = driver.$('-ios predicate string:name == "radar-hq-disc" OR name == "radar-hq-card"');
  await hq.waitForDisplayed({timeout: 25_000});
  await hq.click();
  const knowledge = driver.$('~hq-knowledge-open');
  await knowledge.waitForDisplayed({timeout: 10_000});
  await knowledge.click();
  await driver.$('~knowledge-find').setValue('wireless');
  const result = driver.$('~knowledge-entry-k4');
  await result.waitForDisplayed({timeout: 6000});
  expect(await driver.$('XCUIElementTypeKeyboard').isExisting()).toBe(true);
  await screenshot('knowledge-search-keyboard');
  // One tap must open the entry; the default ScrollView behaviour only dismissed
  // the keyboard and silently swallowed this press.
  await result.click();
  await driver.$('~knowledge-use-summary').waitForExist({timeout: 6000});
  await screenshot('knowledge-search-first-tap');
  const back = driver.$('~knowledge-back');
  if (await back.isExisting()) {
    await back.click();
    expect(await driver.$('~knowledge-find').getValue()).toBe('wireless');
  }
});
