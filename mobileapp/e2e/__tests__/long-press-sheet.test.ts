import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * The long-press sheet, driven the way a finger drives it.
 *
 * A unit test can assert that the impact fires and the groups split; it cannot say whether
 * a real long press on a real row opens the sheet, nor what the sheet looks like once the
 * groups are separate blocks. Both changed on 2026-09-10, and the second is the half that
 * only a screenshot can answer.
 *
 *   GTMUX_E2E_URL="${AUDIT_SERVE_URL:?the isolated fixture URL}" \
 *   GTMUX_E2E_TOKEN="${AUDIT_SERVE_TOKEN:?its synthetic token}" \
 *   GTMUX_E2E_UDID="${AUDIT_SIM_UDID:?the owned simulator UDID}" npm run test:e2e -- -t "long press"
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('long press on a radar row', () => {
  it('opens the sheet and shows its actions in groups', async () => {
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
      return captureOnFailure('lp-no-radar', err);
    }
    await settle(2000);

    // Press and HOLD on the first row — a tap must not open this, which is half of what
    // the gesture is for.
    const rows = await driver.$$(`//*[contains(@name,"${TestIds.agent.row}-")]`).getElements();
    if (rows.length === 0) {
      return captureOnFailure('lp-no-rows', new Error('no agent rows on the radar'));
    }
    const box = await rows[0].getElementRect(await rows[0].elementId);
    const cx = Math.round(box.x + box.width / 2);
    const cy = Math.round(box.y + box.height / 2);
    await driver
      .action('pointer', {parameters: {pointerType: 'touch'}})
      .move({x: cx, y: cy})
      .down()
      .pause(900) // well past the 350ms hold
      .up()
      .perform();
    await settle(700);

    // Wait on an ACTION, not on the card: the card is deliberately not an accessibility
    // element (it would swallow every row inside it), so "the sheet is open" is best
    // asked as "an action is reachable" — which is also the thing that has to be true.
    const jump = driver.$(`~${TestIds.agent.sheetAction}-jump`);
    try {
      await jump.waitForExist({timeout: 6_000});
    } catch (err) {
      return captureOnFailure('lp-no-sheet', err);
    }
    await screenshot('long-press-sheet');

    // EXISTENCE, not isDisplayed: inside a React Native Modal, XCUITest reports these
    // rows as not displayed even when the screenshot plainly shows them, and a lower
    // group can also sit below the sheet's fold.
    //
    // Jump is in every state; the LAST group (diff, when the pane has a branch) is the
    // one that proves the grouping did not drop a block on its way to the screen.
    const src = await driver.getPageSource();
    expect(src).toContain(`${TestIds.agent.sheetAction}-stop`);
    if (src.includes('chore/') || src.includes('main')) {
      expect(src).toContain(`${TestIds.agent.sheetAction}-diff`);
    }

    // Scroll the sheet so the shot carries every group, not just the ones above the fold.
    const {width, height} = await driver.getWindowSize();
    await driver
      .action('pointer', {parameters: {pointerType: 'touch'}})
      .move({x: Math.round(width / 2), y: Math.round(height * 0.8)})
      .down()
      .pause(60)
      .move({x: Math.round(width / 2), y: Math.round(height * 0.55), duration: 300})
      .up()
      .perform();
    await settle(500);
    await screenshot('long-press-sheet-foot');
  });
});
