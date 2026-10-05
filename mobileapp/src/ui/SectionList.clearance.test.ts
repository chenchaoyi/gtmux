import {listContent} from './SectionList';
import {DISC_CLEARANCE} from './HQDisc';

// A list shorter than the screen did not scroll, so whatever the HQ disc covered stayed
// covered (%6, 2026-10-06: the last section's Show, a row's arrow at large text). With
// the disc showing, the content is at least the list's height plus the disc's clearance,
// so it always scrolls that far.
describe('the radar list leaves room for the HQ disc', () => {
  it('is at least a screen plus the clearance when the disc shows', () => {
    const style = listContent(700, DISC_CLEARANCE) as unknown as Array<{minHeight?: number}>;
    expect(style[1].minHeight).toBe(700 + DISC_CLEARANCE);
  });
  it('is unchanged without the disc, and before the list is measured', () => {
    expect(Array.isArray(listContent(700, 0))).toBe(false);
    expect(Array.isArray(listContent(0, DISC_CLEARANCE))).toBe(false);
  });
  it('clears the disc as far as the closing footer does', () => {
    expect(DISC_CLEARANCE).toBe(96);
  });
});
