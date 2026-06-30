import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, openFirstAgentDetail, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Self-verification for the two chat-mode features (run by the dev, not CI):
 *  1. Full-screen works in CHAT mode (⛶ shows, exit pill is visible + tappable).
 *  2. Collapse/expand-all is reachable (fixed bar, not buried in the scroll) and
 *     actually collapses/expands replies.
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e -- -t "chat fullscreen"
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('chat fullscreen + collapse (live, debug-driven)', () => {
  it('opens chat, collapses/expands all, enters+exits full-screen', async () => {
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
      return captureOnFailure('cfc-no-radar', err);
    }

    if (!(await openFirstAgentDetail())) {
      return captureOnFailure('cfc-no-detail', new Error('could not reach Detail'));
    }

    // → chat mode
    const chatTab = driver.$(`~${TestIds.detail.modeChat}`);
    await chatTab.waitForDisplayed({timeout: 8_000});
    await chatTab.click();
    await settle(1200);
    await screenshot('cfc-1-chat'); // resting composer (flat icons) + user avatars

    // collapse-all bar must be present (fixed, reachable without scrolling up)
    const collapse = driver.$(`~${TestIds.detail.collapseAll}`);
    try {
      await collapse.waitForDisplayed({timeout: 6_000});
    } catch (err) {
      return captureOnFailure('cfc-no-collapse-bar', err);
    }
    // Count collapsed-reply rows by testID (XCUITest reports a TouchableOpacity/View
    // as visible=false, so isDisplayed is unreliable — counting elements isn't).
    // >0 = collapsed. Tap the bar until the state flips; the live pane re-renders
    // ~1.5s so a single tap can hit a stale element — retry.
    const countCollapsed = async () => (await driver.$$(`~${TestIds.detail.collapsedReply}`).catch(() => [])).length;
    const tapUntil = async (wantCollapsed: boolean) => {
      for (let i = 0; i < 6; i++) {
        if ((await countCollapsed()) > 0 === wantCollapsed) return true;
        await collapse.click().catch(() => {});
        await settle(700);
      }
      return (await countCollapsed()) > 0 === wantCollapsed;
    };

    if (!(await tapUntil(true))) {
      return captureOnFailure('cfc-collapse-failed', new Error('replies did not collapse'));
    }
    await settle(400);
    await screenshot('cfc-2-collapsed');
    // collapsed turns must still carry the agent avatar (conversation flow)
    expect(await countCollapsed()).toBeGreaterThan(0);

    if (!(await tapUntil(false))) {
      return captureOnFailure('cfc-expand-failed', new Error('replies did not expand'));
    }
    await settle(400);
    await screenshot('cfc-3-expanded');

    // full-screen in chat: tap ⛶, the exit pill must appear, tap it to return
    const fs = driver.$(`~${TestIds.detail.fullscreen}`);
    await fs.waitForDisplayed({timeout: 6_000});
    await fs.click();
    await settle(900);
    await screenshot('cfc-4-fullscreen');
    const exit = driver.$(`~${TestIds.detail.fsExit}`);
    try {
      await exit.waitForDisplayed({timeout: 6_000});
    } catch (err) {
      return captureOnFailure('cfc-no-exit', err);
    }
    expect(await exit.isDisplayed()).toBe(true);
    await exit.click();
    await settle(900);
    await screenshot('cfc-5-exited');
    // back to normal chrome → the mode tabs are visible again
    await chatTab.waitForDisplayed({timeout: 6_000});
  });
});
