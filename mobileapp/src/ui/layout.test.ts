import {isSplitCanvas} from './layout';

// Real device canvases, in points. The bug this pins: width alone put the iPad's
// two-column layout on a phone in landscape — wide enough at 852-932pt, and ~400pt tall.
describe('isSplitCanvas', () => {
  test.each([
    ['iPad portrait', 768, 1024, true],
    ['iPad landscape', 1024, 768, true],
    ['iPad Pro 12.9 landscape', 1366, 1024, true],
    ['iPhone 15 Pro landscape', 852, 393, false],
    ['iPhone 15 Pro Max landscape', 932, 430, false],
    ['iPhone 15 Pro portrait', 393, 852, false],
    ['iPad Slide Over', 320, 1024, false],
    ['iPad 1/2 split', 507, 1024, false],
  ])('%s (%ix%i)', (_name, w, h, want) => {
    expect(isSplitCanvas(w as number, h as number)).toBe(want);
  });
});

// The shell is the one consumer of the breakpoint (change ipad-universal-app, D2).
describe('sizeClassFor and the sidebar width', () => {
  const {sizeClassFor, sidebarWidth, READING_WIDTH} = require('./layout');
  test('regular is exactly the split canvas', () => {
    expect(sizeClassFor(1194, 834)).toBe('regular');
    expect(sizeClassFor(834, 1194)).toBe('regular');
    expect(sizeClassFor(683, 1024)).toBe('compact'); // iPad 1/2 Split View
    expect(sizeClassFor(932, 430)).toBe('compact'); // a phone in landscape
  });
  test('the sidebar narrows under 1000 and never becomes a drawer', () => {
    expect(sidebarWidth(1194)).toBe(300);
    expect(sidebarWidth(1000)).toBe(300);
    expect(sidebarWidth(834)).toBe(280);
    expect(sidebarWidth(768)).toBe(280);
  });
  test('the reading width is wider than any phone and narrower than an 11" main pane', () => {
    expect(READING_WIDTH).toBeGreaterThan(430);
    expect(READING_WIDTH).toBeLessThan(1194 - 300);
  });
});
