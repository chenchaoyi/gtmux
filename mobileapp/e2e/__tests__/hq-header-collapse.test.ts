import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {startFake, Fake} from '../fake-serve/server';
import {FakeAgent} from '../fake-serve/world';

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
 * Runs against the in-process fake with twelve sessions waiting on the user
 * (world.seedCalls), so "Your call" holds more cards than one screen and can be read into.
 */
let fake: Fake;
let calls: FakeAgent[] = [];
beforeAll(async () => {
  fake = await startFake();
  calls = fake.world.seedCalls(12);
});
afterAll(async () => {
  await fake?.close();
});

describe('the HQ header', () => {
  it('comes back after it folds', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
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

    // Drive the "Your call" zone on purpose. The page has two kinds of zone and they hide
    // the header on OPPOSITE gestures: a top-anchored list carries it away as you scroll
    // down into it, while the console is a chat pinned to its tail and folds it as you
    // scroll AWAY from the tail. A test that lands on whichever tab was last open is a
    // coin flip.
    //
    // This used to tap the `acts` tab inside a catch. That zone was removed in #1086, so
    // the tap failed silently and the test drove whichever zone the page opened on — the
    // coin flip it was written to avoid. The tab is required now, and so is a seeded card
    // in the zone it opens.
    await driver.$('~hq-tab-calls').click();
    await settle(900);
    await driver.$(`~hq-call-${calls[0].session}:${calls[0].window}.0`).waitForExist({timeout: 8_000});

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
