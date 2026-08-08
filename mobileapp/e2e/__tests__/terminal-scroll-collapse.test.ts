import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, openFirstAgentDetail, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Self-verification for the scroll-away collapse of the TOP CHROME in terminal
 * mode (run by the dev, not CI): scrolling up into scrollback must fold BOTH the
 * header info block AND the Chat/Terminal segmented control (one `collapse`
 * driver — one gesture, all top chrome folds together), and returning to the
 * live tail must bring them back.
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e -- -t "terminal scroll"
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('terminal scroll collapses segmented (live, debug-driven)', () => {
  it('hides Chat/Terminal tabs while browsing scrollback, restores at the tail', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
    });

    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('tsc-no-radar', err);
    }

    if (!(await openFirstAgentDetail())) {
      return captureOnFailure('tsc-no-detail', new Error('could not reach Detail'));
    }

    // → terminal mode (the segmented is what we're watching; terminal is the
    // mode whose scrollback we drag through)
    const termTab = driver.$(`~${TestIds.detail.modeTerminal}`);
    await termTab.waitForDisplayed({timeout: 8_000});
    await termTab.click();
    await settle(1500); // first mount + a pane poll

    // Baseline: at the live tail the segmented is visible (guards against the
    // "XCUITest reports everything invisible" failure mode before we rely on
    // the negative below).
    expect(await termTab.isDisplayed()).toBe(true);
    await screenshot('tsc-1-at-bottom');

    // Drag DOWN over the pane (finger down = scroll up into history), twice so
    // even a short first drag clearly leaves the live edge.
    const {width, height} = await driver.getWindowSize();
    const cx = Math.round(width / 2);
    const drag = async () => {
      await driver
        .action('pointer', {parameters: {pointerType: 'touch'}})
        .move({x: cx, y: Math.round(height * 0.35)})
        .down()
        .pause(80)
        .move({x: cx, y: Math.round(height * 0.75), duration: 350})
        .up()
        .perform();
      await settle(500);
    };
    await drag();
    await drag();
    await settle(600); // collapse animation is 200ms — let it finish

    await screenshot('tsc-2-scrolled-up');
    // The whole top chrome is folded: the segmented control is gone from view.
    expect(await termTab.isDisplayed()).toBe(false);

    // Flick back to the live tail via the jump-to-bottom FAB → chrome returns.
    const fab = driver.$(`~${TestIds.detail.jumpBottom}`);
    try {
      await fab.waitForDisplayed({timeout: 6_000});
    } catch (err) {
      return captureOnFailure('tsc-no-fab', err);
    }
    await fab.click();
    await settle(900);

    await screenshot('tsc-3-back-at-bottom');
    await termTab.waitForDisplayed({timeout: 6_000});
    expect(await termTab.isDisplayed()).toBe(true);
  });
});
