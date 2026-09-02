import {AnsiLine} from './ansi';
import {flattenGrid} from './term';
import {makeLineCache, wrapLinesCached} from './termLineCache';

// How many rendered rows KEEP both their key and their span identity between two frames?
// Those are the rows a React.memo'd <TermLine key={r.key}> can bail on; the rest are
// re-rendered. This is the measurement behind "typing stutters while the terminal
// refreshes": the cost of a refresh is exactly the number that do NOT survive.
function survivors(a: {rows: Array<{key: string; spans: AnsiLine}>}, b: typeof a): number {
  const before = new Map(a.rows.map(r => [r.key, r.spans]));
  return b.rows.filter(r => before.get(r.key) === r.spans).length;
}

const cache = makeLineCache();
function frame(lines: string[]) {
  const p = wrapLinesCached(lines, {}, {cols: 80, fontSize: 12}, cache);
  return flattenGrid(p.lines, p.lines, p.rows, 80);
}

const screen = (n: number, tail: string) => [...Array.from({length: n}, (_, i) => `line ${i}`), tail];

describe('what a terminal refresh actually re-renders', () => {
  test('an IN-PLACE repaint re-renders only the line that changed', () => {
    const a = frame(screen(60, 'working 3s'));
    const b = frame(screen(60, 'working 4s'));
    // 61 lines; only the footer moved.
    expect(b.rows.length - survivors(a, b)).toBe(1);
  });

  // The case a busy pane hits constantly: new output scrolls the screen. Every line
  // keeps its TEXT and therefore its cached spans, and with CONTENT-addressed keys it
  // keeps its key too, so React drops the row that scrolled off and leaves the rest
  // alone. Measured before the keys changed: 61 of 61 rows re-rendered for one new line,
  // on every poll, while the user was typing into the composer beside it.
  test('a SCROLL re-renders only the new line, not the whole grid', () => {
    const a = frame(screen(60, 'tail'));
    const b = frame([...screen(60, 'tail').slice(1), 'brand new line']);
    // 1, not 61. If this climbs back toward b.rows.length the keying went positional
    // again and every refresh is repainting the whole terminal.
    expect(b.rows.length - survivors(a, b)).toBe(1);
  });
});
