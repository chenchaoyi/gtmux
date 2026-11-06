import {execFileSync} from 'child_process';
import {existsSync, readFileSync, writeFileSync} from 'fs';
import {join} from 'path';
import {getArtifactsDir, getDriver} from '../setup/driver';
import {BUNDLE, launchWithFlags, readDebugLog, writeDebugFlags} from '../setup/app';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {selectUnfoldedInnerDisplay} from '../setup/duoDisplay';
import {startFake, Fake} from '../fake-serve/server';

let fake: Fake;
beforeAll(async () => { fake = await startFake(); fake.world.seedUsage(); });
beforeEach(async () => { await selectUnfoldedInnerDisplay(); }, 60000);
afterAll(async () => { await fake?.close(); });
afterEach(() => {
  const dir = getArtifactsDir();
  const label = expect.getState().currentTestName?.startsWith('opens a cold') ? 'cold' : 'warm';
  writeFileSync(join(dir, `usage-link-${label}-js.json`), JSON.stringify(readDebugLog(), null, 2));
  try {
    const container = execFileSync('xcrun', ['simctl', 'get_app_container', process.env.GTMUX_E2E_UDID!, BUNDLE, 'data'], {encoding: 'utf8'}).trim();
    const trace = join(container, 'Documents', 'gtmux-native-notification-trace.jsonl');
    if (existsSync(trace)) writeFileSync(join(dir, `usage-link-${label}-native.jsonl`), readFileSync(trace));
  } catch (error) {
    writeFileSync(join(dir, `usage-link-${label}-native-error.txt`), String(error));
  }
});

function fixture() {
  const udid = process.env.GTMUX_E2E_UDID;
  if (!udid) throw new Error('requires a named simulator');
  const seed = JSON.stringify([{url: fake.url, token: fake.token, name: 'Fixture Mac'}]);
  const flags = {GTMUX_DEBUG_SERVERS: seed, GTMUX_DEBUG_SHOT_MODE: '1', GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en', GTMUX_DEBUG_LOG_NET: '1'};
  return {flags};
}

async function waitForUsageWithSystemConfirmation(timeout: number) {
  const driver = getDriver();
  const close = driver.$('~hq-usage-close');
  await driver.waitUntil(async () => {
    if (await close.isDisplayed().catch(() => false)) return true;
    const alert = await driver.getAlertText().catch(() => '');
    if (/^Open in [“"]gtmux[”"]\?$/.test(alert.trim())) await driver.acceptAlert();
    return false;
  }, {timeout, interval: 500, timeoutMsg: 'usage sheet did not become visible'});
}

// Native delivery and JS presentation are distinct; retain bounded checks for both.
function nativeSceneTrace() {
  const container = execFileSync('xcrun', ['simctl', 'get_app_container', process.env.GTMUX_E2E_UDID!, BUNDLE, 'data'], {encoding: 'utf8'}).trim();
  return join(container, 'Documents', 'gtmux-native-notification-trace.jsonl');
}

function sceneURLCount(path: string, event = 'scene-url'): number {
  if (!existsSync(path)) return 0;
  return readFileSync(path, 'utf8').split('\n').filter(line => {
    try { return JSON.parse(line).event === event; } catch { return false; }
  }).length;
}

async function waitForSceneDelivery(path: string, previous: number) {
  await getDriver().waitUntil(() => sceneURLCount(path) > previous,
    {timeout: 20000, interval: 500, timeoutMsg: 'native scene did not receive the warm URL'});
}

it('opens a warm usage link through the scene', async () => {
  const {flags} = fixture();
  const driver = getDriver();
  await launchWithFlags(flags);
  await driver.$('~radar-split').waitForExist({timeout: 25000});
  // Wait for the bridge and live server before UIKit delivers the warm link.
  await driver.$('-ios predicate string:label BEGINSWITH "Connection: connected"').waitForExist({timeout: 25000});
  const trace = nativeSceneTrace();
  const previous = sceneURLCount(trace);
  const timing: {opened: number; delivered?: number; visible?: number} = {opened: Date.now()};
  try {
    // XCTest opens the real URL in the specified app. Cold coverage below
    // omits bundleId to exercise the default system URL association.
    await driver.execute('mobile: deepLink', {url: 'gtmux://usage', bundleId: BUNDLE});
    await waitForSceneDelivery(trace, previous);
    timing.delivered = Date.now();
    await waitForUsageWithSystemConfirmation(15000);
    timing.visible = Date.now();
    await screenshot('scene-warm-usage-link');
  } catch (error) {
    return captureOnFailure('scene-warm-usage-link-failed', error);
  } finally {
    writeFileSync(join(getArtifactsDir(), 'usage-link-warm-timing.json'), JSON.stringify(timing, null, 2));
  }
});

it('opens a cold usage link through the scene', async () => {
  const {flags} = fixture();
  const driver = getDriver();
  // Install startup flags in the real app container before cold scene creation.
  await launchWithFlags(flags);
  await driver.$('~radar-split').waitForExist({timeout: 25000});
  writeDebugFlags(flags);
  await driver.terminateApp(BUNDLE);
  expect(await driver.queryAppState(BUNDLE)).toBe(1); // NotRunning
  const trace = nativeSceneTrace();
  const previous = sceneURLCount(trace, 'scene-url-cold');
  const timing: {opened: number; commandReturned?: number; visible?: number} = {opened: Date.now()};
  try {
    // No bundle override: XCTest must resolve the registered URL handler and
    // launch its scene. A pre-existing warm scene cannot satisfy this case.
    await driver.execute('mobile: deepLink', {url: 'gtmux://usage'});
    timing.commandReturned = Date.now();
    await waitForUsageWithSystemConfirmation(20000);
    expect(sceneURLCount(trace, 'scene-url-cold')).toBeGreaterThan(previous);
    timing.visible = Date.now();
    await screenshot('scene-cold-usage-link');
  } catch (error) {
    return captureOnFailure('scene-cold-usage-link-failed', error);
  } finally {
    writeFileSync(join(getArtifactsDir(), 'usage-link-cold-timing.json'), JSON.stringify(timing, null, 2));
  }
});
