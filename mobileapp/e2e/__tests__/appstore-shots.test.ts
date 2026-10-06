import {execFileSync} from 'child_process';
import {mkdirSync, rmSync} from 'fs';
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
 *   GTMUX_E2E_UDID="${AUDIT_SIM_UDID:?the owned 6.9-inch simulator UDID}" npm run test:e2e
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
// Every shot this run must produce. They are deleted before the run and each is
// recorded only once simctl wrote it, so a step that does not happen cannot leave last
// run's picture in its place: a missing usage door used to skip 05-usage, keep the old
// PNG (which frame-shots.mjs then framed) and still log all six as written (%12, 2026-10-06).
const SHOTS = ['01-radar', '02-terminal-approval', '03-hq', '04-console', '05-usage', '06-servers'];
const written: string[] = [];
function shot(name: string): void {
  simctl(['io', UDID, 'screenshot', join(OUT, `${name}.png`)]);
  written.push(name);
}

gated('app store demo shots', () => {
  it('captures the radar, the terminal, HQ, its work, usage and the Macs', async () => {
    mkdirSync(OUT, {recursive: true});
    for (const name of SHOTS) rmSync(join(OUT, `${name}.png`), {force: true});
    // Marketing status bar (clean 9:41, full signal/battery).
    simctl(['status_bar', UDID, 'override', '--time', '9:41', '--batteryState', 'charged',
      '--batteryLevel', '100', '--cellularBars', '4', '--wifiBars', '3']);

    const driver = getDriver();
    // No pairing flags → the app opens on Servers and auto-presents the pairing
    // sheet with the "See a demo" card.
    await launchWithFlags({GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_SHOT_MODE: '1'});

    const demo = driver.$(`~${DEMO_LABEL}`);
    await demo.waitForDisplayed({timeout: 25_000});
    await demo.click();

    // 1) Demo radar — the fleet in the status language + the floating HQ DISC (SHOT_MODE
    //    renders the real disc, not the demo's HQ card). The disc confirms the radar rendered.
    const hqDisc = driver.$('~radar-hq-disc');
    await hqDisc.waitForDisplayed({timeout: 20_000});
    await settle(1400); // let icons + rows settle
    shot('01-radar');

    // 2) Detail — the flagship colored terminal + the approval card (%7 is waiting on
    //    a 1/2/3 permission). Switch to the Terminal tab so the colored mirror shows.
    const heroRow = driver.$(`~${TestIds.agent.row}-%7`);
    await heroRow.waitForDisplayed({timeout: 10_000});
    await heroRow.click();
    // Wait on a control the USER can see, not on the screen's root. The root is a
    // KeyboardAvoidingView; XCUITest reports it present but not `visible`, so waiting on
    // it timed out while the detail screen was fully rendered behind the check (its back
    // button, mode switch and the 1/2/3 approval card were all in the page source).
    await driver.$(`~${TestIds.detail.back}`).waitForDisplayed({timeout: 10_000});
    await driver.$(`~${TestIds.detail.modeTerminal}`).click().catch(() => {});
    await settle(1800); // let the pane content + approval card render
    shot('02-terminal-approval');

    await driver.$(`~${TestIds.detail.back}`).click();
    await hqDisc.waitForDisplayed({timeout: 10_000});

    // 3) HQ command page — the chief-of-staff differentiator: the assessment line +
    //    the your-call / activity / console zones (hq-command-page removed the old fleet
    //    board). Tapping the disc opens the real HQScreen.
    await hqDisc.click();
    await settle(1800);
    shot('03-hq');

    // 4) The console — HQ's words with the acts it recorded threaded in. The page's whole
    //    argument is that a supervisor which does not show what it did is a dashboard.
    //    The acts had their own tab until #1086 folded them in here; this step went on
    //    tapping `hq-tab-acts` and swallowing the failure, so it re-photographed the
    //    screen before it and shipped two identical store shots. Tap for real: a tab that
    //    stops existing must fail the capture, not pass it quietly.
    await driver.$('~hq-tab-console').click();
    await settle(1800);
    shot('04-console');

    // 5) Usage — the plan windows, which is the one number local counting cannot produce.
    //    Reached from the header's usage door, beside the board and the knowledge base.
    await driver.$('~hq-tab-calls').click().catch(() => {});
    await settle(500);
    // Required: a demo HQ page without its usage door is a capture that failed.
    const usageDoor = driver.$('~hq-usage-open');
    await usageDoor.waitForDisplayed({timeout: 10_000});
    await usageDoor.click();
    await settle(1400);
    shot('05-usage');
    await driver.$(`~${TestIds.detail.back}`).click().catch(() => {});
    await driver.$('~hq-usage-close').click().catch(() => {});
    await settle(600);

    // 4) Servers — the multi-Mac story: one phone managing agents across several Macs
    //    (own Macs full-control + a scoped guest connection). Seeded via GTMUX_DEBUG_SERVERS
    //    (no active → the app lands on the two-track Servers page); SHOT_MODE greens the
    //    first row's connected dot.
    const servers = JSON.stringify([
      {url: 'dev-mbp.local:8765', token: 'demo', name: 'MacBook Pro'},
      {url: 'studio.local:8765', token: 'demo', name: 'Mac Studio'},
      {url: 'mac-mini.local:8765', token: 'demo', name: 'Office mini'},
      {url: 'ana-mac.local:8765', token: 'demo', name: "Ana's Mac", scope: 'guest'},
    ]);
    await launchWithFlags({GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_SHOT_MODE: '1', GTMUX_DEBUG_SERVERS: servers});
    // A release on an already-used simulator can open What's New on this second
    // cold launch. Dismiss it before looking for the switch behind the modal.
    const whatsNewDismiss = driver.$(`~${LANG === 'zh' ? '知道了' : 'Got it'}`);
    if (await whatsNewDismiss.isDisplayed()) await whatsNewDismiss.click();
    // The first seeded Mac is active, so the app opens on its radar — hop to the Servers
    // page via the header's server chip (the two-track My-Macs / Guests list).
    const chip = driver.$(`~${TestIds.radar.serverChip}`);
    await chip.waitForDisplayed({timeout: 15_000});
    await chip.click();
    await driver.$(`~${TestIds.servers.screen}`).waitForDisplayed({timeout: 15_000});
    await settle(900);
    shot('06-servers');

    expect(written).toEqual(SHOTS);
    // eslint-disable-next-line no-console
    console.log(`[appstore-shots] wrote ${written.join(' / ')} to ${OUT}`);
  });
});
