import {execFileSync} from 'child_process';
import {getDriver} from '../setup/driver';
import {BUNDLE, launchWithFlags, settle, writeDebugFlags} from '../setup/app';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';

let fake: Fake;
beforeAll(async () => { fake = await startFake(); fake.world.seedUsage(); });
afterAll(async () => { await fake?.close(); });

it('opens the usage link on warm and cold scene entry', async () => {
  const udid = process.env.GTMUX_E2E_UDID;
  if (!udid) throw new Error('requires a named simulator');
  const driver = getDriver();
  const seed = JSON.stringify([{url: fake.url, token: fake.token, name: 'Fixture Mac'}]);
  const flags = {GTMUX_DEBUG_SERVERS: seed, GTMUX_DEBUG_SHOT_MODE: '1', GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'};
  await launchWithFlags(flags);
  await driver.$('~radar-split').waitForExist({timeout: 25000});
  // Wait for the live server before testing a warm URL so the bridge and its
  // subscription have mounted. The link itself is still delivered by UIKit.
  await driver.$('-ios predicate string:label BEGINSWITH "Connection: connected"').waitForExist({timeout: 25000});
  const open = () => execFileSync('xcrun', ['simctl', 'openurl', udid, 'gtmux://usage']);
  open();
  const close = driver.$('~hq-usage-close');
  try {
    await close.waitForDisplayed({timeout: 15000});
  } catch (error) {
    return captureOnFailure('scene-warm-usage-link-failed', error);
  }
  await screenshot('scene-warm-usage-link');
  await close.click();
  await settle(500);
  writeDebugFlags(flags);
  await driver.terminateApp(BUNDLE);
  open();
  await driver.$('~hq-usage-close').waitForDisplayed({timeout: 20000});
  await screenshot('scene-cold-usage-link');
});
