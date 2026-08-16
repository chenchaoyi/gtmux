import {deviceSub} from './ManageMacScreen';

// One row shape on every surface. The menu bar showed only a last-seen time while the app
// showed the platform — off the SAME payload, which the menu bar simply did not decode.
// These pin the phone's half of the agreement (PairedDeviceSubtitleTests pins the Mac's).
describe('deviceSub', () => {
  const now = Math.floor(Date.now() / 1000);

  it('joins what it is, where from, and when — in that order', () => {
    const s = deviceSub('iOS 26.6', '192.168.1.23', now - 120, false)!;
    expect(s.indexOf('iOS 26.6')).toBeLessThan(s.indexOf('192.168.1.23'));
    expect(s.indexOf('192.168.1.23')).toBeLessThan(s.indexOf('ago'));
  });

  it('says so when a device has never connected, instead of going blank', () => {
    expect(deviceSub(undefined, undefined, undefined, false)).toBe('never connected');
    expect(deviceSub(undefined, undefined, undefined, true)).toBe('从未连接');
  });

  it('drops the parts it does not know rather than printing empties', () => {
    expect(deviceSub('iOS 26.6', undefined, undefined, false)).toBe('iOS 26.6 · never connected');
  });
});
