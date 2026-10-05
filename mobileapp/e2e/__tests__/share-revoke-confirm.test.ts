import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {answerAlert, launchWithFlags, settle} from '../setup/app';
import {alertMode} from '../setup/capabilities';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// Revoking a share link asks first, and only the confirmed answer revokes. Under the
// harness's default alert handling the confirmation is cancelled about a second after it
// appears, so this runs only when the session leaves alerts to the test
// (GTMUX_E2E_ALERTS=manual) and answers both buttons itself.
const run = alertMode() === 'manual' ? it : it.skip;

let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

async function openLink(): Promise<void> {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(1500);
  await driver.$(`~${TestIds.radar.settings}`).click();
  await driver.$('-ios predicate string:label BEGINSWITH "Sharing & pairing"').click();
  await driver.$('-ios predicate string:label BEGINSWITH "Lin" AND visible == 1').waitForDisplayed({timeout: 10_000});
  await driver.$('-ios predicate string:label BEGINSWITH "Lin" AND visible == 1').click();
  await settle(1500);
}

run('Revoke asks first: Cancel revokes nothing, Revoke revokes that link', async () => {
  const driver = getDriver();
  await openLink();
  const revoke = driver.$('-ios predicate string:label == "Revoke"');

  await revoke.click();
  expect(await answerAlert('Cancel')).toBe('Lin');
  await settle(1000);
  expect(fake.world.writesTo('/api/devices/revoke')).toEqual([]);

  await revoke.click();
  await driver.$('-ios class chain:**/XCUIElementTypeAlert').waitForExist({timeout: 5000});
  await screenshot('share-revoke-confirm');
  // Still up well after the default handling would have cancelled it.
  await settle(2500);
  expect(await driver.$('-ios class chain:**/XCUIElementTypeAlert').isExisting()).toBe(true);
  await answerAlert('Revoke');
  await settle(1500);
  expect(fake.world.writesTo('/api/devices/revoke')).toEqual([{id: 'g1'}]);
});
