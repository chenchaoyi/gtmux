import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, readDebugLog, settle, typeInto} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

/**
 * Self-verification for the 2026-08-08 key-row changes:
 *   1. the ␣ Space pill is GONE (it did nothing useful — user call),
 *   2. ⏎ sits to the RIGHT of ↑/↓ (navigate → commit reading order),
 *   3. a ⌫ Backspace pill exists and actually ERASES in the pane — sent as tmux
 *      `BSpace` via POST /api/send (allowlisted server-side since v0.10.0).
 *
 * Targets a plain shell in its own session, `qa-keyrow`, opened through the pane
 * browser. It used to be a throwaway tmux session on the machine running the test; it is
 * now a pane of the in-process fake (world.seedShell), whose input line takes the keys
 * the app sends, so the ⌫ tap is still verified end to end: seed "abc" into the pane's
 * input, tap ⌫ in the app, and the pane's line must read "ab".
 */
const SESSION = 'qa-keyrow';
let fake: Fake;
let shell: string;
beforeAll(async () => {
  fake = await startFake();
  shell = fake.world.seedShell(SESSION);
});
afterAll(async () => {
  await fake?.close();
});

describe('composer key row (debug-driven)', () => {
  it('has no Space pill, orders ⏎ after ↑/↓, and ⌫ erases in the pane', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LOG_NET: '1',
    });

    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('kr-no-radar', err);
    }

    // → pane browser → search the shell's session → open its (only) pane.
    // typeInto verifies what landed (XCUITest setValue can drop characters).
    await driver.$(`~${TestIds.radar.panes}`).click();
    await settle(1200);
    await typeInto(TestIds.panes.search, SESSION);
    await settle(900);
    // The shell's own row: the browser lists it as a PLAIN pane, which the fake's
    // /api/panes used to have no way to say (it sent no `tier`).
    const row = driver.$(`~${TestIds.panes.row}-${shell}`);
    try {
      await row.waitForExist({timeout: 8_000});
      await row.click();
      await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 8_000});
    } catch (err) {
      return captureOnFailure('kr-no-pane', err);
    }
    await settle(1500); // first pane poll

    // 1. ␣ is gone; ⌫ exists. (waitForExist, not waitForDisplayed — a busy
    // terminal's AX flood makes visibility probes flaky; see send-repro.)
    const keyId = (k: string) => `${TestIds.composer.controlKey}-${k}`;
    await driver.$(`~${keyId('BSpace')}`).waitForExist({timeout: 10_000});
    expect(await driver.$(`~${keyId('Space')}`).isExisting()).toBe(false);

    // 2. Resting-row order: Tab < ↑ < ↓ < ⏎ < ⌫ by x-position.
    const xs: number[] = [];
    for (const k of ['Tab', 'Up', 'Down', 'Enter', 'BSpace']) {
      const el = driver.$(`~${keyId(k)}`);
      await el.waitForExist({timeout: 5_000});
      xs.push((await el.getLocation()).x);
    }
    for (let i = 1; i < xs.length; i++) {
      expect(xs[i]).toBeGreaterThan(xs[i - 1]);
    }
    await screenshot('kr-keyrow-after');

    // 3. ⌫ end-to-end: seed "abc" into the pane's input (literal, no Enter),
    // tap the pill, and the pane's line must lose exactly the "c".
    //
    // The tap is a RAW W3C pointer tap at the pill's frame center — NOT
    // element.click(). WDA's element click on these small pills in the
    // horizontal ScrollView is unreliable: its AX snapshot sometimes carries
    // stale/duplicated frames (observed: Down and C-c reporting the identical
    // rect), and the click then lands on a neighboring pill (2026-08-08 run:
    // click 200 but NO /api/send ever left the app). The frame we tap was just
    // sanity-checked by the strict x-ordering above, so its center is the
    // visually-correct pill; a real finger taps what it sees, and so does this.
    const bs = driver.$(`~${keyId('BSpace')}`);
    const loc = await bs.getLocation();
    const size = await bs.getSize();
    const tapX = Math.round(loc.x + size.width / 2);
    const tapY = Math.round(loc.y + size.height / 2);
    const capture = () => fake.world.paneText(shell) ?? '';
    fake.world.shellInput(shell, {text: 'abc'}); // typed at the Mac, not through the app
    await settle(800);
    const before = capture().trimEnd();
    await driver
      .action('pointer', {parameters: {pointerType: 'touch'}})
      .move({x: tapX, y: tapY})
      .down()
      .pause(80)
      .up()
      .perform();
    await settle(1800); // /api/send + the pane poll
    const after = capture().trimEnd();
    const lastLine = (t: string) => t.split('\n').filter(l => l.trim() !== '').pop() || '';
    expect(lastLine(before)).toMatch(/abc$/);
    expect(lastLine(after)).toMatch(/ab$/);
    expect(lastLine(after)).not.toMatch(/abc$/);
    // What reached the pane was the key itself, addressed to this pane.
    expect(fake.world.writesTo('/api/send')).toContainEqual(expect.objectContaining({id: shell, key: 'BSpace'}));

    // And the app's own net log shows the key send was ACCEPTED (2xx). An older
    // serve without BSpace in its allowlist would 400 here — that would be a
    // real finding, not a flake, so it fails loudly.
    const sends = readDebugLog().filter(
      e => e.event === 'net' && String(e.path).startsWith('/api/send'),
    );
    expect(sends.length).toBeGreaterThan(0);
    const bad = sends.filter(e => typeof e.status === 'number' && (e.status as number) >= 400);
    if (bad.length) {
      // eslint-disable-next-line no-console
      console.error('[keyrow] rejected /api/send:', JSON.stringify(bad));
    }
    expect(bad).toHaveLength(0);
    await screenshot('kr-after-bspace');
  }, 180_000);
});
