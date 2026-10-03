// Codex pins the prompt of the turn it is working on to the top of its screen, cut to the
// pane's width and ended with "…". Codex runs in the alternate screen, so the rest of that
// prompt is nowhere in the capture: no wrap mode or wider canvas can bring it back (the
// "Original width" toggle tried, 2026-09-28, and could not). The conversation log has the
// full text, so the Detail terminal shows it in a bar of its own instead.
//
// The match is deliberately narrow. Every user message in Codex's history also starts
// with "› ", and so does its composer; only the pinned row is CUT with "…". Anything that
// is not unmistakably a truncated copy of a recent prompt leaves the capture untouched,
// so another agent's screen, or a Codex screen this does not understand, renders exactly
// as before.

import {paneLines} from './term';

const MARK = '› '; // Codex's user-message prefix (U+203A + space)
const CUT = '…';
const MAX_ROWS = 3; // a pinned prompt longer than this is not the shape we know
const MIN_HEAD = 6; // fewer visible characters is too little to identify a prompt by
const RECENT = 10; // prompts to compare against, newest first

// Through paneLines like every other screen read (ui/paneLines.test.ts).
const plain = (line: string): string => (paneLines(line)[0] ?? []).map(s => s.text).join('');
// Whitespace-free, so a prompt matches however Codex wrapped or joined its lines; and
// without variation selectors, which paneLines adds after symbols like ⚠ and the
// conversation log does not have.
const skeleton = (s: string): string => s.replace(/[\s\uFE0E\uFE0F]+/g, '');

export interface CodexPinned {
  /** The capture without the pinned rows. */
  text: string;
  /** The full prompt those rows were cut from. */
  prompt: string;
}

/**
 * Recognises Codex's pinned, truncated prompt at the top of a captured screen.
 * `prompts` are the conversation's prompts, oldest first. Returns null — and the caller
 * keeps the capture as it is — unless the pane is Codex, row 0 is a "› " row ending (by
 * its third row at most) in "…", what precedes the "…" begins one of the recent prompts,
 * and a later row starts with "› " (the composer), so the composer itself is never taken.
 */
export function splitCodexPinned(text: string, agent: string | undefined, prompts: string[]): CodexPinned | null {
  if ((agent ?? '').trim().toLowerCase() !== 'codex' || !text) return null;
  const recent = prompts.filter(p => p && p.trim()).slice(-RECENT);
  if (recent.length === 0) return null;
  const lines = text.split('\n');
  const first = plain(lines[0] ?? '');
  if (!first.startsWith(MARK)) return null;
  let body = '';
  for (let k = 0; k < Math.min(MAX_ROWS, lines.length); k++) {
    const row = k === 0 ? first.slice(MARK.length) : plain(lines[k]);
    // A continuation row is indented under the mark; anything else ends the prompt.
    if (k > 0 && !/^ {2}\S/.test(row)) return null;
    body += row.trim();
    if (!body.endsWith(CUT)) continue;
    const head = skeleton(body.slice(0, -CUT.length));
    if (head.length < MIN_HEAD) return null;
    const rest = lines.slice(k + 1);
    if (!rest.some(l => plain(l).startsWith(MARK))) return null;
    for (let i = recent.length - 1; i >= 0; i--) {
      if (skeleton(recent[i]).startsWith(head)) return {text: rest.join('\n'), prompt: recent[i].trim()};
    }
    return null;
  }
  return null;
}
