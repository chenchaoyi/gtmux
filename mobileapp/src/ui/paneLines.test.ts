import {paneLines, normalizeGlyphs, DOT_REC, DOT_CIRCLE} from './term';

/* eslint-disable @typescript-eslint/no-var-requires */
// Required rather than imported: the app's tsconfig carries no node types, and this is
// the one test that reads the source tree rather than running it.
const {readFileSync, readdirSync} = require('fs');
const {join} = require('path');

// iOS draws U+23FA as a colour emoji where a terminal draws a plain monospace glyph, so
// the renderer swaps it. That fix was written once and then reached from ONE of the three
// places that render a pane: the terminal view had it, the chat's Live card and the HQ
// screen did not, and both showed a colour dot beside monospace text
// 「live的窗口里emoji渲染看着又有些问题，之前在terminal里应该修过这类问题」 (2026-09-21).
//
// A rule three call sites each have to remember is a rule that drifts. So there is one
// function, and the second test here is what makes forgetting it RED rather than a bug
// someone screenshots months later.

describe('paneLines', () => {
  test('normalizes the glyph on its way through the parser', () => {
    const [line] = paneLines(`${DOT_REC} Bash(git fetch)`);
    const text = line.map(s => s.text).join('');
    expect(text).toContain(DOT_CIRCLE);
    expect(text).not.toContain(DOT_REC);
  });

  test('is exactly normalize-then-parse, so the two cannot drift apart', () => {
    const raw = `${DOT_REC} one\n\u001b[31mtwo\u001b[0m ⏸ three`;
    const plain = (ls: {text: string}[][]) => ls.map(l => l.map(s => s.text).join('')).join('\n');
    const {parseAnsi} = require('./ansi');
    expect(plain(paneLines(raw))).toBe(plain(parseAnsi(normalizeGlyphs(raw))));
  });

  test('keeps the ANSI it was given', () => {
    const [line] = paneLines(`${DOT_REC} \u001b[31mred\u001b[0m`);
    expect(line.some(s => s.color)).toBe(true);
  });
});

// A screen that parses a pane itself skips the normalization, which is the defect above.
// The parser is for the renderer's own plumbing; a screen calls paneLines.
describe('no screen parses a pane on its own', () => {
  type Dirent = {name: string; isDirectory: () => boolean};
  const walk = (dir: string): string[] =>
    (readdirSync(dir, {withFileTypes: true}) as Dirent[]).flatMap((e: Dirent) => {
      const p: string = join(dir, e.name);
      if (e.isDirectory()) return walk(p);
      return /\.tsx?$/.test(e.name) && !/\.test\.tsx?$/.test(e.name) ? [p] : [];
    });

  // termLineCache is the renderer's own per-line plumbing: NativeTerm normalizes the whole
  // screen before handing it lines, so the cache parses already-normalized text.
  const ALLOWED = ['src/ui/ansi.ts', 'src/ui/term.ts', 'src/ui/termLineCache.ts'];

  test('parseAnsi is called only where the renderer owns the normalization', () => {
    const offenders = walk('src')
      .filter((p: string) => !ALLOWED.some(a => p.endsWith(a)))
      .filter((p: string) => /\bparseAnsi\s*\(/.test(readFileSync(p, 'utf8') as string));
    expect(offenders).toEqual([]);
  });
});
