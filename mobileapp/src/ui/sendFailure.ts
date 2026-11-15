import type {SendFailureCode} from '../api/sendFailureCode';

export type FailureKind = 'draft' | 'asking' | 'unavailable' | 'gone' | 'key' | 'unconfirmed' | 'refused' | 'not-shared' | 'unknown';
export type FailureAction = 'send-anyway' | 'back-to-radar' | 'retry' | 'pair-again' | 'none';
export interface FailureCopy {title: string; action: FailureAction; actionLabel: string; show: boolean}

/** A refusal describes the last send, never a durable claim about current agent state. */
export function classifySendFailure(reason: string, status?: number, code?: SendFailureCode): FailureKind {
  if (status === 401) return 'refused';
  if (status === 403) return 'not-shared';
  switch (code) {
    case 'input-awaiting-confirmation': return 'asking';
    case 'draft-present': return 'draft';
    case 'outcome-unconfirmed':
    case 'input-unconfirmed': return 'unconfirmed';
    case 'input-state-unavailable':
    case 'conversation-unavailable':
    case 'target-changed':
    case 'session-not-loaded': return 'unavailable';
  }
  const r = (reason || '').toLowerCase();
  if (r.includes('unsent text')) return 'draft';
  // Older servers also used this sentence for unknown native input state.
  if (r.includes('is asking something')) return 'unavailable';
  if (r.includes('pane not found') || r.includes('no such pane')) return 'gone';
  if (r.includes('key not allowed')) return 'key';
  if (r.includes('not confirmed') || r.includes('outcome unconfirmed') || r.includes('status unknown')) return 'unconfirmed';
  return 'unknown';
}

export function failureCopy(kind: FailureKind, zh: boolean): FailureCopy {
  const retry = (title: string): FailureCopy => ({title, action: 'retry', actionLabel: zh ? '重试' : 'Retry', show: true});
  switch (kind) {
    case 'draft': return retry(zh ? 'Mac 上有未提交草稿，处理后重试。' : 'Unsent draft on Mac. Resolve it and try again.');
    case 'asking': return retry(zh ? '上次发送时有问题待回答，请处理后重试。' : 'The last send needed a reply. Answer, then retry.');
    case 'unavailable': return retry(zh ? '未发送，请查看会话后重试。' : 'Not sent. Check the session, then retry.');
    case 'gone': return {title: zh ? '未发送：目标窗格不存在。' : 'Not sent: the target pane no longer exists.', action: 'back-to-radar', actionLabel: zh ? '返回会话' : 'Sessions', show: true};
    case 'refused': return {title: zh ? '内容已恢复到输入框，请重新配对。' : 'Your text is back in the input box; pair again.', action: 'pair-again', actionLabel: zh ? '重新配对' : 'Pair again', show: true};
    case 'not-shared': return {title: zh ? '没有此窗格的输入权限，内容已恢复到输入框。' : 'No input access. Your text is back in the input box.', action: 'none', actionLabel: '', show: true};
    case 'key': return {title: '', action: 'none', actionLabel: '', show: false};
    case 'unconfirmed': return retry(zh ? '发送结果未确认，请先查看会话。' : 'Send not confirmed. Check the session before retrying.');
    default: return retry(zh ? '发送失败，内容已保留。' : 'Send failed. Your message is kept.');
  }
}

/** An accepted text send, with the radar's current status; never a queue promise. */
export function busyNote(status: string | undefined, zh: boolean): string {
  if (status !== 'working') return '';
  return zh ? '已发送到终端，当前会话正在工作。' : 'Sent to terminal. The agent is working.';
}
