// consoleActs — what gtmux recorded, placed in the conversation at the moment it
// happened (hq-work direction A, 2026-09-15: the "HQ's work" tab said in a fourth place
// what HQ's own words, the header's "HQ did" row and the Mac card already said, with
// nothing to act on; an audit is read beside the claim it checks, so the acts now sit as
// small rows between HQ's bubbles). Pure, so the placement and the folding are testable.
import {Act} from '../screens/hqActsModel';
import {TranscriptTurn} from '../api/client';

export type ConsoleRow = {kind: 'act'; act: Act} | {kind: 'fold'; verb: string; n: number; ts: number; acts: Act[]};

/** The unix second a turn began, from its RFC3339 prompt time; 0 when it carried none. */
export function turnSecs(t: TranscriptTurn): number {
  if (!t.time) return 0;
  const ms = Date.parse(t.time);
  return Number.isFinite(ms) ? Math.floor(ms / 1000) : 0;
}

/**
 * foldRuns keeps the rows readable when HQ books many lessons in a row: three or more
 * consecutive acts with the same verb inside one hour fold to a single row ("记账 ×6")
 * that opens on a tap; anything less stays as it is. Newest last, like the turns.
 */
export function foldRuns(acts: Act[]): ConsoleRow[] {
  const out: ConsoleRow[] = [];
  let i = 0;
  while (i < acts.length) {
    let j = i + 1;
    while (j < acts.length && acts[j].verb === acts[i].verb && acts[j].ts - acts[i].ts < 3600 && !acts[j].alarm) j++;
    if (j - i >= 3) out.push({kind: 'fold', verb: acts[i].verb, n: j - i, ts: acts[j - 1].ts, acts: acts.slice(i, j)});
    else for (let k = i; k < j; k++) out.push({kind: 'act', act: acts[k]});
    i = j;
  }
  return out;
}

/**
 * placeActs assigns each act to the turn it precedes: `before[i]` are the rows drawn
 * above turn i (acts between turn i-1 and turn i), `after` the rows past the last turn.
 * Acts older than the conversation shown are dropped with it — they belong to history
 * the reader has not loaded. The floor is the later of `since` (the session's start, as
 * the serve reports it) and the first mounted turn's clock; with neither known (a fresh
 * session whose only turn has no clock), the newest dozen acts stand in, so a week of
 * the journal never lands under one bubble (the simulator, 2026-09-15). Turns without a
 * clock take no acts.
 */
export function placeActs(turns: TranscriptTurn[], acts: Act[], since = 0): {before: ConsoleRow[][]; after: ConsoleRow[]} {
  let sorted = acts.slice().sort((a, b) => a.ts - b.ts);
  const before: ConsoleRow[][] = turns.map(() => []);
  const times = turns.map(turnSecs);
  const first = Math.max(times.find(x => x > 0) ?? 0, since);
  if (first === 0) sorted = sorted.slice(-12);
  const buckets: Act[][] = turns.map(() => []);
  const tail: Act[] = [];
  for (const a of sorted) {
    if (first && a.ts < first) continue;
    let placed = false;
    for (let i = 0; i < turns.length; i++) {
      if (times[i] > 0 && a.ts < times[i]) {
        buckets[i].push(a);
        placed = true;
        break;
      }
    }
    if (!placed) tail.push(a);
  }
  for (let i = 0; i < turns.length; i++) before[i] = foldRuns(buckets[i]);
  return {before, after: foldRuns(tail)};
}
