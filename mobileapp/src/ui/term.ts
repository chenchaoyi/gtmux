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

// PAD is the terminal's content padding (styles.pad in NativeTerm, each side).
// Named here because THREE consumers must agree on it: NativeTerm's padding
// style, colsFor's usable-width math below, and Stage 2's native selection
// overlay geometry (its padTop/padLeft props). Change it in one place only.
export const PAD = 6;

// colsFor: how many CELLS fit a terminal row at this view width. −2×PAD is the
// terminal's horizontal content padding (styles.pad = PAD each side in
// NativeTerm); the extra −1 is the CONSERVATIVE margin the proposal requires —
// a row must NEVER be wide enough for RN Text to native-wrap it (a CJK glyph's
// true advance can slightly exceed 2× the Menlo cell), because a native wrap
// would break the row = grid-row invariant Stage 2's geometry depends on.
export function colsFor(viewWidth: number, fontSize: number): number {
  return Math.max(4, Math.floor((viewWidth - PAD * 2) / cellWidthFor(fontSize)) - 1);
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

// flattenGrid builds BOTH sides of the iOS selection sandwich from the same
// wrap pass: the visual-row stack the color layer renders (from the
// CURSOR-SPLICED lines) and the native selection overlay's plain text (from the
// RAW lines — the reverse-video cursor cell pads its line with spaces, and that
// padding must never leak into a Copy). The one invariant Stage 2's native
// geometry depends on: overlay text rows == rendered stack rows, ALWAYS — the
// native layer maps row = y ÷ rowHeight into the overlay text, so a count drift
// desynchronizes every row below it. The splice only APPENDS to its line
// (cursorSpans keeps existing chars), so raw row boundaries match the displayed
// ones; when the appended cells add a wrap row, empty strings pad the overlay
// text so the count still matches (term.test.ts pins this).
//
// `rendered` = logical lines with the cursor spliced into one of them; `lines` =
// the same array WITHOUT the splice (identity-shared for every other line —
// that's how the spliced line is recognized); `cachedRows` = per-line wrapped
// rows from the line cache (or null to wrap here); `cols` = the grid capacity.
export function flattenGrid(
  rendered: AnsiLine[],
  lines: AnsiLine[],
  cachedRows: AnsiLine[][] | null,
  cols: number,
): {rows: Array<{key: string; spans: AnsiLine}>; text: string} {
  const out: Array<{key: string; spans: AnsiLine}> = [];
  const sel: string[] = [];
  rendered.forEach((spans, i) => {
    const raw = cachedRows ? cachedRows[i] : wrapLine(lines[i] || [], cols);
    const wrapped = spans === lines[i] ? raw : wrapLine(spans, cols);
    for (let j = 0; j < wrapped.length; j++) {
      out.push({key: `${i}-${j}`, spans: wrapped[j]});
      sel.push(j < raw.length ? raw[j].map(s => s.text).join('') : '');
    }
  });
  return {rows: out, text: sel.join('\n')};
}

// ————— Trailing-blank trim (short buffers must not render as a black screen) —————
//
// capture-pane KEEPS the pane grid's trailing blank rows (internal/tmux
// CapturePaneColor preserves them so the bottom-anchored cursor row-count math
// holds). Rendering that blank tail verbatim pinned a mostly-empty tall pane's
// few content lines ABOVE the viewport — the terminal ScrollView follows the
// bottom, so a fresh 200×50 pane with 5 content lines (or any pane right after
// `clear`) showed as an ALL-BLACK screen. The RENDER therefore trims the blank
// tail; the cursor mapping still runs on the UNTRIMMED array (`up` counts from
// the capture's bottom row) and the trim never cuts the row the cursor sits on
// (a cursor can rest BELOW the last content line, or on the top row after
// `clear` when everything under it is blank).

export interface BottomCursor {
  x: number;
  up: number; // rows above the capture's bottom grid row (pane_height−1−cursor_y)
  visible: boolean;
}

// isBlankLine: this parsed line renders nothing visible — only whitespace text
// and no background fill (a bg-painted run of spaces IS visible, e.g. a TUI
// statusline, so it counts as content and is never trimmed).
export function isBlankLine(spans: AnsiLine): boolean {
  return spans.every(s => !s.bg && s.text.trim() === '');
}

// renderView: how many leading logical lines to render (`keep`) and which of
// them carries the cursor (`cursorRow`, −1 when hidden). The cursor mapping is
// the pre-existing bottom-anchored one — skip the capture's trailing-newline
// blank, then count `up` LOGICAL lines up from the bottom grid row — computed
// BEFORE the trim so it can protect the cursor's row. keep ≥ 1: an all-blank
// capture still renders one (empty) row.
export function renderView(lines: AnsiLine[], cursor?: BottomCursor): {keep: number; cursorRow: number} {
  let last = lines.length - 1;
  if (last > 0 && lines[last].length === 0) last--; // the trailing-newline blank
  const hasCursor = !!cursor && cursor.visible !== false;
  const cursorRow = hasCursor ? Math.max(0, Math.min(last, last - (cursor!.up | 0))) : -1;
  let content = last + 1;
  while (content > 1 && isBlankLine(lines[content - 1] || [])) content--;
  return {keep: Math.max(content, cursorRow + 1), cursorRow};
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

// Trailing sentence punctuation that a URL at the end of a sentence must not swallow.
const URL_TRAIL_RE = /[.,;:!?)\]}>"'»]+$/;
const HAS_URL_RE = /https?:\/\//i;

// annotateUrls tags a LOGICAL line's bare http(s) URLs onto the spans themselves,
// splitting a span where a URL starts or ends. It is the same detection `linkify` does,
// moved to the only place that can see a whole URL.
//
// WHY IT CANNOT STAY AT RENDER TIME. The renderer receives VISUAL ROWS, not logical
// lines: `wrapLine` hard-wraps a long line at the grid's column boundary before anything
// draws. So a URL longer than the remaining columns arrives already cut in two, and a
// per-row (or, in the color layer, per-SPAN) linkify sees only `https://ccy` and
// `.pub/projects/…` — the first half becomes a link to a host that does not exist and the
// second half becomes plain text. Reported on a real phone, 2026-08-10. Running the
// detection HERE, before the wrap, means `wrapLine`'s `{...s}` copies the `url` onto both
// halves: every piece of a wrapped URL opens the whole URL. It fixes the SGR-split case
// for free — a URL recoloured halfway through was equally invisible to a per-span scan.
//
// Identity is preserved for any line without a URL (the overwhelming majority): the same
// array comes back, so the per-line parse cache and the per-row `React.memo` still bail.
// An agent-declared OSC 8 `href` always wins — it is what the agent asked for.
export function annotateUrls(spans: AnsiLine): AnsiLine {
  if (spans.length === 0) return spans;
  const text = spans.map(s => s.text).join('');
  if (!HAS_URL_RE.test(text)) return spans;

  const marks: Array<{start: number; end: number; url: string}> = [];
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index ?? 0;
    let url = m[0];
    const trail = url.match(URL_TRAIL_RE)?.[0] ?? '';
    if (trail) url = url.slice(0, url.length - trail.length);
    if (url) marks.push({start, end: start + url.length, url});
  }
  if (marks.length === 0) return spans;

  const out: AnsiLine = [];
  let pos = 0;
  for (const s of spans) {
    const from = pos;
    const to = pos + s.text.length; // UTF-16 units throughout, like String.matchAll
    pos = to;
    if (s.href) {
      out.push(s);
      continue;
    }
    const cuts = new Set<number>([from, to]);
    for (const mk of marks) {
      if (mk.end <= from || mk.start >= to) continue;
      cuts.add(Math.max(mk.start, from));
      cuts.add(Math.min(mk.end, to));
    }
    const bounds = [...cuts].sort((a, b) => a - b);
    if (bounds.length === 2) {
      // No boundary falls INSIDE this span — but it may still lie wholly within a URL
      // (the SGR-recoloured middle of one, or a middle row of a wrapped one). Tag it
      // whole, or hand back the original span so its identity survives.
      const whole = marks.find(mk => mk.start <= from && mk.end >= to);
      out.push(whole ? {...s, url: whole.url} : s);
      continue;
    }
    for (let i = 0; i + 1 < bounds.length; i++) {
      const a = bounds[i];
      const b = bounds[i + 1];
      if (a === b) continue;
      const hit = marks.find(mk => mk.start <= a && mk.end >= b);
      out.push(hit ? {...s, text: s.text.slice(a - from, b - from), url: hit.url} : {...s, text: s.text.slice(a - from, b - from)});
    }
  }
  return out;
}

// tapTarget is the ONE place that decides what a span opens: the agent's own OSC 8
// hyperlink when it declared a web one, else a URL annotateUrls detected in the text.
// Both layers ask this, so they can never disagree about which spans are tappable.
export function tapTarget(s: {href?: string; url?: string}): string | undefined {
  if (s.href && /^https?:\/\//i.test(s.href)) return s.href;
  return s.url;
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
      const href = tapTarget(s);
      if (href) {
        flush();
        out.push({text: s.text, url: href});
      } else {
        run += s.text;
      }
    }
    flush();
    if (i < lines.length - 1) out.push({text: '\n'});
  });
  return out;
}
