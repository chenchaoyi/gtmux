import {getDriver} from '../setup/driver';
import {launchWithFlags, settle} from '../setup/app';
import {captureOnFailure, screenshot} from '../setup/screenshot';
import {selectUnfoldedInnerDisplay} from '../setup/duoDisplay';
import {startFake, Fake} from '../fake-serve/server';
import {TestIds} from '../../src/constants/testIds';

// Tests rotation and typing on the installed Duo simulator. This does not claim
// an open/close pose: Device Hub's fold controls need a separate observed test.
let fake: Fake;
beforeAll(async () => { fake = await startFake(); fake.world.seedLongChat('%13', 12); });
beforeEach(async () => { await selectUnfoldedInnerDisplay(); }, 60000);
afterAll(async () => { await fake?.close(); });

it('keeps the selected session and a draft through simulator rotation', async () => {
  const driver = getDriver();
  await launchWithFlags({GTMUX_DEBUG_PAIR_URL: fake.url, GTMUX_DEBUG_PAIR_TOKEN: fake.token,
    GTMUX_DEBUG_NO_PUSH: '1', GTMUX_DEBUG_LANG: 'en'});
  const original = await driver.getOrientation();
  try {
    const row = driver.$('~agent-row-%13');
    await row.waitForDisplayed({timeout: 25000});
    await row.click();
    const selected = driver.$('~Show full title, 整理这周的发布清单');
    await selected.waitForDisplayed({timeout: 10000});
    const chat = driver.$(`~${TestIds.detail.modeChat}`);
    await chat.waitForDisplayed({timeout: 10000});
    await chat.click();
    // Prove a touch drag moves an actual conversation row, not just that the
    // automation command returned. The seeded conversation is intentionally long.
    await settle(1200);
    const bubbles = await driver.$$('~chat-user-bubble').getElements();
    expect(bubbles.length).toBeGreaterThan(1);
    const anchor = bubbles[bubbles.length - 1];
    const beforeY = (await anchor.getLocation()).y;
    const chatView = driver.$(`~${TestIds.detail.chat}`);
    const canvas = {...await chatView.getLocation(), ...await chatView.getSize()};
    const x = Math.round(canvas.x + canvas.width / 2);
    await driver.action('pointer', {parameters: {pointerType: 'touch'}})
      .move({x, y: Math.round(canvas.y + canvas.height * 0.35)}).down().pause(80)
      .move({x, y: Math.round(canvas.y + canvas.height * 0.75), duration: 350}).up().perform();
    await settle(700);
    expect(Math.abs((await anchor.getLocation()).y - beforeY)).toBeGreaterThan(10);
    await screenshot('duo-scrolled-history');
    const jump = driver.$(`~${TestIds.detail.jumpBottom}`);
    await jump.waitForDisplayed({timeout: 8000});
    await jump.click();
    await settle(800);
    const keyboard = driver.$(`~${TestIds.composer.keyboard}`);
    await keyboard.waitForDisplayed({timeout: 10000});
    await keyboard.click();
    const input = driver.$(`~${TestIds.composer.input}`);
    await input.waitForDisplayed({timeout: 10000});
    await input.setValue('Duo draft fixture');
    expect(await input.getValue()).toBe('Duo draft fixture');
    const systemKeyboard = driver.$('XCUIElementTypeKeyboard');
    expect(await systemKeyboard.isExisting()).toBe(true);
    const rect = {...await input.getLocation(), ...await input.getSize()};
    const bounds = await driver.getWindowSize();
    const keyboardRect = {...await systemKeyboard.getLocation(), ...await systemKeyboard.getSize()};
    expect(rect.x).toBeGreaterThanOrEqual(0);
    expect(rect.x + rect.width).toBeLessThanOrEqual(bounds.width);
    expect(rect.y + rect.height).toBeLessThanOrEqual(keyboardRect.y + 1);
    await screenshot('duo-draft-before-rotation');
    const next = original === 'PORTRAIT' ? 'LANDSCAPE' : 'PORTRAIT';
    await driver.setOrientation(next);
    await settle(1500);
    expect(await driver.getOrientation()).toBe(next);
    expect(await driver.$(`~${TestIds.composer.input}`).getValue()).toBe('Duo draft fixture');
    // The chrome is intentionally hidden from accessibility while the keyboard
    // is open. Close it to inspect the actual selected mode and session, then
    // reopen the composer and verify the draft survived the round trip.
    await driver.$(`~${TestIds.composer.keyboard}`).click();
    await driver.$(`~${TestIds.detail.modeChat}`).waitForDisplayed({timeout: 10000});
    await driver.$(`~${TestIds.detail.chat}`).waitForDisplayed({timeout: 10000});
    await selected.waitForDisplayed({timeout: 10000});
    await driver.$(`~${TestIds.composer.keyboard}`).click();
    await driver.$(`~${TestIds.composer.input}`).waitForDisplayed({timeout: 10000});
    expect(await driver.$(`~${TestIds.composer.input}`).getValue()).toBe('Duo draft fixture');
    await screenshot('duo-draft-after-rotation');
  } catch (error) {
    return captureOnFailure('duo-rotation-failed', error);
  } finally {
    await driver.setOrientation(original);
  }
}, 300000);
