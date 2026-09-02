// hqActs — the supervisor's own work, read out of the journal (hq-page-shows-its-work).
//
// The page used to give its middle zone to the FLEET's lifecycle: sessions starting,
// stopping, waiting. That is what the workers did. Measured over one week on a real
// machine, what the SUPERVISOR did in the same period — 27 dispatches, 4 reclaims, 168
// knowledge entries, 8 self-audits, its own rotations — reached no surface at all. A
// chief of staff that acts on your behalf and never shows its work is a dashboard.
//
// The journal already carries every one of those acts (`gtmux:*`, served by
// /api/hq/events at routine severity). This module decides which of them are ACTS and
// says them in words.

import {HQEvent} from '../api/client';

export interface Act {
  ts: number;
  /** The raw journal event name — the row key, and what a test pins. */
  kind: string;
  /** What the supervisor did, in one word. */
  verb: string;
  /** What it acted on: a pane, a session, or "" when the act has no target. */
  target: string;
  /** The act itself, in the journal's own words. */
  detail: string;
  /** How it ended, when the record says (a dispatch landed, was refused, …). */
  outcome?: string;
  /** An act that is a warning about the supervision itself, not routine work. */
  alarm?: boolean;
}

// Wake delivery is PLUMBING, not work: gtmux knocking on HQ's door 1532 times a week is
// not the supervisor doing something, and including it would bury the 39 acts that are.
// Its failure mode still surfaces — as `wake-degraded`, which is an alarm about the
// supervision channel and is exactly the thing worth hearing.
const plumbing = new Set(['gtmux:audit:wake-delivered', 'gtmux:audit:wake-dropped']);

/** isSupervisorAct partitions the journal: the supervisor's own acts, or the fleet's life. */
export function isSupervisorAct(e: HQEvent): boolean {
  return typeof e.event === 'string' && e.event.startsWith('gtmux:') && !plumbing.has(e.event);
}

// verbs are gtmux's words for its own acts. An unlisted kind falls through to a readable
// derivation rather than a raw token: a journal kind added later must degrade, not leak
// an identifier at the user.
const verbs: Record<string, {en: string; zh: string; alarm?: boolean}> = {
  'gtmux:audit:send': {en: 'dispatched', zh: '派活'},
  'gtmux:audit:reap': {en: 'reclaimed', zh: '回收'},
  'gtmux:audit:knowledge': {en: 'recorded', zh: '记账'},
  'gtmux:audit:rotate': {en: 'rotated', zh: '轮换'},
  'gtmux:audit:hq-session': {en: 'handed over', zh: '换班'},
  'gtmux:self-check': {en: 'self-audit', zh: '自审'},
  'gtmux:distill': {en: 'distilled', zh: '沉淀'},
  'gtmux:wake-degraded': {en: 'wake channel degraded', zh: '唤醒通道异常', alarm: true},
};

/**
 * fallbackVerb turns an unknown journal kind into something readable: the last path
 * segment, hyphens opened out. `gtmux:audit:promote` reads "promote", not the whole
 * token — the reader learns nothing from the prefix they did not already know.
 */
export function fallbackVerb(kind: string): string {
  const tail = kind.split(':').pop() ?? kind;
  return tail.replace(/-/g, ' ');
}

// A uuid is noise on a phone: it identifies a session to a machine, and the reader can
// only ever compare its first few characters anyway.
const uuidRe = /\b([0-9a-f]{8})-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/gi;

export function shortenIds(text: string): string {
  return text.replace(uuidRe, '$1…');
}

/**
 * splitOutcome pulls a leading `state: rest` off a summary. A dispatch records
 * `landed: <payload>` or `refused-draft: <payload>`, and the state is the part a reader
 * scans for — it belongs beside the act, not buried at the head of its text.
 *
 * Only a SHORT, single-token-ish prefix counts, so an ordinary sentence containing a
 * colon keeps its text intact.
 */
export function splitOutcome(summary: string): {outcome?: string; detail: string} {
  const m = /^([a-z][a-z-]{2,24}):\s+([\s\S]+)$/.exec(summary);
  if (!m) return {detail: summary};
  return {outcome: m[1], detail: m[2]};
}

/** actTarget is what the act was aimed at: the pane is the handle a reader recognises. */
export function actTarget(e: HQEvent): string {
  return e.pane || e.session || (e.loc ?? '').split(':')[0] || '';
}

/** actOf renders one journal record as an act. */
export function actOf(e: HQEvent, zh: boolean): Act {
  const known = verbs[e.event];
  const {outcome, detail} = splitOutcome((e.summary ?? '').trim());
  return {
    ts: e.ts,
    kind: e.event,
    verb: known ? (zh ? known.zh : known.en) : fallbackVerb(e.event),
    target: actTarget(e),
    detail: shortenIds(detail),
    outcome,
    alarm: known?.alarm || e.severity === 'important',
  };
}

/** acts is the supervisor's own work out of a mixed journal feed, newest first. */
export function acts(events: HQEvent[], zh: boolean): Act[] {
  return events.filter(isSupervisorAct).map(e => actOf(e, zh));
}

/** fleet is the other half of the same partition — nothing is dropped by the filter. */
export function fleet(events: HQEvent[]): HQEvent[] {
  return events.filter(e => !isSupervisorAct(e));
}

export interface TallyEntry {
  kind: string;
  verb: string;
  n: number;
}

// tallyOrder is how the strip reads, and it is FIXED rather than sorted by count.
//
// Frequency is the wrong ranking twice over: the most numerous act (168 knowledge entries
// in the week this was written) is not the most consequential one (27 dispatches), and a
// strip that reorders itself as the numbers move is re-read from scratch every glance.
// Alarms lead because they are the one thing here that is not routine work.
const tallyOrder = [
  'gtmux:wake-degraded',
  'gtmux:audit:send',
  'gtmux:audit:reap',
  'gtmux:audit:knowledge',
  'gtmux:self-check',
  'gtmux:distill',
  'gtmux:audit:rotate',
  'gtmux:audit:hq-session',
];

/**
 * tally counts each kind of act inside a window — the line that answers "what has my
 * chief of staff been doing lately" before any single row is read. Plumbing is already
 * gone by the time this sees the acts; a kind gtmux does not yet rank sorts last, by
 * name, so a new one appears rather than vanishing.
 */
export function tally(list: Act[], now: number, windowSecs: number): TallyEntry[] {
  const by = new Map<string, TallyEntry>();
  for (const a of list) {
    if (now - a.ts > windowSecs) continue;
    const cur = by.get(a.kind);
    if (cur) cur.n++;
    else by.set(a.kind, {kind: a.kind, verb: a.verb, n: 1});
  }
  const rank = (k: string) => {
    const i = tallyOrder.indexOf(k);
    return i < 0 ? tallyOrder.length : i;
  };
  return [...by.values()].sort((x, y) => rank(x.kind) - rank(y.kind) || x.kind.localeCompare(y.kind));
}

export interface ActDay {
  /** A stable key for the day — the local date, not a rendered label. */
  key: string;
  /** How many days ago, so the view can name today and yesterday itself. */
  daysAgo: number;
  acts: Act[];
}

/**
 * groupByDay buckets acts into local days, newest day first, preserving the feed order
 * inside each. The label is left to the view: "today" is a word, and words are the
 * reader's language.
 */
export function groupByDay(list: Act[], now: number): ActDay[] {
  const dayKey = (ts: number) => {
    const d = new Date(ts * 1000);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  };
  const today = dayKey(now);
  const out: ActDay[] = [];
  const at = new Map<string, ActDay>();
  for (const a of list) {
    const key = dayKey(a.ts);
    let day = at.get(key);
    if (!day) {
      day = {key, daysAgo: daysBetween(key, today), acts: []};
      at.set(key, day);
      out.push(day);
    }
    day.acts.push(a);
  }
  return out;
}

function daysBetween(key: string, today: string): number {
  const ms = Date.parse(today + 'T00:00:00') - Date.parse(key + 'T00:00:00');
  return Math.round(ms / 86400000);
}
