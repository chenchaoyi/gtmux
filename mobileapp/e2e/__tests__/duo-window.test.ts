import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {screenshot} from '../setup/screenshot';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';

// Tests rotation and typing on the installed Duo simulator. This does not claim
// an open/close pose: Device Hub's fold controls need a separate observed test.
let fake: Fake;
beforeAll(async () => { fake = await startFake(); fake.world.seedLongChat('%13', 12); });
afterAll(async () => { await fake?.close(); });

it('keeps the selected session and a draft through simulator rotation', async () => {
  const driver = getDriver();
  const original = await driver.getOrientation();
  try {
    await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
    const row = driver.$('~agent-row-%13');
    await row.waitForDisplayed({timeout: 25000});
    await row.click();
    const chat = driver.$(`~${TestIds.detail.modeChat}`);
    await chat.waitForDisplayed({timeout: 10000});
    await chat.click();
    const keyboard = driver.$(`~${TestIds.composer.keyboard}`);
    await keyboard.waitForDisplayed({timeout: 10000});
    await keyboard.click();
    const input = driver.$(`~${TestIds.composer.input}`);
    await input.waitForDisplayed({timeout: 10000});
    await input.setValue('Duo draft fixture');
    expect(await input.getValue()).toBe('Duo draft fixture');
    expect(await driver.$('XCUIElementTypeKeyboard').isExisting()).toBe(true);
    await screenshot('duo-draft-before-rotation');
    const next = original === 'PORTRAIT' ? 'LANDSCAPE' : 'PORTRAIT';
    await driver.setOrientation(next);
    await settle(1500);
    expect(await driver.getOrientation()).toBe(next);
    expect(await driver.$(`~${TestIds.composer.input}`).getValue()).toBe('Duo draft fixture');
    expect(await driver.$(`~${TestIds.detail.modeChat}`).isExisting()).toBe(true);
    await screenshot('duo-draft-after-rotation');
  } finally {
    await driver.setOrientation(original);
  }
});
