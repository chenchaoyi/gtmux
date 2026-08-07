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
// Two-generation eviction: each call keeps only the lines present in THIS frame, so
// the map is bounded by the buffer size and scrolled-away lines don't leak.
import {AnsiLine, AnsiOpts, parseAnsi} from './ansi';

export interface LineCache {
  sig: string; // options signature (palette/base) — a theme change invalidates everything
  map: Map<string, AnsiLine>;
}

export function makeLineCache(): LineCache {
  return {sig: '', map: new Map()};
}

// The signature of everything besides the raw line that shapes parse output.
export function cacheSig(opts: AnsiOpts): string {
  return `${opts.palette?.join(',') ?? ''}|${opts.base ?? ''}|${opts.bg ? 1 : 0}`;
}

// Parse raw lines through the cache, reusing span-array identities for lines whose
// raw text is unchanged (duplicate lines — e.g. blanks — share one identity; safe
// because consumers never mutate spans: cursorSpans builds a fresh array).
export function parseLinesCached(raws: string[], opts: AnsiOpts, cache: LineCache): AnsiLine[] {
  const sig = cacheSig(opts);
  if (cache.sig !== sig) {
    cache.map.clear();
    cache.sig = sig;
  }
  const next = new Map<string, AnsiLine>();
  const out = raws.map(raw => {
    let spans = next.get(raw) ?? cache.map.get(raw);
    if (!spans) spans = parseAnsi(raw, opts)[0] ?? [];
    next.set(raw, spans);
    return spans;
  });
  cache.map = next;
  return out;
}
