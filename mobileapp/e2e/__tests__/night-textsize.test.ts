// Night exploration: where text size starts to break fixed-size chrome. Radar and a waiting
// pane's detail at each content size from the default up to the first accessibility size.
import {execFileSync} from 'child_process';
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver, getArtifactsDir} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
let fake: Fake;
beforeAll(async () => { fake = await startFake(); });
afterAll(async () => {
  try { execFileSync('xcrun', ['simctl', 'ui', UDID, 'content_size', 'large']); } catch { /* best effort */ }
  await fake?.close();
});

const SIZES = ['large', 'extra-large', 'extra-extra-large', 'extra-extra-extra-large', 'accessibility-medium'];

it('radar and a waiting detail at each text size', async () => {
  const driver = getDriver();
  for (const size of SIZES) {
    execFileSync('xcrun', ['simctl', 'ui', UDID, 'content_size', size]);
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
    await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    await settle(2500);
    await screenshot(`ts-${size}-radar`);
    await driver.$('~agent-row-%11').click();
    await settle(3000);
    await screenshot(`ts-${size}-waiting`);
    writeFileSync(join(getArtifactsDir(), `ts-${size}-waiting.xml`), await driver.getPageSource(), 'utf8');
  }
});
