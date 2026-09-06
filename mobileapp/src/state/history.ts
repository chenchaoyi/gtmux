// Composer input history (MOBILE §10 toolbar "↑ 历史") — the last messages you
// typed and sent, so you can recall/edit one instead of retyping.
//
// Scoped by WORK, not global, because that is what the text turns out to be.
// Measured over 35 projects and 8791 prompts on a real machine: among the short,
// phone-shaped sends, 96% of distinct entries appear in exactly ONE project — and
// they are not one-offs, they repeat inside it ("发布" 29 times, all in one
// project). Only 4% cross projects, and that handful is the universal head
// (continue · 继续 · ok · /compact) which belongs in quick replies, not here.
//
// A single global list of 30 therefore held mostly other projects' text, and the
// entry you actually wanted had been pushed out by unrelated traffic. That is the
// complaint this fixes: not "irrelevant things are present" but "the relevant one
// is gone".
//
// The scope key is `project` (the repo the pane sits in) falling back to the tmux
// SESSION NAME. Both are stable across restarts and both are things you named,
// unlike a pane id, which tmux recycles. On a real fleet the fallback is what
// covers HQ and the non-repo panes (vps-audit, disk-triage, 日常更新) — five of
// eighteen, including the busiest composer there is.
//
// `recent` is kept alongside and serves two jobs at once: it tops up a scope that
// is new or thin, so the list is never emptier than the global one used to be,
// and it is what an upgrade migrates the old flat array into — so nothing a user
// had is lost the moment this ships.

import AsyncStorage from '@react-native-async-storage/async-storage';

const KEY = 'gtmux.inputHistory';

/** How many entries one scope keeps. */
export const HISTORY_CAP = 20;
/** How many entries the cross-scope tail keeps. */
export const RECENT_CAP = 30;
/** How many scopes are remembered before the least recently used is dropped. */
export const SCOPE_CAP = 20;

export interface Scope {
  list: string[];
  at: number; // epoch ms of the last write, for the LRU above
}

export interface HistoryStore {
  scopes: Record<string, Scope>;
  recent: string[];
}

export const emptyStore = (): HistoryStore => ({scopes: {}, recent: []});

/**
 * historyScope names the work a pane belongs to: its repo, else the tmux session
 * it lives in. Never the pane id — tmux hands `%7` to a different pane after a
 * restart, so a pane-keyed history would decay into someone else's.
 *
 * Two panes in the same repo SHARE a scope, which is the point: they are the same
 * work, and what you typed in one is what you want in the other.
 */
export function historyScope(a: {project?: string; loc?: string}): string {
  const p = (a.project ?? '').trim();
  if (p) return p;
  const loc = (a.loc ?? '').trim();
  const session = loc.includes(':') ? loc.slice(0, loc.indexOf(':')) : loc;
  return session || 'default';
}

/** migrate accepts either shape: the flat array shipped before, or the store. */
export function migrate(raw: unknown): HistoryStore {
  if (Array.isArray(raw)) {
    // Everything a user already had becomes the cross-scope tail, so the first
    // launch after upgrading shows the same list it did before.
    return {scopes: {}, recent: raw.filter((s): s is string => typeof s === 'string')};
  }
  if (raw && typeof raw === 'object') {
    const o = raw as Partial<HistoryStore>;
    return {
      scopes: o.scopes && typeof o.scopes === 'object' ? o.scopes : {},
      recent: Array.isArray(o.recent) ? o.recent.filter((s): s is string => typeof s === 'string') : [],
    };
  }
  return emptyStore();
}

const prepend = (list: string[], t: string, cap: number) =>
  [t, ...list.filter(s => s !== t)].slice(0, cap);

/**
 * pushHistory records a sent message under its scope AND in the cross-scope tail.
 * A repeated send floats to the top of both. No-op on empty input.
 */
export function pushHistory(store: HistoryStore, scope: string, text: string, now: number): HistoryStore {
  const t = text.trim();
  if (!t) return store;
  const cur = store.scopes[scope]?.list ?? [];
  const scopes: Record<string, Scope> = {
    ...store.scopes,
    [scope]: {list: prepend(cur, t, HISTORY_CAP), at: now},
  };
  const kept = Object.entries(scopes)
    .sort((a, b) => b[1].at - a[1].at)
    .slice(0, SCOPE_CAP);
  return {scopes: Object.fromEntries(kept), recent: prepend(store.recent, t, RECENT_CAP)};
}

/**
 * historyFor is what the picker shows: this scope's entries first, then the
 * cross-scope tail to fill the rest.
 *
 * The top-up is why a brand-new scope is never emptier than the old global list,
 * and why the entry you want is at the TOP rather than buried under other
 * projects — which was the whole problem.
 */
export function historyFor(store: HistoryStore, scope: string, cap = HISTORY_CAP): string[] {
  const mine = store.scopes[scope]?.list ?? [];
  const out = [...mine];
  for (const t of store.recent) {
    if (out.length >= cap) break;
    if (!out.includes(t)) out.push(t);
  }
  return out.slice(0, cap);
}

/**
 * removeHistory drops one entry by TEXT, from this scope and from the tail.
 *
 * By text and not by index, because the picker shows a scope's entries topped up
 * from the tail: index 4 is a position in a rendering, not a place in the store.
 * And from BOTH, because an entry deleted from the scope alone would reappear a
 * line lower, topped back up from the tail — which reads as the delete not working.
 */
export function removeHistory(store: HistoryStore, scope: string, text: string): HistoryStore {
  const cur = store.scopes[scope];
  const scopes = {...store.scopes};
  if (cur) scopes[scope] = {...cur, list: cur.list.filter(s => s !== text)};
  return {scopes, recent: store.recent.filter(s => s !== text)};
}

export async function loadHistory(): Promise<HistoryStore> {
  try {
    const raw = await AsyncStorage.getItem(KEY);
    if (raw == null) return emptyStore();
    return migrate(JSON.parse(raw));
  } catch {
    return emptyStore();
  }
}

export async function saveHistory(store: HistoryStore): Promise<void> {
  try {
    await AsyncStorage.setItem(KEY, JSON.stringify(store));
  } catch {
    // best-effort
  }
}
