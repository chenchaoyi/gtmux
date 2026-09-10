import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * The HQ page's header folds as you read into a zone and returns at the top.
 *
 * On 2026-09-10 it folded once and never came back. A parameter had been removed from the
 * middle of the fold rule's positional arguments; this page kept passing four numbers, so
 * its header height landed on the clock and its clock landed on the animation length.
 * Every number went to a number, so nothing failed to compile and every unit test stayed
 * green — the mistake was entirely at the call site.
 *
 * So the guard is here, where a page is scrolled and the header is looked for.
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e -- -t "HQ header"
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('the HQ header', () => {
  it('comes back after it folds', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
    });

    const disc = driver.$('~radar-hq-disc');
    try {
      await disc.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('hqh-no-radar', err);
    }
    await disc.click();
    const verdict = driver.$('~hq-verdict');
    try {
      await verdict.waitForDisplayed({timeout: 12_000});
    } catch (err) {
      return captureOnFailure('hqh-no-hq', err);
    }
    await settle(1200);

    // Drive the ACTS tab on purpose. The page has two kinds of zone and they fold on
    // OPPOSITE gestures: a top-anchored list folds as you scroll down into it, while the
    // console is a chat pinned to its tail and folds as you scroll AWAY from the tail.
    // A test that lands on whichever tab was last open is a coin flip.
    await driver.$('~hq-tab-acts').click().catch(() => {});
    await settle(900);

    const {width, height} = await driver.getWindowSize();
    const cx = Math.round(width / 2);
    const drag = async (fromY: number, toY: number) => {
      await driver
        .action('pointer', {parameters: {pointerType: 'touch'}})
        .move({x: cx, y: fromY})
        .down()
        .pause(70)
        .move({x: cx, y: toY, duration: 340})
        .up()
        .perform();
      await settle(700);
    };

    // Read down into the list: the header folds.
    await drag(Math.round(height * 0.72), Math.round(height * 0.34));
    await drag(Math.round(height * 0.72), Math.round(height * 0.34));
    await settle(700);
    await screenshot('hq-header-folded');
    expect(await verdict.isDisplayed().catch(() => false)).toBe(false);

    // Back to the top: it must return. This is the half that broke.
    await drag(Math.round(height * 0.34), Math.round(height * 0.8));
    await drag(Math.round(height * 0.34), Math.round(height * 0.8));
    await drag(Math.round(height * 0.34), Math.round(height * 0.8));
    await settle(1000);
    await screenshot('hq-header-back');
    await verdict.waitForDisplayed({timeout: 6_000});
    expect(await verdict.isDisplayed()).toBe(true);
  });
});
