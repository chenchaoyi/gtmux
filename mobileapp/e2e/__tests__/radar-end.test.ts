import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * The radar has to say where it ends. It used to stop at its last row and leave dark
 * space, which reads as "still loading" rather than "that was everything" (2026-09-05).
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" npm run test:e2e -- -t "radar end"
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('radar end', () => {
  it('closes the list where it ends', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('re-no-radar', err);
    }
    await settle(2500);

    const {width, height} = await driver.getWindowSize();
    const cx = Math.round(width / 2);
    for (let i = 0; i < 8; i++) {
      await driver
        .action('pointer', {parameters: {pointerType: 'touch'}})
        .move({x: cx, y: Math.round(height * 0.8)})
        .down()
        .pause(50)
        .move({x: cx, y: Math.round(height * 0.2), duration: 250})
        .up()
        .perform();
    }
    await settle(1200);
    await screenshot('radar-end');

    const end = driver.$(`~${TestIds.radar.end}`);
    try {
      await end.waitForDisplayed({timeout: 6_000});
    } catch (err) {
      return captureOnFailure('re-no-end', err);
    }
    expect(await end.isDisplayed()).toBe(true);
  });
});
