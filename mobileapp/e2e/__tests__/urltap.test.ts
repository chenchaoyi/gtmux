import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

// Manual interaction check for TAPPABLE URLs in the native terminal. Seed a pane with a
// URL (e.g. `tmux new-session -d -s urltest "printf 'x https://example.com/a-b\n'; sleep 1d"`),
// then open it via the pane browser (GTMUX_PANE=%N). Screenshots the terminal (eyeball:
// the URL is underlined), and with GTMUX_TAP_FX/FY taps there and logs the app state
// (foreground=4; a URL that opened the browser → background=2/3). GTMUX_LP=1 long-presses
// instead, to confirm text selection (the Copy callout) still works. Gating like
// nativeterm.test.ts. Regression guard for the "overlay swallowed the tap" fix.
const on = process.env.GTMUX_URLTAP && process.env.GTMUX_E2E_URL && process.env.GTMUX_E2E_TOKEN;
const gated = on ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const PANE = process.env.GTMUX_PANE || '';
const OUT = resolve(__dirname, '../../.e2e-artifacts/urltap');
function shot(tag: string) {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${tag}.png`)], {stdio: 'ignore'});
  // eslint-disable-next-line no-console
  console.log(`[urltap] wrote ${tag}.png`);
}

gated('url tap', () => {
  it('open the url pane, screenshot, optionally tap', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: process.env.GTMUX_E2E_URL!,
      GTMUX_DEBUG_PAIR_TOKEN: process.env.GTMUX_E2E_TOKEN!,
      GTMUX_DEBUG_PAIR_NAME: 'urltap',
      GTMUX_DEBUG_NO_PUSH: '1',
    });
    await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    await settle(1000);
    await driver.$(`~${TestIds.radar.panes}`).click();
    await driver.$(`~${TestIds.panes.screen}`).waitForDisplayed({timeout: 8_000});
    await settle(800);
    await driver.$(`~${TestIds.panes.row}-${PANE}`).click();
    await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 8_000});
    // A plain (non-agent) pane opens straight to the terminal — no chat/terminal toggle.
    const term = driver.$(`~${TestIds.detail.modeTerminal}`);
    if (await term.isExisting()) {
      await term.click();
    }
    await driver.$(`~${TestIds.detail.pane}`).waitForDisplayed({timeout: 8_000});
    await settle(2500);
    shot('01-urlpane');

    const fx = parseFloat(process.env.GTMUX_TAP_FX || '0');
    const fy = parseFloat(process.env.GTMUX_TAP_FY || '0');
    if (fx && fy) {
      const {width, height} = await driver.getWindowRect();
      const tx = Math.round(fx * width);
      const ty = Math.round(fy * height);
      // eslint-disable-next-line no-console
      console.log(`[urltap] tap at (${tx},${ty}) of ${width}x${height}`);
      // eslint-disable-next-line no-console
      console.log('[urltap] app state BEFORE tap (4=foreground):', await driver.queryAppState('com.gtmux.app'));
      const hold = process.env.GTMUX_LP === '1';
      await driver.action('pointer').move({x: tx, y: ty}).down().pause(hold ? 900 : 50).up().perform();
      await settle(hold ? 1500 : 2500);
      if (hold) {
        // eslint-disable-next-line no-console
        console.log('[urltap] long-press — eyeball 02-longpress for the selection highlight');
        shot('02-longpress');
      } else {
        // eslint-disable-next-line no-console
        console.log('[urltap] app state AFTER tap (2/3=backgrounded→Safari opened):', await driver.queryAppState('com.gtmux.app'));
        shot('02-after-tap');
      }
    }
  });
});
