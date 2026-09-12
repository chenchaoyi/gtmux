import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * The HQ page's top chrome must not argue with itself. Before 2026-09-12 a SMALL scroll
 * on this page folded the header, the fold grew the viewport, the console's distance from
 * its tail shrank by that much and asked for the header back, and so on — the header
 * flickered until a scroll longer than its own height out-ran the loop. Detail's chrome
 * floats and never did this.
 *
 * The demo HQ console has one turn (nothing to scroll), so this runs against a live serve
 * whose HQ has a history. It only READS the page: no chip, no composer.
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" npm run test:e2e -- hq-chrome
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/hq-chrome');

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

gated('hq chrome stability', () => {
  it('a small scroll folds the header once, and it stays folded', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: url!, GTMUX_DEBUG_PAIR_TOKEN: token!, GTMUX_DEBUG_NO_PUSH: '1'});
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

  it('a top-anchored zone scrolls the chrome away in step, and brings it back', async () => {
    const driver = getDriver();
    const acts = driver.$('~hq-tab-acts');
    await acts.waitForDisplayed({timeout: 10_000});
    await acts.click();
    await settle(1200);
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
    const mid = Math.round(height * 0.6);
    // A small scroll: the chrome moves up by that much and STAYS there — no fold, no
    // blank band, and the tabs are still on screen.
    await drag(mid, mid - 60);
    await settle(600);
    const afterSmall = await sampleHeader(8, 100);
    shot('04-acts-after-small-scroll');
    // eslint-disable-next-line no-console
    console.log('[hq-chrome] acts zone, header shown after a small scroll:', afterSmall.join(' '));
    expect(new Set(afterSmall).size).toBe(1);
    expect(await driver.$('~hq-tab-acts').isDisplayed()).toBe(true);
    // A long one: the chrome is off screen.
    await drag(mid, mid - 500);
    await settle(1200);
    shot('05-acts-scrolled');
    expect(await driver.$('~hq-tab-acts').isDisplayed()).toBe(false);
    // Back to the top: it is back.
    await drag(Math.round(height * 0.3), Math.round(height * 0.95));
    await drag(Math.round(height * 0.3), Math.round(height * 0.95));
    await settle(1200);
    shot('06-acts-back-at-top');
    expect(await driver.$('~hq-board-open').isDisplayed()).toBe(true);
  });
});
