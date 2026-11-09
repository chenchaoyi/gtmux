/* eslint-env node */
// Synthetic driver only; never connects to ADB or an Appium server.
function fixture({
  ignored = 0,
  dialogPackage = 'com.google.android.permissioncontroller',
  message = 'Allow gtmux to take pictures and record video?',
  alert = true,
  granted = false,
  error,
} = {}) {
  let clock = 0,
    visible = true,
    declined = false,
    clicks = 0;
  const attempts = [],
    writes = [];
  const press = async () => {
    if (error) throw error;
    clicks++;
    if (clicks > ignored) {
      visible = false;
      declined = true;
    }
  };
  const button = {
    elementId: 'same-camera-deny',
    isDisplayed: async () => visible,
    isEnabled: async () => true,
    click: press,
    getLocation: async () => ({ x: 133, y: 1478 }),
    getSize: async () => ({ width: 814, height: 147 }),
  };
  const driver = {
    $: selector =>
      selector.includes('permission_message')
        ? { getText: async () => message }
        : selector.includes('permission_deny_button')
        ? button
        : {
            waitForDisplayed: async ({ timeout }) => {
              if (timeout !== 30000 || !declined || !alert)
                throw Error('App denial message missing');
            },
          },
    getCurrentPackage: async () =>
      visible ? dialogPackage : 'com.gtmux.app.qa',
    execute: async () => press(),
    waitUntil: async (predicate, { timeout }) => {
      const end = clock + timeout;
      while (clock <= end) {
        if (await predicate()) return;
        clock += 200;
      }
      throw Error('waitUntil condition timed out');
    },
  };
  return {
    driver,
    button,
    attempts,
    writes,
    now: () => clock,
    pause: async ms => {
      clock += ms;
    },
    get clicks() {
      return clicks;
    },
    get declined() {
      return declined;
    },
    grant: granted,
  };
}
module.exports = { fixture };
