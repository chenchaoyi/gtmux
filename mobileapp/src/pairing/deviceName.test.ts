import {deviceLabel, displayDeviceName} from './deviceName';

// The pair roster's job is letting you tell YOUR devices apart well enough to revoke the
// right one. It was failing twice over: every entry read `gtmux • iPhone` — a "gtmux"
// prefix inside gtmux's own roster (nothing in that list is not a gtmux device) over a
// word true of every iPhone ever made. Two paired phones were indistinguishable.

describe('a device names itself with what it actually knows', () => {
  test('the idiom, and no product prefix', () => {
    expect(deviceLabel('ios', 'phone')).toBe('iPhone');
    expect(deviceLabel('ios', 'pad')).toBe('iPad');
    expect(deviceLabel('ios', 'phone')).not.toMatch(/gtmux/i);
  });

  // The OS version reaches the Mac on every request (X-Gtmux-Client) and the roster row
  // prints it underneath the name. A name carrying it said the same thing twice, and its
  // copy froze at pairing while the line below stayed current.
  test('never the OS version: the row already shows the live one', () => {
    for (const v of [deviceLabel('ios', 'phone'), deviceLabel('ios', 'pad'), deviceLabel('android')]) {
      expect(v).not.toMatch(/iOS|Android\s*[0-9]|[0-9]/);
    }
  });

  test('a missing part is left out, never rendered as "undefined"', () => {
    expect(deviceLabel('ios', undefined)).toBe('iPhone');
    expect(deviceLabel('android')).toBe('Android');
    for (const v of [deviceLabel('ios'), deviceLabel('android'), deviceLabel('')]) {
      expect(v).not.toMatch(/undefined|null|NaN/);
      expect(v).not.toBe('');
    }
  });
});

describe('roster rows tidy up without a re-pair', () => {
  test('the legacy prefix is stripped in every form it was written', () => {
    expect(displayDeviceName('gtmux • iPhone')).toBe('iPhone');
    expect(displayDeviceName('gtmux · iPad')).toBe('iPad');
    expect(displayDeviceName('gtmux iPhone')).toBe('iPhone');
    expect(displayDeviceName('GTMUX • iPhone · iOS 18.5')).toBe('iPhone');
  });

  // A roster paired under the older rule reads "iPad · iOS 26.6.1" over a second line
  // that already says "iOS 26.6.1 · …" (seen 2026-09-20). The frozen copy comes off here,
  // so the list tidies itself without a re-pair.
  test('an OS version the name carried is dropped: the row prints the live one', () => {
    expect(displayDeviceName('iPad · iOS 26.6.1')).toBe('iPad');
    expect(displayDeviceName('iPhone · iOS 18.5')).toBe('iPhone');
    expect(displayDeviceName('Android 34')).toBe('Android 34'); // not a "· version" tail
  });

  test('a name of their own is untouched', () => {
    expect(displayDeviceName('dev-mbp.local')).toBe('dev-mbp.local');
    expect(displayDeviceName('ccy')).toBe('ccy');
    expect(displayDeviceName('Lin · iPad')).toBe('Lin · iPad');
  });

  test('a bare generic kind is title-cased, not left as a lowercase word', () => {
    expect(displayDeviceName('browser')).toBe('Browser');
    expect(displayDeviceName('terminal')).toBe('Terminal');
  });

  test('a row never renders empty', () => {
    // A device legitimately named after the tool keeps something to show.
    expect(displayDeviceName('gtmux')).toBe('gtmux');
    expect(displayDeviceName('')).toBe('—');
  });
});
