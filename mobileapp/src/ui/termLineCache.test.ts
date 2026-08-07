import {parseAnsi} from './ansi';
import {makeLineCache, parseLinesCached} from './termLineCache';

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
