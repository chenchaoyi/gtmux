import {parseAnsi} from './ansi';
import {charCells, wrapLine} from './term';
import {makeLineCache, parseLinesCached, wrapLinesCached} from './termLineCache';

const OPTS = {base: '#D6D6DA', bg: true};
const E = '\u001b';

const SAMPLE = [
  `${E}[32m✳${E}[0m Sautéed for 44s`,
  'plain text line',
  '',
  `${E}[1;31mbold red${E}[0m and ${E}[38;5;208m256-orange${E}[0m`,
  `${E}[38;2;10;200;100mtruecolor${E}[0m tail`,
];

describe('parseLinesCached', () => {
  it('matches the whole-buffer parseAnsi exactly (parseAnsi is per-line stateless)', () => {
    // The cache is only correct while a line's parse depends on nothing but its own
    // text. If parseAnsi ever carries SGR state across lines, this test fails first.
    const whole = parseAnsi(SAMPLE.join('\n'), OPTS);
    const cached = parseLinesCached(SAMPLE, OPTS, makeLineCache());
    expect(cached).toEqual(whole);
    expect(cached[3].length).toBeGreaterThan(1); // sanity: SGR actually parsed into styled spans
  });

  it('reuses span-array identities for unchanged lines across calls', () => {
    const cache = makeLineCache();
    const a = parseLinesCached(SAMPLE, OPTS, cache);
    const b = parseLinesCached(SAMPLE, OPTS, cache);
    b.forEach((spans, i) => expect(spans).toBe(a[i]));
  });

  it('re-parses only the changed line; neighbors keep identity', () => {
    const cache = makeLineCache();
    const a = parseLinesCached(SAMPLE, OPTS, cache);
    const changed = [...SAMPLE];
    changed[3] = `${E}[1;31mbold red${E}[0m CHANGED`;
    const b = parseLinesCached(changed, OPTS, cache);
    expect(b[0]).toBe(a[0]);
    expect(b[1]).toBe(a[1]);
    expect(b[2]).toBe(a[2]);
    expect(b[3]).not.toBe(a[3]);
    expect(b[4]).toBe(a[4]);
  });

  it('shifted lines (scroll) still reuse identities — content-addressed, not index-addressed', () => {
    const cache = makeLineCache();
    const a = parseLinesCached(SAMPLE, OPTS, cache);
    const scrolled = [...SAMPLE.slice(1), 'new bottom line'];
    const b = parseLinesCached(scrolled, OPTS, cache);
    // old line i+1 is new line i — same raw text, same spans identity
    for (let i = 0; i < 4; i++) {
      expect(b[i]).toBe(a[i + 1]);
    }
  });

  it('duplicate raw lines share one identity within a call', () => {
    const cache = makeLineCache();
    const out = parseLinesCached(['', 'x', '', 'x'], OPTS, cache);
    expect(out[0]).toBe(out[2]);
    expect(out[1]).toBe(out[3]);
  });

  it('an options change invalidates the cache', () => {
    const cache = makeLineCache();
    const a = parseLinesCached(SAMPLE, OPTS, cache);
    const b = parseLinesCached(SAMPLE, {...OPTS, base: '#FFFFFF'}, cache);
    expect(b[1]).not.toBe(a[1]);
    expect(b[1][0].color).toBe('#FFFFFF');
  });

  it('evicts lines absent from the current frame (two-generation)', () => {
    const cache = makeLineCache();
    const a = parseLinesCached(['gone', 'stays'], OPTS, cache);
    parseLinesCached(['stays'], OPTS, cache); // 'gone' drops out
    expect(cache.map.size).toBe(1);
    const c = parseLinesCached(['gone', 'stays'], OPTS, cache);
    expect(c[0]).not.toBe(a[0]); // re-parsed after eviction
    expect(c[1]).toBe(a[1]); // never left, identity preserved throughout
  });
});

// ————— Wrapped-rows cache (mobile-native-term-selection Stage 1, iOS grid) —————

const LAYOUT = {cols: 10, fontSize: 12};
const WRAP_SAMPLE = [
  ...SAMPLE,
  '这是一行没有空格的中文长内容会被硬折行成多个视觉行', // no-space CJK long line (the classic wrap case)
  'a plain ASCII line long enough to need several visual rows at ten cells',
];

describe('wrapLinesCached', () => {
  it('parity: joined rows text equals the logical line text; no row exceeds cols', () => {
    const {lines, rows} = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, makeLineCache());
    const txt = (spans: {text: string}[]) => spans.map(s => s.text).join('');
    lines.forEach((line, i) => {
      expect(rows[i].map(txt).join('')).toBe(txt(line)); // no loss
      for (const row of rows[i]) {
        const c = row.reduce((n, s) => n + [...s.text].reduce((m, ch) => m + charCells(ch), 0), 0);
        expect(c).toBeLessThanOrEqual(LAYOUT.cols);
      }
    });
  });

  it('rows match an uncached wrapLine of the parsed spans', () => {
    const {lines, rows} = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, makeLineCache());
    lines.forEach((line, i) => expect(rows[i]).toEqual(wrapLine(line, LAYOUT.cols)));
  });

  it('unchanged lines keep BOTH spans and rows identities across calls', () => {
    const cache = makeLineCache();
    const a = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, cache);
    const b = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, cache);
    b.lines.forEach((spans, i) => expect(spans).toBe(a.lines[i]));
    b.rows.forEach((rows, i) => expect(rows).toBe(a.rows[i]));
  });

  it('re-wraps only the changed line; neighbors keep rows identity', () => {
    const cache = makeLineCache();
    const a = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, cache);
    const changed = [...WRAP_SAMPLE];
    changed[1] = 'a different line';
    const b = wrapLinesCached(changed, OPTS, LAYOUT, cache);
    expect(b.rows[0]).toBe(a.rows[0]);
    expect(b.rows[1]).not.toBe(a.rows[1]);
    expect(b.rows[5]).toBe(a.rows[5]);
  });

  it('a cols change invalidates the wrapped rows', () => {
    const cache = makeLineCache();
    const a = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, cache);
    const b = wrapLinesCached(WRAP_SAMPLE, OPTS, {...LAYOUT, cols: 20}, cache);
    expect(b.rows[5]).not.toBe(a.rows[5]);
    expect(b.rows[5].length).toBeLessThan(a.rows[5].length); // wider → fewer visual rows
  });

  it('a fontSize change invalidates too (part of the signature)', () => {
    const cache = makeLineCache();
    const a = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, cache);
    const b = wrapLinesCached(WRAP_SAMPLE, OPTS, {...LAYOUT, fontSize: 16}, cache);
    expect(b.rows[0]).not.toBe(a.rows[0]);
  });

  it('shifted lines (scroll) still reuse rows identity — content-addressed', () => {
    const cache = makeLineCache();
    const a = wrapLinesCached(WRAP_SAMPLE, OPTS, LAYOUT, cache);
    const scrolled = [...WRAP_SAMPLE.slice(1), 'new bottom line'];
    const b = wrapLinesCached(scrolled, OPTS, LAYOUT, cache);
    for (let i = 0; i < WRAP_SAMPLE.length - 1; i++) {
      expect(b.lines[i]).toBe(a.lines[i + 1]);
      expect(b.rows[i]).toBe(a.rows[i + 1]);
    }
  });

  it('an empty logical line still owns one (empty) visual row', () => {
    const {rows} = wrapLinesCached([''], OPTS, LAYOUT, makeLineCache());
    expect(rows[0]).toEqual([[]]);
  });
});
