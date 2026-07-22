// hqZones — the HQ command page's pure logic (hq-command-page), kept out of the view so
// it can be tested without a renderer.
//
// The page exists to answer what the RADAR cannot: what does the supervisor think is
// going on, what is actually blocked on me, and what has happened. Everything here
// derives those three answers deterministically from `/api/digest` and
// `/api/hq/{board,events}` — no LLM, no guessing.

import {DigestRow, HQEvent} from '../api/client';

// Zone is which body the page is showing. The command bar spans all three.
export type Zone = 'calls' | 'activity' | 'console';

// workerRows drops the supervisor: HQ is the meta-layer and must never appear as one
// more session inside its own page.
export function workerRows(digest: DigestRow[]): DigestRow[] {
  return digest.filter(r => r.role !== 'supervisor');
}

// decisions are the sessions actually blocked on the user, oldest-waiting first — the
// one who has been stuck longest is the one to unblock.
export function decisions(digest: DigestRow[]): DigestRow[] {
  return workerRows(digest)
    .filter(r => r.status === 'waiting')
    .sort((a, b) => (a.since ?? 0) - (b.since ?? 0));
}

export function fleetCounts(digest: DigestRow[]): {waiting: number; working: number; idle: number} {
  const rows = workerRows(digest);
  const n = (s: string) => rows.filter(r => r.status === s).length;
  return {waiting: n('waiting'), working: n('working'), idle: n('idle')};
}

// sessionName is the human handle for a row: the session part of `session:window.pane`.
export function sessionName(row: DigestRow): string {
  const loc = row.loc ?? '';
  return loc.split(':')[0] || loc || row.pane_id || row.agent;
}

// windowNo is the tmux window number ("" when the locator doesn't carry one) — the
// handle you actually type at a keyboard, worth showing next to the name.
export function windowNo(row: DigestRow): string {
  const wp = (row.loc ?? '').split(':')[1];
  return wp ? wp.split('.')[0] : '';
}

// assessment is the page's headline: the deterministic chief-of-staff conclusion. It is
// single-source in SPIRIT with the HQ card's fleetHeadline (same rule, digest rows rather
// than agent rows), so the card you tapped and the page you land on cannot disagree.
export function assessment(digest: DigestRow[], zh: boolean): string {
  const workers = workerRows(digest);
  const waiting = decisions(digest);
  if (workers.length === 0) return zh ? '暂无其它 agent 会话' : 'no other agent sessions';
  if (waiting.length === 0) return zh ? '都正常 · 无需你介入' : 'all normal — nothing needs you';
  const name = sessionName(waiting[0]);
  if (waiting.length === 1) {
    const rest = workers.length - 1;
    if (rest > 0) {
      return zh ? `${name} 在等你拍板 · 其余 ${rest} 个正常` : `${name} needs you · ${rest} others normal`;
    }
    return zh ? `${name} 在等你拍板` : `${name} needs you`;
  }
  return zh ? `${waiting.length} 个会话在等你拍板` : `${waiting.length} sessions need you`;
}

// askOf is what a decision card puts in its body: the pending question when the hook
// captured one, else the goal, else an honest "we know it's blocked but not on what" —
// never an empty card.
export function askOf(row: DigestRow, zh: boolean): string {
  if (row.ask) return row.ask;
  if (row.goal) return row.goal;
  return zh ? '（没抓到具体问题，打开会话看看）' : '(no question captured — open the session)';
}

// eventPhrase renders one ledger record as a human clause. The ledger's own `Format` is
// a terminal log line (`HH:MM:SS state·kind loc agent (event)`); a phone reader needs
// prose, and the record already carries everything needed to write it deterministically.
export function eventPhrase(e: HQEvent, zh: boolean): string {
  switch (e.event) {
    case 'Waiting':
      if (e.kind) {
        const k = zh
          ? {permission: '要授权', plan: '要你看方案', question: '有问题问你'}[e.kind] ?? '在等你'
          : {permission: 'wants permission', plan: 'wants a plan reviewed', question: 'has a question'}[e.kind] ??
            'is waiting';
        return k;
      }
      return zh ? '在等你' : 'is waiting';
    case 'Stop':
      if (e.class === 'asking') return zh ? '问了你一个问题' : 'ended on a question';
      return zh ? '跑完一轮' : 'finished a turn';
    case 'StopFailure':
      return zh ? '这一轮崩了' : 'a turn crashed';
    case 'UserPromptSubmit':
      return e.origin === 'instruction' ? (zh ? '收到指令' : 'got an instruction') : zh ? '收到输入' : 'got input';
    case 'SessionStart':
      return zh ? '新会话' : 'session started';
    case 'SessionEnd':
      return zh ? '会话结束' : 'session ended';
    case 'Resumed':
      return zh ? '接回会话' : 'session resumed';
    case 'PreCompact':
      return zh ? '上下文压缩' : 'context compacted';
    default:
      return e.event;
  }
}

// eventSession is the handle shown on a feed line.
export function eventSession(e: HQEvent): string {
  return e.session || (e.loc ?? '').split(':')[0] || e.pane || e.agent || '';
}

// hasNewActivity reports whether the feed's newest record is one the user hasn't seen,
// so the activity tab can carry a dot while they're on another zone. Compared by `seq`
// (the ledger's total order); a ledger without seq falls back to the timestamp.
export function hasNewActivity(events: HQEvent[], seenMark: number): boolean {
  if (events.length === 0) return false;
  return eventMark(events[0]) > seenMark;
}

export function eventMark(e: HQEvent): number {
  return e.seq && e.seq > 0 ? e.seq : e.ts;
}

// initialZone: opening HQ while a session is blocked means the block is why you opened
// it. Otherwise the console — the thing you came to talk to.
export function initialZone(digest: DigestRow[]): Zone {
  return decisions(digest).length > 0 ? 'calls' : 'console';
}

// relTime is the compact "since" label used across the page (40s / 4m / 1h / 2d).
export function relTime(since: number | undefined, nowSecs: number): string {
  if (!since) return '';
  const s = Math.max(0, Math.floor(nowSecs) - since);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

// boardFreshness labels how old the supervisor's assessment is. A board is a synthesis a
// person maintains, so its AGE is part of how much to trust it — an hours-old board is
// worth reading with that in mind, and the label says so rather than hiding it.
export function boardFreshness(updatedAt: number | undefined, nowSecs: number, zh: boolean): string {
  if (!updatedAt) return zh ? '态势板' : 'situation board';
  const ago = relTime(updatedAt, nowSecs);
  return zh ? `态势板 · ${ago}前` : `situation board · ${ago} ago`;
}

// planLabel compacts a usage-window label for the status strip: "week (all models)" →
// wk/周, "week (fable)" → the model name, "session" → 5h.
export function planLabel(label: string, zh: boolean): string {
  if (label.includes('all models')) return zh ? '周' : 'wk';
  const m = label.match(/\(([^)]+)\)/);
  if (m) return m[1].charAt(0).toUpperCase() + m[1].slice(1);
  if (label.startsWith('session')) return '5h';
  return label;
}
