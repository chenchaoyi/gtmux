import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * The demo on an iPad is the regular shell over sample data (SURFACES.md §3, App Review's
 * only view of the iPad app): sidebar + main pane, the DEMO banner and the pairing call to
 * action inside the sidebar, a row opening in place, the HQ card opening the HQ page.
 *
 *   GTMUX_E2E_UDID=<ipad udid> GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- ipad-demo
 */
const gated = /ipad/i.test(process.env.GTMUX_E2E_DEVICE || '') ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/ipad');
const DEMO_LABELS = ['No Mac? See a demo', '没有 Mac？看看演示'];

function shot(name: string): void {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${name}.png`)], {stdio: 'ignore'});
}

gated('the demo on an iPad', () => {
  it('is the regular shell over sample data', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await driver.setOrientation('LANDSCAPE').catch(() => {});
    await launchWithFlags({GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_RESET_SERVERS: '1'});
    let opened = false;
    for (const label of DEMO_LABELS) {
      const demo = driver.$(`~${label}`);
      if (await demo.waitForDisplayed({timeout: 8_000}).catch(() => false)) {
        await demo.click();
        opened = true;
        break;
      }
    }
    expect(opened).toBe(true);
    await driver.$(`~${TestIds.radar.split}`).waitForDisplayed({timeout: 15_000});
    await settle(1500);
    shot('12-demo-landscape');
    // The demo's own affordances live inside the sidebar panel.
    expect(await driver.$('~Pair your Mac').isExisting() || await driver.$('~配对你的 Mac').isExisting()).toBe(true);
    expect(await driver.$(`~${TestIds.detail.back}`).isExisting()).toBe(false);
    // The HQ card opens the HQ page in the main pane.
    const hqCard = driver.$('~radar-hq-card');
    await hqCard.waitForDisplayed({timeout: 8_000});
    await hqCard.click();
    await driver.$('~hq-inspector').waitForDisplayed({timeout: 10_000});
    await settle(1000);
    shot('13-demo-hq');
    // All panes in the main pane — the browser polls without a navigator here (it used to
    // reach for the navigation focus and crash the demo on the iPad).
    await driver.$(`~${TestIds.radar.panes}`).click();
    await driver.$(`~${TestIds.panes.search}`).waitForDisplayed({timeout: 10_000});
    await settle(800);
    shot('14-demo-panes');
    // Leave the demo: the exit lives in the sidebar header.
    const exit = driver.$('~Close demo');
    if (await exit.isExisting()) await exit.click();
    else await driver.$('~关闭演示').click();
    await settle(800);
  });
});
