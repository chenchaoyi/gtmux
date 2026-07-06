import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, openFirstAgentDetail} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * README screenshot capture — NOT a regression test. Drives the app through its
 * key screens and saves polished PNGs (clean 9:41 status bar) to
 * mobileapp/.e2e-artifacts/shots/. Gated on GTMUX_SHOTS so the normal suite
 * skips it. Regenerate the README images with:
 *
 *   GTMUX_SHOTS=1 GTMUX_E2E_URL=http://127.0.0.1:8765 \
 *   GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   GTMUX_E2E_UDID=<booted-udid> npm run test:e2e
 */
const on = process.env.GTMUX_SHOTS && process.env.GTMUX_E2E_URL && process.env.GTMUX_E2E_TOKEN;
const gated = on ? describe : describe.skip;

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const NAME = process.env.GTMUX_SHOTS_NAME || 'ccy-mac';
const OUT = resolve(__dirname, '../../.e2e-artifacts/shots');

function simctl(args: string[]): void {
  execFileSync('xcrun', ['simctl', ...args], {stdio: 'ignore'});
}
function shot(name: string): void {
  simctl(['io', UDID, 'screenshot', join(OUT, `${name}.png`)]);
}

gated('readme screenshots', () => {
  it('captures radar, detail, and the connection page', async () => {
    mkdirSync(OUT, {recursive: true});
    // Polished marketing status bar.
    simctl(['status_bar', UDID, 'override', '--time', '9:41', '--batteryState', 'charged',
      '--batteryLevel', '100', '--cellularBars', '4', '--wifiBars', '3']);

    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: process.env.GTMUX_E2E_URL!,
      GTMUX_DEBUG_PAIR_TOKEN: process.env.GTMUX_E2E_TOKEN!,
      GTMUX_DEBUG_PAIR_NAME: NAME,
      GTMUX_DEBUG_NO_PUSH: '1',
    });

    // Radar
    const radar = driver.$(`~${TestIds.radar.screen}`);
    await radar.waitForDisplayed({timeout: 25_000});
    await new Promise(r => setTimeout(r, 1200)); // let icons + rows settle
    shot('radar');

    // Detail — switch to the Terminal tab so the shot shows the live screen in
    // color (Chat is empty without a session log).
    if (!(await openFirstAgentDetail())) throw new Error('could not open an agent Detail');
    await driver.$(`~${TestIds.detail.modeTerminal}`).click();
    await new Promise(r => setTimeout(r, 1800)); // let the pane content render
    shot('detail');
    await driver.$(`~${TestIds.detail.back}`).click();
    await radar.waitForDisplayed({timeout: 10_000});

    // Connection page (multi-server)
    await driver.$(`~${TestIds.radar.serverChip}`).click();
    await driver.$(`~${TestIds.servers.screen}`).waitForDisplayed({timeout: 10_000});
    await new Promise(r => setTimeout(r, 600));
    shot('servers');

    // eslint-disable-next-line no-console
    console.log(`[shots] wrote radar/detail/servers to ${OUT}`);
  });
});
