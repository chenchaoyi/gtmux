// rowSheetModel — what a long-press on a radar row should say, as data.
//
// Kept apart from the rendering because the interesting part is not the layout: it is
// that the radar carries FOUR different kinds of thing (an agent in tmux, a session
// sensed outside tmux, a plain pane someone promoted onto the list, a session that ended
// on an error) and the alert this replaces gave all four the same two lines — the agent
// name and the task, both already on the row that was pressed. For two of those kinds
// that is not merely terse, it is wrong: a sensed session cannot be jumped to, and a
// plain pane has no agent status to report.

import {Agent, primary} from '../api/types';
import {Lang, statusLabel} from '../i18n';

// The actions are what you can DO to a session from the list, ordered by how often you
// would want to. The first set this sheet shipped with was ranked the other way round:
// `open` (which is just tapping the row), `copy the jump command` (paste it where? you
// are holding a phone, away from the Mac), `jump on the Mac` (same), and `see the
// changes` — three of four worth nothing to someone standing in a kitchen.
//
// What a phone is actually for here is unblocking: a session that is waiting wants an
// answer, and every session sometimes wants "carry on" or "stop". Those are one tap away
// now instead of two screens.
export type SheetActionKey = 'reply' | 'continue' | 'stop' | 'ask-hq' | 'diff' | 'jump';

export type SheetGroup = 'answer' | 'drive' | 'look';

export interface SheetAction {
  key: SheetActionKey;
  group: SheetGroup;
  title: string;
  sub: string;
  // A disabled action still appears, WITH its reason. Hiding it would leave the reader
  // wondering whether they missed it; offering it broken is worse than both.
  disabled?: boolean;
}

export interface RowSheetModel {
  kind: 'agent' | 'native' | 'watched';
  title: string;
  // Absent for a watched plain pane: it carries no agent status, and rendering "idle"
  // for it would invent one.
  status?: string;
  identity: string[];
  task?: string;
  error?: string;
  background?: string;
  // True when this session is blocked on the user. The sheet then fetches the pane's
  // numbered choices and leads with them: the options ARE the question in the form the
  // reader can act on, and the radar row carries no ask text of its own (that field
  // belongs to the digest, not to /api/agents).
  blocked?: boolean;
  actions: SheetAction[];
}

// The command a pane id turns into — the same shape the pane browser, the menu bar and
// the web already offer, since the point of surfacing `%N` is that the token means the
// same thing everywhere.
export function focusCommand(paneID: string): string {
  return `gtmux focus ${paneID}`;
}

export function humanSince(since: number | undefined, nowSecs: number, lang: Lang): string {
  if (!since) return '';
  const s = Math.max(0, nowSecs - since);
  const zh = lang === 'zh';
  if (s < 60) return zh ? `${s} 秒` : `${s}s`;
  if (s < 3600) return zh ? `${Math.floor(s / 60)} 分钟` : `${Math.floor(s / 60)}m`;
  if (s < 86400) return zh ? `${Math.floor(s / 3600)} 小时` : `${Math.floor(s / 3600)}h`;
  return zh ? `${Math.floor(s / 86400)} 天` : `${Math.floor(s / 86400)}d`;
}

export function buildRowSheet(a: Agent, lang: Lang, nowSecs: number): RowSheetModel {
  const zh = lang === 'zh';
  const native = a.source === 'native';
  const watched = !!a.watched;
  const kind: RowSheetModel['kind'] = watched ? 'watched' : native ? 'native' : 'agent';

  // Identity leads with the pane id wherever there is one. Names are a gloss and the id
  // is the anchor: two panes running one project are indistinguishable once a row
  // truncates their titles, which is the reason the share picker leads with `%N` too.
  const identity: string[] = [];
  if (native) {
    identity.push(zh ? '不在 tmux 里（只能感知，不能操作）' : 'Not in tmux (sensed only, not controllable)');
  } else {
    identity.push(a.loc ? `${a.pane_id} · ${a.loc}` : a.pane_id);
  }
  if (a.project || a.branch) {
    identity.push([a.project, a.branch].filter(Boolean).join(' · '));
  }
  if (a.terminal) {
    identity.push(zh ? `终端 ${a.terminal}` : `Terminal ${a.terminal}`);
  }

  const dur = humanSince(a.since || a.activity_at, nowSecs, lang);
  let status: string | undefined;
  if (!watched) {
    status = statusLabel(a.status, lang);
    if (a.error) status = zh ? '出错' : 'errored';
    if (dur) status += ` · ${dur}`;
  }

  const actions: SheetAction[] = [];
  const waiting = a.status === 'waiting';

  // ANSWER — only when there is something to answer. A session blocked on you is the one
  // case where the phone can finish the job rather than route you to the Mac.
  if (waiting && !native) {
    actions.push({
      key: 'reply',
      group: 'answer',
      title: zh ? '回答它' : 'Answer it',
      sub: zh ? '直接选一个，或写一句' : 'Pick an option, or write a line',
    });
  }

  // DRIVE — the two things anyone says to an agent most often.
  if (!native && !watched) {
    actions.push({
      key: 'continue',
      group: 'drive',
      title: zh ? '继续' : 'Carry on',
      sub: zh ? '让它接着做' : 'Tell it to keep going',
    });
    actions.push({
      key: 'stop',
      group: 'drive',
      title: zh ? '停下' : 'Stop it',
      sub: zh ? '打断当前这一轮' : 'Interrupt the current turn',
    });
    actions.push({
      key: 'ask-hq',
      group: 'drive',
      title: zh ? '问参谋长' : 'Ask the supervisor',
      sub: zh ? '让中控看一眼再替你回' : 'Have HQ look at it and answer for you',
    });
  }

  // LOOK — the read-only ones, last. `jump` stays because it is the one action that
  // matters when you ARE at the Mac; it is simply not the common case.
  if (a.branch) {
    actions.push({
      key: 'diff',
      group: 'look',
      title: zh ? '看改动' : 'See the changes',
      sub: zh ? `${a.branch} 上未提交的改动` : `Uncommitted changes on ${a.branch}`,
    });
  }
  actions.push({
    key: 'jump',
    group: 'look',
    title: zh ? '在 Mac 上跳过去' : 'Jump to it on the Mac',
    sub: native
      ? (zh ? '这个会话不在 tmux 里，gtmux 无法切过去' : 'This session is not in tmux, so gtmux cannot switch to it')
      : (zh ? '把 Mac 的终端切到这个 pane' : "Move the Mac's terminal to this pane"),
    disabled: native,
  });

  return {
    kind,
    title: watched ? (a.task || a.pane_id) : (a.agent || (zh ? 'agent' : 'Agent')),
    status,
    identity,
    task: watched ? undefined : primary(a),
    error: a.error && a.error_text ? a.error_text : undefined,
    background: a.bg && a.bg_text ? a.bg_text : undefined,
    blocked: a.status === 'waiting' && a.source !== 'native',
    actions,
  };
}
