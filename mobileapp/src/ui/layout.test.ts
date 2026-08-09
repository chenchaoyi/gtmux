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
