import {safeAreaEdges} from './DetailScreen';

// The top chrome was four bands — header, neighbour strip, Chat/Terminal, controls —
// stacked to 155pt, 21% of the usable screen, each with its own divider (user report,
// 2026-09-09). Two of them were one row of controls wearing two dividers.
//
// These pin the two decisions that came out of it. The layout itself is pinned by
// `liveEdge.test.ts`, which proves the fold cannot oscillate for ANY chrome height —
// which is why the height had to be counted, not just folded.
describe('the detail screen’s top chrome', () => {
  it('still reserves nothing at the top in full screen', () => {
    // Unchanged by the merge, and worth re-stating: the two live in the same stack.
    expect(safeAreaEdges(true)).not.toContain('top');
    expect(safeAreaEdges(false)).toContain('top');
  });
});

// A guard on the source, because the thing that regressed is structural and a render
// test would not see it: the controls row was the one band left OUT of the fold, which
// made its own comment ("one gesture, all top chrome folds together") untrue.
describe('every band folds, and every folding band is counted', () => {
  // Read the source itself: what regressed is structural, and no render can see it.
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const req = require as unknown as {resolve: (m: string) => string};
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const src: string = require('fs').readFileSync(req.resolve('./DetailScreen.tsx'), 'utf8');

  it('counts each band’s height in chromeH', () => {
    // chromeH is now two things: how far the floating chrome slides out, and the constant
    // top padding the content carries so its oldest line clears the chrome. A band missing
    // from the sum would be a band that slides only partway out, or content that starts
    // underneath it.
    const sum = src.slice(src.indexOf('const chromeH ='), src.indexOf('const chrome ='));
    for (const band of ['headerH', 'neighborH', 'ctlH']) {
      expect(sum).toContain(band);
    }
  });

  it('folds by sliding, never by resizing — the whole point of the rewrite', () => {
    // Animating a band's HEIGHT resizes the scroll viewport under it, and every symptom
    // this screen has produced came from that: the fold oscillating, the content bouncing
    // up 115pt, the scroll sticking at the fold point, and the jump-to-bottom animation
    // being cancelled by the compensation. The chrome floats now. If a `height:` shows up
    // on the collapse driver again, all four come back with it.
    expect(src).not.toMatch(/height:[^\n]*collapse\.interpolate/);
    expect(src).toContain('transform: [{translateY: collapse.interpolate(');
    expect(src).toContain('styles.chrome');
    // Nothing may drive the scroll offset from the fold any more; there is nothing to
    // compensate, and those writes are what cancelled the arrival animation.
    expect(src).not.toContain('collapse.addListener');
    expect(src).not.toContain('shiftRef');
  });

  it('pads the content by the chrome’s height, in BOTH layers', () => {
    // The chrome covers the top of the scroll view. Without this the OLDEST line can
    // never be scrolled clear of it.
    expect((src.match(/topPad=\{chromeH\}/g) ?? []).length).toBe(2);
  });

  it('paints the chrome above the mode layers', () => {
    // The floating chrome is a SIBLING of the body, and the body's visible layer carries
    // its own zIndex (that is how the two modes stack). Without an explicit, higher one
    // here the terminal paints over the chrome: on 2026-09-10 the bands were laid out
    // correctly the whole time — the e2e dump put the back button at y=65 — and were
    // simply invisible, which reads as "the top folded and never came back".
    const zOf = (name: string) => {
      const m = src.match(new RegExp(name + ':[^}]*zIndex: (\\d+)'));
      return m ? Number(m[1]) : -1;
    };
    expect(zOf('chrome')).toBeGreaterThan(zOf('layerOn'));
    expect(zOf('layerOn')).toBeGreaterThanOrEqual(0); // the comparison must mean something
  });

  it('gives the floating chrome its own ground', () => {
    // It floats over scrollback now. Transparent, the title and the controls read on top
    // of terminal output — legible in neither direction (measured on the simulator,
    // 2026-09-10, before this line existed).
    const style = src.slice(src.indexOf('styles.chrome,'), src.indexOf('translateY: collapse'));
    expect(style).toContain('backgroundColor: pal.bg');
  });

  it('keeps the mode toggle and the controls on ONE row', () => {
    // Two adjacent rows of controls cost a band and a divider for nothing. If the
    // segmented gets its own wrapper again, this is the reminder of why it did not.
    expect(src).not.toContain('styles.segWrap');
    expect(src).toContain('styles.segInline');
  });
});
