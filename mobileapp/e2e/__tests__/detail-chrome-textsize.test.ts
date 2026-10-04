import {execFileSync} from 'child_process';
import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// The Detail screen's top row (the Chat/Terminal switch and the pills beside it) at the
// accessibility text sizes. Measured on 2026-10-05: at the first accessibility size the
// switch broke "Terminal" over two lines, and at the largest the pills ran off the right
// edge, taking the full-screen button with them. The row follows the text size up to the
// largest standard one and stops there.
//
// Changes the simulator's text size, so it runs only on a simulator named for it
// (GTMUX_E2E_UDID), and puts the size back afterwards.
const udid = process.env.GTMUX_E2E_UDID;
const run = udid ? it : it.skip;
const size = (s: string) => execFileSync('xcrun', ['simctl', 'ui', udid!, 'content_size', s]);

let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  if (udid) size('large');
  await fake?.close();
});

async function openWaitingPane(): Promise<void> {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token, GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
  await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
  await settle(2000);
  await driver.$('~agent-row-%11').click();
  await driver.$(`~${TestIds.detail.modeTerminal}`).waitForDisplayed({timeout: 15_000});
  await settle(1500);
}

run('the top row keeps one line and stays on screen at the accessibility sizes', async () => {
  const driver = getDriver();
  size('large');
  await openWaitingPane();
  const base = await driver.$(`~${TestIds.detail.modeTerminal}`).getSize();
  const screenW = (await driver.getWindowSize()).width;

  for (const s of ['accessibility-medium', 'accessibility-extra-extra-extra-large']) {
    size(s);
    await openWaitingPane();
    await screenshot(`chrome-${s}`);
    const seg = await driver.$(`~${TestIds.detail.modeTerminal}`).getSize();
    // One line: the segment may grow with the capped text, but not to a second line.
    expect(seg.height).toBeLessThan(base.height * 1.6);
    // The last pill is the full-screen button: it has to be reachable, on screen.
    const fs = driver.$(`~${TestIds.detail.fullscreen}`);
    expect(await fs.isExisting()).toBe(true);
    const at = await fs.getLocation();
    const box = await fs.getSize();
    expect(at.x + box.width).toBeLessThanOrEqual(screenW);
  }
});
