import {safeAreaEdges} from './DetailScreen';

// Full-screen is the reading mode, and it kept not being full screen. Twice the operator
// reported the top of the content sitting far below the top of the phone; twice the cause
// was space reserved above it rather than anything visible. This pins the decision so the
// third report doesn't happen.
describe('safeAreaEdges', () => {
  it('reserves NOTHING at the top in full screen', () => {
    // The status bar is hidden here, so the top inset (~59pt on a Dynamic Island phone)
    // was held for a bar that isn't drawn. The exit control clears the island itself.
    expect(safeAreaEdges(true)).not.toContain('top');
  });

  it('keeps the top inset in the normal mode, where the header lives under the bar', () => {
    expect(safeAreaEdges(false)).toContain('top');
  });

  it('keeps the SIDE insets in both modes — in landscape the notch is on a side', () => {
    for (const fs of [true, false]) {
      expect(safeAreaEdges(fs)).toEqual(expect.arrayContaining(['left', 'right']));
    }
  });
});
