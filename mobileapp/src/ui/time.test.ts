import {fmtTurnTime} from './time';

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
