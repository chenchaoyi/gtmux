import {PAD, cellWidthFor, charCells, colsFor, cursorSpans, linkify, linkSegsForLines, nativeFontFamily, normalizeGlyphs, rowHeightFor, wrapLine, DOT_REC, DOT_CIRCLE} from './term';
import {AnsiLine} from './ansi';

describe('nativeFontFamily', () => {
  it('resolves every pref to the system monospace (native links no picker fonts yet)', () => {
    for (const pref of [undefined, 'auto', 'system', 'JetBrains Mono', 'Hack']) {
      expect(nativeFontFamily(pref)).toBe('Menlo');
    }
  });
});

describe('normalizeGlyphs', () => {
  it('maps U+23FA record glyph to U+25CF black circle', () => {
    expect(normalizeGlyphs(`${DOT_REC} Update`)).toBe(`${DOT_CIRCLE} Update`);
  });

  it('replaces every occurrence', () => {
    expect(normalizeGlyphs(`${DOT_REC}a${DOT_REC}`)).toBe(`${DOT_CIRCLE}a${DOT_CIRCLE}`);
  });

  it('leaves text without the glyph untouched (same string)', () => {
    const s = 'no record glyph here';
    expect(normalizeGlyphs(s)).toBe(s);
  });

  it('forces text presentation on a BARE text-default symbol (⏸ manual mode)', () => {
    // U+23F8 alone → append U+FE0E so Core Text renders the text pause, like a terminal.
    expect(normalizeGlyphs('⏸ manual mode')).toBe('⏸\uFE0E manual mode');
  });

  it('leaves an agent-requested emoji (…U+FE0F) as color', () => {
    // the agent asked for the emoji explicitly → don't force text.
    expect(normalizeGlyphs('⏸\uFE0F done')).toBe('⏸\uFE0F done');
  });

  it('leaves a default-presentation emoji (✅) as color', () => {
    // ✅ (Emoji_Presentation=Yes) is not in the set → untouched.
    expect(normalizeGlyphs('✅ ok')).toBe('✅ ok');
  });

  it("doesn't double the selector if one is already present", () => {
    expect(normalizeGlyphs('⏸\uFE0E x')).toBe('⏸\uFE0E x');
  });
});

const CUR = '#bbc1ff';
const BG = '#17171a';

// total text length of a line's spans
const len = (l: AnsiLine) => l.reduce((n, s) => n + s.text.length, 0);
// flatten a line's text
const txt = (l: AnsiLine) => l.map(s => s.text).join('');

describe('cursorSpans', () => {
  it('paints a reverse-video cell inside a span (splits around column x)', () => {
    const line: AnsiLine = [{text: 'abcde', color: '#fff'}];
    const out = cursorSpans(line, 2, CUR, BG);
    expect(txt(out)).toBe('abcde'); // content preserved
    // the cell at x=2 is the char 'c', reverse-video (fg=bg, bg=cursor color)
    const cell = out.find(s => s.bg === CUR);
    expect(cell).toBeTruthy();
    expect(cell!.text).toBe('c');
    expect(cell!.color).toBe(BG);
  });

  it('appends a blank cell at end-of-line when x === lineLen', () => {
    const line: AnsiLine = [{text: 'ab', color: '#fff'}];
    const out = cursorSpans(line, 2, CUR, BG);
    expect(txt(out)).toBe('ab '); // one padded cell
    const cell = out[out.length - 1];
    expect(cell.bg).toBe(CUR);
    expect(cell.text).toBe(' ');
  });

  it('pads with spaces when x is past the content', () => {
    const line: AnsiLine = [{text: 'ab', color: '#fff'}];
    const out = cursorSpans(line, 5, CUR, BG);
    expect(len(out)).toBe(6); // 'ab' + 3 pad + 1 cursor cell
    expect(out[out.length - 1].bg).toBe(CUR);
  });

  it('handles an empty line (cursor at column 0)', () => {
    const out = cursorSpans([], 0, CUR, BG);
    expect(out).toHaveLength(1);
    expect(out[0].bg).toBe(CUR);
    expect(out[0].text).toBe(' ');
  });

  it('preserves bold on the painted cell', () => {
    const line: AnsiLine = [{text: 'xy', color: '#fff', bold: true}];
    const out = cursorSpans(line, 0, CUR, BG);
    const cell = out.find(s => s.bg === CUR);
    expect(cell!.bold).toBe(true);
  });

  it('keeps spans on either side of the cursor cell', () => {
    const line: AnsiLine = [{text: 'red', color: '#f00'}, {text: 'grn', color: '#0f0'}];
    const out = cursorSpans(line, 3, CUR, BG); // 'g' = first char of the second span
    expect(txt(out)).toBe('redgrn');
    const cell = out.find(s => s.bg === CUR);
    expect(cell!.text).toBe('g');
  });
});

describe('linkify', () => {
  it('leaves URL-free text as one plain segment', () => {
    expect(linkify('just some output')).toEqual([{text: 'just some output'}]);
    expect(linkify('')).toEqual([{text: ''}]);
  });

  it('detects a bare https URL incl. hyphens, keeping surrounding text', () => {
    const url = 'https://claude.ai/code/artifact/7528ce32-2fa8-4064-a4c0-e6f7680d6831';
    expect(linkify(`see ${url} now`)).toEqual([{text: 'see '}, {text: url, url}, {text: ' now'}]);
  });

  it('keeps trailing sentence punctuation OUT of the link', () => {
    expect(linkify('open https://foo.com/x.')).toEqual([
      {text: 'open '},
      {text: 'https://foo.com/x', url: 'https://foo.com/x'},
      {text: '.'},
    ]);
    expect(linkify('(https://foo.com)')).toEqual([
      {text: '('},
      {text: 'https://foo.com', url: 'https://foo.com'},
      {text: ')'},
    ]);
  });

  it('handles multiple URLs and http as well as https', () => {
    const out = linkify('a http://x.io b https://y.io c');
    expect(out.filter(s => s.url).map(s => s.url)).toEqual(['http://x.io', 'https://y.io']);
  });

  it('does not linkify a non-http scheme (file:// stays plain)', () => {
    expect(linkify('file:///Users/x/a.png')).toEqual([{text: 'file:///Users/x/a.png'}]);
  });
});

describe('linkSegsForLines', () => {
  it('makes an OSC 8 hyperlink span (anchor text) tappable via its href', () => {
    // The reported bug: bare URLs tapped through but anchor-text (OSC 8) links did not,
    // because the overlay only carried linkify(bare). Now span.href rides the overlay.
    const lines: AnsiLine[] = [
      [
        {text: 'see ', color: '#fff'},
        {text: 'Wikipedia', color: '#5af', href: 'https://en.wikipedia.org/wiki/Sonder'},
        {text: ' now', color: '#fff'},
      ],
    ];
    expect(linkSegsForLines(lines)).toEqual([
      {text: 'see '},
      {text: 'Wikipedia', url: 'https://en.wikipedia.org/wiki/Sonder'},
      {text: ' now'},
    ]);
  });

  it('still tags a bare URL the agent printed as plain text', () => {
    const lines: AnsiLine[] = [[{text: 'open https://foo.com/x here', color: '#fff'}]];
    expect(linkSegsForLines(lines)).toEqual([
      {text: 'open '},
      {text: 'https://foo.com/x', url: 'https://foo.com/x'},
      {text: ' here'},
    ]);
  });

  it('catches a bare URL split across SGR spans within a line', () => {
    const lines: AnsiLine[] = [[{text: 'https://foo.com/', color: '#f00'}, {text: 'a-b', color: '#0f0'}]];
    expect(linkSegsForLines(lines).filter(s => s.url).map(s => s.url)).toEqual(['https://foo.com/a-b']);
  });

  it('ignores a non-web OSC 8 href (file:// image ref stays plain, not tappable)', () => {
    const lines: AnsiLine[] = [[{text: 'img', color: '#fff', href: 'file:///a.png'}]];
    expect(linkSegsForLines(lines)).toEqual([{text: 'img'}]);
  });

  it('joins lines with newline and preserves the plain text exactly', () => {
    const lines: AnsiLine[] = [[{text: 'a', color: '#fff'}], [{text: 'b', color: '#fff'}]];
    const segs = linkSegsForLines(lines);
    expect(segs.map(s => s.text).join('')).toBe('a\nb');
  });
});

// ————— Uniform grid (mobile-native-term-selection Stage 1) —————

// cell cost of a row, by the same arithmetic the wrapper uses
const cells = (l: AnsiLine) => l.reduce((n, s) => n + [...s.text].reduce((m, ch) => m + charCells(ch), 0), 0);

describe('grid metrics', () => {
  it('rowHeightFor = round(fontSize * 1.6) — one uniform row height', () => {
    expect(rowHeightFor(12)).toBe(19);
    expect(rowHeightFor(14)).toBe(22);
    expect(rowHeightFor(16)).toBe(26);
  });

  it('cellWidthFor = Menlo advance ratio 0.602, 2 decimals', () => {
    expect(cellWidthFor(12)).toBe(7.22);
    expect(cellWidthFor(14)).toBe(8.43);
  });

  it('colsFor bakes in the −2×PAD padding and the conservative −1 cell', () => {
    // 390pt phone at 12pt: (390−12)/7.22 = 52.35… → 52 − 1 = 51
    expect(colsFor(390, 12)).toBe(51);
    expect(colsFor(390, 12)).toBe(Math.floor((390 - PAD * 2) / cellWidthFor(12)) - 1);
    // PAD is the shared constant three consumers couple through (styles.pad,
    // colsFor, the native selection overlay's pad props) — pin its value.
    expect(PAD).toBe(6);
  });

  it('colsFor never goes below 4 (degenerate widths stay renderable)', () => {
    expect(colsFor(0, 12)).toBe(4);
    expect(colsFor(20, 40)).toBe(4);
  });
});

describe('charCells', () => {
  it('East-Asian wide/fullwidth chars cost 2 cells', () => {
    for (const ch of ['中', '文', '。', '、', '「', 'あ', 'ア', '한', 'Ａ', '１', '，', '😀']) {
      expect(charCells(ch)).toBe(2);
    }
  });

  it('Latin/ASCII costs 1 cell', () => {
    for (const ch of ['a', 'Z', ' ', '-', '⏸']) {
      expect(charCells(ch)).toBe(1);
    }
  });

  it('variation selectors and ZWJ cost 0 (they modify a base glyph)', () => {
    expect(charCells('︎')).toBe(0);
    expect(charCells('️')).toBe(0);
    expect(charCells('‍')).toBe(0);
  });
});

describe('wrapLine', () => {
  const S = (text: string, extra: Partial<AnsiLine[number]> = {}): AnsiLine[number] => ({text, color: '#fff', ...extra});
  const rowTxt = (rows: AnsiLine[]) => rows.map(r => r.map(s => s.text).join(''));

  it('loses nothing: joined rows equal the original line text', () => {
    const line = [S('hello '), S('世界 and 更多的中文内容 mixed in', {color: '#0f0'})];
    const rows = wrapLine(line, 8);
    expect(rowTxt(rows).join('')).toBe('hello 世界 and 更多的中文内容 mixed in');
  });

  it('no row exceeds the cell budget', () => {
    const line = [S('abc 中文混排 def 一二三四五六七八九十 ghi jkl mno')];
    for (const cols of [4, 7, 10, 33]) {
      for (const row of wrapLine(line, cols)) {
        expect(cells(row)).toBeLessThanOrEqual(cols);
      }
    }
  });

  it('splits MID-SPAN and preserves color/bold/bg/href on both halves', () => {
    const line = [S('abcdefgh', {bold: true, bg: '#222', href: 'https://x.io'})];
    const rows = wrapLine(line, 4);
    expect(rowTxt(rows)).toEqual(['abcd', 'efgh']);
    for (const row of rows) {
      expect(row).toHaveLength(1);
      expect(row[0].color).toBe('#fff');
      expect(row[0].bold).toBe(true);
      expect(row[0].bg).toBe('#222');
      expect(row[0].href).toBe('https://x.io');
    }
  });

  it('CJK counts 2 cells', () => {
    expect(rowTxt(wrapLine([S('一二三四五')], 4))).toEqual(['一二', '三四', '五']);
  });

  it('a wide char that does not fit the remaining cell moves whole to the next row', () => {
    // 'abc' = 3 cells, '中' would be 5 > 4 → next row (never half a glyph).
    expect(rowTxt(wrapLine([S('abc中')], 4))).toEqual(['abc', '中']);
  });

  it('variation selectors ride their base glyph at 0 cells', () => {
    // 一(2)+FE0F(0)+二(2) fills cols=4 exactly; 三 wraps.
    expect(rowTxt(wrapLine([S('一️二三')], 4))).toEqual(['一️二', '三']);
  });

  it('ZWJ costs 0 cells', () => {
    expect(rowTxt(wrapLine([S('a‍b')], 2))).toEqual(['a‍b']);
  });

  it('empty line yields ONE empty row (the grid still owns its height)', () => {
    expect(wrapLine([], 8)).toEqual([[]]);
  });

  it('an exact fit stays one row', () => {
    expect(rowTxt(wrapLine([S('abcd')], 4))).toEqual(['abcd']);
  });

  it('a break landing on a span edge keeps each span whole', () => {
    const rows = wrapLine([S('ab', {color: '#f00'}), S('cd', {color: '#0f0'})], 2);
    expect(rowTxt(rows)).toEqual(['ab', 'cd']);
    expect(rows[0][0].color).toBe('#f00');
    expect(rows[1][0].color).toBe('#0f0');
  });

  it('never splits a surrogate pair (code-point iteration)', () => {
    const rows = wrapLine([S('a😀b')], 3); // a(1)+😀(2)=3 → 'b' wraps
    expect(rowTxt(rows)).toEqual(['a😀', 'b']);
  });
});
