import {diffLineColor} from './diff';
import {paletteFor} from './theme';

const pal = paletteFor('dark');

describe('diffLineColor', () => {
  it('colors additions green and deletions red', () => {
    expect(diffLineColor('+added', pal)).toBe('#22C55E');
    expect(diffLineColor('-removed', pal)).toBe('#EF4444');
  });
  it('colors hunk headers cyan', () => {
    expect(diffLineColor('@@ -1,3 +1,4 @@', pal)).toBe('#06B6D4');
  });
  it('dims file headers/meta before the +/- rule (so +++/--- are not green/red)', () => {
    expect(diffLineColor('+++ b/x', pal)).toBe(pal.fg3);
    expect(diffLineColor('--- a/x', pal)).toBe(pal.fg3);
    expect(diffLineColor('diff --git a/x b/x', pal)).toBe(pal.fg3);
    expect(diffLineColor('index 1a2b..3c4d', pal)).toBe(pal.fg3);
    expect(diffLineColor('# branch main', pal)).toBe(pal.fg3);
  });
  it('uses context color for unchanged/blank lines', () => {
    expect(diffLineColor(' unchanged', pal)).toBe(pal.fg2);
    expect(diffLineColor('', pal)).toBe(pal.fg2);
  });
});
