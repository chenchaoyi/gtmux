import {pushHistory, HISTORY_CAP} from './history';

describe('pushHistory', () => {
  it('prepends a new entry (newest first)', () => {
    expect(pushHistory(['a'], 'b')).toEqual(['b', 'a']);
  });

  it('trims and ignores empty/whitespace input', () => {
    expect(pushHistory(['a'], '   ')).toEqual(['a']);
    expect(pushHistory(['a'], '  b  ')).toEqual(['b', 'a']);
  });

  it('floats a repeated entry to the top without duplicating', () => {
    expect(pushHistory(['a', 'b', 'c'], 'c')).toEqual(['c', 'a', 'b']);
  });

  it('caps the list at HISTORY_CAP, dropping the oldest', () => {
    const long = Array.from({length: HISTORY_CAP}, (_, i) => `e${i}`);
    const out = pushHistory(long, 'new');
    expect(out).toHaveLength(HISTORY_CAP);
    expect(out[0]).toBe('new');
    expect(out).not.toContain(`e${HISTORY_CAP - 1}`); // oldest dropped
  });
});
