import {deviceLabel, displayDeviceName} from './deviceName';

// The pair roster's job is letting you tell YOUR devices apart well enough to revoke the
// right one. It was failing twice over: every entry read `gtmux • iPhone` — a "gtmux"
// prefix inside gtmux's own roster (nothing in that list is not a gtmux device) over a
// word true of every iPhone ever made. Two paired phones were indistinguishable.

describe('a device names itself with what it actually knows', () => {
  test('iOS carries the version, and no product prefix', () => {
    expect(deviceLabel('ios', '18.5', 'phone')).toBe('iPhone · iOS 18.5');
    expect(deviceLabel('ios', '18.5', 'pad')).toBe('iPad · iOS 18.5');
    expect(deviceLabel('ios', '18.5', 'phone')).not.toMatch(/gtmux/i);
  });

  test('a numeric version (Platform.Version) works as well as a string', () => {
    expect(deviceLabel('ios', 18, 'phone')).toBe('iPhone · iOS 18');
  });

  test('a missing part is left out, never rendered as "undefined"', () => {
    expect(deviceLabel('ios', undefined, 'phone')).toBe('iPhone');
    expect(deviceLabel('ios', '', undefined)).toBe('iPhone');
    expect(deviceLabel('android', undefined)).toBe('Android');
    for (const v of [deviceLabel('ios', undefined), deviceLabel('android', undefined), deviceLabel('')]) {
      expect(v).not.toMatch(/undefined|null|NaN/);
      expect(v).not.toBe('');
    }
  });

  test('android reports its version too', () => {
    expect(deviceLabel('android', 34)).toBe('Android 34');
  });
});

describe('roster rows tidy up without a re-pair', () => {
  test('the legacy prefix is stripped in every form it was written', () => {
    expect(displayDeviceName('gtmux • iPhone')).toBe('iPhone');
    expect(displayDeviceName('gtmux · iPad')).toBe('iPad');
    expect(displayDeviceName('gtmux iPhone')).toBe('iPhone');
    expect(displayDeviceName('GTMUX • iPhone · iOS 18.5')).toBe('iPhone · iOS 18.5');
  });

  test('a name without the prefix is untouched', () => {
    expect(displayDeviceName('iPhone · iOS 18.5')).toBe('iPhone · iOS 18.5');
    expect(displayDeviceName('ccy-mbp.local')).toBe('ccy-mbp.local');
  });

  test('a row never renders empty', () => {
    // A device legitimately named after the tool keeps something to show.
    expect(displayDeviceName('gtmux')).toBe('gtmux');
    expect(displayDeviceName('')).toBe('—');
  });
});
