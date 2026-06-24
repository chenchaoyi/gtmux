import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, openFirstAgentDetail, readDebugLog} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Deeper UI-scenario test, powered by the launch-arg debug layer
 * (src/debug + native DebugSettings). Gated on env so the committed test holds
 * no secret — run it against a live `gtmux serve`:
 *
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e
 *
 * It launches with GTMUX_DEBUG_PAIR_* (auto-pair, skip the manual pairing
 * screen), GTMUX_DEBUG_NO_PUSH (no permission prompt over the UI), and
 * GTMUX_DEBUG_LOG_NET (record every API call). It drives radar → a pane's
 * Detail, then asserts on the recorded NETWORK log — exercising the UI and its
 * underlying calls together, the way a user scenario would.
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('radar (live, debug-driven)', () => {
  it('auto-pairs, opens a pane, and the network calls are recorded', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });

    // 1. Auto-pair lands straight on the radar.
    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('no-radar', err);
    }
    await screenshot('radar');

    // 2. Open the first agent row → Detail (retry-wrapped against SSE re-renders).
    if (!(await openFirstAgentDetail())) {
      return captureOnFailure('no-detail', new Error('could not reach Detail after retries'));
    }
    const back = driver.$(`~${TestIds.detail.back}`);
    await screenshot('detail');

    // 3. Back to the radar.
    await back.click();
    await radar.waitForDisplayed({timeout: 10_000});

    // 4. Assert the UNDERLYING network calls happened (debug log written by the
    //    app to Documents/gtmux-debug.jsonl, read back via simctl). This is the
    //    "deeper" check: the UI flow drove real /api/agents + /api/pane calls.
    const log = readDebugLog();
    const net = log.filter(e => e.event === 'net');
    const paths = net.map(e => String(e.path));
    expect(paths.some(p => p.startsWith('/api/agents'))).toBe(true);
    expect(paths.some(p => p.startsWith('/api/pane'))).toBe(true);
    // None should be a client/server error.
    const bad = net.filter(e => typeof e.status === 'number' && (e.status as number) >= 400);
    if (bad.length) {
      // eslint-disable-next-line no-console
      console.error('[radar] non-2xx API calls:', JSON.stringify(bad));
    }
    expect(bad).toHaveLength(0);
  });
});
