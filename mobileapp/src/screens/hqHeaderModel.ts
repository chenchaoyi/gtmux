// hqHeaderModel — the HQ page's standing header, as DATA (hq-page-shows-its-work).
//
// The header used to be four stacked bands costing ~200pt before the body started, and
// what it bought was mostly fleet counts and a disk/memory line — both one swipe away on
// the radar — while the one interactive zone underneath paid for them on every keystroke.
//
// So the standing header now carries the judgment and nothing else, and everything it
// used to show standing moves behind a disclosure on that line. Deciding WHAT goes where
// is this module's whole job; the view renders what it is handed.
//
// The name is `hqHeaderModel`, not `hqHeader`, because macOS resolves module paths
// case-insensitively: a `hqHeader.ts` beside a `HQHeader.tsx` is one name to the
// resolver, and the import silently lands on the wrong file (this repo has been caught
// by it once before, in rowSheet/RowSheet).
//
// Two rules live here because they are judgments, not layout:
//   - the disclosure opens with the supervisor's OWN latest brief when it has one. HQ
//     already produces a periodic brief in its `⟣` register, and its own words beat a
//     recomputed "3 需要你 · 0 运行 · 12 空闲".
//   - a resource condition is promoted OUT of the disclosure only at the CRITICAL tier.
//     The old header printed disk/memory unconditionally, which is why it read as noise:
//     a line that is always there says nothing when it matters.

import {TranscriptTurn} from '../api/client';
import {DigestRow} from '../api/client';
import {fleetCounts, planLabel} from './hqZones';

/** The machine-resource slice the header cares about. */
export interface ResourceState {
  warn?: string;
  diskGB?: number | null;
  memTier?: string;
  /** The core's own tier. `red` is the only one that earns the standing line. */
  tier?: 'amber' | 'red';
}

/** One subscription window, as the usage endpoint reports it. */
export interface WindowPct {
  label: string;
  pct: number;
}

export interface HeaderModel {
  /** The one-line judgment — the only prose in the standing header. */
  verdict: string;
  /** Render the verdict in the attention colour (someone is blocked on the user). */
  urgent: boolean;
  /** Promoted out of the disclosure: shown standing, under the verdict. Null in the normal case. */
  standing: string | null;
  /** The supervisor's own most recent brief, or null when it has produced none. */
  brief: string | null;
  /** The derived fleet line, for the disclosure. */
  fleet: string;
  /** The derived resource line, for the disclosure. Null when nothing is known. */
  resource: string | null;
}

// briefMark is the supervisor's signal register (playbook v2+). A reply that opens with
// it is HQ addressing the user about the situation, which is exactly what a brief is; a
// reply that does not is an answer to something specific and is not one.
const briefMark = '⟣';

/**
 * supervisorBrief returns the supervisor's most recent brief — the newest reply written
 * in its own signal register — with the register glyph stripped (the disclosure labels
 * the line itself, so keeping it would print the mark twice).
 *
 * Null when the supervisor has produced none: an empty disclosure is honest, and picking
 * "the newest reply" instead would present an answer to some specific question as if it
 * were a standing brief.
 */
export function supervisorBrief(turns: TranscriptTurn[]): string | null {
  for (let i = turns.length - 1; i >= 0; i--) {
    const text = (turns[i]?.response ?? '').trim();
    if (text.startsWith(briefMark)) {
      const body = text.slice(briefMark.length).trim();
      if (body !== '') return body;
    }
  }
  return null;
}

/** fleetLine is the derived state tally, plus each subscription window's usage. */
export function fleetLine(digest: DigestRow[], week: WindowPct[], zh: boolean): string {
  const c = fleetCounts(digest);
  const head = zh
    ? `${c.waiting} 需要你 · ${c.working} 运行 · ${c.idle} 空闲`
    : `${c.waiting} need you · ${c.working} working · ${c.idle} idle`;
  if (week.length === 0) return head;
  return head + '  ·  ' + week.map(w => `${planLabel(w.label, zh)} ${w.pct}%`).join(' · ');
}

/**
 * resourceLine is the machine's own line. `warn` is the core's sentence and wins; the
 * disk/memory figures are the fallback for when there is nothing to warn about but the
 * numbers are still worth having in the disclosure.
 */
export function resourceLine(res: ResourceState | null, zh: boolean): string | null {
  if (!res) return null;
  if (res.warn) return res.warn;
  if (res.diskGB == null) return null;
  return zh
    ? `磁盘 ${res.diskGB}GB · 内存 ${res.memTier ?? '—'}`
    : `disk ${res.diskGB}GB · mem ${res.memTier ?? '—'}`;
}

/**
 * isCritical reports whether a resource condition has earned the standing header.
 *
 * The tier is the core's judgment and is the only thing consulted — not the presence of
 * a `warn` string, which the core also sets at the amber tier. Promoting amber would put
 * a line back in the standing header most of the time, which is the state this change
 * exists to leave.
 */
export function isCritical(res: ResourceState | null): boolean {
  return res?.tier === 'red';
}

/**
 * headerModel assembles the whole header from what the page already polls.
 *
 * `verdict` arrives rendered because the sentence is the caller's language and the
 * judgment behind it is the core's (see hqZones.assessment) — this module decides
 * placement, not wording.
 */
export function headerModel(args: {
  verdict: string;
  urgent: boolean;
  digest: DigestRow[];
  turns: TranscriptTurn[];
  week: WindowPct[];
  res: ResourceState | null;
  zh: boolean;
}): HeaderModel {
  const resource = resourceLine(args.res, args.zh);
  return {
    verdict: args.verdict,
    urgent: args.urgent,
    standing: isCritical(args.res) ? resource : null,
    brief: supervisorBrief(args.turns),
    fleet: fleetLine(args.digest, args.week, args.zh),
    resource,
  };
}
