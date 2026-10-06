import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';
import {FakeAgent} from '../fake-serve/world';

/**
 * The HQ page's top chrome must not argue with itself. Before 2026-09-12 a SMALL scroll
 * on this page folded the header, the fold grew the viewport, the console's distance from
 * its tail shrank by that much and asked for the header back, and so on — the header
 * flickered until a scroll longer than its own height out-ran the loop. Detail's chrome
 * floats and never did this.
 *
 * The demo HQ console has one turn (nothing to scroll), so this runs against the
 * in-process fake with both of the page's zones made long enough to scroll: HQ's console
 * seeded with a twenty-turn conversation (world.seedLongChat), and "Your call" with
 * twelve sessions waiting on the user (world.seedCalls). It only READS the page: no chip,
 * no composer.
 */
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/hq-chrome');
let fake: Fake;
let calls: FakeAgent[] = [];
beforeAll(async () => {
  fake = await startFake();
  fake.world.seedLongChat('%6', 20);
  calls = fake.world.seedCalls(12);
});
afterAll(async () => {
  await fake?.close();
});

function shot(name: string): void {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${name}.png`)], {stdio: 'ignore'});
}

/** The chrome's state, sampled: is a control that lives in the header on screen? */
async function sampleHeader(n: number, everyMs: number): Promise<boolean[]> {
  const driver = getDriver();
  const out: boolean[] = [];
  for (let i = 0; i < n; i++) {
    out.push(await driver.$('~hq-board-open').isDisplayed().catch(() => false));
    await settle(everyMs);
  }
  return out;
}

describe('hq chrome stability', () => {
  it('a small scroll folds the header once, and it stays folded', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1'});
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('hq-chrome-no-radar', err);
    }
    const disc = driver.$('~radar-hq-disc');
    await disc.waitForDisplayed({timeout: 20_000});
    await settle(1200);
    await disc.click();
    const consoleTab = driver.$('~hq-tab-console');
    await consoleTab.waitForDisplayed({timeout: 15_000});
    await consoleTab.click();
    await settle(2500); // the transcript loads and pins to its tail
    shot('01-console-at-tail');
    expect(await driver.$('~hq-board-open').isDisplayed()).toBe(true);

    const {width, height} = await driver.getWindowSize();
    const cx = Math.round(width / 2);
    const drag = async (fromY: number, toY: number) => {
      await driver
        .action('pointer', {parameters: {pointerType: 'touch'}})
        .move({x: cx, y: fromY})
        .down()
        .pause(60)
        .move({x: cx, y: toY, duration: 250})
        .up()
        .perform();
    };

    // Into history by a little more than the fold line (72pt), well under the header's
    // height — the exact stretch that used to flicker.
    const mid = Math.round(height * 0.5);
    await drag(mid, mid + 95);
    await settle(400); // one fold animation (200ms), plus slack
    const afterSmall = await sampleHeader(12, 100);
    shot('02-after-small-scroll');
    // eslint-disable-next-line no-console
    console.log('[hq-chrome] header shown, sampled after a small scroll:', afterSmall.join(' '));
    expect(new Set(afterSmall).size).toBe(1); // one state, held — no flicker
    expect(afterSmall[0]).toBe(false); // and it is the folded one

    // Back to the tail: the header returns, and stays.
    await drag(mid + 95, mid - 400);
    await settle(1200);
    const atTail = await sampleHeader(8, 100);
    shot('03-back-at-tail');
    // eslint-disable-next-line no-console
    console.log('[hq-chrome] header shown, back at the tail:', atTail.join(' '));
    expect(new Set(atTail).size).toBe(1);
    expect(atTail[0]).toBe(true);
  });

  // The page's other kind of zone. "Your call" is a list read from the top, and it does NOT
  // fold: a fold at 72pt would leave a blank band the rest of the chrome's height above
  // the first card (simulator, 2026-09-12). Its chrome scrolls away WITH the content, in
  // step and clamped at its own height, and comes back the same way (HQScreen,
  // `zoneOffset`). This case used to drive `acts`, a zone removed in #1086; its tab no
  // longer existed, so the case could not reach anything it meant to check.
  it('a top-anchored zone scrolls the chrome away in step, and brings it back', async () => {
    const driver = getDriver();
    const tab = driver.$('~hq-tab-calls');
    await tab.waitForDisplayed({timeout: 10_000});
    await tab.click();
    await settle(1200);
    // The zone is the one we asked for, and it holds the seeded decisions, a card per
    // waiting session. An empty zone has nothing to scroll and would pass the checks below
    // for the wrong reason. The card watched here is the longest-waiting seeded one, near
    // the top of the list.
    const card = driver.$(`~hq-call-${calls[0].session}:${calls[0].window}.0`);
    await card.waitForExist({timeout: 10_000});
    const {width, height} = await driver.getWindowSize();
    const cx = Math.round(width / 2);
    const drag = async (fromY: number, toY: number) => {
      await driver
        .action('pointer', {parameters: {pointerType: 'touch'}})
        .move({x: cx, y: fromY})
        .down()
        .pause(60)
        .move({x: cx, y: toY, duration: 250})
        .up()
        .perform();
    };
    const tabY0 = (await tab.getLocation()).y;
    const cardY0 = (await card.getLocation()).y;
    const mid = Math.round(height * 0.6);
    // A small scroll: the chrome moves up by that much and STAYS there — no fold, no
    // blank band, and the tabs are still on screen.
    await drag(mid, mid - 60);
    await settle(600);
    const afterSmall = await sampleHeader(8, 100);
    shot('04-calls-after-small-scroll');
    // eslint-disable-next-line no-console
    console.log('[hq-chrome] calls zone, header shown after a small scroll:', afterSmall.join(' '));
    expect(new Set(afterSmall).size).toBe(1);
    expect(await tab.isDisplayed()).toBe(true);
    // In step: the chrome travelled exactly as far as the card did. A fold would
    // have moved it by its whole height (or faded it out) whatever the content did.
    const cardShift = cardY0 - (await card.getLocation()).y;
    const tabShift = tabY0 - (await tab.getLocation()).y;
    // eslint-disable-next-line no-console
    console.log(`[hq-chrome] calls zone, small scroll moved a card ${cardShift}pt and the tabs ${tabShift}pt`);
    expect(cardShift).toBeGreaterThan(0);
    expect(Math.abs(tabShift - cardShift)).toBeLessThanOrEqual(3);
    // A long one: the chrome is off screen.
    await drag(mid, mid - 500);
    await settle(1200);
    shot('05-calls-scrolled');
    expect(await tab.isDisplayed()).toBe(false);
    // Back to the top: it is back, where it started.
    await drag(Math.round(height * 0.3), Math.round(height * 0.95));
    await drag(Math.round(height * 0.3), Math.round(height * 0.95));
    await settle(1200);
    shot('06-calls-back-at-top');
    expect(await driver.$('~hq-board-open').isDisplayed()).toBe(true);
    expect(Math.abs((await tab.getLocation()).y - tabY0)).toBeLessThanOrEqual(3);
  });
});
