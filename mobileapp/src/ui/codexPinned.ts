// Codex pins the prompt of the turn on screen to the top of its screen, cut to the pane's
// width and ended with "…". Codex runs in the alternate screen, so the rest of that
// prompt is nowhere in the capture: no wrap mode or wider canvas can bring it back.
// Original width preserves captured history layout; it cannot recover missing bytes.
// The conversation log has the full text, so Detail provides a separate full-prompt reader.
//
// What the row looks like, read off a real Codex (v0.160.0, 2026-10-04) in 60- and
// 80-column panes: ONE row, "› " then the prompt with its newlines joined, cut to the
// pane's width minus one cell (a wide character that does not fit leaves one more), then
// "…". It stays while Codex is idle, until that turn's prompt scrolls back into view. A
// user message in Codex's history starts with the same "› " in the same style, but wraps
// onto indented rows instead of being cut; so does the composer, below.
//
// The match is deliberately narrow. Anything that is not unmistakably a cut copy of one
// recent prompt leaves the capture untouched, so another agent's screen, or a Codex
// screen this does not understand, renders exactly as before.

import {charCells, paneLines} from './term';

const MARK = '› '; // Codex's user-message prefix (U+203A + space)
const CUT = '…';
const MIN_HEAD = 6; // fewer visible characters is too little to identify a prompt by
const RECENT = 10; // prompts to compare against, newest first
const SLACK = 2; // cells short of the pane's width a cut row may end (Codex leaves one)

// Through paneLines like every other screen read (ui/paneLines.test.ts).
const plain = (line: string): string => (paneLines(line)[0] ?? []).map(s => s.text).join('');
// Whitespace-free, so a prompt matches however Codex joined its lines; and without
// variation selectors, which paneLines adds after symbols like ⚠ and the conversation
// log does not have.
const skeleton = (s: string): string => s.replace(/[\s︎️]+/g, '');
const cells = (s: string): number => {
  let n = 0;
  for (const ch of s) n += charCells(ch);
  return n;
};

export interface CodexPinned {
  /** The capture without the pinned row. */
  text: string;
  /** The full prompt that row was cut from. */
  prompt: string;
}

interface CutRow {
  head: string; // the visible prompt text before the "…", as a skeleton
  next: string; // the row under it, as a skeleton
  rest: string[]; // the capture without the row
}

// The pinned row's shape, before any prompt is consulted: Codex's pane, row 0 a "› " row
// that ends in "…" at the pane's right edge, and the composer's "› " further down.
function cutRow(text: string, agent: string | undefined, cols: number | undefined): CutRow | null {
  if ((agent ?? '').trim().toLowerCase() !== 'codex' || !text || !cols) return null;
  const lines = text.split('\n');
  const first = plain(lines[0] ?? '').trimEnd();
  if (!first.startsWith(MARK) || !first.endsWith(CUT)) return null;
  // Cut to the width: a row that ends short of the edge ends with the user's own "…".
  const w = cells(first);
  if (w > cols || w < cols - SLACK) return null;
  const head = skeleton(first.slice(MARK.length, -CUT.length));
  if (head.length < MIN_HEAD) return null;
  const rest = lines.slice(1);
  if (!rest.some(l => plain(l).startsWith(MARK))) return null;
  return {head, next: skeleton(plain(lines[1] ?? '')), rest};
}

/**
 * True when the capture shows Codex's pinned, cut prompt row, whether or not a known
 * prompt explains it yet. The Detail screen uses it to fetch the conversation log when
 * the row is there but unmatched: the prompt may simply not be in the log it has.
 */
export function hasCodexCutRow(text: string, agent: string | undefined, cols: number | undefined): boolean {
  return cutRow(text, agent, cols) !== null;
}

/**
 * Recognises Codex's pinned, cut prompt at the top of a captured screen. `prompts` are
 * the conversation's prompts, oldest first: only ones Codex has actually received, never
 * a send still on its way. `cols` is the pane's width. Returns null — and the caller
 * keeps the capture as it is — unless the row has the shape above and exactly ONE recent
 * prompt explains it: it begins with the visible text, it is longer than that text (so
 * something was cut), and the row under it does not carry on with the same prompt (that
 * is the prompt wrapped in history, not cut). Two different prompts with the same opening
 * are ambiguous, and an ambiguous row stays on screen.
 */
export function splitCodexPinned(
  text: string,
  agent: string | undefined,
  prompts: string[],
  cols: number | undefined,
): CodexPinned | null {
  const row = cutRow(text, agent, cols);
  if (!row) return null;
  const {head, next, rest} = row;
  let found: string | undefined;
  const seen = new Set<string>();
  for (const p of prompts.filter(q => q && q.trim()).slice(-RECENT).reverse()) {
    const full = skeleton(p);
    if (!full.startsWith(head) || full === head + CUT || full.length <= head.length) continue;
    if (next && full.startsWith(head + CUT + next)) return null;
    seen.add(full);
    if (seen.size > 1) return null;
    found = found ?? p.trim();
  }
  return found ? {text: rest.join('\n'), prompt: found} : null;
}
