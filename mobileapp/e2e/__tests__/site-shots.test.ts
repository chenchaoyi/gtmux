import {execFileSync} from 'child_process';
import {mkdirSync, rmSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';
import {iconDir, iconFile} from '../fake-serve/icons';

/**
 * Website screenshots (ccy.dev): the App Store's demo world, with each agent's real mark.
 *
 * The App Store captures (appstore-shots*.test.ts) photograph the in-app Demo, which
 * fetches no icons, so every agent wears its neutral monogram. That is deliberate and
 * stays: App Review's third-party trademark rules. The website wants the marks, so this
 * suite connects the app to the fake serve instead, seeded with the SAME world
 * (World.seedDemo reads it from the Demo's own client) plus an icon hint on every agent
 * row (World.seedIcons), and the app fetches each mark over /api/icon as from a real Mac.
 *
 * The repository ships no Claude mark, so GTMUX_FAKE_ICON_DIR must point at a LOCAL,
 * uncommitted directory holding a PNG per agent the demo runs (claude.png, codex.png,
 * gemini.png). The run refuses to start without all of them: a website image with a
 * monogram in it is the failure this suite exists to prevent.
 *
 *   GTMUX_SITE_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_FAKE_ICON_DIR="$HOME/site-icons" \
 *   GTMUX_E2E_UDID="${AUDIT_SIM_UDID:?the owned phone simulator UDID}" npm run test:e2e -- site-shots
 *
 *   GTMUX_SITE_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_FAKE_ICON_DIR="$HOME/site-icons" \
 *   GTMUX_E2E_UDID="${AUDIT_IPAD_UDID:?the owned iPad simulator UDID}" \
 *   GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- site-shots
 *
 * Phone → .e2e-artifacts/site/<lang>/: 01-radar, 02-terminal-approval, 04-console.
 * iPad  → .e2e-artifacts/site/ipad-<lang>/: 02-hq, 03-panes.
 * Each name is its App Store twin's, so a website image maps to the store capture it
 * replaces. The app's language is forced with GTMUX_DEBUG_LANG; set the simulator's own
 * language to match too (docs/appstore-shots.md) so system chrome agrees.
 */
const on = !!process.env.GTMUX_SITE_SHOTS;
const ipad = /ipad/i.test(process.env.GTMUX_E2E_DEVICE || '');
const phoneGated = on && !ipad ? describe : describe.skip;
const ipadGated = on && ipad ? describe : describe.skip;

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const LANG: 'en' | 'zh' = process.env.GTMUX_SHOTS_LANG === 'zh' ? 'zh' : 'en';
// The Mac's name in the radar header. Generic, never the capturing machine's own.
const MAC_NAME = process.env.GTMUX_SHOTS_NAME || 'MacBook Pro';
const OUT = resolve(__dirname, `../../.e2e-artifacts/site/${ipad ? 'ipad-' : ''}${LANG}`);

function simctl(args: string[]): void {
  execFileSync('xcrun', ['simctl', ...args], {stdio: 'ignore'});
}

let fake: Fake;
/** The agents the demo world runs, each of which must have a mark to show. */
let marks: string[] = [];

/**
 * Start the fake with the demo world and its icon hints, and refuse to go on unless every
 * agent in it has a picture: without one that row would be a monogram on the website.
 */
async function startSiteFake(): Promise<void> {
  fake = await startFake();
  await fake.world.seedDemo(LANG);
  marks = [...new Set(fake.world.agents.map(a => a.agent).filter(Boolean))].sort();
  const missing = marks.filter(m => !iconFile(m));
  if (missing.length > 0) {
    throw new Error(
      `[site-shots] no icon for ${missing.join(', ')} in ${iconDir()}. Point GTMUX_FAKE_ICON_DIR at a local ` +
        'directory with a <key>.png for each (claude.png, codex.png, gemini.png); the repo ships no claude.png.',
    );
  }
  expect(fake.world.seedIcons()).toEqual(marks);
}

/** Pair the app with the fake, in this run's language, and cold-launch it. */
async function launchPaired(): Promise<void> {
  await launchWithFlags({
    GTMUX_DEBUG_PAIR_URL: fake.url,
    GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_PAIR_NAME: MAC_NAME,
    GTMUX_DEBUG_NO_PUSH: '1',
    GTMUX_DEBUG_LANG: LANG,
  });
}

/**
 * Wait until the app has fetched every mark, so no frame catches a row before its icon
 * arrives. The fake counts what it served (World.iconsServed); a mark the app never asks
 * for fails the run rather than leaving a monogram in the picture.
 */
async function marksLoaded(timeout = 20_000): Promise<void> {
  const until = Date.now() + timeout;
  for (;;) {
    const waiting = marks.filter(m => !fake.world.iconsServed.get(m));
    if (waiting.length === 0) return;
    if (Date.now() > until) throw new Error(`[site-shots] the app never fetched the icon for ${waiting.join(', ')}`);
    await settle(250);
  }
}

/**
 * The run's shots: deleted before it starts, recorded only once simctl wrote each, so a
 * step that does not happen cannot leave last run's picture behind (appstore-shots.test.ts).
 */
function shots(names: string[], upright: boolean) {
  mkdirSync(OUT, {recursive: true});
  for (const name of names) rmSync(join(OUT, `${name}.png`), {force: true});
  const written: string[] = [];
  const shot = (name: string): void => {
    const file = join(OUT, `${name}.png`);
    simctl(['io', UDID, 'screenshot', file]);
    if (upright) {
      // As appstore-shots-ipad.test.ts: simctl has returned landscape captures both
      // sideways and upright across iOS releases, so ask the file and turn only a
      // portrait one.
      const dims = execFileSync('sips', ['-g', 'pixelWidth', '-g', 'pixelHeight', file]).toString();
      const w = Number(/pixelWidth:\s*(\d+)/.exec(dims)?.[1] ?? 0);
      const h = Number(/pixelHeight:\s*(\d+)/.exec(dims)?.[1] ?? 0);
      if (h > w) execFileSync('sips', ['-r', '270', file], {stdio: 'ignore'});
    }
    written.push(name);
  };
  return {shot, written};
}

phoneGated('website shots (phone)', () => {
  beforeAll(startSiteFake);
  afterAll(async () => {
    await fake?.close();
  });

  it('captures the radar, the terminal with its approval card and the HQ console, with real marks', async () => {
    const SHOTS = ['01-radar', '02-terminal-approval', '04-console'];
    const {shot, written} = shots(SHOTS, false);
    simctl(['status_bar', UDID, 'override', '--time', '9:41', '--batteryState', 'charged',
      '--batteryLevel', '100', '--cellularBars', '4', '--wifiBars', '3']);
    const driver = getDriver();
    await launchPaired();

    // 1) The radar: the fleet in the status language and the floating HQ disc.
    const hqDisc = driver.$('~radar-hq-disc');
    await hqDisc.waitForDisplayed({timeout: 25_000});
    await marksLoaded();
    await settle(1400);
    shot('01-radar');

    // 2) The hero (%7, waiting on a 1/2/3 permission) in its colored terminal, with the
    //    approval card. The card's first answer is what proves the menu arrived.
    const hero = driver.$(`~${TestIds.agent.row}-%7`);
    await hero.waitForDisplayed({timeout: 10_000});
    await hero.click();
    await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 10_000});
    await driver.$(`~${TestIds.detail.modeTerminal}`).click();
    await driver.$('~reply-1').waitForDisplayed({timeout: 10_000});
    await settle(1800);
    shot('02-terminal-approval');

    await driver.$(`~${TestIds.detail.back}`).click();
    await hqDisc.waitForDisplayed({timeout: 10_000});

    // 4) The HQ console: its words with the acts it recorded threaded in. Tapped for real,
    //    as in the store suite: a tab that stops existing must fail the capture.
    await hqDisc.click();
    const consoleTab = driver.$('~hq-tab-console');
    await consoleTab.waitForDisplayed({timeout: 10_000});
    await consoleTab.click();
    await settle(1800);
    shot('04-console');

    expect(written).toEqual(SHOTS);
    // eslint-disable-next-line no-console
    console.log(`[site-shots] wrote ${written.join(' / ')} to ${OUT}`);
  });
});

ipadGated('website shots (iPad)', () => {
  beforeAll(startSiteFake);
  afterAll(async () => {
    await fake?.close();
  });

  it('captures HQ with its inspector and the pane grid, with real marks', async () => {
    const SHOTS = ['02-hq', '03-panes'];
    const {shot, written} = shots(SHOTS, true);
    simctl(['status_bar', UDID, 'override', '--time', '9:41', '--batteryState', 'charged',
      '--batteryLevel', '100', '--wifiBars', '3']);
    const driver = getDriver();
    await driver.setOrientation('LANDSCAPE').catch(() => {});
    await launchPaired();
    await driver.$(`~${TestIds.radar.split}`).waitForDisplayed({timeout: 25_000});
    await marksLoaded();

    await driver.$('~radar-hq-card').click();
    await driver.$('~hq-inspector').waitForDisplayed({timeout: 10_000});
    await settle(1600);
    shot('02-hq');

    await driver.$(`~${TestIds.radar.panes}`).click();
    await driver.$(`~${TestIds.panes.search}`).waitForDisplayed({timeout: 10_000});
    await settle(1400);
    shot('03-panes');

    expect(written).toEqual(SHOTS);
    // eslint-disable-next-line no-console
    console.log(`[site-shots] wrote ${written.join(' / ')} to ${OUT}`);
  });
});
