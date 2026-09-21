import {fmtTurnTime, separatorLabels} from './time';

// Fixed reference "now": 2026-06-29 (a Monday), local time. All cases inject it so
// the relative labels (today/yesterday) and the same-year branch are deterministic.
const NOW = new Date(2026, 5, 29, 15, 0, 0); // month is 0-based → June

describe('fmtTurnTime', () => {
  it('returns "" for missing or invalid input', () => {
    expect(fmtTurnTime(undefined, 'en', NOW)).toBe('');
    expect(fmtTurnTime('not-a-date', 'en', NOW)).toBe('');
  });

  it('labels same-day as Today/今天 + HH:MM', () => {
    const iso = new Date(2026, 5, 29, 14, 35).toISOString();
    expect(fmtTurnTime(iso, 'en', NOW)).toBe('Today 14:35');
    expect(fmtTurnTime(iso, 'zh', NOW)).toBe('今天 14:35');
  });

  it('zero-pads the minutes', () => {
    const iso = new Date(2026, 5, 29, 9, 5).toISOString();
    expect(fmtTurnTime(iso, 'en', NOW)).toBe('Today 09:05');
  });

  it('labels the previous day as Yesterday/昨天', () => {
    const iso = new Date(2026, 5, 28, 8, 0).toISOString();
    expect(fmtTurnTime(iso, 'en', NOW)).toBe('Yesterday 08:00');
    expect(fmtTurnTime(iso, 'zh', NOW)).toBe('昨天 08:00');
  });

  it('uses a calendar date (no year) for an older same-year day', () => {
    const iso = new Date(2026, 0, 3, 7, 9).toISOString(); // Jan 3, 2026
    expect(fmtTurnTime(iso, 'en', NOW)).toBe('Jan 3, 07:09');
    expect(fmtTurnTime(iso, 'zh', NOW)).toBe('1月3日 07:09');
  });

  it('includes the year when not the current year', () => {
    const iso = new Date(2025, 11, 31, 23, 59).toISOString(); // Dec 31, 2025
    expect(fmtTurnTime(iso, 'en', NOW)).toBe('Dec 31 2025, 23:59');
    expect(fmtTurnTime(iso, 'zh', NOW)).toBe('2025年12月31日 23:59');
  });
});

// A separator marks where the conversation STOPPED and started again. The rule used to be
// "the formatted label changed", and since the label carries HH:MM that meant "a minute
// later", so nearly every turn wore a timestamp and none of them marked anything
// (chat-time-separator).
describe('separatorLabels', () => {
  // Same day as NOW (2026-06-29), so the labels read "Today HH:MM".
  const at = (h: number, m: number, day = 29) => new Date(2026, 5, day, h, m).toISOString();

  it('says nothing through a working back-and-forth', () => {
    const out = separatorLabels([at(14, 0), at(14, 2), at(14, 9), at(14, 20)], 'en', NOW);
    expect(out.slice(1)).toEqual(['', '', '']); // 20 past the hour is still 20 min from 14:00
    expect(out[0]).toBe('Today 14:00'); // the history has a beginning
  });

  it('marks a real pause', () => {
    const out = separatorLabels([at(9, 0), at(9, 5), at(10, 30), at(10, 33)], 'en', NOW);
    expect(out).toEqual(['Today 09:00', '', 'Today 10:30', '']);
  });

  it('marks the turn that crosses midnight, whatever the gap', () => {
    const late = new Date(2026, 5, 28, 23, 58).toISOString();
    const soon = new Date(2026, 5, 29, 0, 1).toISOString(); // three minutes later, new day
    expect(separatorLabels([late, soon], 'en', NOW)[1]).toBe('Today 00:01');
  });

  it('measures the gap from the previous turn, not from the last separator', () => {
    // Four turns 10 minutes apart span 30, and none of the steps reaches the gap.
    const out = separatorLabels([at(9, 0), at(9, 10), at(9, 20), at(9, 30)], 'en', NOW);
    expect(out.slice(1)).toEqual(['', '', '']);
  });

  it('is invisible to a turn with no usable clock', () => {
    const out = separatorLabels([at(9, 0), undefined, 'not-a-date', at(9, 4)], 'en', NOW);
    expect(out).toEqual(['Today 09:00', '', '', '']); // the clockless pair neither marks nor breaks
  });

  it('speaks the reader’s language and honours the gap it is given', () => {
    expect(separatorLabels([at(9, 0), at(9, 6)], 'zh', NOW, 5)[1]).toBe('今天 09:06');
    expect(separatorLabels([at(9, 0), at(9, 6)], 'zh', NOW, 30)[1]).toBe('');
  });

  it('holds its own on an empty conversation', () => {
    expect(separatorLabels([], 'en', NOW)).toEqual([]);
  });
});
