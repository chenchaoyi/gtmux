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

// iOS Core Text renders U+23FA "⏺ BLACK CIRCLE FOR RECORD" (Claude Code's tool-call
// marker) as the glossy RED record-button COLOR EMOJI, ignoring the ANSI color.
// Swap it for U+25CF "● BLACK CIRCLE" (no emoji presentation) so it renders as a
// clean text glyph tinted by the surrounding SGR color — same fix as the xterm
// path (gen-xterm-asset.mjs normalizeGlyphs).
export const DOT_REC = '⏺';
export const DOT_CIRCLE = '●';

export function normalizeGlyphs(t: string): string {
  return t.indexOf(DOT_REC) === -1 ? t : t.split(DOT_REC).join(DOT_CIRCLE);
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
