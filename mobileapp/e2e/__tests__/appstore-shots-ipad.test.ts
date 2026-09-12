import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * App Store screenshots for the 13" iPad slot, from DEMO mode on the iPad Pro 13"
 * simulator in landscape: the split shell, the HQ page with its inspector, All panes as a
 * grid, the knowledge sheet side by side. Gated on GTMUX_DEMO_SHOTS + an iPad device.
 * Raw PNGs land in .e2e-artifacts/appstore/ipad-<lang>/; frame them with
 *   node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-en \
 *     --lang ipad-en --out fastlane/screenshots/en-US --prefix ipad-
 *
 *   GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID=<ipad udid> \
 *   GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad
 */
const on = process.env.GTMUX_DEMO_SHOTS && /ipad/i.test(process.env.GTMUX_E2E_DEVICE || '');
const gated = on ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const LANG = process.env.GTMUX_SHOTS_LANG || 'en';
const OUT = resolve(__dirname, `../../.e2e-artifacts/appstore/ipad-${LANG}`);
const DEMO_LABEL = LANG === 'zh' ? '没有 Mac？看看演示' : 'No Mac? See a demo';

function simctl(args: string[]): void {
  execFileSync('xcrun', ['simctl', ...args], {stdio: 'ignore'});
}
function shot(name: string): void {
  const file = join(OUT, `${name}.png`);
  simctl(['io', UDID, 'screenshot', file]);
  // simctl captures the panel's native (portrait) buffer even in landscape: the app comes
  // out rotated 90° clockwise. Turn it upright so the frame gets a landscape image.
  execFileSync('sips', ['-r', '270', file], {stdio: 'ignore'});
}

gated('app store demo shots (iPad)', () => {
  it('captures the split shell, HQ with its inspector, the pane grid and the knowledge sheet', async () => {
    mkdirSync(OUT, {recursive: true});
    simctl(['status_bar', UDID, 'override', '--time', '9:41', '--batteryState', 'charged',
      '--batteryLevel', '100', '--wifiBars', '3']);
    const driver = getDriver();
    await driver.setOrientation('LANDSCAPE').catch(() => {});
    await launchWithFlags({GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_SHOT_MODE: '1', GTMUX_DEBUG_LANG: LANG});
    const demo = driver.$(`~${DEMO_LABEL}`);
    await demo.waitForDisplayed({timeout: 25_000});
    await demo.click();
    await driver.$(`~${TestIds.radar.split}`).waitForDisplayed({timeout: 20_000});
    // The hero (waiting on a permission) in the main pane, terminal mode.
    const hero = driver.$(`~${TestIds.agent.row}-%7`);
    await hero.waitForDisplayed({timeout: 10_000});
    await hero.click();
    await driver.$(`~${TestIds.detail.modeTerminal}`).click().catch(() => {});
    await settle(1800);
    shot('01-split');

    await driver.$('~radar-hq-card').click();
    await driver.$('~hq-inspector').waitForDisplayed({timeout: 10_000});
    await settle(1600);
    shot('02-hq');

    await driver.$(`~${TestIds.radar.panes}`).click();
    await driver.$(`~${TestIds.panes.search}`).waitForDisplayed({timeout: 10_000});
    await settle(1400);
    shot('03-panes');

    await driver.$('~radar-hq-card').click();
    await driver.$('~hq-knowledge-open').waitForDisplayed({timeout: 10_000});
    await driver.$('~hq-knowledge-open').click();
    await driver.$('~knowledge-find').waitForDisplayed({timeout: 10_000});
    const entry = driver.$("-ios predicate string:name BEGINSWITH 'knowledge-entry-'");
    if (await entry.waitForDisplayed({timeout: 5_000}).catch(() => false)) await entry.click();
    await settle(1400);
    shot('04-knowledge');
    await driver.$('~knowledge-close').click().catch(() => {});
    // eslint-disable-next-line no-console
    console.log(`[appstore-shots-ipad] wrote 01-split / 02-hq / 03-panes / 04-knowledge to ${OUT}`);
  });
});
