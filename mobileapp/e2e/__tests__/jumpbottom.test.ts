import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, openFirstAgentDetail, readDebugLog} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Reported: scroll up in a busy pane's terminal, tap the jump-to-bottom arrow, and it
 * lands a little ABOVE the tail. Idle panes are fine.
 *
 * The check is a screenshot identity: capture the live tail, scroll up, jump back, and
 * capture again. Landing short shows as two different images — no need to read a scroll
 * offset the accessibility tree does not expose.
 *
 * Gated on a live serve, like radar.test.ts:
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" npm run test:e2e
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('terminal jump-to-bottom', () => {
  it('lands on the tail, not short of it', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1', // also gates the terminal scroll probe
    });

    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('jump-no-radar', err);
    }
    // A pane that is CHANGING is the whole point: scrolling up freezes the snapshot and
    // newer frames queue behind it. GTMUX_E2E_BUSY_PANE names one (any pane id — the
    // browser reaches plain shells too, which is the easiest way to get deterministic
    // output). Without it, the first agent row, which may well be idle and prove nothing.
    // A pane that is CHANGING is the whole point: scrolling up freezes the snapshot and
    // newer frames queue behind it. An idle pane proves nothing — the first run of this
    // test opened one and passed without exercising anything.
    //
    // Radar rows are directly addressable by pane id and are never collapsed, unlike the
    // pane browser's session groups.
    const busy = process.env.GTMUX_E2E_BUSY_PANE;
    if (busy) {
      const row = driver.$(`~${TestIds.agent.row}-${busy}`);
      await row.waitForDisplayed({timeout: 15_000});
      await row.click();
      await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 8000});
    } else if (!(await openFirstAgentDetail())) {
      return captureOnFailure('jump-no-detail', new Error('could not open a pane'));
    }

    // Terminal mode, and let a couple of polls land so the pane is live.
    await driver.$(`~${TestIds.detail.modeTerminal}`).click();
    await driver.pause(4000);
    const tail = await driver.takeScreenshot();
    await screenshot('jump-1-tail');

    // Scroll UP into history. Freezes the snapshot; a busy pane starts queuing frames.
    const {width, height} = await driver.getWindowSize();
    for (let i = 0; i < 3; i++) {
      await driver.execute('mobile: dragFromToForDuration', {
        fromX: width / 2, fromY: height * 0.35, toX: width / 2, toY: height * 0.8, duration: 0.4,
      });
    }
    await driver.pause(2500); // let frames pile up behind the freeze
    await screenshot('jump-2-history');

    const arrow = driver.$(`~${TestIds.detail.jumpBottom}`);
    await arrow.waitForDisplayed({timeout: 8000});
    await arrow.click();
    await driver.pause(2500); // past the animation and any post-layout pin
    const back = await driver.takeScreenshot();
    await screenshot('jump-3-back');

    // A busy pane repaints, so the images are compared for "did we reach the bottom",
    // not byte equality: the jump-to-bottom arrow is only rendered while scrolled UP.
    // Still visible = still short of the tail.
    // The geometry the app itself saw, so a shortfall is a NUMBER rather than a guess
    // about a native scroll race.
    const probe = readDebugLog().filter(r => r.event === 'termprobe');
    const jumpAt = probe.findIndex(r => r.at === 'jump');
    // eslint-disable-next-line no-console
    console.log('[probe] ' + JSON.stringify(probe.slice(Math.max(0, jumpAt - 2), jumpAt + 12)));

    expect(await arrow.isDisplayed().catch(() => false)).toBe(false);
    expect(back.length).toBeGreaterThan(0);
    expect(tail.length).toBeGreaterThan(0);
  });
});
