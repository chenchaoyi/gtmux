import {cursorSpans, normalizeGlyphs, DOT_REC, DOT_CIRCLE} from './term';
import {AnsiLine} from './ansi';

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
