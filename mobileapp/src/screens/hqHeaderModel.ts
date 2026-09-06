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
//   - the disclosure quotes the supervisor's LATEST word, and shows nothing when that word
//     was routine. Measured on a real HQ session: 41 `▪ noted:` against 7 `✅`, 2 `⚠` and
//     1 `◈ brief`. Quoting "the newest `⟣` reply" therefore quoted bookkeeping four times
//     out of five ("…水位到 32134") under a byline calling it a brief; but skipping PAST
//     the ledger to the newest interesting line is worse, because a later note is often
//     exactly what retires an earlier alarm ("上一条升级撤回", 2026-09-05) — that header
//     would have gone on showing a withdrawn escalation for hours under a verdict reading
//     "all normal". So the newest register line decides, and a routine one decides for
//     silence. See `supervisorSignal`.
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
  /** Whose plan. Absent from a serve older than 0.93 — see `tightestPerPlan`. */
  agent?: string;
}

/** One run of the brief. `code` marks what HQ wrote between backticks. */
export interface InlineSeg {
  text: string;
  code: boolean;
}

/**
 * The grade of a supervisor signal, as the playbook's register defines it. Only these
 * three reach the header; see `supervisorSignal`.
 */
export type SignalGrade = 'escalation' | 'done' | 'brief';

/** The supervisor's own latest signal, graded, attributed and dated. */
export interface Signal {
  grade: SignalGrade;
  /** The headline. */
  segments: InlineSeg[];
  /** A brief's indented `· ` items. Empty for the other grades. */
  bullets: InlineSeg[][];
  /** "12m ago" / "12分钟前". Null when the turn carries no time — never invented. */
  age: string | null;
}

/**
 * One row of the disclosure. The KEY is what the row answers, and the three keys
 * are the three questions a chief-of-staff report has — in that order.
 */
/** The knowledge debt, as the header needs it. */
export interface OwedKnowledge {
  pending: number;
  /** "12d" / "12天" — rendered by the caller, which owns the language of time. */
  oldestLabel?: string;
  /** Past the floor `gtmux doctor` uses (~2 weeks). Only this earns amber. */
  overdue?: boolean;
}

export interface Row {
  key: 'owed' | 'did' | 'context';
  label: string;
  value: string;
  /** Amber. Only `owed` ever earns it, and only past the floor doctor uses. */
  tone?: 'warn';
}

export interface HeaderModel {
  /** The one-line judgment — the only prose in the standing header. */
  verdict: string;
  /** Render the verdict in the attention colour (someone is blocked on the user). */
  urgent: boolean;
  /** Promoted out of the disclosure: shown standing, under the verdict. Null in the normal case. */
  standing: string | null;
  /** The supervisor's own most recent header-grade signal, or null when it has none. */
  signal: Signal | null;
  /**
   * The disclosure, in the order a chief of staff reports: what is owed to you,
   * what it did, and only then the context. A row with nothing to say is absent —
   * a line that is always there says nothing when it matters.
   */
  rows: Row[];
}

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

// signalMark is the supervisor's signal register (playbook v2+). A reply that opens with
// it is HQ speaking about the situation rather than answering a question.
const signalMark = '⟣';

// The grades that earn the header, keyed by the glyph the playbook assigns them. The two
// that are absent are absent on purpose: `▪` (ledger) is by its own definition a routine
// outcome needing no action, and `📓` (captured) is a knowledge-base write the KNOWLEDGE
// row already counts. Both stay in the console, where reading them is the point.
const headerGrades: {glyph: string; grade: SignalGrade}[] = [
  {glyph: '⚠', grade: 'escalation'},
  {glyph: '✅', grade: 'done'},
  {glyph: '◈', grade: 'brief'},
];

// A brief opens `◈ brief 14:30 │ …` / `◈ 简报 14:30 │ …`. The byline already says who and
// how long ago, so the word and the clock are struck; if the line does not have that
// shape it is left exactly as HQ wrote it.
const briefPreamble = /^(?:brief|简报)\s+\d{1,2}:\d{2}\s*(?:│|\|)\s*/;

// A brief's items are indented continuation lines. HQ writes `· `; a model that has
// drifted to `- ` or `• ` means the same thing and is read the same way.
const bulletLine = /^\s*[·•-]\s+(.*)$/;

/**
 * supervisorSignal returns the supervisor's most recent word about the situation — its
 * newest reply written in the signal register — but only when HQ graded that word as one
 * worth standing. The register glyph is stripped (the block's own attribution carries the
 * mark, so keeping it would print it twice).
 *
 * Null when the newest register line was routine, and null when there is none at all. It
 * deliberately does NOT search backwards past a routine line for something more
 * interesting: a `▪ noted:` is frequently the thing that RETIRES the alarm above it, so a
 * header that skipped it would keep showing a withdrawn escalation. Latest word, or
 * nothing.
 *
 * Replies that are not in the register are answers to specific questions, not status
 * claims, and are passed over rather than treated as either.
 *
 * The age comes from the turn's timestamp, which the transcript records for the PROMPT —
 * the wake that produced the reply, a turn ahead of it. At the minute granularity shown
 * that is the same number, and it is a real reading rather than an invented one: a turn
 * with no time gets no age, never a guessed one.
 */
export function supervisorSignal(
  turns: TranscriptTurn[],
  nowSecs: number,
  zh: boolean,
): Signal | null {
  for (let i = turns.length - 1; i >= 0; i--) {
    const text = (turns[i]?.response ?? '').trim();
    if (!text.startsWith(signalMark)) continue;
    const body = text.slice(signalMark.length).trim();
    const hit = headerGrades.find(g => body.startsWith(g.glyph));
    if (!hit) return null; // the latest word was routine — say nothing rather than the one before it
    let rest = body.slice(hit.glyph.length).trim();
    if (rest === '') continue;
    if (hit.grade === 'brief') rest = rest.replace(briefPreamble, '');

    const lines = rest.split('\n');
    const head: string[] = [];
    const bullets: InlineSeg[][] = [];
    for (const line of lines) {
      const m = bulletLine.exec(line);
      if (m && m[1].trim() !== '') bullets.push(inlineSegments(m[1].trim()));
      else if (bullets.length === 0 && line.trim() !== '') head.push(line.trim());
    }
    const headline = head.join(' ').trim();
    if (headline === '' && bullets.length === 0) continue;

    const at = Date.parse(turns[i]?.time ?? '');
    const age = Number.isNaN(at)
      ? null
      : zh
        ? `${relTime(Math.floor(at / 1000), nowSecs)}前`
        : `${relTime(Math.floor(at / 1000), nowSecs)} ago`;
    return {grade: hit.grade, segments: inlineSegments(headline), bullets, age};
  }
  return null;
}

/** gradeLabel names the grade in the byline, so the reader knows what they are reading. */
export function gradeLabel(grade: SignalGrade, zh: boolean): string {
  if (grade === 'escalation') return zh ? '升级' : 'escalation';
  if (grade === 'done') return zh ? '完成' : 'done';
  return zh ? '简报' : 'brief';
}

/**
 * tightestPerPlan keeps ONE window per plan: the one that runs out first.
 *
 * A summary line has room for one window per plan, not for every window. Codex's
 * plan doubled the count and the row started truncating mid-number ("Fable 11…"),
 * which is the one thing a percentage must never do. A serve older than 0.93 sends
 * no `agent`, so each window groups under its own label and nothing is merged.
 */
export function tightestPerPlan(week: WindowPct[]): WindowPct[] {
  const at = new Map<string, number>();
  const out: WindowPct[] = [];
  for (const w of week) {
    const key = w.agent ?? w.label;
    const i = at.get(key);
    if (i === undefined) {
      at.set(key, out.length);
      out.push(w);
    } else if (w.pct > out[i].pct) {
      out[i] = w;
    }
  }
  return out;
}

/**
 * isCritical reports whether a resource condition has earned the standing header.
 *
 * The tier is the core's judgment and is the only thing consulted — not the
 * presence of a `warn` string, which the core also sets at the amber tier.
 * Promoting amber would put a line back in the standing header most of the time,
 * which is the state this design exists to leave.
 */
export function isCritical(res: ResourceState | null): boolean {
  return res?.tier === 'red';
}

/**
 * owedRow is the debt only YOU can discharge.
 *
 * It leads the disclosure because it is the one thing on this card that is both
 * actionable and nobody else's job. It used to be last and dimmest — "370 entries
 * · 8 waiting on you" at the bottom of a stack of sensor readings — while the
 * loudest colour on the card went to a disk warning you cannot act on from a
 * phone. That ranking is backwards, and it is the reason the card read as a
 * dashboard rather than a report.
 *
 * Agents waiting on you are NOT folded in: the verdict already says that, in the
 * attention colour, one line up.
 */
export function owedRow(k: OwedKnowledge | null, zh: boolean): Row | null {
  const pending = k?.pending ?? 0;
  if (pending <= 0) return null;
  const age = k?.oldestLabel ? (zh ? ` · 最老 ${k.oldestLabel}` : ` · oldest ${k.oldestLabel}`) : '';
  return {
    key: 'owed',
    label: zh ? '待你' : 'owed',
    value: (zh ? `${pending} 条待带走` : `${pending} to carry`) + age,
    tone: k?.overdue ? 'warn' : undefined,
  };
}

/**
 * didRow is what the supervisor did while you were away.
 *
 * This is the row the card was missing, and its absence is what made the page a
 * dashboard: a chief of staff that does not show you what it did is an instrument
 * panel with a chat box. The acts are already collected and ranked for the "HQ's
 * work" zone; this is their headline.
 */
export function didRow(tally: {verb: string; n: number}[], zh: boolean): Row | null {
  const top = tally.filter(t => t.n > 0).slice(0, 3);
  if (top.length === 0) return null;
  return {
    key: 'did',
    label: zh ? '它做了' : 'HQ did',
    value: top.map(t => `${t.verb} ${t.n}`).join(' · '),
  };
}

/**
 * contextRow is everything the radar and the usage view already show, compressed
 * to one line and included only where it is not the ordinary case.
 *
 * The page's own rule is that the fleet LIST belongs to the radar and the pixels
 * here must not be sold to what a swipe away already says. The fleet COUNT had
 * crept back as a full row reading "0 need you · 0 working · 17 idle" — three
 * numbers to say nothing is happening, directly under a verdict that just said
 * so. So the count appears only when something is actually moving, the usage
 * figure is the tightest window rather than every window, and the machine appears
 * only when the core has a warning to give.
 */
export function contextRow(
  digest: DigestRow[],
  week: WindowPct[],
  res: ResourceState | null,
  zh: boolean,
): Row | null {
  const parts: string[] = [];
  const c = fleetCounts(digest);
  if (c.waiting > 0 || c.working > 0) {
    const bits: string[] = [];
    if (c.waiting > 0) bits.push(zh ? `${c.waiting} 等你` : `${c.waiting} need you`);
    if (c.working > 0) bits.push(zh ? `${c.working} 运行` : `${c.working} working`);
    parts.push(bits.join(' '));
  }
  const tight = tightestPerPlan(week).sort((a, b) => b.pct - a.pct)[0];
  if (tight) parts.push(`${planLabel(tight, zh)} ${tight.pct}%`);
  if (res?.warn) parts.push(res.warn);
  if (parts.length === 0) return null;
  return {key: 'context', label: zh ? '现状' : 'context', value: parts.join('  ·  ')};
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
  /** What HQ did recently, already ranked by hqActsModel.tally. */
  did: {verb: string; n: number}[];
  /** The knowledge debt — the one obligation only the commander can discharge. */
  owed: OwedKnowledge | null;
  nowSecs: number;
  zh: boolean;
}): HeaderModel {
  const critical = isCritical(args.res);
  return {
    verdict: args.verdict,
    urgent: args.urgent,
    // A critical machine is read standing, so it does not also sit in the rows
    // below — the same figure in two places reads as two facts.
    standing: critical ? (args.res?.warn ?? null) : null,
    signal: supervisorSignal(args.turns, args.nowSecs, args.zh),
    rows: [
      owedRow(args.owed, args.zh),
      didRow(args.did, args.zh),
      critical ? null : contextRow(args.digest, args.week, args.res, args.zh),
    ].filter((x): x is Row => x != null),
  };
}
