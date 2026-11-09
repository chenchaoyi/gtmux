/* eslint-env node */
// QA driver only. Never accepts a grant action, Settings change or another app.
const { performance } = require('node:perf_hooks');
const DENY =
  '//*[@resource-id="com.android.permissioncontroller:id/permission_deny_button"]';
const MESSAGE =
  '//*[@resource-id="com.android.permissioncontroller:id/permission_message"]';
const CONTROLLERS = new Set([
  'com.android.permissioncontroller',
  'com.google.android.permissioncontroller',
]);

async function denyCameraPermission(
  driver,
  {
    now = () => performance.now(),
    pause = ms => new Promise(resolve => setTimeout(resolve, ms)),
    onAttempt = () => {},
  } = {},
) {
  const started = now(),
    deadline = started + 10000;
  let attempts = 0,
    observed = false,
    lastClick = -Infinity,
    dialogElement;
  const within = () => {
    if (now() >= deadline)
      throw new Error(
        `Camera permission dialog did not close within 10000ms (${attempts} Deny clicks)`,
      );
  };
  // Commands remain serial: an in-flight WebDriver call keeps its existing
  // transport timeout; on return the deadline prevents any further click.
  for (;;) {
    within();
    const button = await driver.$(DENY);
    within();
    let visible;
    try {
      visible = await button.isDisplayed();
    } catch (error) {
      if (
        !/no such element|stale element reference/i.test(
          String(error?.message || error),
        )
      )
        throw error;
      visible = false;
    }
    within();
    if (!visible) {
      if (observed) return { attempts, elapsed_ms: now() - started };
      await pause(100);
      continue;
    }
    const owner = await driver.getCurrentPackage();
    within();
    const message = await (await driver.$(MESSAGE)).getText();
    within();
    if (
      !CONTROLLERS.has(owner) ||
      message !== 'Allow gtmux to take pictures and record video?'
    ) {
      throw new Error(
        'Refusing to act on an unexpected camera permission dialog',
      );
    }
    if (
      !button.elementId ||
      (dialogElement && dialogElement !== button.elementId)
    ) {
      throw new Error(
        'Camera permission dialog changed; refusing another prompt',
      );
    }
    dialogElement = button.elementId;
    observed = true;
    const enabled = await button.isEnabled();
    within();
    if (enabled && attempts < 2 && now() - lastClick >= 750) {
      attempts++;
      onAttempt(attempts);
      within();
      await button.click();
      within();
      lastClick = now();
    }
    await pause(100);
  }
}
module.exports = { denyCameraPermission };
