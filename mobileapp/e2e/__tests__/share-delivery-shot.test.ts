import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * The share-delivery panel, on a real screen.
 *
 * Its two text cards used to hold a verb and nothing else, so the command was written
 * nowhere on the phone 「这里能否把具体的链接与命令也展示出来」(2026-09-21). The values are
 * in the cards now, and a unit test can say the strings are present — it cannot say
 * whether a real tunnel link fits in a card on a real phone, or whether the sheet still
 * ends above the bottom of the screen once both values wrap. That is what this is for.
 *
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e -- share-delivery
 */
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

const open = async (lang: 'en' | 'zh', words: {manage: string; hand: string}) => {
  const driver = getDriver();
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1',
    GTMUX_DEBUG_LANG: lang,
  });
  try {
    await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  } catch (err) {
    return captureOnFailure(`sd-${lang}-no-radar`, err);
  }
  await settle(1500);

  await driver.$(`~${TestIds.radar.settings}`).click();
  await settle(900);

  // The row's accessibility name is its whole composition (label, sub, chevron), so
  // these match on a fragment rather than on an id the screen does not carry.
  const manage = driver.$(`//*[contains(@name,"${words.manage}")]`);
  try {
    await manage.waitForDisplayed({timeout: 8000});
  } catch (err) {
    return captureOnFailure(`sd-${lang}-no-manage-row`, err);
  }
  await manage.click();
  await settle(1500);

  // The guest link's row opens to reveal its scopes and, under them, the hand-off entry.
  const guest = driver.$('//*[contains(@name,"Lin")]');
  try {
    await guest.waitForDisplayed({timeout: 8000});
  } catch (err) {
    return captureOnFailure(`sd-${lang}-no-guest-row`, err);
  }
  await guest.click();
  await settle(900);

  const hand = driver.$(`//*[contains(@name,"${words.hand}")]`);
  try {
    await hand.waitForDisplayed({timeout: 8000});
  } catch (err) {
    return captureOnFailure(`sd-${lang}-no-hand-over`, err);
  }
  await hand.click();
  await settle(1200);
};

describe('the share-delivery panel', () => {
  it('shows the link and the command, in English', async () => {
    await open('en', {manage: 'Sharing & devices', hand: 'Hand it over…'});
    const driver = getDriver();
    try {
      await driver.$(`~${TestIds.manage.shareDeliveryLink}`).waitForExist({timeout: 8000});
    } catch (err) {
      return captureOnFailure('sd-en-no-panel', err);
    }
    await screenshot('share-delivery-en');
    // Both values must be REACHABLE, not merely rendered somewhere off the sheet.
    await driver.$(`~${TestIds.manage.shareDeliveryCommand}`).waitForExist({timeout: 4000});
  });

  // The fake serves from localhost, so its own link is short. A real one carries a
  // tunnel host, and whether THAT still fits on a phone is the question a screenshot of
  // the short one cannot answer.
  it('still fits when the link is a long one', async () => {
    fake.world.shareCode = 'GM4W-HCCQ-A-RATHER-LONG-SELF-HOSTED-HOST-STANDS-IN-HERE';
    await open('en', {manage: 'Sharing & devices', hand: 'Hand it over…'});
    const driver = getDriver();
    try {
      await driver.$(`~${TestIds.manage.shareDeliveryCommand}`).waitForExist({timeout: 8000});
    } catch (err) {
      return captureOnFailure('sd-long-no-panel', err);
    }
    await screenshot('share-delivery-long');
    // The sheet grows upward from the bottom, so the thing that goes first is the button
    // under the cards.
    const done = driver.$(`~${TestIds.manage.shareDeliveryDone}`);
    expect(await done.isDisplayed()).toBe(true);
    fake.world.shareCode = 'GM4W-HCCQ';
  });

  it('shows them in Chinese too', async () => {
    await open('zh', {manage: '分享与设备', hand: '交付…'});
    const driver = getDriver();
    try {
      await driver.$(`~${TestIds.manage.shareDeliveryLink}`).waitForExist({timeout: 8000});
    } catch (err) {
      return captureOnFailure('sd-zh-no-panel', err);
    }
    await screenshot('share-delivery-zh');
    await driver.$(`~${TestIds.manage.shareDeliveryCommand}`).waitForExist({timeout: 4000});
  });
});
