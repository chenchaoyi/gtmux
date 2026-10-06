import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';
import {startFake, Fake} from '../fake-serve/server';

// Runs against the in-process fake with the full usage report seeded
// (world.seedUsage): every window carries the agent whose plan it is, which is what the
// sheet groups by and what its row ids are built from. In English, because a window's
// name is worded per language and the id below carries the name.
let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
  fake.world.seedUsage();
});
afterAll(async () => {
  await fake?.close();
});

describe('usage sheet', () => {
  it('opens from the usage door and shows the plan', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: fake.url,
      GTMUX_DEBUG_PAIR_TOKEN: fake.token,
      GTMUX_DEBUG_NO_PUSH: '1',
      GTMUX_DEBUG_LANG: 'en',
    });
    try {
      await driver.$(`~${TestIds.radar.screen}`).waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('us-no-radar', err);
    }
    await settle(2000);
    await driver.$('~radar-hq-disc').click();
    await driver.$('~hq-verdict').waitForDisplayed({timeout: 10_000});
    await settle(2500);
    await driver.$('~hq-verdict').click();
    await settle(700);
    const row = driver.$('~hq-usage-open');
    try {
      await row.waitForDisplayed({timeout: 6_000});
    } catch (err) {
      return captureOnFailure('us-no-usage-door', err);
    }
    await row.click();
    await settle(1200);
    await screenshot('usage-sheet');
    // The sheet exists because the header compressed three sensors into one line
    // on the argument that the detail lives here. It has to actually be here.
    const plan = driver.$('~usage-window-claude week (all models)');
    try {
      await plan.waitForDisplayed({timeout: 6_000});
    } catch (err) {
      return captureOnFailure('us-no-plan', err);
    }
    expect(await plan.isDisplayed()).toBe(true);

    // The machine block is at the BOTTOM, under however many sessions there are, and it
    // is the part that carries an identity icon per resource (2026-09-10). Scroll to it
    // and capture: a unit test can assert the icons are wired, not that they render.
    const {width, height} = await driver.getWindowSize();
    const cx = Math.round(width / 2);
    for (let i = 0; i < 4; i++) {
      await driver
        .action('pointer', {parameters: {pointerType: 'touch'}})
        .move({x: cx, y: Math.round(height * 0.8)})
        .down()
        .pause(60)
        .move({x: cx, y: Math.round(height * 0.25), duration: 320})
        .up()
        .perform();
      await settle(250);
    }
    await settle(600);
    await screenshot('usage-sheet-machine');
  });
});
