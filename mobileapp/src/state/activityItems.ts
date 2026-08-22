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

// The rows the lock screen is ABOUT: your agents. It excludes the same things the radar
// list excludes (ui/theme.ts) — the supervisor, a meta layer with its own card and not
// one of the workers, and a watched plain pane, which has no agent status at all.
//
// The Go push side has always filtered the supervisor out (events.go: Role !=
// "supervisor"); this side did not, so the card said different things depending on who
// wrote it last. Whenever the app was open it counted gtmux's own supervisor as work in
// progress, and the lock screen led with the supervisor's internal bookkeeping —
// "1 working · gtmux tick 序列" — while every session the user cared about sat idle.
export function isWorkerRow(a: Agent): boolean {
  return a.role !== 'supervisor' && !a.watched;
}

export function buildActivityItems(agents: Agent[]): {items: ActivityItem[]; more: number} {
  const byRecent = (a: Agent, b: Agent) => (b.since ?? 0) - (a.since ?? 0); // newest first
  const rows = agents.filter(isWorkerRow);
  const waiters = rows.filter(a => a.status === 'waiting').sort(byRecent);
  const workers = rows.filter(a => a.status === 'working').sort(byRecent);
  const ordered = [...waiters, ...workers];
  const items = ordered.slice(0, MAX_ITEMS).map(a => ({
    title: a.task || a.session || a.loc || a.agent,
    status: a.status,
    since: a.since ?? 0,
  }));
  const more = Math.max(0, waiters.length + workers.length - items.length);
  return {items, more};
}
