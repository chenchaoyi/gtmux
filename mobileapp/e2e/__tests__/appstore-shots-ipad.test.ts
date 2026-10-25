import {execFileSync} from 'child_process';
import {mkdirSync, rmSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {demoEntrySelector} from '../setup/demoEntry';
import {storeCaptureTimeout} from '../setup/capabilities';
import {TestIds} from '../../src/constants/testIds';

/**
 * App Store screenshots for the 13" iPad slot, from DEMO mode on the iPad Pro 13"
 * simulator in landscape: the split shell, the HQ page with its inspector, All panes as a
 * grid, the knowledge sheet side by side. Gated on GTMUX_DEMO_SHOTS + an iPad device.
 * Raw PNGs land in .e2e-artifacts/appstore/ipad-<lang>/; frame them with
 *   node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-en \
 *     --lang ipad-en --out fastlane/screenshots/en-US --prefix ipad-
 *
 *   GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID="${AUDIT_IPAD_UDID:?the owned iPad simulator UDID}" \
 *   GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad
 */
const on = process.env.GTMUX_DEMO_SHOTS && /ipad/i.test(process.env.GTMUX_E2E_DEVICE || '');
const gated = on ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const LANG = process.env.GTMUX_SHOTS_LANG || 'en';
const OUT = resolve(__dirname, `../../.e2e-artifacts/appstore/ipad-${LANG}`);

function simctl(args: string[]): void {
  execFileSync('xcrun', ['simctl', ...args], {stdio: 'ignore'});
}
// Deleted before the run, so a run that stops early cannot leave last run's pictures
// for frame-shots.mjs to frame (as appstore-shots.test.ts).
const SHOTS = ['00-demo-entry', '01-split', '02-hq', '03-panes', '04-knowledge'];
function capture(file: string): void {
  simctl(['io', UDID, 'screenshot', file]);
  // simctl used to capture the panel's native PORTRAIT buffer even in landscape, so this
  // step turned the image upright. On iOS 26.5 it hands back an image that is already
  // upright, and turning it again laid the whole set on its side (2026-09-18). So ask the
  // file which way it came out instead of assuming: rotate only a portrait one.
  const dims = execFileSync('sips', ['-g', 'pixelWidth', '-g', 'pixelHeight', file]).toString();
  const w = Number(/pixelWidth:\s*(\d+)/.exec(dims)?.[1] ?? 0);
  const h = Number(/pixelHeight:\s*(\d+)/.exec(dims)?.[1] ?? 0);
  if (h > w) {
    execFileSync('sips', ['-r', '270', file], {stdio: 'ignore'});
  }
}
async function shot(name: string): Promise<void> {
  capture(join(OUT, `${name}.png`));
  if (process.env.GTMUX_STORE_REVIEW === '1') {
    mkdirSync(join(OUT, 'review'), {recursive: true});
    try {
      simctl(['ui', UDID, 'appearance', 'dark']);
      await settle(800);
      capture(join(OUT, 'review', `${name}.png`));
    } finally {
      simctl(['ui', UDID, 'appearance', 'light']);
      await settle(400);
    }
  }
  console.log(`[store capture ipad-${LANG}] ${new Date().toISOString()} wrote ${name}`);
}

gated('app store demo shots (iPad)', () => {
  it('captures the split shell, HQ with its inspector, the pane grid and the knowledge sheet', async () => {
    mkdirSync(OUT, {recursive: true});
    for (const name of SHOTS) rmSync(join(OUT, `${name}.png`), {force: true});
    simctl(['status_bar', UDID, 'override', '--time', '9:41', '--batteryState', 'charged',
      '--batteryLevel', '100', '--wifiBars', '3']);
    const driver = getDriver();
    await driver.setOrientation('LANDSCAPE').catch(() => {});
    await launchWithFlags({GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_SHOT_MODE: '1', GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_LANG: LANG});
    const demo = driver.$(demoEntrySelector(LANG));
    await demo.waitForDisplayed({timeout: 25_000});
    await shot('00-demo-entry');
    await demo.click();
    await driver.$(`~${TestIds.radar.split}`).waitForDisplayed({timeout: 20_000});
    // The hero (waiting on a permission) in the main pane, terminal mode.
    const hero = driver.$(`~${TestIds.agent.row}-%7`);
    await hero.waitForDisplayed({timeout: 10_000});
    await hero.click();
    await driver.$(`~${TestIds.detail.modeTerminal}`).click().catch(() => {});
    await settle(1800);
    await shot('01-split');

    await driver.$('~radar-hq-card').click();
    await driver.$('~hq-inspector').waitForDisplayed({timeout: 10_000});
    await driver.$('~hq-verdict').waitForDisplayed({timeout: 15_000});
    await driver.$('~hq-call-api:0.0').waitForDisplayed({timeout: 20_000});
    await shot('02-hq');

    await driver.$(`~${TestIds.radar.panes}`).click();
    await driver.$(`~${TestIds.panes.search}`).waitForDisplayed({timeout: 10_000});
    await settle(1400);
    await shot('03-panes');

    await driver.$('~radar-hq-card').click();
    await driver.$('~hq-knowledge-open').waitForDisplayed({timeout: 10_000});
    await driver.$('~hq-knowledge-open').click();
    await driver.$('~knowledge-find').waitForDisplayed({timeout: 10_000});
    // The demo always includes k2. Require its detail to load: a bare list screenshot
    // contradicts the caption and previously passed when the generic selector missed.
    const entry = driver.$('~knowledge-entry-k2');
    await entry.waitForDisplayed({timeout: 10_000});
    await entry.click();
    await driver.$('~knowledge-act-carry').waitForDisplayed({timeout: 10_000});
    await settle(1400);
    await shot('04-knowledge');
    await driver.$('~knowledge-close').click().catch(() => {});
    // eslint-disable-next-line no-console
    console.log(`[appstore-shots-ipad] wrote 01-split / 02-hq / 03-panes / 04-knowledge to ${OUT}`);
  }, storeCaptureTimeout());
});
