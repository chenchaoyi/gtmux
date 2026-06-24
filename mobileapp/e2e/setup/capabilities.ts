/**
 * Appium capabilities for the gtmux iOS-simulator target.
 *
 * Defaults to an iPhone 17 Pro on iOS 26.5; override via env when your sim
 * differs (GTMUX_E2E_DEVICE / GTMUX_E2E_OS / GTMUX_E2E_UDID). platformVersion
 * disambiguates same-named devices across runtimes (26.4 vs 26.5).
 */
const udid = process.env.GTMUX_E2E_UDID;

export const iosCapabilities = {
  platformName: 'iOS',
  'appium:platformVersion': process.env.GTMUX_E2E_OS || '26.5',
  'appium:deviceName': process.env.GTMUX_E2E_DEVICE || 'iPhone 17 Pro',
  'appium:automationName': 'XCUITest',
  'appium:bundleId': 'com.gtmux.app',
  // Pin a specific simulator when set (skips device matching entirely).
  ...(udid ? {'appium:udid': udid} : {}),
  // Don't reinstall the app each session — the e2e harness builds + installs it
  // first (npm run e2e:build). Faster iteration; app data (Keychain) persists.
  'appium:noReset': true,
  // Default 60s; bump so a long-running test step doesn't reset the session.
  'appium:newCommandTimeout': 120,
} as const;

export const appiumPort = 4723;
export const appiumServerUrl = `http://127.0.0.1:${appiumPort}`;
