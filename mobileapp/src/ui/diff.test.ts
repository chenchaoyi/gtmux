import {diffLineColor} from './diff';

// Fixed colors for the diff's always-dark surface (mirror diff.ts).
const META = 'rgba(235,235,245,0.4)';
const CONTEXT = 'rgba(235,235,245,0.8)';

describe('diffLineColor', () => {
  it('colors additions green and deletions red', () => {
    expect(diffLineColor('+added')).toBe('#22C55E');
    expect(diffLineColor('-removed')).toBe('#EF4444');
  });
  it('colors hunk headers cyan', () => {
    expect(diffLineColor('@@ -1,3 +1,4 @@')).toBe('#06B6D4');
  });
  it('dims file headers/meta before the +/- rule (so +++/--- are not green/red)', () => {
    expect(diffLineColor('+++ b/x')).toBe(META);
    expect(diffLineColor('--- a/x')).toBe(META);
    expect(diffLineColor('diff --git a/x b/x')).toBe(META);
    expect(diffLineColor('index 1a2b..3c4d')).toBe(META);
    expect(diffLineColor('# branch main')).toBe(META);
  });
  it('uses context color for unchanged/blank lines', () => {
    expect(diffLineColor(' unchanged')).toBe(CONTEXT);
    expect(diffLineColor('')).toBe(CONTEXT);
  });
});
