import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';

let fake: Fake;
beforeAll(async () => { fake = await startFake(); });
afterAll(async () => { await fake?.close(); });

it('keeps the iPad sidebar and selected session through rotation', async () => {
  const driver = getDriver();
  const original = await driver.getOrientation();
  try {
    await driver.setOrientation('LANDSCAPE');
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
    const split = driver.$(`~${TestIds.radar.split}`);
    await split.waitForDisplayed({timeout: 25000});
    const row = driver.$('~agent-row-%13');
    await row.waitForDisplayed({timeout: 10000});
    await row.click();
    const identity = driver.$('-ios predicate string:label == "Show full title, 整理这周的发布清单"');
    await identity.waitForDisplayed({timeout: 10000});
    const chat = driver.$(`~${TestIds.detail.modeChat}`);
    await chat.waitForDisplayed({timeout: 10000});
    await chat.click();
    await screenshot('ipad-adaptive-landscape');
    await driver.setOrientation('PORTRAIT');
    await settle(1500);
    expect(await split.isDisplayed()).toBe(true);
    expect(await driver.$(`~${TestIds.detail.modeChat}`).isExisting()).toBe(true);
    expect(await identity.isDisplayed()).toBe(true);
    expect(await driver.$(`~${TestIds.detail.back}`).isExisting()).toBe(false);
    await screenshot('ipad-adaptive-portrait');
  } finally {
    await driver.setOrientation(original);
  }
});
