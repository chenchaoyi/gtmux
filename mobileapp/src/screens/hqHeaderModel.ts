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
import {fleetCounts, planLabel, relTime} from './hqZones';

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

/** One run of the brief. `code` marks what HQ wrote between backticks. */
export interface InlineSeg {
  text: string;
  code: boolean;
}

/** The supervisor's own latest brief, attributed and dated. */
export interface Brief {
  segments: InlineSeg[];
  /** "12m ago" / "12分钟前". Null when the turn carries no time — never invented. */
  age: string | null;
}

/**
 * One labelled figure in the disclosure. The three kinds used to be one gray sentence
 * that wrapped ("0 need you · 1 working · 16 idle · 5h 21% · wk 53% · Fable 78% disk
 * 16GB free"), so nothing said where the fleet ended and the subscription began. They
 * are three different questions and get three rows.
 */
export interface Stat {
  key: 'fleet' | 'usage' | 'machine';
  label: string;
  value: string;
  /**
   * `warn` when the value is the core's own warning rather than a plain reading. An
   * amber machine stays inside the disclosure (only red is read standing), but rendering
   * "disk 16GB free" in the same gray as "5h 24%" tells the reader it is a figure when
   * it is a warning.
   */
  tone?: 'warn';
}

export interface HeaderModel {
  /** The one-line judgment — the only prose in the standing header. */
  verdict: string;
  /** Render the verdict in the attention colour (someone is blocked on the user). */
  urgent: boolean;
  /** Promoted out of the disclosure: shown standing, under the verdict. Null in the normal case. */
  standing: string | null;
  /** The supervisor's own most recent brief, or null when it has produced none. */
  brief: Brief | null;
  /** The derived figures, for the disclosure. A row with nothing to say is absent. */
  stats: Stat[];
}

// briefMark is the supervisor's signal register (playbook v2+). A reply that opens with
// it is HQ addressing the user about the situation, which is exactly what a brief is; a
// reply that does not is an answer to something specific and is not one.
const briefMark = '⟣';

/**
 * inlineSegments splits HQ's prose on backticks so the view can set what HQ marked as
 * code in a monospace face.
 *
 * It exists because the header used to print the backticks themselves: HQ writes
 * "`%19` 下拉刷新修好了", and a header that shows the punctuation of a markup language
 * it does not render is the single clearest way to look unfinished. An unpaired backtick
 * stays literal text — guessing where the run ends would silently re-style the rest of
 * the sentence.
 */
export function inlineSegments(text: string): InlineSeg[] {
  const parts = text.split('`');
  if (parts.length % 2 === 0) return [{text, code: false}]; // unpaired: leave it alone
  const out: InlineSeg[] = [];
  parts.forEach((t, i) => {
    if (t !== '') out.push({text: t, code: i % 2 === 1});
  });
  return out.length > 0 ? out : [{text, code: false}];
}

/**
 * supervisorBrief returns the supervisor's most recent brief — the newest reply written
 * in its own signal register — with the register glyph stripped (the block's own
 * attribution carries the mark, so keeping it would print it twice).
 *
 * Null when the supervisor has produced none: an empty disclosure is honest, and picking
 * "the newest reply" instead would present an answer to some specific question as if it
 * were a standing brief.
 *
 * The age comes from the turn's timestamp, which the transcript records for the PROMPT —
 * the wake that produced the brief, a turn ahead of the reply. At the minute granularity
 * shown that is the same number, and it is a real reading rather than an invented one:
 * a turn with no time gets no age, never a guessed one.
 */
export function supervisorBrief(
  turns: TranscriptTurn[],
  nowSecs: number,
  zh: boolean,
): Brief | null {
  for (let i = turns.length - 1; i >= 0; i--) {
    const text = (turns[i]?.response ?? '').trim();
    if (!text.startsWith(briefMark)) continue;
    const body = text.slice(briefMark.length).trim();
    if (body === '') continue;
    const at = Date.parse(turns[i]?.time ?? '');
    const age = Number.isNaN(at)
      ? null
      : zh
        ? `${relTime(Math.floor(at / 1000), nowSecs)}前`
        : `${relTime(Math.floor(at / 1000), nowSecs)} ago`;
    return {segments: inlineSegments(body), age};
  }
  return null;
}

/** fleetStat is the state tally: how many of the fleet are in each state. */
export function fleetStat(digest: DigestRow[], zh: boolean): Stat {
  const c = fleetCounts(digest);
  return {
    key: 'fleet',
    label: zh ? '舰队' : 'fleet',
    value: zh
      ? `${c.waiting} 需要你 · ${c.working} 运行 · ${c.idle} 空闲`
      : `${c.waiting} need you · ${c.working} working · ${c.idle} idle`,
  };
}

/**
 * usageStat is each subscription window's burn. Null when the endpoint reported none —
 * a row reading "usage —" says less than no row at all.
 */
export function usageStat(week: WindowPct[], zh: boolean): Stat | null {
  if (week.length === 0) return null;
  return {
    key: 'usage',
    label: zh ? '用量' : 'usage',
    value: week.map(w => `${planLabel(w.label, zh)} ${w.pct}%`).join('  ·  '),
  };
}

/**
 * machineStat is the machine's own line. `warn` is the core's sentence and wins; the
 * disk figure is the fallback for when there is nothing to warn about.
 *
 * A memory tier is appended only when the core reported one. The old line printed
 * "mem —" whenever it had not, which spends a reader's attention to tell them nothing.
 */
export function machineStat(res: ResourceState | null, zh: boolean): Stat | null {
  const label = zh ? '机器' : 'machine';
  if (!res) return null;
  if (res.warn) return {key: 'machine', label, value: res.warn, tone: 'warn'};
  if (res.diskGB == null) return null;
  const disk = zh ? `磁盘 ${res.diskGB}GB 可用` : `${res.diskGB}GB free`;
  const mem = res.memTier ? (zh ? ` · 内存 ${res.memTier}` : ` · mem ${res.memTier}`) : '';
  return {key: 'machine', label, value: disk + mem};
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
  nowSecs: number;
  zh: boolean;
}): HeaderModel {
  const machine = machineStat(args.res, args.zh);
  const critical = isCritical(args.res);
  return {
    verdict: args.verdict,
    urgent: args.urgent,
    // A critical machine is read standing, so it does not also sit in the list below —
    // the same figure in two places reads as two facts.
    standing: critical ? (machine?.value ?? null) : null,
    brief: supervisorBrief(args.turns, args.nowSecs, args.zh),
    stats: [
      fleetStat(args.digest, args.zh),
      usageStat(args.week, args.zh),
      critical ? null : machine,
    ].filter((x): x is Stat => x != null),
  };
}
