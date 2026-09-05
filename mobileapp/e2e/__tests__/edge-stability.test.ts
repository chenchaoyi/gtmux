import {getDriver} from '../setup/driver';
import {captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, openFirstAgentDetail, readDebugLog, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * A trace of the top chrome around the live tail, read from the terminal's own per-frame
 * probe (`termprobe` records gap / content / offset / VIEWPORT on every scroll frame).
 *
 * The chrome collapsing changes the viewport, the viewport decides the edge state, and the
 * edge state drives the chrome — so an unstable header shows up here as `view` sawtoothing
 * between two heights while the finger is not moving. Run it by hand, read the numbers.
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" npm run test:e2e -- -t "edge"
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('edge stability', () => {
  it('traces the viewport around the live tail', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('edge-no-radar', err);
    }
    if (!(await openFirstAgentDetail())) {
      return captureOnFailure('edge-no-detail', new Error('could not reach Detail'));
    }
    const termTab = driver.$(`~${TestIds.detail.modeTerminal}`);
    await termTab.waitForDisplayed({timeout: 8_000});
    await termTab.click();
    await settle(2000);

    const {width, height} = await driver.getWindowSize();
    const cx = Math.round(width / 2);
    const swipe = async (fromY: number, toY: number) => {
      await driver
        .action('pointer', {parameters: {pointerType: 'touch'}})
        .move({x: cx, y: fromY})
        .down()
        .pause(60)
        .move({x: cx, y: toY, duration: 300})
        .up()
        .perform();
      await settle(700);
    };

    // Up into scrollback, then back down to the tail — the return is where it jumps.
    await swipe(Math.round(height * 0.35), Math.round(height * 0.78));
    await swipe(Math.round(height * 0.35), Math.round(height * 0.78));
    await settle(800);
    await swipe(Math.round(height * 0.78), Math.round(height * 0.32));
    await swipe(Math.round(height * 0.78), Math.round(height * 0.32));
    await settle(2500); // stand still: anything moving now is the app arguing with itself

    const frames = readDebugLog().filter(e => e.event === 'termprobe');
    const line = (e: Record<string, unknown>) =>
      `${String(e.at).padEnd(12)} view=${e.view} gap=${e.gap} off=${e.off} content=${e.content} stick=${e.stick}`;
    // eslint-disable-next-line no-console
    console.log('[edge]\n' + frames.slice(-60).map(line).join('\n'));

    // The assertion. Revealing the chrome SHRINKS the viewport; once that has started, it
    // must not grow again — a growth mid-reveal is the chrome changing its mind, which is
    // the whole bug. Measured before the fix (2026-09-05): 692.7 → 619.7 → 628.7 → 576,
    // three reversals and 117pt of overshoot. After: 692.7 → 690.3 → … → 576, monotone.
    const views = frames.map(e => Number(e.view)).filter(v => Number.isFinite(v));
    const peak = views.indexOf(Math.max(...views));
    const bumps: string[] = [];
    for (let i = peak + 1; i < views.length; i++) {
      if (views[i] > views[i - 1] + 1) bumps.push(`${views[i - 1]} → ${views[i]}`);
    }
    // eslint-disable-next-line no-console
    console.log('[edge] viewport after the peak:', views.slice(peak).join(' → '));
    expect(bumps).toEqual([]);
  });
});
