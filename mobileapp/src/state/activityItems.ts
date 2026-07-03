// Build the Live Activity's listed sessions from the agent set — the same shape +
// ordering the Go push side produces (internal/server/events.go topTallyItems), so
// the lock screen looks identical whether updated from the foreground (this) or a
// background push. Waiting first, then working, each most-recent-first, capped.

import {Agent} from '../api/types';

export interface ActivityItem {
  title: string;
  status: string; // waiting | working
  since: number; // epoch seconds the state started; 0 if unknown. The Live
  // Activity widget renders the relative time LOCALLY from this (auto-updating on
  // the lock screen), so a clock tick isn't a change that needs a fresh push.
}

const MAX_ITEMS = 3;

export function buildActivityItems(agents: Agent[]): {items: ActivityItem[]; more: number} {
  const byRecent = (a: Agent, b: Agent) => (b.since ?? 0) - (a.since ?? 0); // newest first
  const waiters = agents.filter(a => a.status === 'waiting').sort(byRecent);
  const workers = agents.filter(a => a.status === 'working').sort(byRecent);
  const ordered = [...waiters, ...workers];
  const items = ordered.slice(0, MAX_ITEMS).map(a => ({
    title: a.task || a.session || a.loc || a.agent,
    status: a.status,
    since: a.since ?? 0,
  }));
  const more = Math.max(0, waiters.length + workers.length - items.length);
  return {items, more};
}
