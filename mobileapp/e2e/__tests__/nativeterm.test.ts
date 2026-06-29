import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Manual interaction check for the NATIVE terminal renderer (NativeTerm). Pairs to
 * a live serve, opens an agent's Terminal pane, then:
 *   1. taps the pane            → screenshot (expect: NO soft keyboard pops up)
 *   2. long-presses the pane    → screenshot (expect: iOS selection + Copy callout)
 * Eyeball the .e2e-artifacts/nativeterm/*.png. Same gating/env as cursor.test.ts.
 *
 *   GTMUX_NT=1 GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   GTMUX_E2E_UDID=<booted> GTMUX_NT_IDX=1 npm run test:e2e -- -t nativeterm
 */
const on = process.env.GTMUX_NT && process.env.GTMUX_E2E_URL && process.env.GTMUX_E2E_TOKEN;
const gated = on ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/nativeterm');

function shot(tag: string) {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${tag}.png`)], {stdio: 'ignore'});
  // eslint-disable-next-line no-console
  console.log(`[nativeterm] wrote ${join(OUT, `${tag}.png`)}`);
}

gated('native terminal', () => {
  it('tap / long-press / wrap on the native pane', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: process.env.GTMUX_E2E_URL!,
      GTMUX_DEBUG_PAIR_TOKEN: process.env.GTMUX_E2E_TOKEN!,
      GTMUX_DEBUG_PAIR_NAME: 'nativeterm',
      GTMUX_DEBUG_NO_PUSH: '1',
    });

    await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    const idx = parseInt(process.env.GTMUX_NT_IDX || '1', 10);
    const rows = await driver.$$(`-ios predicate string:name BEGINSWITH '${TestIds.agent.row}-'`);
    await settle(800);
    await rows[idx].click();
    await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 8_000});
    const term = driver.$(`~${TestIds.detail.modeTerminal}`);
    await term.waitForDisplayed({timeout: 8_000});
    await term.click();
    await settle(2500);

    const {width, height} = await driver.getWindowRect();
    const cx = Math.round(width / 2);
    const cy = Math.round(height * 0.45); // mid-pane

    // 1. a plain tap — must NOT raise the keyboard (no hidden textarea on native).
    await driver.action('pointer').move({x: cx, y: cy}).down().pause(60).up().perform();
    await settle(1200);
    shot('tap-no-keyboard');

    // 2. long-press — should bring up iOS text selection + the Copy callout.
    await driver.action('pointer').move({x: cx, y: cy}).down().pause(900).up().perform();
    await settle(1200);
    shot('longpress-select');
  });
});
