// Unsent composer drafts, kept per pane.
//
// A half-typed message used to die the moment you left the pane: the composer
// holds its text in local state, and going back to the radar unmounts the screen.
// So "check what the other one is doing, then finish this sentence" cost you the
// sentence — and the phone is where that happens most, because the reason you
// looked away is usually a notification.
//
// Keyed by PANE, never global. A draft is about the session it was being written
// to, and showing it in another pane's box would be worse than losing it: the
// composer types into a live terminal, so a stray draft is one Enter away from
// being sent somewhere it makes no sense. Each pane sees its own or nothing.
//
// Stored as ONE map rather than a key per pane, so the whole thing is read in one
// call and pruned in the same write. Panes are per-server sequence numbers that
// get reused after a tmux restart, which is also why old drafts must expire
// rather than wait for their pane to come back.

import AsyncStorage from '@react-native-async-storage/async-storage';

const KEY = 'gtmux.drafts';

/** How many panes may hold a draft. Beyond this the oldest are dropped. */
export const DRAFT_CAP = 20;

/** A draft older than this is not offered back — see the pane-id note above. */
export const DRAFT_TTL_MS = 7 * 24 * 60 * 60 * 1000;

export interface Draft {
  text: string;
  at: number; // epoch ms of the last keystroke
}

export type DraftMap = Record<string, Draft>;

/**
 * putDraft records (or clears) one pane's draft and prunes the map. Pure, so the
 * rules below are testable without a storage backend.
 *
 * Empty text REMOVES the entry rather than storing a blank one: an empty draft
 * and no draft are the same fact, and keeping it would let blanks push real
 * drafts out of the cap.
 */
export function putDraft(map: DraftMap, key: string, text: string, now: number): DraftMap {
  const next: DraftMap = {...map};
  if (text.trim() === '') delete next[key];
  else next[key] = {text, at: now};
  return prune(next, now);
}

/** prune drops expired drafts, then the oldest beyond the cap. */
export function prune(map: DraftMap, now: number): DraftMap {
  const live = Object.entries(map).filter(
    ([, d]) => d && typeof d.text === 'string' && now - d.at < DRAFT_TTL_MS,
  );
  live.sort((a, b) => b[1].at - a[1].at); // newest first
  return Object.fromEntries(live.slice(0, DRAFT_CAP));
}

/** getDraft returns a pane's live draft, or "" when it has none or it expired. */
export function getDraft(map: DraftMap, key: string, now: number): string {
  const d = map[key];
  if (!d || now - d.at >= DRAFT_TTL_MS) return '';
  return d.text;
}

export async function loadDrafts(): Promise<DraftMap> {
  try {
    const raw = await AsyncStorage.getItem(KEY);
    if (raw == null) return {};
    const o = JSON.parse(raw);
    return o && typeof o === 'object' && !Array.isArray(o) ? (o as DraftMap) : {};
  } catch {
    return {};
  }
}

export async function saveDrafts(map: DraftMap): Promise<void> {
  try {
    await AsyncStorage.setItem(KEY, JSON.stringify(map));
  } catch {
    // best-effort: a lost draft is the status quo, never an error to show
  }
}
