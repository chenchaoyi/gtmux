import {execFileSync} from 'child_process';
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver} from '../setup/driver';
import {BUNDLE, launchWithFlags, settle, writeDebugFlags} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';

// Real simulator notification response against an isolated fake server. An action
// must send one digit once, even when UIKit delivers both scene and delegate paths.
let fake: Fake;
beforeAll(async () => { fake = await startFake(); });
afterAll(async () => { await fake?.close(); });

it('delivers a cold notification action exactly once', async () => {
  const udid = process.env.GTMUX_E2E_UDID;
  if (!udid || process.env.GTMUX_E2E_ACCEPT_ALERTS !== '1') throw new Error('requires a named simulator and notification alerts');
  const driver = getDriver();
  const seed = JSON.stringify([{url: fake.url, token: fake.token, name: 'Fixture Mac'}]);
  const flags = {GTMUX_DEBUG_SERVERS: seed, GTMUX_DEBUG_SHOT_MODE: '1', GTMUX_DEBUG_LANG: 'en'};
  await launchWithFlags(flags);
  await settle(5000); // register the app's notification categories
  writeDebugFlags(flags);
  await driver.terminateApp(BUNDLE);
  const body = 'Duo scene action fixture';
  const payload = {aps: {alert: {title: 'Fixture Mac', body}, category: 'AGENT_WAITING_2', sound: 'default'}, pane: '%12', server: 'Fixture Mac'};
  const file = join(getArtifactsDir(), 'scene-action-push.json');
  writeFileSync(file, JSON.stringify(payload));
  execFileSync('xcrun', ['simctl', 'push', udid, BUNDLE, file]);
  const banner = driver.$(`-ios predicate string:label CONTAINS "${body}"`);
  await banner.waitForDisplayed({timeout: 15000});
  await banner.longPress({duration: 1500});
  const action = driver.$('-ios predicate string:label == "2"');
  await action.waitForDisplayed({timeout: 10000});
  await screenshot('scene-action-before-tap');
  await action.click();
  await driver.waitUntil(() => fake.world.writesTo('/api/send').length > 0, {timeout: 20000, interval: 500});
  await settle(2000);
  expect(fake.world.writesTo('/api/send')).toEqual([expect.objectContaining({id: '%12', text: '2'})]);
  await screenshot('scene-action-after-tap');
});
