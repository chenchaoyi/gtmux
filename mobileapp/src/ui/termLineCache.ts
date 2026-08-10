// Per-line parse cache for the native terminal (perf: per-line memo rendering).
//
// Why this is CORRECT: parseAnsi is per-line stateless — it resets SGR style/href at
// the start of every line (tmux `capture-pane -e` re-emits the SGR codes a line needs
// at its start), so a line's parsed spans depend ONLY on its raw text + the parse
// options. That makes a content-addressed cache exact: same raw line → same spans.
// termLineCache.test.ts pins this parity against the whole-buffer parse; if parseAnsi
// ever grows cross-line state, that test screams.
//
// Why it exists: the terminal re-parses and re-renders the WHOLE capped buffer (up to
// MAX_LINES) on every poll of a busy pane, even when the pane repainted in place and
// only a spinner/footer line actually changed. Reusing the EXACT span-array identity
// for unchanged lines lets the per-line React.memo rows bail, so an in-place repaint
// re-parses and re-renders only the lines that differ. A true scroll changes every
// line and degrades to today's full re-render — no regression.
//
// Stage 1 of mobile-native-term-selection extends each entry with the line's WRAPPED
// VISUAL ROWS for the current grid layout (cols × fontSize, part of the cache
// signature — a rotation/font change invalidates everything, which is fine). Wrapping
// is as cache-exact as parsing: rows depend only on the spans + cols. An unchanged
// line keeps BOTH identities (spans AND rows array) stable across polls, so the
// per-row React.memo blocks still bail on an in-place repaint.
//
// Two-generation eviction: each call keeps only the lines present in THIS frame, so
// the map is bounded by the buffer size and scrolled-away lines don't leak.
import {AnsiLine, AnsiOpts, parseAnsi} from './ansi';
import {annotateUrls, wrapLine} from './term';

export interface LineEntry {
  spans: AnsiLine; // parsed spans (identity-stable while the raw line is unchanged)
  rows?: AnsiLine[]; // wrapped visual rows for the layout in the cache signature (iOS grid)
}

export interface LineCache {
  sig: string; // options+layout signature — a theme/cols/fontSize change invalidates everything
  map: Map<string, LineEntry>;
}

// The grid layout the wrapped rows were computed for. fontSize rides along even
// though cols already derives from it — over-invalidating on a font change is
// harmless and keeps the signature honest about everything that shaped the rows.
export interface GridLayout {
  cols: number;
  fontSize: number;
}

export function makeLineCache(): LineCache {
  return {sig: '', map: new Map()};
}

// The signature of everything besides the raw line that shapes the cached output.
export function cacheSig(opts: AnsiOpts, layout?: GridLayout): string {
  const grid = layout ? `|${layout.cols}c${layout.fontSize}` : '';
  return `${opts.palette?.join(',') ?? ''}|${opts.base ?? ''}|${opts.bg ? 1 : 0}${grid}`;
}

function linesThrough(
  raws: string[],
  opts: AnsiOpts,
  layout: GridLayout | undefined,
  cache: LineCache,
): {lines: AnsiLine[]; rows: AnsiLine[][]} {
  const sig = cacheSig(opts, layout);
  if (cache.sig !== sig) {
    cache.map.clear();
    cache.sig = sig;
  }
  const next = new Map<string, LineEntry>();
  const lines: AnsiLine[] = [];
  const rows: AnsiLine[][] = [];
  for (const raw of raws) {
    let e = next.get(raw) ?? cache.map.get(raw);
    // annotateUrls runs on the LOGICAL line and BEFORE wrapLine, which is the whole
    // point: the wrap cuts a long URL across visual rows, and anything downstream of it
    // can only ever see half of one. Placed here so it is paid once per UNIQUE raw line
    // (cached like the parse) and so both platforms get it — Android reads the same
    // spans through parseLinesCached. A line with no URL comes back identical, so the
    // per-row memo still bails on an in-place repaint.
    if (!e) e = {spans: annotateUrls(parseAnsi(raw, opts)[0] ?? [])};
    if (layout && !e.rows) e.rows = wrapLine(e.spans, layout.cols);
    next.set(raw, e);
    lines.push(e.spans);
    if (layout) rows.push(e.rows!);
  }
  cache.map = next;
  return {lines, rows};
}

// Parse raw lines through the cache, reusing span-array identities for lines whose
// raw text is unchanged (duplicate lines — e.g. blanks — share one identity; safe
// because consumers never mutate spans: cursorSpans builds a fresh array). The
// layout-less path (Android's paragraph renderer + the chat view's tests).
export function parseLinesCached(raws: string[], opts: AnsiOpts, cache: LineCache): AnsiLine[] {
  return linesThrough(raws, opts, undefined, cache).lines;
}

// wrapLinesCached: parse AND hard-wrap raw lines through the cache (the iOS grid
// path). Returns the logical lines (for cursor splicing, which stays on logical
// lines) plus each line's wrapped visual rows; BOTH keep identity for unchanged
// raw lines so per-row memo rendering bails on an in-place repaint.
export function wrapLinesCached(
  raws: string[],
  opts: AnsiOpts,
  layout: GridLayout,
  cache: LineCache,
): {lines: AnsiLine[]; rows: AnsiLine[][]} {
  return linesThrough(raws, opts, layout, cache);
}
