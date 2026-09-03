import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * A dropped stream must heal itself (hq's list).
 *
 * The live connection is an SSE stream, and it does not survive a tunnel hiccup, a phone
 * that slept, or a Mac that briefly went away. If the app needs a relaunch to come back,
 * a reader cannot tell that from a Mac that is genuinely gone — the screen says the same
 * thing either way, and the wrong one of those two is the one you act on.
 *
 * Only testable against a fake: cutting the real serve's stream would cut the commander's
 * own session.
 */
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

// The connection state is spelled out only in the dot's accessibility label — the chip
// deliberately shows no "live" word (DESIGN D9), so a screenshot cannot tell the states
// apart either. The label is a whole sentence ("Connection: connected"), so it is read
// off the element rather than looked up as a name.
const CONNECTED = ['connected', '已连接'];
const state = async (): Promise<string> => {
  const driver = getDriver();
  // Read it out of the page source rather than by element type: the node's XCUI class
  // depends on how RN flattens the view, and pinning that would make the test about the
  // renderer instead of about the connection.
  const src = await driver.getPageSource();
  const m = /name="(Connection:[^"]*)"/.exec(src) ?? /name="(连接：[^"]*)"/.exec(src);
  return m ? m[1] : '(none)';
};
const isConnected = (s: string) => CONNECTED.some(w => s.includes(w));

describe('a dropped live stream', () => {
  it('comes back to connected without a relaunch', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('sse-no-radar', err);
    }
    await settle(3000);
    const first = await state();
    // eslint-disable-next-line no-console
    console.log('[sse] before:', first);
    expect(isConnected(first)).toBe(true);

    const dropped = fake.dropStreams();
    // eslint-disable-next-line no-console
    console.log('[sse] dropped', dropped, 'stream(s)');
    expect(dropped).toBeGreaterThan(0);
    await settle(1500);
    await screenshot('sse-1-dropped');

    // Give the client its retry budget. What matters is that it gets there on its own.
    let back = '';
    for (let i = 0; i < 20; i++) {
      back = await state();
      if (isConnected(back)) break;
      await settle(1000);
    }
    await screenshot('sse-2-back');
    // eslint-disable-next-line no-console
    console.log('[sse] state after the drop:', back);
    expect(isConnected(back)).toBe(true);
  });
});
