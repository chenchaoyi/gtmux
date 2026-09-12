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
 * knowledgeValue is the header row's value — the base's size, and what it owes.
 *
 * The owed count is the part that earns a place on a standing row: it is a debt with a
 * clock on it (doctor flags one past two weeks), and a reader who has to open the sheet to
 * discover they owe something will not open the sheet.
 *
 * It does NOT name the base. The header's rows carry their name in a key column, so a
 * value reading "knowledge · 352" prints the word twice on one line.
 */
/**
 * knowledgeOverdue reports whether the queue has passed the line doctor uses.
 *
 * The colour rule the header follows is 色=状态, and amber means "past its line" — so a
 * count of pending promotions must NOT be amber merely for being non-zero. Seven waiting
 * and none of them old is a healthy queue; painting it the same as a machine warning
 * spends the one colour that is supposed to mean something.
 */
export function knowledgeOverdue(idx: KnowledgeIndex): boolean {
  return (idx.promotions?.pending ?? 0) > 0 && (idx.promotions?.oldest_sec ?? 0) >= PROMOTION_STALE_SECS;
}

export function knowledgeValue(idx: KnowledgeIndex, zh: boolean): string | null {
  const n = idx.entries?.length ?? 0;
  const owed = idx.promotions?.pending ?? 0;
  if (n === 0 && owed === 0) return null; // no base: no row, rather than a row saying zero
  const head = zh ? `${n} 条` : `${n} entries`;
  if (owed === 0) return head;
  return head + (zh ? ` · ${owed} 待带走` : ` · ${owed} waiting on you`);
}

/**
 * matchEntries filters the base by what someone typed.
 *
 * 396 entries across 7 topics, and until now the only way in was knowing which topic
 * holds the one you want — which is knowledge about the knowledge base, not about your
 * machine (2026-09-09). Matching is case-insensitive across the title, the id and the
 * topic: the id is how HQ refers to an entry in a dispatch, so it is a thing people
 * actually paste in.
 *
 * Whitespace splits the query into terms that must ALL match somewhere. A single term is
 * the common case; two is how you say "the pitfalls one about ps".
 */
export function matchEntries(entries: KnowledgeEntry[], query: string): KnowledgeEntry[] {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return [];
  return entries.filter(e => {
    const hay = `${e.title} ${e.id} ${e.topic}`.toLowerCase();
    return terms.every(t => hay.includes(t));
  });
}

/** The one word an audience wears on every screen. */
export function audienceWord(audience: string | undefined, zh: boolean): string {
  switch (audience ?? '') {
    case 'hq':
      return 'HQ';
    case 'machine':
      return zh ? '本机' : 'this machine';
    case 'repo':
      return zh ? '仓库' : 'a repository';
    case 'everyone':
      return zh ? '全体' : 'everyone';
    default:
      return '';
  }
}

/**
 * axesLine renders the three axes as one line of metadata: what it is (with a ? while
 * the kind is only the migration's guess), where it came from and how often, who must
 * know it. Empty for a row that predates the axes.
 */
export function axesLine(e: KnowledgeEntry, zh: boolean): string {
  const parts: string[] = [];
  if (e.kind) parts.push(e.kind + (e.kind_assumed ? '?' : ''));
  if (e.provenance) parts.push((zh ? '来自 ' : 'from ') + e.provenance + (e.hits && e.hits > 1 ? ` ×${e.hits}` : ''));
  const aud = audienceWord(e.audience, zh);
  if (aud) parts.push((zh ? '给 ' : 'for ') + aud);
  if (e.status === 'hypothesis') parts.push(zh ? '待验证' : 'hypothesis');
  return parts.join(' · ');
}

/** One act the entry pane can offer. `feedback` opens a URL; the rest post to serve. */
export type EntryAct =
  | {kind: 'carry'; id: string}
  | {kind: 'feedback'; url: string}
  | {kind: 'land'; id: string}
  | {kind: 'withdraw'; id: string}
  | {kind: 'retire'; id: string};

/**
 * actsFor lists what an entry offers, in order — the exit its AUDIENCE has. gtmux carries
 * hq / machine / repo; a person opens the issue for everyone (then lands with its URL); a
 * promotion with no audience can only be landed by hand or withdrawn. An entry that is
 * not pending offers nothing but retire here: promoting needs the audience question,
 * which stays on the Mac and in the CLI.
 */
export function actsFor(e: KnowledgeEntry): EntryAct[] {
  if (!isPending(e)) return [{kind: 'retire', id: e.id}];
  switch (e.audience ?? '') {
    case 'hq':
    case 'machine':
    case 'repo':
      return [{kind: 'carry', id: e.id}, {kind: 'withdraw', id: e.id}, {kind: 'retire', id: e.id}];
    case 'everyone': {
      const acts: EntryAct[] = [];
      if (e.issue_url) acts.push({kind: 'feedback', url: e.issue_url});
      return [...acts, {kind: 'land', id: e.id}, {kind: 'withdraw', id: e.id}, {kind: 'retire', id: e.id}];
    }
    default:
      return [{kind: 'land', id: e.id}, {kind: 'withdraw', id: e.id}, {kind: 'retire', id: e.id}];
  }
}

/** The button's words for an act — the literal thing that happens. */
export function actButtonLabel(a: EntryAct, zh: boolean): string {
  switch (a.kind) {
    case 'carry':
      return zh ? '写进去' : 'Write it in';
    case 'feedback':
      return zh ? '反馈给 gtmux ↗' : 'Feedback to gtmux ↗';
    case 'land':
      return zh ? '标记为已落地…' : 'Mark it landed…';
    case 'withdraw':
      return zh ? '撤回晋升…' : 'Withdraw the promotion…';
    case 'retire':
      return zh ? '这条不再成立…' : 'It no longer holds…';
  }
}

export function withdrawPrompt(zh: boolean): {title: string; hint: string; placeholder: string} {
  return zh
    ? {title: '撤回这次晋升', hint: '条目留着，只撤掉晋升。为什么不值得搬？理由会留在事件流里。', placeholder: '例如 只在这台机器上成立'}
    : {title: 'withdraw this promotion', hint: 'The entry stays; only the promotion goes. Why is it not worth carrying? The reason survives in the journal.', placeholder: 'e.g. only true on this machine'};
}

export function carryPrompt(zh: boolean): {title: string; hint: string; placeholder: string} {
  return zh
    ? {title: '让 gtmux 搬进去', hint: 'gtmux 把它写到这个读者看的地方 —— 你的 LOCAL.md、本机每个 agent 的知识块、或那个仓库的指令文件（不提交）—— 然后标记为已落地。', placeholder: ''}
    : {title: 'let gtmux carry it', hint: "gtmux writes it where this audience reads — your LOCAL.md, every agent's knowledge block on this machine, or the repository's instruction file (not committed) — and marks it landed.", placeholder: ''};
}
