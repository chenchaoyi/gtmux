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

  it('counts each folding band’s height in chromeH', () => {
    // The fold/reveal thresholds are derived from the height being switched. A band that
    // folds without being counted shrinks the gap by more than the threshold allows, and
    // the oscillation liveEdge exists to make impossible becomes possible again.
    const sum = src.slice(src.indexOf('chromeH.current ='), src.indexOf('const chrome ='));
    for (const band of ['headerH', 'neighborH', 'ctlH']) {
      expect(sum).toContain(band);
    }
  });

  it('has no band that folds without being in that sum', () => {
    const folding = [...src.matchAll(/outputRange: \[(\w+H), 0\]/g)].map(m => m[1]);
    const sum = src.slice(src.indexOf('chromeH.current ='), src.indexOf('const chrome ='));
    for (const h of new Set(folding)) {
      expect(sum).toContain(h);
    }
    expect(folding.length).toBeGreaterThan(0);
  });

  it('keeps the mode toggle and the controls on ONE row', () => {
    // Two adjacent rows of controls cost a band and a divider for nothing. If the
    // segmented gets its own wrapper again, this is the reminder of why it did not.
    expect(src).not.toContain('styles.segWrap');
    expect(src).toContain('styles.segInline');
  });
});
