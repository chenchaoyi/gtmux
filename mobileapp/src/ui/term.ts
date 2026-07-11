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
