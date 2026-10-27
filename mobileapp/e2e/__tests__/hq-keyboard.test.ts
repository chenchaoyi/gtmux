import {execFileSync} from 'child_process';
import {getDriver} from '../setup/driver';
import {screenshot} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {startFake, Fake} from '../fake-serve/server';

const target = process.env.GTMUX_E2E_UDID;
const run = process.env.GTMUX_HQ_KEYBOARD_LANDSCAPE === '1' && target ? it : it.skip;
let fake: Fake;
beforeAll(async () => { fake = await startFake(); });
afterAll(async () => { await fake?.close(); });

run('keeps the HQ input visible with a large-text landscape software keyboard', async () => {
  const driver = getDriver();
  const ui = (...args: string[]) => execFileSync('xcrun', ['simctl', 'ui', target!, ...args], {encoding: 'utf8'}).trim();
  const originalSize = ui('content_size');
  const originalOrientation = await driver.getOrientation();
  try {
    await driver.setOrientation('PORTRAIT');
    expect((await driver.getWindowSize()).width).toBeLessThan(600);
    ui('content_size', 'accessibility-large');
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'zh'});
    const hq = driver.$('~radar-hq-disc');
    await hq.waitForDisplayed({timeout: 25_000});
    await hq.click();
    await driver.$('~hq-tab-console').click();
    await driver.setOrientation('LANDSCAPE');
    await driver.$('~composer-kbd').click();
    const input = driver.$('~composer-input');
    await input.setValue('Landscape draft');
    await settle(700);
    await screenshot('hq-landscape-large-input');
    expect(await driver.$('XCUIElementTypeKeyboard').isExisting()).toBe(true);
    expect(await input.isDisplayed()).toBe(true);
    expect(await driver.$('~hq-verdict').isDisplayed().catch(() => false)).toBe(false);
    await driver.setOrientation('PORTRAIT');
    expect(await input.getValue()).toBe('Landscape draft');
    await driver.$('~composer-kbd').click();
    await driver.$('~hq-verdict').waitForDisplayed({timeout: 6000});
  } finally {
    ui('content_size', originalSize);
    await driver.setOrientation(originalOrientation);
  }
});
