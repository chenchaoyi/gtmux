import {PAD, annotateUrls, cellWidthFor, charCells, colsFor, cursorSpans, flattenGrid, isBlankLine, linkify, linkSegsForLines, nativeFontFamily, normalizeGlyphs, renderView, rowHeightFor, tapTarget, wrapLine, DOT_REC, DOT_CIRCLE, jumpMayAnimate} from './term';
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

// ————— Trailing-blank trim (the all-black mostly-empty pane bug) —————

describe('isBlankLine', () => {
  const S = (text: string, extra: Partial<AnsiLine[number]> = {}): AnsiLine[number] => ({text, color: '#fff', ...extra});

  it('empty / whitespace-only lines are blank', () => {
    expect(isBlankLine([])).toBe(true);
    expect(isBlankLine([S('')])).toBe(true);
    expect(isBlankLine([S('   ')])).toBe(true);
  });

  it('any glyph makes it content', () => {
    expect(isBlankLine([S('  x ')])).toBe(false);
  });

  it('a bg-painted run of spaces is VISIBLE content (TUI statusline), not blank', () => {
    expect(isBlankLine([S('    ', {bg: '#222'})])).toBe(false);
  });
});

describe('renderView', () => {
  const S = (text: string): AnsiLine[number] => ({text, color: '#fff'});
  // capture-pane shape: grid rows then the trailing-newline blank ('') line.
  const capture = (rows: string[]): AnsiLine[] => [...rows.map(t => (t ? [S(t)] : [])), []];

  it('trims the blank tail of a mostly-empty tall pane (content starts visible)', () => {
    // 5 content lines + 45 blank grid rows: rendering all 50 bottom-anchored the
    // viewport into the blanks → the all-black pane. Keep only the content.
    const lines = capture(['a', 'b', 'c', 'd', 'e', ...Array(45).fill('')]);
    expect(renderView(lines)).toEqual({keep: 5, cursorRow: -1});
  });

  it('cursor mapping is computed on the UNTRIMMED array (bottom-anchored up)', () => {
    // cursor on the last content line: up = 45 blank rows below it.
    const lines = capture(['a', 'b', 'c', 'd', 'e', ...Array(45).fill('')]);
    expect(renderView(lines, {x: 0, up: 45, visible: true})).toEqual({keep: 5, cursorRow: 4});
  });

  it('after `clear`: cursor on the TOP row, everything below blank → keep 1', () => {
    const lines = capture(['$ ', ...Array(49).fill('')]);
    expect(renderView(lines, {x: 2, up: 49, visible: true})).toEqual({keep: 1, cursorRow: 0});
  });

  it('never trims the cursor row even when it sits BELOW the last content line', () => {
    // content rows 0..4, cursor resting on blank row 6 → rows 5..6 must survive.
    const lines = capture(['a', 'b', 'c', 'd', 'e', ...Array(45).fill('')]);
    expect(renderView(lines, {x: 0, up: 43, visible: true})).toEqual({keep: 7, cursorRow: 6});
  });

  it('a full pane (content on the bottom row) trims nothing but the newline blank', () => {
    const lines = capture(['a', 'b', 'c', 'prompt $']);
    expect(renderView(lines, {x: 8, up: 0, visible: true})).toEqual({keep: 4, cursorRow: 3});
  });

  it('a hidden cursor still trims (alt-screen TUIs hide it)', () => {
    const lines = capture(['a', '', '']);
    expect(renderView(lines, {x: 0, up: 0, visible: false})).toEqual({keep: 1, cursorRow: -1});
    expect(renderView(lines)).toEqual({keep: 1, cursorRow: -1});
  });

  it('an all-blank capture keeps one row (the grid never renders empty)', () => {
    expect(renderView(capture(['', '', '']))).toEqual({keep: 1, cursorRow: -1});
    expect(renderView([[]])).toEqual({keep: 1, cursorRow: -1});
  });

  it('a bg-painted blank row anchors the trim (statusline at the bottom survives)', () => {
    const lines: AnsiLine[] = [[S('top')], [{text: '   ', color: '#fff', bg: '#333'}], [], []];
    expect(renderView(lines)).toEqual({keep: 2, cursorRow: -1});
  });
});

describe('flattenGrid', () => {
  const S = (text: string): AnsiLine[number] => ({text, color: '#fff'});
  const CURSOR = '#bbc1ff';
  const DARK = '#17171a';

  // Build the sandwich exactly the way NativeTerm does: parse-free lines, a
  // cursor spliced into ONE of them (identity distinguishes it), wrap at cols.
  const build = (rawLines: string[], cursorLine: number, cursorX: number, cols: number) => {
    const lines: AnsiLine[] = rawLines.map(t => (t ? [S(t)] : []));
    const rendered = lines.slice();
    rendered[cursorLine] = cursorSpans(rendered[cursorLine] || [], cursorX, CURSOR, DARK);
    return flattenGrid(rendered, lines, null, cols);
  };

  it('THE invariant: overlay text rows == rendered stack rows, cursor padding included', () => {
    // cursor x=7 on a 4-char line at cols=4: the padded spliced line wraps into
    // 2 visual rows where the raw line wraps into 1 — the overlay must pad.
    const g = build(['abcd', 'xy', ''], 0, 7, 4);
    expect(g.text.split('\n')).toHaveLength(g.rows.length);
  });

  it('pads the EXTRA cursor wrap rows with empty overlay rows (no cursor-cell leak)', () => {
    const g = build(['abcd'], 0, 7, 4);
    // stack: 'abcd' + the padded cursor row; overlay: 'abcd' + ''.
    expect(g.rows.map(r => r.spans.map(s => s.text).join(''))).toEqual(['abcd', '    ']);
    expect(g.text).toBe('abcd\n');
  });

  it('non-cursor rows pass through raw (identity) with matching texts', () => {
    const g = build(['hello world', '中文内容测试', ''], 2, 0, 6);
    const stack = g.rows.map(r => r.spans.map(s => s.text).join(''));
    const overlay = g.text.split('\n');
    expect(overlay).toHaveLength(stack.length);
    // every NON-padded overlay row equals its stack row's text
    overlay.forEach((t, i) => {
      if (t !== '') expect(t).toBe(stack[i]);
    });
  });

  it('a cursor INSIDE the line changes no row boundaries', () => {
    const g = build(['abcdefgh', 'tail'], 0, 2, 4);
    expect(g.text.split('\n')).toEqual(['abcd', 'efgh', 'tail']);
    expect(g.rows).toHaveLength(3);
  });

  it('CJK wrap + trailing blank line keep counts aligned', () => {
    const g = build(['一二三四五', '', 'abc', ''], 3, 10, 4);
    expect(g.text.split('\n')).toHaveLength(g.rows.length);
  });

  it('uses cachedRows when provided (identity path)', () => {
    const lines: AnsiLine[] = [[S('abcdefgh')], []];
    const cached = lines.map(l => wrapLine(l, 4));
    const g = flattenGrid(lines, lines, cached, 4);
    expect(g.rows.map(r => r.spans)).toEqual([...cached[0], ...cached[1]]);
    expect(g.text).toBe('abcd\nefgh\n');
  });
});

describe('annotateUrls (a wrapped or recoloured URL still opens whole)', () => {
  const plain = (text: string): AnsiLine => [{text, color: '#fff'}];

  it('leaves a line without a URL identical (memo identity must survive)', () => {
    const spans = plain('no links here, just prose');
    expect(annotateUrls(spans)).toBe(spans);
  });

  it('tags a bare URL and keeps the surrounding text intact', () => {
    const out = annotateUrls(plain('see https://ccy.pub/x now'));
    expect(out.map(s => s.text).join('')).toBe('see https://ccy.pub/x now');
    expect(out.filter(s => s.url).map(s => s.text)).toEqual(['https://ccy.pub/x']);
    expect(out.find(s => s.url)!.url).toBe('https://ccy.pub/x');
  });

  // THE REPORTED BUG (real phone, 2026-08-10). The grid hard-wraps a long line into
  // visual rows BEFORE anything renders, so a per-row scan saw only `https://ccy` and
  // linked it to a host that does not exist. Both halves must now open the WHOLE URL.
  it('survives the grid wrap — every visual row of a split URL opens the full URL', () => {
    const url = 'https://ccy.pub/projects/pica/rw?u=code&view=profile&lang=zh';
    const rows = wrapLine(annotateUrls(plain(`用 ${url} 远程看报告`)), 24);
    expect(rows.length).toBeGreaterThan(1); // the wrap really did cut it

    const linked = rows.map(r => r.filter(s => s.url));
    expect(linked.filter(r => r.length > 0).length).toBeGreaterThan(1); // on ≥2 rows
    for (const row of linked) {
      for (const s of row) expect(s.url).toBe(url); // never a truncated target
    }
    // The pieces reassemble into exactly the URL — no character gained or lost.
    expect(linked.flat().map(s => s.text).join('')).toBe(url);
    // And the wrap still reproduces the line verbatim.
    expect(rows.map(r => r.map(s => s.text).join('')).join('')).toBe(`用 ${url} 远程看报告`);
  });

  it('survives an SGR change mid-URL (a per-span scan could not see this either)', () => {
    const out = annotateUrls([
      {text: 'https://ccy', color: '#fff'},
      {text: '.pub/x', color: '#0f0'},
    ]);
    expect(out.filter(s => s.url).map(s => s.url)).toEqual(['https://ccy.pub/x', 'https://ccy.pub/x']);
    expect(out.map(s => s.text).join('')).toBe('https://ccy.pub/x');
  });

  it("does not swallow a sentence's trailing punctuation", () => {
    const out = annotateUrls(plain('go to https://ccy.pub/x.'));
    expect(out.find(s => s.url)!.url).toBe('https://ccy.pub/x');
    expect(out.map(s => s.text).join('')).toBe('go to https://ccy.pub/x.');
  });

  it("leaves an agent's own OSC 8 hyperlink alone", () => {
    const spans: AnsiLine = [{text: 'the docs', color: '#fff', href: 'https://example.com/a'}];
    const out = annotateUrls(spans);
    expect(out).toEqual(spans);
  });

  it('routes both link sources through tapTarget, and only web ones', () => {
    expect(tapTarget({href: 'https://a/'})).toBe('https://a/');
    expect(tapTarget({url: 'https://b/'})).toBe('https://b/');
    expect(tapTarget({href: 'file:///tmp/x.png'})).toBeUndefined();
    expect(tapTarget({})).toBeUndefined();
  });

  it('reaches the Android overlay too (linkSegsForLines carries the annotation)', () => {
    const url = 'https://ccy.pub/projects/pica/rw?u=code';
    const segs = linkSegsForLines([annotateUrls(plain(`用 ${url} 看`))]);
    expect(segs.filter(s => s.url).map(s => s.url)).toEqual([url]);
    expect(segs.map(s => s.text).join('')).toBe(`用 ${url} 看`);
  });
});

// Scroll up in a Codex pane, tap the jump-to-bottom arrow, and it stops a little short of
// the tail. Claude panes are fine. The difference is that a Codex pane is almost always
// producing something (its footer timer ticks), so a frozen frame is waiting when you tap:
// flushing it grows the content, while the animated scroll already issued is animating
// toward the height it is replacing — and it overrides the post-layout pin.
describe('jumpMayAnimate', () => {
  test('a waiting frame means no animation: it would target the old height', () => {
    expect(jumpMayAnimate(true)).toBe(false);
  });

  test('with nothing waiting the jump keeps its animation', () => {
    expect(jumpMayAnimate(false)).toBe(true);
  });
});
