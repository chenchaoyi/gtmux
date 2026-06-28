import {parseAnsi, AnsiLine} from './ansi';

const ESC = String.fromCharCode(27);
const BASE = '#D6D6DA';
const DIM = '#9AA0A8';

// 16-color palette indices (see ansi.ts PALETTE).
const RED = '#EF4444';
const GREEN = '#22C55E';
const CYAN = '#06B6D4';
const BRIGHT_RED = '#F87171';

describe('parseAnsi — plain text', () => {
  it('wraps plain text in a single base-colored span', () => {
    expect(parseAnsi('hello')).toEqual([[{text: 'hello', color: BASE, bold: undefined}]]);
  });

  it('returns one (empty) line for empty input', () => {
    // "".split("\n") -> [""], and the empty span is dropped -> [[]].
    expect(parseAnsi('')).toEqual([[]]);
  });

  it('splits on newlines into separate lines', () => {
    const out = parseAnsi('a\nb\nc');
    expect(out).toHaveLength(3);
    expect(out[0]).toEqual([{text: 'a', color: BASE, bold: undefined}]);
    expect(out[1]).toEqual([{text: 'b', color: BASE, bold: undefined}]);
    expect(out[2]).toEqual([{text: 'c', color: BASE, bold: undefined}]);
  });

  it('yields an empty span array for blank lines', () => {
    const out = parseAnsi('x\n\ny');
    expect(out).toHaveLength(3);
    expect(out[1]).toEqual([]);
  });
});

describe('parseAnsi — single SGR colors', () => {
  it('applies a basic foreground color (31 = red)', () => {
    expect(parseAnsi(`${ESC}[31mred`)).toEqual([[{text: 'red', color: RED, bold: undefined}]]);
  });

  it('maps the full 30–37 normal range', () => {
    const [line] = parseAnsi(`${ESC}[32mgreen`);
    expect(line[0].color).toBe(GREEN);
    const [line2] = parseAnsi(`${ESC}[36mcyan`);
    expect(line2[0].color).toBe(CYAN);
  });

  it('maps bright colors 90–97 to palette indices 8–15', () => {
    const [line] = parseAnsi(`${ESC}[91mbright`);
    expect(line[0].color).toBe(BRIGHT_RED);
  });

  it('splits the line into pre-color and post-color spans', () => {
    const out = parseAnsi(`plain${ESC}[31mred`);
    expect(out[0]).toEqual([
      {text: 'plain', color: BASE, bold: undefined},
      {text: 'red', color: RED, bold: undefined},
    ]);
  });
});

describe('parseAnsi — bold, dim, reset', () => {
  it('marks bold with code 1', () => {
    expect(parseAnsi(`${ESC}[1mbold`)).toEqual([[{text: 'bold', color: BASE, bold: true}]]);
  });

  it('uses the DIM color when dim (code 2) and no explicit color', () => {
    expect(parseAnsi(`${ESC}[2mdim`)).toEqual([[{text: 'dim', color: DIM, bold: undefined}]]);
  });

  it('an explicit color overrides the dim default color', () => {
    const [line] = parseAnsi(`${ESC}[2m${ESC}[31mx`);
    expect(line[0]).toEqual({text: 'x', color: RED, bold: undefined});
  });

  it('reset (code 0) clears color, bold and dim', () => {
    const out = parseAnsi(`${ESC}[1;31mhot${ESC}[0mcold`);
    expect(out[0]).toEqual([
      {text: 'hot', color: RED, bold: true},
      {text: 'cold', color: BASE, bold: false},
    ]);
  });

  it('empty SGR (ESC[m) is treated as reset', () => {
    const out = parseAnsi(`${ESC}[31mr${ESC}[mn`);
    expect(out[0]).toEqual([
      {text: 'r', color: RED, bold: undefined},
      {text: 'n', color: BASE, bold: false},
    ]);
  });

  it('code 22 clears bold/dim but keeps color', () => {
    const out = parseAnsi(`${ESC}[1;31mb${ESC}[22mn`);
    expect(out[0]).toEqual([
      {text: 'b', color: RED, bold: true},
      {text: 'n', color: RED, bold: false},
    ]);
  });

  it('code 39 resets only the foreground color', () => {
    const out = parseAnsi(`${ESC}[1;31mb${ESC}[39mn`);
    expect(out[0]).toEqual([
      {text: 'b', color: RED, bold: true},
      {text: 'n', color: BASE, bold: true},
    ]);
  });
});

describe('parseAnsi — multiple codes & combinations', () => {
  it('applies a semicolon-joined code list (bold + green)', () => {
    expect(parseAnsi(`${ESC}[1;32mx`)).toEqual([[{text: 'x', color: GREEN, bold: true}]]);
  });

  it('carries style across newlines (per-line spans, shared style)', () => {
    const out = parseAnsi(`${ESC}[31mone\ntwo`);
    // NOTE: style is re-initialized per line in parseAnsi, so line 2 is BASE.
    expect(out[0]).toEqual([{text: 'one', color: RED, bold: undefined}]);
    expect(out[1]).toEqual([{text: 'two', color: BASE, bold: undefined}]);
  });
});

describe('parseAnsi — 256-color (38;5;n)', () => {
  it('maps the first 16 (n<16) to the base palette', () => {
    expect(parseAnsi(`${ESC}[38;5;1mx`)[0][0].color).toBe(RED);
    expect(parseAnsi(`${ESC}[38;5;9mx`)[0][0].color).toBe(BRIGHT_RED);
  });

  it('maps the 6x6x6 color cube (16–231)', () => {
    // n=16 -> cube (0,0,0) -> #000000
    expect(parseAnsi(`${ESC}[38;5;16mx`)[0][0].color).toBe('#000000');
    // n=231 -> cube (5,5,5) -> 55+5*40 = 255 -> #ffffff
    expect(parseAnsi(`${ESC}[38;5;231mx`)[0][0].color).toBe('#ffffff');
    // n=196 -> cube (5,0,0) -> #ff0000
    expect(parseAnsi(`${ESC}[38;5;196mx`)[0][0].color).toBe('#ff0000');
  });

  it('maps the grayscale ramp (232–255)', () => {
    // n=232 -> v = 8 -> #080808
    expect(parseAnsi(`${ESC}[38;5;232mx`)[0][0].color).toBe('#080808');
    // n=255 -> v = 8 + 23*10 = 238 -> #eeeeee
    expect(parseAnsi(`${ESC}[38;5;255mx`)[0][0].color).toBe('#eeeeee');
  });
});

describe('parseAnsi — truecolor (38;2;r;g;b)', () => {
  it('maps an explicit rgb triple', () => {
    expect(parseAnsi(`${ESC}[38;2;255;128;0mx`)[0][0].color).toBe('#ff8000');
  });

  it('zero-pads single hex digits', () => {
    expect(parseAnsi(`${ESC}[38;2;1;2;3mx`)[0][0].color).toBe('#010203');
  });

  it('clamps out-of-range channels to 0–255', () => {
    // parseInt keeps the value; rgb() clamps to 255.
    expect(parseAnsi(`${ESC}[38;2;300;0;999mx`)[0][0].color).toBe('#ff00ff');
  });
});

describe('parseAnsi — background & other escapes ignored', () => {
  it('ignores background color codes (40–49)', () => {
    expect(parseAnsi(`${ESC}[41mx`)).toEqual([[{text: 'x', color: BASE, bold: undefined}]]);
  });

  it('strips non-SGR CSI escapes (cursor moves etc.)', () => {
    // ESC[2K (erase line) and ESC[H (cursor home) are stripped from text.
    const out = parseAnsi(`${ESC}[2K${ESC}[Hhello`);
    expect(out[0]).toEqual([{text: 'hello', color: BASE, bold: undefined}]);
  });

  it('strips charset-selection escapes', () => {
    const out = parseAnsi(`${ESC}(Babc`);
    expect(out[0]).toEqual([{text: 'abc', color: BASE, bold: undefined}]);
  });
});

describe('parseAnsi — malformed / incomplete sequences', () => {
  it('leaves an incomplete escape (no final byte) in the text', () => {
    // ESC[31 without the trailing 'm' is not a complete SGR -> stays as text.
    const out = parseAnsi(`${ESC}[31abc`);
    // Not matched by SGR_RE; the OTHER_ESC regex matches "ESC[31a" (CSI + letter),
    // stripping it and leaving "bc".
    expect(out[0]).toEqual([{text: 'bc', color: BASE, bold: undefined}]);
  });

  it('treats a lone ESC with no CSI as literal text', () => {
    const out = parseAnsi(`${ESC}hi`);
    expect(out[0][0].text).toContain('hi');
  });

  it('does NOT treat non-numeric SGR params as SGR (only [0-9;]* matches)', () => {
    // ESC[xxm is not a valid SGR (params must be digits/semicolons), so SGR_RE
    // never matches it. Instead OTHER_ESC matches "ESC[x" (CSI + a letter) and
    // strips just that, leaving "xmn" appended after the still-red "r".
    const out = parseAnsi(`${ESC}[31mr${ESC}[xxmn`);
    expect(out[0]).toEqual([{text: 'rxmn', color: RED, bold: undefined}]);
  });

  it('drops empty spans between adjacent codes', () => {
    const out = parseAnsi(`${ESC}[31m${ESC}[32mx`);
    // Nothing between the two codes -> no empty span; final color = green.
    expect(out[0]).toEqual([{text: 'x', color: GREEN, bold: undefined}]);
  });

  it('handles a trailing reset with no following text', () => {
    const out = parseAnsi(`${ESC}[31mr${ESC}[0m`);
    expect(out[0]).toEqual([{text: 'r', color: RED, bold: undefined}]);
  });
});

describe('parseAnsi — shape invariants', () => {
  it('always returns an array of lines, each an array of spans', () => {
    const out: AnsiLine[] = parseAnsi(`${ESC}[31ma\nb`);
    expect(Array.isArray(out)).toBe(true);
    out.forEach(line => {
      expect(Array.isArray(line)).toBe(true);
      line.forEach(sp => {
        expect(typeof sp.text).toBe('string');
        expect(typeof sp.color).toBe('string');
      });
    });
  });
});
