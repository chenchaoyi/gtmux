import {dropIndex, moveTo, shiftFor} from './ReorderableList';

// Where a lifted row lands, and what steps aside for it (ReorderableList).
describe('reorder geometry', () => {
  const tops = [0, 52, 104, 156];
  const heights = [52, 52, 52, 52];

  it('trades places at half a row, in either direction', () => {
    expect(dropIndex(tops, heights, 0, 0)).toBe(0);
    expect(dropIndex(tops, heights, 0, 30)).toBe(1); // its bottom edge is past the next row's centre
    expect(dropIndex(tops, heights, 0, 20)).toBe(0); // not yet
    expect(dropIndex(tops, heights, 3, -110)).toBe(1);
    expect(dropIndex(tops, heights, 3, -400)).toBe(0);
    expect(dropIndex(tops, heights, 1, 400)).toBe(3);
  });

  it('reads a taller row by its own centre', () => {
    // Row 1 carries a second line.
    expect(dropIndex([0, 52, 128], [52, 76, 52], 0, 40)).toBe(1); // bottom 92 > that row's centre 90
    expect(dropIndex([0, 52, 128], [52, 76, 52], 0, 30)).toBe(0); // bottom 82, not yet
  });

  it('steps the rows between aside, by the lifted row\'s height', () => {
    expect([0, 1, 2, 3].map(i => shiftFor(i, 0, 2, 52))).toEqual([0, -52, -52, 0]);
    expect([0, 1, 2, 3].map(i => shiftFor(i, 3, 1, 52))).toEqual([0, 52, 52, 0]);
    expect([0, 1, 2].map(i => shiftFor(i, 1, 1, 52))).toEqual([0, 0, 0]);
  });

  it('moves a key and clamps', () => {
    expect(moveTo(['a', 'b', 'c'], 'c', 0)).toEqual(['c', 'a', 'b']);
    expect(moveTo(['a', 'b', 'c'], 'a', 9)).toEqual(['b', 'c', 'a']);
    const same = ['a', 'b'];
    expect(moveTo(same, 'z', 0)).toBe(same);
  });
});
