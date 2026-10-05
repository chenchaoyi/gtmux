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

  // A log's lines are unique, but the ROWS they wrap into are not: the tail of a long line
  // ("·······", "ok"), and blank lines, repeat all over it. Numbering repeated rows from the
  // top renumbered every one of them below the rows that scrolled off, so they re-rendered
  // too. Measured on a 2000-line log scrolling 30 lines a poll: 274 of 1244 rows.
  test('a SCROLL through rows that repeat re-renders only what is new', () => {
    const tail = (k: number) => '·'.repeat(80 + (k % 3)); // wraps at 80 into a repeating tail
    const log = (from: number, n: number) =>
      Array.from({length: n}, (_, i) => (((from + i) % 4 === 3) ? '' : `step ${from + i} ${tail(from + i)}`));
    const a = frame(log(0, 120));
    const b = frame(log(5, 120)); // five lines left at the top, five new at the bottom
    const newRows = b.rows.length - survivors(a, b);
    // The five new lines are at most ten rows; anything near b.rows.length means repeats
    // are numbered from the top again.
    expect(newRows).toBeLessThanOrEqual(10);
    expect(new Set(b.rows.map(r => r.key)).size).toBe(b.rows.length);
  });

  test('keys stay unique when whole lines repeat', () => {
    const g = frame(['a', '', 'b', '', 'a', '', '', 'a', '', 'b', '']);
    expect(new Set(g.rows.map(r => r.key)).size).toBe(g.rows.length);
  });
});
