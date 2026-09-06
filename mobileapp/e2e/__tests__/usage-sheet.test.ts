import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

gated('usage sheet', () => {
  it('opens from the usage door and shows the plan', async () => {
    const driver = getDriver();
    await launchWithFlags({
      GTMUX_DEBUG_PAIR_URL: url!,
      GTMUX_DEBUG_PAIR_TOKEN: token!,
      GTMUX_DEBUG_NO_PUSH: '1',
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
  });
});
