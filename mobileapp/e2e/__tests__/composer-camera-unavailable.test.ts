import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// Attach › Camera on a device with no camera — which every simulator is. Measured on
// 2026-10-05: the card closed and nothing else happened, because the picker reports a
// missing camera (and a refused permission) as a result the composer never read. The
// composer now says why on its error line.
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

it('Camera with no camera says so instead of doing nothing', async () => {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2000);
  await driver.$('~agent-row-%12').click();
  await settle(2500);
  await driver.$(`~${TestIds.composer.keyboard}`).click();
  await driver.$(`~${TestIds.composer.input}`).waitForDisplayed({timeout: 8000});
  await driver.$(`~${TestIds.composer.attach}`).click();
  await settle(1500);
  await driver.$('~attach-1').click();
  await settle(4000);
  await screenshot('camera-unavailable');
  expect(await driver.$('-ios predicate string:label CONTAINS "No camera on this device" AND visible == 1').isExisting()).toBe(true);
  // Nothing was sent or uploaded on the way.
  expect(fake.world.writesTo('/api/upload')).toHaveLength(0);
});
