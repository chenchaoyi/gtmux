// Pure span/glyph helpers for the native terminal renderer (NativeTerm),
// extracted so the cursor-cell rewriting and glyph normalization are
// unit-testable without rendering a component.

import {AnsiLine, Span} from './ansi';

// nativeFontFamily resolves the font-pref config to an actual iOS font family,
// the SINGLE source of truth shared by the terminal renderer AND the chat view so
// they always match. Native only has system fonts linked (the bundled picker
// woff2 are webview-only), so every pref currently resolves to the system
// monospace; centralized here so when native font-linking lands, both surfaces
// follow the same config automatically.
export function nativeFontFamily(_fontPref?: string): string {
  return 'Menlo';
}

// GOAL: the terminal view in the app must look exactly like the real terminal.
// iOS Core Text draws some symbols as a COLOR emoji where a terminal renders a plain
// monospace TEXT glyph. Unicode's rule (which terminals follow): a "text-default"
// symbol (Emoji_Presentation=No, e.g. ⏸ ⏹ ⚠) is TEXT when bare and only becomes an
// emoji with a trailing U+FE0F; a "default emoji" (✅) stays color. Core Text ignores
// that for BARE text-default symbols and emojifies them. So normalizeGlyphs:
//   • bare text-default symbol → append U+FE0E (force text) to match the terminal;
//   • a symbol the AGENT explicitly made an emoji (…U+FE0F) → leave it (stays color);
//   • default-presentation emoji (✅, no selector) → leave it (stays color).
// U+23FA (record dot ⏺) has no reliable monospace text glyph, so it's SWAPPED to the
// always-text U+25CF ● instead of relying on U+FE0E.
export const DOT_REC = '⏺';
export const DOT_CIRCLE = '●';

const VS15 = '\uFE0E'; // text-presentation variation selector
const VS16 = '\uFE0F'; // emoji-presentation variation selector

// Common Emoji_Presentation=No symbols coding agents emit as text (⏸ ⏹, media
// skip/step ⏭ ⏮ ⏯, timers ⏱ ⏲, ⚠ warning, ℹ info, ▶ ◀ play, ✔ ✖ check/cross,
// ❤). Extend as more surface. (U+23FA record dot is handled by the swap above.)
const TEXT_DEFAULT = new Set(
  [0x23cf, 0x23ed, 0x23ee, 0x23ef, 0x23f1, 0x23f2, 0x23f8, 0x23f9, 0x2139, 0x25b6, 0x25c0, 0x26a0, 0x2714, 0x2716, 0x2764].map(
    c => String.fromCodePoint(c),
  ),
);

export function normalizeGlyphs(t: string): string {
  const s = t.indexOf(DOT_REC) === -1 ? t : t.split(DOT_REC).join(DOT_CIRCLE);
  let out = '';
  let changed = false;
  for (let i = 0; i < s.length; i++) {
    const ch = s[i];
    out += ch;
    if (TEXT_DEFAULT.has(ch)) {
      const nx = s[i + 1];
      if (nx !== VS16 && nx !== VS15) {
        out += VS15; // bare → force text presentation
        changed = true;
      }
    }
  }
  return changed ? out : s;
}

// cursorSpans rewrites one line's spans to paint a reverse-video block at column x
// (the pane's text cursor). Approximated on CHAR offset (the cursor is near the
// input line, ~ASCII, so char≈cell); pads with spaces when x is past the content.
export function cursorSpans(spans: AnsiLine, x: number, curColor: string, bg: string): AnsiLine {
  const lineLen = spans.reduce((n, s) => n + s.text.length, 0);
  const cell = (ch: string): Span => ({text: ch || ' ', color: bg, bg: curColor});
  if (x >= lineLen) {
    const out = [...spans];
    if (x > lineLen) out.push({text: ' '.repeat(x - lineLen), color: bg});
    out.push(cell(' '));
    return out;
  }
  const out: AnsiLine = [];
  let col = 0;
  for (const s of spans) {
    const end = col + s.text.length;
    if (x < col || x >= end) {
      out.push(s);
      col = end;
      continue;
    }
    const i = x - col;
    if (i > 0) out.push({...s, text: s.text.slice(0, i)});
    out.push({...cell(s.text[i]), bold: s.bold});
    if (i + 1 < s.text.length) out.push({...s, text: s.text.slice(i + 1)});
    col = end;
  }
  return out;
}

// ————— Uniform grid (mobile-native-term-selection Stage 1, iOS) —————
//
// The iOS terminal renders a TRUE uniform grid: every visual row has the same
// explicit height, and logical lines are pre-wrapped into visual rows by CELL
// ARITHMETIC in JS (never by RN Text's own layout). Stage 2's native selection
// layer (UITextInput) then computes row geometry as pure arithmetic:
// row = floor(y / rowHeightFor(fontSize)). These helpers are the SINGLE source
// of those numbers — Stage 2 must consume the same values, never re-derive.

// rowHeightFor is the explicit per-row lineHeight for the terminal grid.
// 1.6× comfortably contains both Menlo (Latin) and the PingFang CJK fallback at
// the same size — no glyph clipping — and makes a CJK row and a Latin row the
// SAME height (Core Text's mixed-font line-height variance was one poison of
// every selection-alignment bug; an explicit lineHeight kills it).
export function rowHeightFor(fontSize: number): number {
  return Math.round(fontSize * 1.6);
}

// cellWidthFor is Menlo's monospace advance at `fontSize` (measured ratio
// ≈ 0.602×; rounded to 2 decimals so the number is stable across JS engines).
export function cellWidthFor(fontSize: number): number {
  return Math.round(fontSize * 0.602 * 100) / 100;
}

// colsFor: how many CELLS fit a terminal row at this view width. −12 is the
// terminal's horizontal content padding (styles.pad = 6 each side in
// NativeTerm); the extra −1 is the CONSERVATIVE margin the proposal requires —
// a row must NEVER be wide enough for RN Text to native-wrap it (a CJK glyph's
// true advance can slightly exceed 2× the Menlo cell), because a native wrap
// would break the row = grid-row invariant Stage 2's geometry depends on.
export function colsFor(viewWidth: number, fontSize: number): number {
  return Math.max(4, Math.floor((viewWidth - 12) / cellWidthFor(fontSize)) - 1);
}

// charCells is the terminal cell cost of one code point (the wcwidth the grid
// arithmetic uses): 2 for East-Asian wide/fullwidth (CJK unified + extensions,
// Hangul, Kana, fullwidth forms, CJK punctuation, emoji — the common ranges),
// 0 for the variation selectors (U+FE0E/U+FE0F) and ZWJ that modify a base
// glyph without occupying a cell, else 1.
export function charCells(ch: string): number {
  const cp = ch.codePointAt(0) ?? 0;
  if (cp === 0xfe0e || cp === 0xfe0f || cp === 0x200d) return 0;
  if (
    (cp >= 0x1100 && cp <= 0x115f) || // Hangul Jamo
    (cp >= 0x2e80 && cp <= 0x303e) || // CJK radicals, Kangxi, CJK symbols/punctuation (、。「」…)
    (cp >= 0x3041 && cp <= 0x33ff) || // Hiragana, Katakana, CJK compatibility
    (cp >= 0x3400 && cp <= 0x4dbf) || // CJK ext A
    (cp >= 0x4e00 && cp <= 0x9fff) || // CJK unified
    (cp >= 0xa000 && cp <= 0xa4cf) || // Yi
    (cp >= 0xac00 && cp <= 0xd7a3) || // Hangul syllables
    (cp >= 0xf900 && cp <= 0xfaff) || // CJK compatibility ideographs
    (cp >= 0xfe30 && cp <= 0xfe4f) || // CJK compatibility forms
    (cp >= 0xff00 && cp <= 0xff60) || // fullwidth forms
    (cp >= 0xffe0 && cp <= 0xffe6) || // fullwidth signs
    (cp >= 0x1f300 && cp <= 0x1faff) || // emoji blocks (2 cells in terminals)
    (cp >= 0x20000 && cp <= 0x3fffd) // CJK ext B+
  ) {
    return 2;
  }
  return 1;
}

// wrapLine hard-wraps ONE parsed logical line into visual rows at cell
// boundaries — exactly how a terminal char-wraps a pane (tmux does the same on
// the Mac; word-wrap would drift from it). Splits MID-SPAN when the boundary
// falls inside one, preserving the span's color/bold/bg/href on both halves.
// Zero-width chars (selectors/ZWJ) ride with their base glyph; a wide char that
// doesn't fit the remaining cells moves whole to the next row. An empty line
// yields ONE empty row (the grid still owns that row's height). Iterates by
// code point, so surrogate pairs never split.
export function wrapLine(spans: AnsiLine, cols: number): AnsiLine[] {
  const rows: AnsiLine[] = [];
  let row: AnsiLine = [];
  let used = 0;
  for (const s of spans) {
    let buf = '';
    for (const ch of s.text) {
      const w = charCells(ch);
      if (used > 0 && used + w > cols) {
        if (buf) {
          row.push({...s, text: buf});
          buf = '';
        }
        rows.push(row);
        row = [];
        used = 0;
      }
      buf += ch;
      used += w;
    }
    if (buf) row.push({...s, text: buf});
  }
  rows.push(row);
  return rows;
}

// A bare http(s) URL in terminal output. Stops at whitespace and the bracket/quote
// characters that normally delimit a URL. Hyphens/dots/slashes/query chars stay IN.
const URL_RE = /https?:\/\/[^\s<>"'`|\\^{}]+/gi;

// linkify splits a string into text segments, tagging bare http(s) URLs with a `url`
// so the terminal renderer can make them tappable — the same open-in-browser behavior
// an OSC 8 hyperlink already gets, but for URLs an agent merely PRINTED as plain text.
// Trailing sentence punctuation / a wrapping close-bracket is kept OUT of the link (a
// URL ending a sentence shouldn't swallow the period). Returns one plain segment when
// there's no URL, so callers can fast-path the common line.
export function linkify(text: string): Array<{text: string; url?: string}> {
  const out: Array<{text: string; url?: string}> = [];
  let last = 0;
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index ?? 0;
    let url = m[0];
    const trail = url.match(/[.,;:!?)\]}>"'»]+$/)?.[0] ?? '';
    if (trail) url = url.slice(0, url.length - trail.length);
    if (start > last) out.push({text: text.slice(last, start)});
    out.push({text: url, url});
    if (trail) out.push({text: trail});
    last = start + m[0].length;
  }
  if (out.length === 0) return [{text}];
  if (last < text.length) out.push({text: text.slice(last)});
  return out;
}

// linkSegsForLines flattens rendered lines into the tap-segments the TRANSPARENT
// selection overlay needs. That overlay sits ON TOP and is the ONLY layer that receives
// touches, so it must carry EVERY link the color layer draws: an OSC 8 hyperlink
// (span.href — e.g. an anchor-text link like a Wikipedia title; tmux 3.7b's
// `capture-pane -e` preserves OSC 8) AND a bare URL the agent printed as text. The
// earlier overlay only linkified bare URLs, so anchor-text OSC 8 links were swallowed
// (bare URLs tapped through, hyperlinks didn't). Non-href runs are linkified per LINE
// (joined) so a bare URL split across SGR spans is still caught. The concatenated
// segment text equals the joined plain text (linkify drops nothing, lines join on '\n'),
// so the overlay stays aligned with the color layer; kept flat so the iOS selection
// highlight still paints.
export function linkSegsForLines(lines: AnsiLine[]): Array<{text: string; url?: string}> {
  const out: Array<{text: string; url?: string}> = [];
  lines.forEach((spans, i) => {
    let run = '';
    const flush = () => {
      if (run) {
        for (const seg of linkify(run)) out.push(seg);
        run = '';
      }
    };
    for (const s of spans) {
      if (s.href && /^https?:\/\//i.test(s.href)) {
        flush();
        out.push({text: s.text, url: s.href});
      } else {
        run += s.text;
      }
    }
    flush();
    if (i < lines.length - 1) out.push({text: '\n'});
  });
  return out;
}
