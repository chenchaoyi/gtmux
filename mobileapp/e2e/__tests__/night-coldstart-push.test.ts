// Night repro (review of #1308): a push from one Mac that cold-starts the app must be
// handled once. Switching to another Mac afterwards must not bounce back to the Mac the
// push came from, and a later, genuinely new tap must still be handled.
//
// Needs notification permission: run with GTMUX_E2E_ACCEPT_ALERTS=1 (the session accepts
// the system prompt). Pushes are delivered with `xcrun simctl push`, which iOS presents
// exactly like a remote notification; the banner is tapped on the home screen.
import {execFileSync} from 'child_process';
import {writeFileSync} from 'fs';
import {join} from 'path';
import {getDriver, getArtifactsDir} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {BUNDLE, launchWithFlags, settle, writeDebugFlags} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const HOME_PORT = Number(process.env.NIGHT_HOME_PORT) || 47811;
const OFFICE_PORT = Number(process.env.NIGHT_OFFICE_PORT) || 47812;

let home: Fake;
let office: Fake;
beforeAll(async () => {
  home = await startFake({port: HOME_PORT});
  office = await startFake({port: OFFICE_PORT});
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
async function where(): Promise<string> {
  const driver = getDriver();
  const src = await driver.getPageSource();
  const chip = /name="radar-server-chip"[^>]*label="([^"]*)"/.exec(src)?.[1] ?? /label="([^"]*)"[^>]*name="radar-server-chip"/.exec(src)?.[1] ?? '-';
  const detail = src.includes(`name="${TestIds.detail.screen}"`);
  const radar = src.includes(`name="${TestIds.radar.screen}"`);
  const servers = src.includes(`name="${TestIds.servers.screen}"`);
  const h = (src.match(/home-/g) ?? []).length;
  const o = (src.match(/office-/g) ?? []).length;
  return `chip=${chip} radar=${radar} detail=${detail} servers=${servers} home=${h} office=${o}`;
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

describe('night: a cold-start push is handled once', () => {
  const log: string[] = [];
  const note = async (step: string) => {
    const w = await where();
    log.push(`${step}: ${w}`);
    // eslint-disable-next-line no-console
    console.log(`[cold] ${step}: ${w}`);
    await dump(`c-${step}`);
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
    await note('A0-home-connected');
    writeDebugFlags(active);

    await driver.terminateApp(BUNDLE);
    await settle(1500);
    push('Home', '%12', 'night-cold-A');
    log.push(`A banner tapped: ${await tapBanner('night-cold-A')}`);
    await settle(7000);
    await note('A1-after-cold-tap');

    await toServer('Office');
    await note('A2-switch-office');
    await toServer('Home');
    await note('A3-switch-home');
    await toServer('Office');
    await note('A4-switch-office-again');

    await driver.execute('mobile: backgroundApp', {seconds: 3});
    await settle(4000);
    await note('A5-after-background');

    await driver.execute('mobile: pressButton', {name: 'home'});
    await settle(1500);
    push('Office', '%13', 'night-warm-A');
    log.push(`A warm banner tapped: ${await tapBanner('night-warm-A')}`);
    await settle(7000);
    await note('A6-after-warm-tap-office13');
    await toServer('Home');
    await note('A7-switch-home-after-warm');
  });

  it('B: cold start with no active Mac (server list), then the user picks Office', async () => {
    const driver = getDriver();
    writeDebugFlags({GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_SERVERS: SEED()});
    await driver.terminateApp(BUNDLE);
    await settle(1500);
    push('Home', '%12', 'night-cold-B');
    log.push(`B banner tapped: ${await tapBanner('night-cold-B')}`);
    await settle(6000);
    await note('B1-after-cold-tap');
    await toServer('Office');
    await note('B2-picked-office');
    await toServer('Office');
    await note('B3-picked-office-again');
  });

  it('C: a launch with no notification switches where it is pointed', async () => {
    const driver = getDriver();
    await launchWithFlags({GTMUX_DEBUG_LOG_NET: '1', GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_SERVERS: SEED()});
    await settle(3000);
    await toServer('Home');
    await note('C1-home');
    await toServer('Office');
    await note('C2-office');
    void driver;
  });
});
