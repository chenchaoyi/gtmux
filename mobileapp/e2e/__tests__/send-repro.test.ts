import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, openFirstAgentDetail, settle, readDebugLog} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Live repro of the "app can't send — appears then disappears, stuck" report.
 * Targets the FIRST radar agent (arranged to be a throwaway %40 claude), types
 * "继续", taps send, and asserts NO send-failed bar. Dumps the app's own net log
 * so we can see the exact /api/send status the client received.
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN=<serve token> \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e -- -t "send repro"
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('send repro (live, debug-driven)', () => {
  it('types + sends 继续 and reports whether a failed-send bar appears', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });

    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('sr-no-radar', err);
    }
    await screenshot('sr-0-radar');

    if (!(await openFirstAgentDetail())) {
      return captureOnFailure('sr-no-detail', new Error('could not reach Detail'));
    }
    await settle(1000);

    // Terminal mode (default). The free-text input is hidden behind the ⌨ toggle —
    // tap it to reveal the TextInput + ↑ send button.
    const termTab = driver.$(`~${TestIds.detail.modeTerminal}`);
    if (await termTab.isExisting().catch(() => false)) {
      await termTab.click().catch(() => {});
      await settle(800);
    }
    await screenshot('sr-1-detail');

    // waitForExist, NOT waitForDisplayed: on a busy pane the terminal's hundreds
    // of <Text> rows overwhelm WDA's AX visibility queries ("Cannot determine
    // visiblity … Defaulting to: 0", 2-4s per probe), so isDisplayed returns a
    // false negative for a button that is plainly on screen (2026-08-08 run).
    // Existence + click is reliable — the click itself fails if it can't land.
    const kbd = driver.$(`~${TestIds.composer.keyboard}`);
    try {
      await kbd.waitForExist({timeout: 10_000});
      await kbd.click();
      await settle(700);
    } catch (err) {
      return captureOnFailure('sr-no-keyboard', err);
    }

    const input = driver.$(`~${TestIds.composer.input}`);
    try {
      await input.waitForDisplayed({timeout: 8_000});
    } catch (err) {
      return captureOnFailure('sr-no-composer', err);
    }
    await input.click();
    await settle(300);
    await input.addValue('继续');
    await settle(500);
    await screenshot('sr-2-typed');

    const sendBtn = driver.$(`~${TestIds.composer.send}`);
    await sendBtn.waitForDisplayed({timeout: 5_000});
    await sendBtn.click();
    await settle(3500);
    await screenshot('sr-3-after');

    const bars = await driver.$$('~send-failed-bar').catch(() => [] as unknown[]);
    const n = (bars as unknown[]).length;
    await screenshot('sr-4-final');

    const net = readDebugLog().filter(e => String((e as {path?: string}).path || '').includes('/api/send'));
    // eslint-disable-next-line no-console
    console.log('SEND_REPRO failed_bar_count =', n);
    // eslint-disable-next-line no-console
    console.log('SEND_REPRO net /api/send =', JSON.stringify(net, null, 2));

    expect(n).toBe(0);
  }, 180_000);
});
