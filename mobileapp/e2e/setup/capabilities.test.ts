import {alertCapabilities, alertMode, captureStartupCapabilities, headlessCapabilities, sessionConnectionOptions, storeCaptureTimeout} from './capabilities';

// What a session does with an alert has to be the run's explicit choice. The default
// (dismiss) also cancels the APP's own confirmations, which is how a Revoke that "did
// nothing" was nearly reported as a product bug (2026-10-05); manual must leave every
// alert to the test.
describe('alert handling is chosen per run', () => {
  test('manual handles nothing on the test’s behalf', () => {
    const caps = alertCapabilities(alertMode({GTMUX_E2E_ALERTS: 'manual'}));
    expect(caps).not.toHaveProperty('appium:autoDismissAlerts');
    expect(caps).not.toHaveProperty('appium:autoAcceptAlerts');
  });

  test('the default dismisses, and the older accept switch still accepts', () => {
    expect(alertCapabilities(alertMode({}))).toEqual({'appium:autoDismissAlerts': true});
    expect(alertCapabilities(alertMode({GTMUX_E2E_ACCEPT_ALERTS: '1'}))).toEqual({'appium:autoAcceptAlerts': true});
  });

  test('the explicit mode wins over the older switch', () => {
    expect(alertMode({GTMUX_E2E_ALERTS: 'manual', GTMUX_E2E_ACCEPT_ALERTS: '1'})).toBe('manual');
  });

  test('a misspelt mode stops the run instead of falling back to dismiss', () => {
    expect(() => alertMode({GTMUX_E2E_ALERTS: 'manaul'})).toThrow(/manual/);
  });
});

test('headless capture is explicit and local GUI runs keep their behavior', () => {
  expect(headlessCapabilities({})).toEqual({});
  expect(headlessCapabilities({GTMUX_E2E_HEADLESS: '0'})).toEqual({});
  expect(headlessCapabilities({GTMUX_E2E_HEADLESS: '1'})).toEqual({'appium:isHeadless': true});
});

test('only explicit hosted capture separates WDA build and readiness with one bounded connection', () => {
  for (const env of [{}, {GTMUX_E2E_CAPTURE_CI: '0'}, {GTMUX_E2E_HEADLESS: '1'}]) {
    expect(captureStartupCapabilities(env)).toEqual({});
    expect(sessionConnectionOptions(env)).toEqual({connectionRetryTimeout: 360_000});
  }
  const env = {GTMUX_E2E_CAPTURE_CI: '1'};
  expect(captureStartupCapabilities(env)).toEqual({
    'appium:usePrebuiltWDA': true,
    'appium:wdaLaunchTimeout': 240_000,
    'appium:wdaStartupRetries': 1,
    'appium:showXcodeLog': true,
  });
  expect(captureStartupCapabilities(env)).not.toHaveProperty('appium:prebuildWDA');
  expect(sessionConnectionOptions(env)).toEqual({connectionRetryTimeout: 900_000, connectionRetryCount: 0});
});

test('only hosted multi-screen store captures receive the longer journey budget', () => {
  expect(storeCaptureTimeout({})).toBe(120_000);
  expect(storeCaptureTimeout({GTMUX_E2E_HEADLESS: '1'})).toBe(120_000);
  expect(storeCaptureTimeout({GTMUX_E2E_CAPTURE_CI: '1'})).toBe(300_000);
});
