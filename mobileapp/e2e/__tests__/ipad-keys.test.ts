import {execFileSync} from 'child_process';
import {mkdirSync} from 'fs';
import {join, resolve} from 'path';
import {getDriver} from '../setup/driver';
import {captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, readDebugLog, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

/**
 * Hardware-keyboard commands on the iPad (change ipad-universal-app, phase 3): the keymap
 * is installed as main-menu commands, so XCTest's hardware-keyboard path drives them the
 * way an attached keyboard would. ⌘⇧P opens All panes in the main pane, ⌘⇧H the HQ page,
 * ⌘2 the second radar row, ↓ moves the selection, ⌃⌘S hides the sidebar.
 *
 *   GTMUX_E2E_UDID=<ipad udid> GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' \
 *   GTMUX_E2E_URL=http://127.0.0.1:8765 GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
 *   npm run test:e2e -- ipad-keys
 */
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token && /ipad/i.test(process.env.GTMUX_E2E_DEVICE || '') ? describe : describe.skip;
const UDID = process.env.GTMUX_E2E_UDID || 'booted';
const OUT = resolve(__dirname, '../../.e2e-artifacts/ipad');

// XCUIKeyModifierFlags: shift 1<<17, control 1<<18, alternate 1<<19, command 1<<20.
const CMD = 1 << 20;
const SHIFT = 1 << 17;
const CTRL = 1 << 18;

function shot(name: string): void {
  execFileSync('xcrun', ['simctl', 'io', UDID, 'screenshot', join(OUT, `${name}.png`)], {stdio: 'ignore'});
}

async function press(key: string, modifierFlags = 0): Promise<void> {
  const driver = getDriver();
  await driver.execute('mobile: keys', {keys: [{key, modifierFlags}]});
  await settle(900);
}

gated('hardware keyboard on the iPad', () => {
  it('drives the shell from the keymap', async () => {
    mkdirSync(OUT, {recursive: true});
    const driver = getDriver();
    await driver.setOrientation('LANDSCAPE').catch(() => {});
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: url!, GTMUX_DEBUG_PAIR_TOKEN: token!, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LOG_NET: '1'});
    try {
      await driver.$(`~${TestIds.radar.split}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('ipad-keys-no-split', err);
    }
    await settle(2500);

    await press('p', CMD | SHIFT);
    // What the bridge saw: registration, then the command ids that reached JS.
    // eslint-disable-next-line no-console
    console.log('[keys]', JSON.stringify(readDebugLog().filter(e => String(e.event).startsWith('key'))));
    await driver.$(`~${TestIds.panes.search}`).waitForDisplayed({timeout: 8_000});
    shot('07-keys-panes');

    await press('h', CMD | SHIFT);
    await driver.$('~hq-knowledge-open').waitForDisplayed({timeout: 10_000});
    shot('08-keys-hq');

    await press('2', CMD);
    await driver.$(`~${TestIds.detail.modeChat}`).waitForDisplayed({timeout: 8_000});
    shot('09-keys-row2');

    await press('XCUIKeyboardKeyDownArrow');
    await settle(600);
    shot('10-keys-down');
    expect(await driver.$(`~${TestIds.detail.modeChat}`).isDisplayed()).toBe(true);

    await press('s', CMD | CTRL);
    await driver.$(`~${TestIds.radar.showSidebar}`).waitForDisplayed({timeout: 5_000});
    shot('11-keys-sidebar-hidden');
    await press('s', CMD | CTRL);
    await driver.$(`~${TestIds.radar.hideSidebar}`).waitForDisplayed({timeout: 5_000});
  });
});
