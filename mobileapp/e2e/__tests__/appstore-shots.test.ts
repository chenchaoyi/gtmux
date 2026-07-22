import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * App Store screenshot capture from DEMO mode — clean, generic sample data (never
 * the user's real project names), showcasing the polished colored terminal + HQ
 * intelligence headline + chief-of-staff screen. NOT a regression test; gated on
 * GTMUX_DEMO_SHOTS. Saves PNGs to mobileapp/.e2e-artifacts/appstore/<lang>/.
 *
 *   GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en \
 *   GTMUX_E2E_UDID=<booted 6.9" sim udid> npm run test:e2e
 *
 * Run once per locale (GTMUX_SHOTS_LANG=en|zh; set the sim's language to match first).
 */
const on = process.env.GTMUX_DEMO_SHOTS;
const gated = on ? describe : describe.skip;

const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const LANG = process.env.GTMUX_SHOTS_LANG || 'en';
const OUT = resolve(__dirname, `../../.e2e-artifacts/appstore/${LANG}`);
const DEMO_LABEL = LANG === 'zh' ? '没有 Mac？看看演示' : 'No Mac? See a demo';

function simctl(args: string[]): void {
  execFileSync('xcrun', ['simctl', ...args], {stdio: 'ignore'});
}
function shot(name: string): void {
  simctl(['io', UDID, 'screenshot', join(OUT, `${name}.png`)]);
}

gated('app store demo shots', () => {
  it('captures radar, colored terminal + approval, and the HQ command screen', async () => {
    mkdirSync(OUT, {recursive: true});
    // Marketing status bar (clean 9:41, full signal/battery).
    simctl(['status_bar', UDID, 'override', '--time', '9:41', '--batteryState', 'charged',
      '--batteryLevel', '100', '--cellularBars', '4', '--wifiBars', '3']);

    const driver = getDriver();
    // No pairing flags → the app opens on Servers and auto-presents the pairing
    // sheet with the "See a demo" card.
    await launchWithFlags({GTMUX_DEBUG_NO_PUSH: '1'});

    const demo = driver.$(`~${DEMO_LABEL}`);
    await demo.waitForDisplayed({timeout: 25_000});
    await demo.click();

    // 1) Demo radar — the HQ card (intelligence headline) + the fleet in the status
    //    language. The HQ card confirms the demo radar rendered.
    const hqCard = driver.$('~radar-hq-card');
    await hqCard.waitForDisplayed({timeout: 20_000});
    await settle(1400); // let icons + rows settle
    shot('01-radar');

    // 2) Detail — the flagship colored terminal + the approval card (%7 is waiting on
    //    a 1/2/3 permission). Switch to the Terminal tab so the colored mirror shows.
    const heroRow = driver.$(`~${TestIds.agent.row}-%7`);
    await heroRow.waitForDisplayed({timeout: 10_000});
    await heroRow.click();
    await driver.$(`~${TestIds.detail.screen}`).waitForDisplayed({timeout: 10_000});
    await driver.$(`~${TestIds.detail.modeTerminal}`).click().catch(() => {});
    await settle(1800); // let the pane content + approval card render
    shot('02-terminal-approval');

    await driver.$(`~${TestIds.detail.back}`).click();
    await hqCard.waitForDisplayed({timeout: 10_000});

    // 3) HQ command screen — the chief-of-staff differentiator (fleet board with
    //    telemetry + command console). Captured last so we never need its (untested)
    //    back control.
    await hqCard.click();
    await settle(1800);
    shot('03-hq');

    // eslint-disable-next-line no-console
    console.log(`[appstore-shots] wrote 01-radar / 02-terminal-approval / 03-hq to ${OUT}`);
  });
});
