// The HQ page's top chrome folds the way Detail's does: by SLIDING out over a scroll view
// whose frame never changes. This page kept animating the chrome's height for a month
// after Detail stopped (2026-09-10), and the user found the loop that mechanism cannot
// avoid: a light upward scroll folded the header, the viewport grew by the header's
// height, the console's distance from its tail shrank by the same amount and asked for the
// header back — fold, unfold, fold, until a scroll longer than the header out-ran it
// (2026-09-12: "轻轻滑动一下…反复折叠展开来回换"). The terminal page did not do it, because
// its chrome no longer participates in layout.
//
// What regressed is structural, and no render test would see it, so these read the source
// the way `detailChrome.test.ts` does.
// eslint-disable-next-line @typescript-eslint/no-var-requires
const req = require as unknown as {resolve: (m: string) => string};
// eslint-disable-next-line @typescript-eslint/no-var-requires
const src: string = require('fs').readFileSync(req.resolve('./HQScreen.tsx'), 'utf8');
// eslint-disable-next-line @typescript-eslint/no-var-requires
const acts: string = require('fs').readFileSync(req.resolve('./HQActs.tsx'), 'utf8');

describe('the HQ page’s top chrome', () => {
  it('folds by sliding, never by resizing', () => {
    // Animating a band's HEIGHT resizes the scroll viewport under it, and the viewport is
    // what "am I at the tail" is measured against — the loop above. The chrome floats
    // (absolute), slides on translateY, and runs on the UI thread.
    expect(src).not.toMatch(/height:\s*\w+\s*>\s*0\s*\?\s*collapse\.interpolate/);
    expect(src).toContain("chrome: {position: 'absolute'");
    expect(src).toMatch(/collapse\.interpolate\(\{inputRange: \[0, 1\], outputRange: \[0, -chromeH\]\}\)/);
    expect(src).not.toContain('useNativeDriver: false');
  });

  it('counts both folding bands in chromeH', () => {
    // chromeH is how far the chrome slides out AND the padding the content carries. A
    // band folded but not counted slides only partway out, or leaves content under it.
    const sum = src.slice(src.indexOf('const chromeH ='), src.indexOf('const chrome ='));
    expect(sum).toContain('headerH');
    expect(sum).toContain('tabsH');
  });

  it('scrolls the chrome away with a top-anchored zone instead of folding it', () => {
    // A fold at 72pt in a zone that reads downward from under the chrome would leave a
    // blank band above the first row (the simulator showed ~140pt of it). The offset
    // drives the chrome directly, clamped at its own height, on the UI thread.
    expect(src).toContain("Animated.event([{nativeEvent: {contentOffset: {y: zoneOffset}}}], {useNativeDriver: true})");
    expect(src).toMatch(/zoneOffset\.interpolate\(\{[^}]*extrapolate: 'clamp'/);
    // and the zones' scroll views can carry that event
    expect(src.match(/<Animated\.ScrollView/g)?.length).toBe(1); // calls
    expect(acts).toContain('<Animated.ScrollView'); // HQ's work
    // No JS-side fold decision is taken from a zone's offset any more.
    expect(src).not.toContain('onLiveEdge(e.nativeEvent.contentOffset.y)');
  });

  it('gives every zone the chrome’s height as top padding', () => {
    // Three zones, three scroll views, each carrying the constant padding so its first
    // row can be scrolled clear of the chrome — and none measuring a distance the fold
    // itself changes.
    expect(src.match(/topPad=\{chromeH\}/g)?.length).toBe(1); // acts (HQActs)
    expect(src).toContain('consoleEl(chromeH, onLiveEdge)'); // the console, through its element factory
    expect(src).toContain('paddingTop: chromeH + styles.pad.paddingVertical'); // calls
    expect(acts).toContain('contentContainerStyle={[styles.pad, topPad > 0 && {paddingTop: topPad}]}');
  });

  it('keeps the acts switch inside the scroll view, not fixed above it', () => {
    // A row fixed above the scroll view would sit under the floating chrome, or leave an
    // empty band its height once the chrome folds.
    const scroll = acts.indexOf('<Animated.ScrollView');
    const row = acts.indexOf('styles.switchRow');
    expect(scroll).toBeGreaterThan(0);
    expect(row).toBeGreaterThan(scroll);
  });
});
