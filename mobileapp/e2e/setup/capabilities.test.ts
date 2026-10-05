import {alertCapabilities, alertMode} from './capabilities';

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
