import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * The Mac goes away entirely, and comes back at the same address.
 *
 * This is the outage the phone actually meets: every `gtmux update` restarts serve, a
 * sleeping Mac stops answering, a tunnel blinks. It is NOT the same shape as the dropped
 * stream in sse-reconnect.test.ts, and the difference is the whole point — a stream that
 * ENDS cleanly makes react-native-sse poll again, while a connection that cannot be made
 * at all lands in `xhr.onerror`, which dispatches an error and stops retrying. Found on
 * 2026-09-20 in a simulator round: with the serve gone for 25 seconds and then back, the
 * app still said "Can't reach" half a minute later, and a fleet change never arrived.
 *
 * The app now rebuilds the subscription itself, with a backoff, and reads the fleet over
 * HTTP on every attempt.
 */
let fake: Fake;
let port = 0;

beforeAll(async () => {
  fake = await startFake();
  port = fake.port;
});
afterAll(async () => {
  await fake?.close();
});

const state = async (): Promise<string> => {
  const src = await getDriver().getPageSource();
  const m = /name="(Connection:[^"]*)"/.exec(src) ?? /name="(连接：[^"]*)"/.exec(src);
  return m ? m[1] : '(none)';
};
const connected = (s: string) => ['connected', '已连接'].some(w => s.includes(w));

const waitFor = async (want: (s: string) => boolean, seconds: number): Promise<string> => {
  let last = '';
  for (let i = 0; i < seconds; i++) {
    last = await state();
    if (want(last)) return last;
    await settle(1000);
  }
  return last;
};

describe('a Mac that goes away and comes back', () => {
  it('reconnects on its own, without a relaunch', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('restart-no-radar', err);
    }
    await settle(3000);
    expect(connected(await state())).toBe(true);

    // The whole serve disappears: not a dropped response, a refused connection.
    await fake.close();
    const gone = await waitFor(s => !connected(s), 20);
    // eslint-disable-next-line no-console
    console.log('[restart] while it was gone:', gone);
    await screenshot('restart-1-gone');
    expect(connected(gone)).toBe(false);

    // Back at the same address, as a restarted serve is.
    fake = await startFake({port});
    const back = await waitFor(connected, 60);
    await screenshot('restart-2-back');
    // eslint-disable-next-line no-console
    console.log('[restart] after it came back:', back);
    expect(connected(back)).toBe(true);

    // And the stream is real, not just a chip that says so: a fleet change has to arrive
    // without anyone pulling to refresh. The proof is the COUNT line — a radar row
    // collapses to its testID in the accessibility tree, so the row's own text cannot be
    // read back, while the summary above it can.
    const summary = async (): Promise<string> => {
      const els = await driver.$$('-ios class chain:**/XCUIElementTypeStaticText');
      for (const el of els) {
        try {
          const l = (await el.getAttribute('label')) ?? '';
          if (l.includes('waiting') || l.includes('等你')) return l;
        } catch {
          /* re-render */
        }
      }
      return '';
    };
    const before = await summary();
    const row = fake.world.agent('%13');
    if (row) row.status = 'waiting';
    fake.bumpAgents();
    let changed = false;
    for (let i = 0; i < 15; i++) {
      await settle(1000);
      const now = await summary();
      if (now && now !== before) {
        changed = true;
        // eslint-disable-next-line no-console
        console.log('[restart] the fleet line changed:', before, '→', now);
        break;
      }
    }
    await screenshot('restart-3-change-arrived');
    expect(changed).toBe(true);
  }, 240_000);
});
