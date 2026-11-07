// A push from one Mac that cold-starts the app is handled once (#1308). Switching to
// another Mac afterwards must not bounce back to the Mac the push came from — before
// #1308 every switch did, and so did a return from the background — and a later,
// genuinely new tap is still handled.
//
// Needs notification permission, so it runs only with GTMUX_E2E_ACCEPT_ALERTS=1 (the
// session accepts the system prompt) and a named simulator (GTMUX_E2E_UDID). Pushes are
// delivered with `xcrun simctl push`, which iOS presents like a remote notification; the
// banner is tapped on the home screen.
// The case where no Mac is active at the cold start is left out: what that tap should do
// once a Mac is picked is still an open design question.
import {execFileSync} from 'child_process';
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver, getArtifactsDir, recordSceneStep} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {BUNDLE, launchWithFlags, readDebugLog, settle, waitForNotificationAuthorization, writeDebugFlags} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

const UDID = process.env.GTMUX_E2E_UDID;

let home: Fake;
let office: Fake;
beforeAll(async () => {
  home = await startFake();
  office = await startFake();
  for (const a of home.world.agents) {
    a.session = `home-${a.session}`;
    a.task = `home-${a.task || a.pane_id}`;
  }
  for (const a of office.world.agents) {
    a.session = `office-${a.session}`;
    a.task = `office-${a.task || a.pane_id}`;
  }
});
afterAll(async () => {
  await home?.close();
  await office?.close();
});

async function dump(name: string) {
  await screenshot(name);
  writeFileSync(join(getArtifactsDir(), `${name}.xml`), await getDriver().getPageSource(), 'utf8');
}

/** Where the app is: the server chip's label, which fleet is on screen, Detail or not. */
type Where = {chip: string; radar: boolean; detail: boolean; servers: boolean; home: number; office: number};
async function where(): Promise<Where> {
  const driver = getDriver();
  const src = await driver.getPageSource();
  const chip = /name="radar-server-chip"[^>]*label="([^"]*)"/.exec(src)?.[1] ?? /label="([^"]*)"[^>]*name="radar-server-chip"/.exec(src)?.[1] ?? '-';
  const detail = src.includes(`name="${TestIds.detail.screen}"`);
  const radar = src.includes(`name="${TestIds.radar.screen}"`);
  const servers = src.includes(`name="${TestIds.servers.screen}"`);
  const h = (src.match(/home-/g) ?? []).length;
  const o = (src.match(/office-/g) ?? []).length;
  return {chip, radar, detail, servers, home: h, office: o};
}

function push(server: string, pane: string, body: string) {
  const payload = {
    aps: {alert: {title: `${pane} waiting`, subtitle: server, body}, sound: 'default'},
    pane,
    server,
  };
  const f = join(getArtifactsDir(), `push-${body}.json`);
  writeFileSync(f, JSON.stringify(payload), 'utf8');
  execFileSync('xcrun', ['simctl', 'push', UDID!, BUNDLE, f], {timeout: 15000, maxBuffer: 16384, killSignal: 'SIGKILL'});
}

/** Tap a delivered banner by its body text (home screen showing). */
async function tapBanner(body: string): Promise<boolean> {
  const driver = getDriver();
  const banner = driver.$(`-ios predicate string:label CONTAINS "${body}"`);
  try {
    await banner.waitForExist({timeout: 8000});
    await banner.click();
    return true;
  } catch {
    try {
      await dump(`c-banner-${body}`);
    } catch (error) {
      console.error(`[e2e] failed to capture missing notification banner: ${error}`);
    }
    return false;
  }
}

async function toServer(name: string) {
  const driver = getDriver();
  const back = driver.$(`~${TestIds.detail.back}`);
  if (await back.isDisplayed().catch(() => false)) {
    await back.click();
    await settle(1200);
  }
  const chip = driver.$(`~${TestIds.radar.serverChip}`);
  if (await chip.isDisplayed().catch(() => false)) {
    await chip.click();
    await settle(1500);
  }
  const row = driver.$(`-ios predicate string:label BEGINSWITH "${name}" AND visible == 1`);
  await row.click();
  await settle(6000);
}

const SEED = () =>
  JSON.stringify([
    {url: home.url, token: home.token, name: 'Home'},
    {url: office.url, token: office.token, name: 'Office'},
  ]);

// Pushes go to one named simulator only: with no GTMUX_E2E_UDID, 'booted' could be any of
// several running ones, someone else's included.
if (process.env.GTMUX_SCENE_EVIDENCE_ROOT && (process.env.GTMUX_E2E_ACCEPT_ALERTS !== '1' || !UDID || UDID === 'booted')) {
  throw new Error('Migration notification cases require explicit UDID and alert acceptance');
}
const run = process.env.GTMUX_E2E_ACCEPT_ALERTS === '1' && UDID ? describe : describe.skip;

run('a cold-start push is handled once', () => {
  const log: string[] = [];
  const note = async (step: string): Promise<Where> => {
    const w = await where();
    log.push(`${step}: ${JSON.stringify(w)}`);
    await dump(`c-${step}`);
    writeFileSync(join(getArtifactsDir(), `debug-${step}.json`), JSON.stringify(readDebugLog(), null, 2));
    writeFileSync(join(getArtifactsDir(), `requests-${step}.json`), JSON.stringify({home: home.requests, office: office.requests}, null, 2));
    return w;
  };
  // On a Mac's radar, showing that Mac's sessions and none of the other's.
  const onRadar = (w: Where, mac: 'Home' | 'Office') => {
    expect(w.radar).toBe(true);
    expect(w.detail).toBe(false);
    expect(w.chip).toContain(mac);
    expect(mac === 'Home' ? w.office : w.home).toBe(0);
    expect(mac === 'Home' ? w.home : w.office).toBeGreaterThan(0);
  };
  // In a session's Detail, opened from a push on that Mac.
  const inDetail = (w: Where, mac: 'Home' | 'Office') => {
    expect(w.detail).toBe(true);
    expect(mac === 'Home' ? w.office : w.home).toBe(0);
    expect(mac === 'Home' ? w.home : w.office).toBeGreaterThan(0);
  };
  afterAll(() => writeFileSync(join(getArtifactsDir(), 'cold-summary.txt'), log.join('\n') + '\n', 'utf8'));

  it('A: cold start with an active Mac, switches, background, a new warm tap', async () => {
    const driver = getDriver();
    // Seed two Macs and connect Home: the first workspace asks for push permission (accepted).
    // Set up "a saved, active Mac at launch" deterministically: both Macs are
    // seeded and Home is active (SHOT_MODE's only effect here). The ad-hoc main
    // and candidate comparison uses these same flags without changing Keychain.
    const active = {GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_SERVERS: SEED(), GTMUX_DEBUG_SHOT_MODE: '1'};
    await launchWithFlags(active);
    await settle(5000);
    onRadar(await note('A0-home-connected'), 'Home');
    recordSceneStep('A0-home-connected');
    await waitForNotificationAuthorization();
    writeDebugFlags(active);

    await driver.terminateApp(BUNDLE);
    await settle(1500);
    push('Home', '%12', 'cold-A');
    expect(await tapBanner('cold-A')).toBe(true);
    await settle(7000);
    try {
      await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 30000});
    } catch (error) {
      await note('A1-cold-not-ready');
      throw error;
    }
    inDetail(await note('A1-after-cold-tap'), 'Home');
    recordSceneStep('A1-after-cold-tap');

    await toServer('Office');
    onRadar(await note('A2-switch-office'), 'Office');
    recordSceneStep('A2-switch-office');
    await toServer('Home');
    onRadar(await note('A3-switch-home'), 'Home');
    recordSceneStep('A3-switch-home');
    await toServer('Office');
    onRadar(await note('A4-switch-office-again'), 'Office');
    recordSceneStep('A4-switch-office-again');

    await driver.execute('mobile: backgroundApp', {seconds: 3});
    await settle(4000);
    onRadar(await note('A5-after-background'), 'Office');
    recordSceneStep('A5-after-background');

    await driver.execute('mobile: pressButton', {name: 'home'});
    await settle(1500);
    const backgroundStates: number[] = [];
    try {
      await driver.waitUntil(async () => {
        const state = await driver.queryAppState(BUNDLE);
        if (backgroundStates.at(-1) !== state) backgroundStates.push(state);
        return state !== 4;
      }, {timeout: 15000, interval: 250, timeoutMsg: 'app did not leave the foreground before the warm push'});
    } finally {
      writeFileSync(join(getArtifactsDir(), 'c-warm-app-states.json'), JSON.stringify(backgroundStates));
    }
    push('Office', '%13', 'warm-A');
    expect(await tapBanner('warm-A')).toBe(true);
    await settle(7000);
    inDetail(await note('A6-after-warm-tap-office13'), 'Office');
    recordSceneStep('A6-after-warm-tap-office13');
    await toServer('Home');
    onRadar(await note('A7-switch-home-after-warm'), 'Home');
    recordSceneStep('A7-switch-home-after-warm');
  }, 420000); // The hosted journey includes cold launch, four switches and a warm tap.

  it('C: a launch with no notification switches where it is pointed', async () => {
    await launchWithFlags({GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_SERVERS: SEED()});
    await settle(3000);
    await toServer('Home');
    onRadar(await note('C1-home'), 'Home');
    recordSceneStep('C1-home');
    await toServer('Office');
    onRadar(await note('C2-office'), 'Office');
    recordSceneStep('C2-office');
  });
});
