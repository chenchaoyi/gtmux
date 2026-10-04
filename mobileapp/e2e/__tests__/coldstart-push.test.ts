// A push from one Mac that cold-starts the app is handled once (#1308). Switching to
// another Mac afterwards must not bounce back to the Mac the push came from — before
// #1308 every switch did, and so did a return from the background — and a later,
// genuinely new tap is still handled.
//
// Needs notification permission, so it runs only with GTMUX_E2E_ACCEPT_ALERTS=1 (the
// session accepts the system prompt). Pushes are delivered with `xcrun simctl push`,
// which iOS presents like a remote notification; the banner is tapped on the home screen.
// The case where no Mac is active at the cold start is left out: what that tap should do
// once a Mac is picked is still an open design question.
import {execFileSync} from 'child_process';
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver, getArtifactsDir} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {BUNDLE, launchWithFlags, settle, writeDebugFlags} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

const UDID = process.env.GTMUX_E2E_UDID || 'booted';

let home: Fake;
let office: Fake;
beforeAll(async () => {
  home = await startFake();
  office = await startFake();
  for (const a of home.world.agents) a.session = `home-${a.session}`;
  for (const a of office.world.agents) a.session = `office-${a.session}`;
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
  execFileSync('xcrun', ['simctl', 'push', UDID, BUNDLE, f]);
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

const run = process.env.GTMUX_E2E_ACCEPT_ALERTS === '1' ? describe : describe.skip;

run('a cold-start push is handled once', () => {
  const log: string[] = [];
  const note = async (step: string): Promise<Where> => {
    const w = await where();
    log.push(`${step}: ${JSON.stringify(w)}`);
    await dump(`c-${step}`);
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
    // An unsigned simulator build cannot keep servers in the Keychain, so "a saved, active
    // Mac at launch" is set up by the debug layer: both Macs seeded, the first (Home) made
    // active (SHOT_MODE's only effect here). Every launch below starts that way.
    const active = {GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_SERVERS: SEED(), GTMUX_DEBUG_SHOT_MODE: '1'};
    await launchWithFlags(active);
    await settle(5000);
    onRadar(await note('A0-home-connected'), 'Home');
    writeDebugFlags(active);

    await driver.terminateApp(BUNDLE);
    await settle(1500);
    push('Home', '%12', 'cold-A');
    expect(await tapBanner('cold-A')).toBe(true);
    await settle(7000);
    inDetail(await note('A1-after-cold-tap'), 'Home');

    await toServer('Office');
    onRadar(await note('A2-switch-office'), 'Office');
    await toServer('Home');
    onRadar(await note('A3-switch-home'), 'Home');
    await toServer('Office');
    onRadar(await note('A4-switch-office-again'), 'Office');

    await driver.execute('mobile: backgroundApp', {seconds: 3});
    await settle(4000);
    onRadar(await note('A5-after-background'), 'Office');

    await driver.execute('mobile: pressButton', {name: 'home'});
    await settle(1500);
    push('Office', '%13', 'warm-A');
    expect(await tapBanner('warm-A')).toBe(true);
    await settle(7000);
    inDetail(await note('A6-after-warm-tap-office13'), 'Office');
    await toServer('Home');
    onRadar(await note('A7-switch-home-after-warm'), 'Home');
  });

  it('C: a launch with no notification switches where it is pointed', async () => {
    await launchWithFlags({GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_SERVERS: SEED()});
    await settle(3000);
    await toServer('Home');
    onRadar(await note('C1-home'), 'Home');
    await toServer('Office');
    onRadar(await note('C2-office'), 'Office');
  });
});
