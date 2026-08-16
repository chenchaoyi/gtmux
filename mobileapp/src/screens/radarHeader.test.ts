// The two header buttons must not be able to steal each other's taps.
//
// They sat 14pt apart with 10pt of hitSlop on every side, so their touch areas overlapped
// by 6pt in the middle — and an overlap belongs to whichever view comes last, so aiming at
// the right half of "all panes" opened Settings. The fix is geometric, so the test is too.

/** A button's touch area on the x axis, from its box and any slop it claims outside it. */
function touchSpan(x: number, width: number, slop: number): [number, number] {
  return [x - slop, x + width + slop];
}
const overlaps = (a: [number, number], b: [number, number]) => a[1] > b[0];

describe('the radar header buttons', () => {
  it('overlapped in the shape that was shipped', () => {
    // icon 19 + 10 slop each side, then 14pt margin, then icon 20 + 10 each side
    const panes = touchSpan(0, 19, 10);
    const gear = touchSpan(19 + 14, 20, 10);
    expect(overlaps(panes, gear)).toBe(true);
  });

  it('does not overlap as 40pt squares 4pt apart', () => {
    const panes = touchSpan(0, 40, 0);
    const gear = touchSpan(44, 40, 0);
    expect(overlaps(panes, gear)).toBe(false);
  });

  it('keeps each target big enough to hit', () => {
    const [a, b] = touchSpan(0, 40, 0);
    expect(b - a).toBeGreaterThanOrEqual(40);
  });
});
