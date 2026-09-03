// knowledgeModel — what the knowledge surface shows, and in what order, as DATA
// (hq-knowledge-on-phone).
//
// The base on a working machine is a few hundred entries. A phone cannot be a browser for
// that and should not try; it has to answer three questions, in this order:
//
//   1. WHAT DOES IT OWE ME — the promotion queue. HQ judged these charter-level and wrote
//      an export brief; only the commander can carry one somewhere durable and say so.
//      This is the only part of the whole knowledge lifecycle that BLOCKS on a person, so
//      it leads, and a brief past the doctor's floor is marked overdue.
//   2. WHAT DID IT JUST LEARN — the newest entries. A lesson recorded wrong is not inert:
//      it is echoed into every dispatch, so it repeats. Spot-checking recent writes is the
//      cheapest way to catch that, and reading is what a phone is good at.
//   3. WHERE IS EVERYTHING — the topics, with counts.
//
// The rules below are judgments (what leads, what counts as overdue, what a topic with no
// entries means), which is why they live here and are tested, rather than in the view.

import {KnowledgeEntry, KnowledgeIndex, KnowledgeTopic} from '../api/client';

/**
 * The age at which a pending promotion is overdue.
 *
 * Same floor `gtmux doctor` uses (~2 weeks). Picking a different number here would mean
 * the phone and the doctor disagree about the same queue, and a reader who saw one would
 * mistrust the other.
 */
export const PROMOTION_STALE_SECS = 14 * 24 * 3600;

export interface PromotionRow {
  entry: KnowledgeEntry;
  /** Seconds since it was promoted, 0 when unknown. */
  ageSec: number;
  overdue: boolean;
}

export interface KnowledgeView {
  /** What the commander owes, oldest first — the debt, not the newest news. */
  promotions: PromotionRow[];
  /** The most recent writes, newest first. */
  recent: KnowledgeEntry[];
  /** The vocabulary, biggest first, with empty topics dropped. */
  topics: KnowledgeTopic[];
  /** Every live entry, newest first — what a topic drill-down filters. */
  entries: KnowledgeEntry[];
  candidates: {pending: number; oldestSec: number};
  /** True when there is nothing at all: no entries, no queue. */
  empty: boolean;
}

/** isPending reports an entry whose promotion is still waiting to be carried. */
export function isPending(e: KnowledgeEntry): boolean {
  return !!e.promoted_at && !e.landed_at;
}

/**
 * buildKnowledgeView assembles the whole surface.
 *
 * `recentCount` is a glance, not a page: the phone is for spot-checking what HQ just
 * learned, and a reader who wants the twentieth-newest entry wants the topic list.
 */
export function buildKnowledgeView(idx: KnowledgeIndex, nowSecs: number, recentCount = 6): KnowledgeView {
  const entries = idx.entries ?? [];
  const promotions = entries
    .filter(isPending)
    .map(e => {
      const ageSec = e.promoted_at ? Math.max(0, nowSecs - e.promoted_at) : 0;
      return {entry: e, ageSec, overdue: ageSec >= PROMOTION_STALE_SECS};
    })
    // Oldest first: this is a debt list, and the oldest debt is the one rotting.
    .sort((a, b) => b.ageSec - a.ageSec);
  return {
    promotions,
    recent: entries.slice(0, recentCount),
    // Empty topics are vocabulary, not content — six built-ins ship with every install, so
    // listing them all would put five empty rows above the one that has 137 entries.
    topics: (idx.topics ?? []).filter(t => t.count > 0).sort((a, b) => b.count - a.count),
    entries,
    candidates: {pending: idx.candidates?.pending ?? 0, oldestSec: idx.candidates?.oldest_sec ?? 0},
    empty: entries.length === 0 && (idx.promotions?.pending ?? 0) === 0,
  };
}

/** entriesOfTopic filters the live set, keeping the index's newest-first order. */
export function entriesOfTopic(view: KnowledgeView, topic: string): KnowledgeEntry[] {
  return view.entries.filter(e => e.topic === topic);
}

/**
 * provenanceOf describes where a lesson came from, or null when the entry carries nothing
 * worth a line.
 *
 * Provenance is the reason to trust an entry, so it is shown — but only what is actually
 * recorded. A row reading "from —" would be worse than no row.
 */
export function provenanceOf(e: KnowledgeEntry, zh: boolean): string | null {
  const parts: string[] = [];
  if (e.pane) parts.push(e.pane);
  if (e.task) parts.push(zh ? `任务 ${e.task}` : `task ${e.task}`);
  if (e.capture) parts.push(zh ? '由 capture 折入' : 'from a capture');
  if (e.legacy) parts.push(zh ? '迁移自旧文件' : 'migrated from a legacy file');
  return parts.length > 0 ? parts.join(' · ') : null;
}

/**
 * actLabel names what an action will do in the words of the thing it does.
 *
 * The sheet redesign earlier this week established the rule: an action row says the
 * literal thing that happens, not a verb the reader has to interpret ("carry on" told
 * nobody it sent the word continue).
 */
export function landPrompt(zh: boolean): {title: string; hint: string; placeholder: string} {
  return zh
    ? {title: '标记为已落地', hint: '落到哪儿了？PR、spec、runbook 名都行 —— 这条会留在账本里。', placeholder: '例如 AGENTS.md / PR #888'}
    : {title: 'mark it landed', hint: 'Where did it land? A PR, a spec, a runbook name — this survives in the ledger.', placeholder: 'e.g. AGENTS.md / PR #888'};
}

export function retirePrompt(zh: boolean): {title: string; hint: string; placeholder: string} {
  return zh
    ? {title: '退休这一条', hint: '为什么？理由会留在账本里 —— 「这条后来错在哪」将来只能从它读到。', placeholder: '例如 办公网已修好，这条不再成立'}
    : {title: 'retire this entry', hint: 'Why? The reason survives in the ledger — it is the only place a later reader can learn what was wrong with it.', placeholder: 'e.g. the office network was fixed'};
}

/**
 * splitTitleKey pulls the KEY off the head of an entry title.
 *
 * HQ writes titles as "kb-entry-date-is-utc KB 条目落款用 UTC,…" — a stable key followed by
 * the prose. Both matter, but not equally and not in the same voice: the key is an
 * identifier (the repo already sets those in a monospace face — branch chips, pane ids,
 * versions) and the prose is what the reader is scanning for. Printed as one run in the
 * title face, four kebab words take the most prominent line on the card and say the least.
 *
 * The split is decided by EVIDENCE, not by shape. A first pass keyed on "looks kebab-case"
 * turned "well-known trap in the office network" into a key plus a fragment — and a title
 * losing its first phrase to a guess is worse than a title with a plain head. So the head
 * must also match the entry's own id, which the ledger derives from the title: only then
 * is it demonstrably the key rather than a hyphenated adjective.
 */
export function splitTitleKey(title: string, id?: string): {key: string | null; rest: string} {
  const m = /^([a-z0-9]+(?:-[a-z0-9]+)+)[:\s]\s*(\S.*)$/s.exec(title.trim());
  if (!m) return {key: null, rest: title};
  const slug = (id ?? '').split('/').pop() ?? '';
  if (!slug.startsWith(m[1])) return {key: null, rest: title};
  return {key: m[1], rest: m[2]};
}

/**
 * knowledgeLine is the header row's label — the base's size, and what it owes.
 *
 * The owed count is the part that earns a place on a standing row: it is a debt with a
 * clock on it (doctor flags one past two weeks), and a reader who has to open the sheet to
 * discover they owe something will not open the sheet.
 */
export function knowledgeLine(idx: KnowledgeIndex, zh: boolean): string | null {
  const n = idx.entries?.length ?? 0;
  const owed = idx.promotions?.pending ?? 0;
  if (n === 0 && owed === 0) return null; // no base: no row, rather than a row saying zero
  const head = zh ? `知识库 · ${n} 条` : `knowledge · ${n}`;
  if (owed === 0) return head;
  return head + (zh ? ` · ${owed} 待带走` : ` · ${owed} waiting on you`);
}
