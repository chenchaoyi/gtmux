/**
 * Appium capabilities for the gtmux iOS-simulator target.
 *
 * Defaults to an iPhone 17 Pro on iOS 26.5; override via env when your sim
 * differs (GTMUX_E2E_DEVICE / GTMUX_E2E_OS / GTMUX_E2E_UDID). platformVersion
 * disambiguates same-named devices across runtimes (26.4 vs 26.5).
 */
const udid = process.env.GTMUX_E2E_UDID;

/**
 * What the session does with an alert nobody asked about (GTMUX_E2E_ALERTS):
 *
 *   dismiss (the default) — taps its cancel button. A stray SYSTEM alert can wedge a whole
 *     run: on a dev sim with other apps installed, handling a URL can pop iOS's "Open in
 *     <app>?" (a colliding URL scheme), which blocks every later command until dismissed.
 *   accept — taps its accept button: a run that needs the push permission (a notification
 *     has to be shown to be tapped). GTMUX_E2E_ACCEPT_ALERTS=1 still means this.
 *   manual — touches nothing; the test reads and taps every alert itself.
 *
 * dismiss and accept cover the APP's own alerts too, not only the system's: an Alert.alert
 * confirmation is cancelled about a second after it appears, so a suite that taps Revoke
 * sees nothing happen and could pass for the wrong reason (simulator, 2026-10-05: the
 * sharing page's Revoke "did nothing" until the app's log showed the confirmation shown
 * and hidden 1.1s later). A suite that asserts a confirmation runs in manual.
 */
export type AlertMode = 'dismiss' | 'accept' | 'manual';

export function alertMode(env: NodeJS.ProcessEnv = process.env): AlertMode {
  const m = env.GTMUX_E2E_ALERTS;
  if (m === 'dismiss' || m === 'accept' || m === 'manual') return m;
  if (m) throw new Error(`GTMUX_E2E_ALERTS must be dismiss, accept or manual, not "${m}"`);
  return env.GTMUX_E2E_ACCEPT_ALERTS === '1' ? 'accept' : 'dismiss';
}

export function alertCapabilities(mode: AlertMode): Record<string, boolean> {
  if (mode === 'accept') return {'appium:autoAcceptAlerts': true};
  if (mode === 'dismiss') return {'appium:autoDismissAlerts': true};
  return {};
}

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
  // Alerts nobody asked about: see alertMode. The app's own push prompt is kept out of
  // the way by GTMUX_DEBUG_NO_PUSH.
  ...alertCapabilities(alertMode()),
  // Two runs on one Mac each need their own WebDriverAgent port and build directory.
  ...(process.env.GTMUX_E2E_WDA_PORT ? {'appium:wdaLocalPort': Number(process.env.GTMUX_E2E_WDA_PORT)} : {}),
  ...(process.env.GTMUX_E2E_WDA_DERIVED ? {'appium:derivedDataPath': process.env.GTMUX_E2E_WDA_DERIVED} : {}),
  // Reuse the WebDriverAgent already built in that directory instead of building it again
  // (a native build) at every session start.
  ...(process.env.GTMUX_E2E_WDA_PREBUILT === '1' ? {'appium:usePrebuiltWDA': true} : {}),
  // Type through the on-screen keyboard, as a phone does: Return is then the keyboard's own
  // key (composer-return-sends), not a hardware keyboard's.
  ...(process.env.GTMUX_E2E_SOFT_KEYBOARD === '1'
    ? {'appium:connectHardwareKeyboard': false, 'appium:forceTurnOnSoftwareKeyboardSimulator': true}
    : {}),
} as const;

export const appiumPort = Number(process.env.GTMUX_E2E_APPIUM_PORT || 4723);
export const appiumServerUrl = `http://127.0.0.1:${appiumPort}`;
