// What HQ dispatched and whether it is still running (chat-background-tasks).
//
// The dispatch ledger says what was sent, to whom and when; the radar says whether that
// pane is waiting, working or idle now. serve joins them, and this file holds the shapes
// and the reading that the chat's row and sheet both rest on, so neither one decides for
// itself what "running" means.

/** A task's pane state. `gone` means the pane no longer exists; the task still happened. */
export type TaskStatus = 'waiting' | 'working' | 'idle' | 'gone';

export type BackgroundTask = {
  id: string;
  goal: string;
  agent: string;
  pane: string;
  status: TaskStatus;
  /** unix seconds, when it was dispatched */
  since: number;
};

export function isTaskStatus(x: unknown): x is TaskStatus {
  return x === 'waiting' || x === 'working' || x === 'idle' || x === 'gone';
}

/**
 * Running means the pane is still busy with it: WAITING counts, because a task waiting on
 * you has not finished — it has stopped and is asking. Rolling it into "finished" would
 * hide the one state that needs a person, which is the whole reason the row exists.
 */
export function isRunning(t: BackgroundTask): boolean {
  return t.status === 'waiting' || t.status === 'working';
}

/** The tally the row above the composer reads. */
export type RunningTally = {running: number; waiting: number; finished: number};

export function tally(tasks: BackgroundTask[]): RunningTally {
  let running = 0;
  let waiting = 0;
  let finished = 0;
  for (const t of tasks) {
    if (isRunning(t)) {
      running++;
      if (t.status === 'waiting') waiting++;
    } else {
      finished++;
    }
  }
  return {running, waiting, finished};
}

/** The row is rendered only when something is actually in flight. */
export function showRow(t: RunningTally): boolean {
  return t.running > 0;
}

/**
 * The row's words. Waiting is said SEPARATELY and first, because it is the part that needs
 * a person; the rest is said as a remainder so the number still adds up. When nothing is
 * waiting there is one clause and no colour claim beyond "working".
 */
export function rowText(t: RunningTally, zh: boolean): {lead: string; rest: string} {
  if (t.waiting > 0) {
    const lead = zh ? `${t.waiting} 个在等你输入` : `${t.waiting} needs you`;
    const others = t.running - t.waiting;
    if (others <= 0) return {lead, rest: ''};
    return {lead, rest: zh ? ` · 另外 ${others} 个在跑` : ` · ${others} more running`};
  }
  return {lead: zh ? `${t.running} 个在跑` : `${t.running} running`, rest: ''};
}

/** Which state colour the row wears: the most urgent state present, never "tappable". */
export function rowState(t: RunningTally): 'waiting' | 'working' {
  return t.waiting > 0 ? 'waiting' : 'working';
}

/**
 * Sort for the sheet: what needs you first, then what is working, then what is done, each
 * newest first. A gone task sorts with the finished — there is nothing to go to.
 */
const RANK: Record<TaskStatus, number> = {waiting: 0, working: 1, idle: 2, gone: 3};

export function ordered(tasks: BackgroundTask[]): BackgroundTask[] {
  return [...tasks].sort((a, b) => {
    const r = RANK[a.status] - RANK[b.status];
    return r !== 0 ? r : b.since - a.since;
  });
}

/** Tapping a row goes to its pane; a task whose pane is gone has nowhere to go. */
export function canOpen(t: BackgroundTask): boolean {
  return t.status !== 'gone' && t.pane !== '';
}

/** The second line under a goal: which agent has it, which pane, and how long. */
export function rowDetail(t: BackgroundTask, nowSec: number, zh: boolean, age: (s: number) => string): string {
  const where = [t.agent, t.pane].filter(Boolean).join(' · ');
  const elapsed = t.since > 0 ? age(Math.max(0, nowSec - t.since)) : '';
  const state = statusWord(t.status, zh);
  return [where, elapsed ? `${state} ${elapsed}` : state].filter(Boolean).join(' · ');
}

export function statusWord(s: TaskStatus, zh: boolean): string {
  switch (s) {
    case 'waiting':
      return zh ? '等你输入' : 'needs you';
    case 'working':
      return zh ? '在干' : 'working';
    case 'idle':
      return zh ? '干完了' : 'done';
    default:
      return zh ? '会话已关' : 'pane closed';
  }
}

/**
 * How long, as a duration rather than a point in time: the row says how long this has been
 * going, not when it started. Minutes up to an hour, then hours, then days — the reader is
 * deciding whether to go look, and a second's precision never changes that answer.
 */
export function elapsed(secs: number): string {
  if (secs < 60) return '<1m';
  if (secs < 3600) return `${Math.floor(secs / 60)}m`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}h`;
  return `${Math.floor(secs / 86400)}d`;
}
