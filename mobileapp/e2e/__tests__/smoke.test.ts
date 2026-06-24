import {getDriver} from '../setup/driver';
import {screenshot, captureOnFailure} from '../setup/screenshot';
import {launchWithFlags, settle, typeInto} from '../setup/app';
import {TestIds} from '../../src/constants/testIds';

// Launch clean to the connection page, regardless of any leftover Keychain from
// a prior run (RESET_SERVERS wipes saved servers; NO_PUSH keeps the auth prompt
// out of the way). Self-isolating, so re-runs don't need a rebuild.
const cleanLaunch = () => launchWithFlags({GTMUX_DEBUG_RESET_SERVERS: '1', GTMUX_DEBUG_NO_PUSH: '1'});

/**
 * Phase-1 smoke — proves the whole e2e toolchain end-to-end: Appium server
 * (global-setup) → webdriverio session on the booted sim → find by
 * accessibility-id (RN testID) → type → tap → assert → capture.
 */
describe('smoke', () => {
  it('launches to the connection page and rejects an unreachable server', async () => {
    const driver = getDriver();
    await cleanLaunch();

    const connect = driver.$(`~${TestIds.pairing.connect}`);
    try {
      await connect.waitForDisplayed({timeout: 20_000});
    } catch (err) {
      return captureOnFailure('no-pairing-form', err);
    }

    // Type a host that resolves to nothing, then connect → expect the
    // "can't reach this server" validation error (real UI round-trip).
    await settle(1000); // the Add-server modal is still animating in
    await typeInto(TestIds.pairing.host, '127.0.0.1:1');
    await typeInto(TestIds.pairing.token, 'nope', {secure: true});
    try {
      await driver.execute('mobile: hideKeyboard', {keys: ['return']});
    } catch {
      /* no keyboard up — fine */
    }
    await connect.click();

    const error = driver.$(`~${TestIds.pairing.error}`);
    try {
      await error.waitForDisplayed({timeout: 20_000});
    } catch (err) {
      return captureOnFailure('no-error-shown', err);
    }
    expect(await error.getText()).toMatch(/reach|连不上/i);
    await screenshot('smoke-bad-server');
  });

  // Live pairing → radar. Gated on env so the committed test carries no secret.
  // Run locally:  GTMUX_E2E_URL=http://127.0.0.1:8765 GTMUX_E2E_TOKEN=<tok> npm run test:e2e
  const live = process.env.GTMUX_E2E_URL && process.env.GTMUX_E2E_TOKEN ? it : it.skip;
  live('pairs with a live server and reaches the radar', async () => {
    const driver = getDriver();
    await cleanLaunch();

    await driver.$(`~${TestIds.pairing.connect}`).waitForDisplayed({timeout: 20_000});
    await settle(1000); // modal animating in
    await typeInto(TestIds.pairing.host, process.env.GTMUX_E2E_URL!);
    await typeInto(TestIds.pairing.token, process.env.GTMUX_E2E_TOKEN!, {secure: true});
    try {
      await driver.execute('mobile: hideKeyboard', {keys: ['return']});
    } catch {
      /* fine */
    }
    await driver.$(`~${TestIds.pairing.connect}`).click();

    const radar = driver.$(`~${TestIds.radar.screen}`);
    try {
      await radar.waitForDisplayed({timeout: 25_000});
    } catch (err) {
      return captureOnFailure('no-radar', err);
    }
    await screenshot('smoke-radar');
  });
});
