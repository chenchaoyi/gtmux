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

import {linkUsage} from './ManageMacScreen';

// A share link is a standing grant handed to someone else. Its row already says what it
// PERMITS; this line says whether anyone walked through it.
describe('linkUsage', () => {
  it('says plainly when nobody has used a link', () => {
    expect(linkUsage({}, false)).toBe('not used yet');
    expect(linkUsage({}, true)).toBe('还没有人用过');
  });

  it('names the device and the address once someone has', () => {
    const s = linkUsage({platform: 'Chrome 141 · macOS', lastIP: '10.0.0.7', lastSeen: Math.floor(Date.now() / 1000) - 60}, false);
    expect(s).toContain('Chrome 141 · macOS');
    expect(s).toContain('10.0.0.7');
    expect(s).toContain('last used');
  });
});
