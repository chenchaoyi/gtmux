// Build the Live Activity's listed sessions from the agent set — the same shape +
// ordering the Go push side produces (internal/server/events.go topTallyItems), so
// the lock screen looks identical whether updated from the foreground (this) or a
// background push. Waiting first, then working, each most-recent-first, capped.

import {Agent} from '../api/types';

export interface ActivityItem {
  title: string;
  status: string; // waiting | working
  time: string; // compact relative time, e.g. "now" / "2m"
}

const MAX_ITEMS = 3;

// relTime → "now" (<60s), "Nm" (<1h), "Nh" (<1d), else "Nd". now/since in epoch
// SECONDS. "" when since is unknown.
export function relTime(nowSec: number, since?: number): string {
  if (!since || since <= 0) return '';
  const d = Math.max(0, nowSec - since);
  if (d < 60) return 'now';
  if (d < 3600) return `${Math.floor(d / 60)}m`;
  if (d < 86400) return `${Math.floor(d / 3600)}h`;
  return `${Math.floor(d / 86400)}d`;
}

export function buildActivityItems(agents: Agent[], nowSec: number): {items: ActivityItem[]; more: number} {
  const byRecent = (a: Agent, b: Agent) => (b.since ?? 0) - (a.since ?? 0); // newest first
  const waiters = agents.filter(a => a.status === 'waiting').sort(byRecent);
  const workers = agents.filter(a => a.status === 'working').sort(byRecent);
  const ordered = [...waiters, ...workers];
  const items = ordered.slice(0, MAX_ITEMS).map(a => ({
    title: a.task || a.session || a.loc || a.agent,
    status: a.status,
    time: relTime(nowSec, a.since),
  }));
  const more = Math.max(0, waiters.length + workers.length - items.length);
  return {items, more};
}
