import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Manual cursor-position check (NOT a regression assertion). Pairs to a live
 * serve, opens an agent's TERMINAL pane, and screenshots so we can eyeball where
 * the cyan cursor decoration lands vs the real input cursor — and confirm the
 * terminal is NOT black (the failure mode that #200 shipped to the device).
 *
 *   GTMUX_CURSOR=1 GTMUX_CURSOR_TAG=baseline \
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e -- -t cursor
 */
const on = process.env.GTMUX_CURSOR && process.env.GTMUX_E2E_URL && process.env.GTMUX_E2E_TOKEN;
const gated = on ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/cursor');
const TAG = process.env.GTMUX_CURSOR_TAG || 'shot';

gated('terminal cursor', () => {
  it('opens a terminal pane and screenshots the cursor', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: process.env.GTMUX_E2E_URL!,
      GTMUX_DEBUG_PAIR_TOKEN: process.env.GTMUX_E2E_TOKEN!,
      GTMUX_DEBUG_PAIR_NAME: 'cursor',
      GTMUX_DEBUG_NO_PUSH: '1',
    });

    const radar = driver.$(`~${TestIds.radar.screen}`);
    await radar.waitForDisplayed({timeout: 25_000});
    // Open the agent row at GTMUX_CURSOR_IDX (default 0). Index 1+ is typically an
    // IDLE agent whose input cursor is VISIBLE — needed to eyeball placement (the
    // first/working agent hides its cursor).
    const idx = parseInt(process.env.GTMUX_CURSOR_IDX || '0', 10);
    const rows = await driver.$$(`-ios predicate string:name BEGINSWITH '${TestIds.agent.row}-'`);
    await settle(800);
    await rows[idx].click();
    await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 8_000});

    // switch to the TERMINAL (xterm) mode
    const term = driver.$(`~${TestIds.detail.modeTerminal}`);
    await term.waitForDisplayed({timeout: 8_000});
    await term.click();
    await settle(3500); // let the webview pane render + cursor place

    execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${TAG}.png`)], {stdio: 'ignore'});
    // eslint-disable-next-line no-console
    console.log(`[cursor] wrote ${join(OUT, `${TAG}.png`)}`);
  });
});
