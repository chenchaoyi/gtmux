// sendFailure — turning the server's refusal into something the reader can act on.
//
// The core refuses a send for reasons that call for different responses, and until this
// existed they all reached the phone as one bar reading "the input box didn't confirm".
// A reader seeing that has no way to tell "someone is typing in that pane right now" from
// "that session is gone", and both of those have an obvious next move that the generic
// sentence hides.
//
// The strings are the ones serve actually returns (internal/app/serve.go), wrapped by the
// handler as "send failed: <err>". They are matched loosely on their distinctive part so
// a reworded message degrades to the generic case instead of vanishing.

export type FailureKind = 'draft' | 'asking' | 'gone' | 'key' | 'unconfirmed' | 'refused' | 'not-shared' | 'unknown';

/** What the reader can do about it. */
export type FailureAction = 'send-anyway' | 'back-to-radar' | 'retry' | 'pair-again' | 'none';

export interface FailureCopy {
  /** The sentence in the bar. */
  title: string;
  /** The one action offered beside it, or 'none'. */
  action: FailureAction;
  /** Label for that action. "" when there is none. */
  actionLabel: string;
  /**
   * False for a failure the reader cannot cause and cannot fix — it goes to the log and
   * nowhere else. Interrupting someone with a fault in our own request is noise.
   */
  show: boolean;
}

export function classifySendFailure(reason: string, status?: number): FailureKind {
  // 401: the server refused THIS PHONE (its token was revoked, or is wrong); no retry can
  // land, only pairing again. 403 is not that: the token is good and this pane is not
  // open to it for typing (internal/server: "input not shared for this pane", or a share
  // gone stale), which pairing again would not change. Read off the status, the server's
  // verdict; neither body is one of the refusals below.
  if (status === 401) return 'refused';
  if (status === 403) return 'not-shared';
  const r = (reason || '').toLowerCase();
  if (r.includes('unsent text')) return 'draft';
  if (r.includes('is asking something')) return 'asking';
  if (r.includes('pane not found') || r.includes('no such pane')) return 'gone';
  if (r.includes('key not allowed')) return 'key';
  if (r.includes('not confirmed')) return 'unconfirmed';
  return 'unknown';
}

export function failureCopy(kind: FailureKind, zh: boolean): FailureCopy {
  switch (kind) {
    case 'draft':
      // No override is offered, and that is a correction to the candidate this was built
      // from: it proposed a "send anyway", and `POST /api/send` has no field to carry one.
      // The core's ClobberDraft exists, but only the CLI can reach it. Adding an API field
      // for it is a contract change, not a display change, so it is not smuggled in here.
      //
      // Retry is the honest action: the pane is busy with someone else's half-written
      // line, and a moment later it may not be.
      return {
        title: zh
          ? '未发送：窗格内有未提交内容，请先处理草稿后重试。'
          : 'Not sent: the pane contains an unsent draft. Resolve the draft and try again.',
        action: 'retry',
        actionLabel: zh ? '重试' : 'Retry',
        show: true,
      };
    case 'asking':
      // The pane is showing the agent's choice menu (a permission prompt, a question). It
      // used to read as someone typing, and a retry cannot get past it: answering it can.
      return {
        title: zh
          ? '未发送：会话正在等待输入或确认，请先处理。'
          : 'Not sent: the session is asking for input or approval. Resolve the request first.',
        action: 'none',
        actionLabel: '',
        show: true,
      };
    case 'gone':
      return {
        title: zh ? '未发送：目标窗格不存在。' : 'Not sent: the target pane no longer exists.',
        action: 'back-to-radar',
        actionLabel: zh ? '返回会话列表' : 'Back to sessions',
        show: true,
      };
    case 'refused':
      // Retry was offered here and could never work: every attempt is refused the same
      // way. The text goes back into the box (DetailView), so nothing is lost on the way to
      // pairing again (simulator, 2026-10-05).
      return {
        title: zh
          ? '未发送：服务器拒绝访问。内容已恢复到输入框，请重新配对。'
          : 'Not sent: the server denied access. Your text is back in the input box; pair again to send it.',
        action: 'pair-again',
        actionLabel: zh ? '重新配对' : 'Pair again',
        show: true,
      };
    case 'not-shared':
      // A guest whose typing into this pane was taken back, or a share gone stale: the
      // owner decides, and neither a retry nor pairing again changes it. The text goes
      // back into the box, as for a refusal.
      return {
        title: zh
          ? '未发送：当前连接没有此窗格的输入权限。内容已恢复到输入框。'
          : 'Not sent: this connection has no input access to the pane. Your text is back in the input box.',
        action: 'none',
        actionLabel: '',
        show: true,
      };
    case 'key':
      // A control key the server will never run. The reader cannot produce this: the app
      // chose the key. It belongs in the log.
      return {title: '', action: 'none', actionLabel: '', show: false};
    case 'unconfirmed':
      return {
        title: zh
          ? '发送结果未确认，请检查窗格后重试。'
          : "The full message was not confirmed. Check the pane before retrying.",
        action: 'retry',
        actionLabel: zh ? '重试' : 'Retry',
        show: true,
      };
    default:
      return {
        title: zh ? '发送失败' : 'Send failed',
        action: 'retry',
        actionLabel: zh ? '重试' : 'Retry',
        show: true,
      };
  }
}

/** An accepted text send, with the radar's current status; never a queue promise. */
export function busyNote(status: string | undefined, zh: boolean): string {
  if (status !== 'working') return '';
  return zh ? '已发送到终端，当前会话正在工作。' : 'Sent to terminal. The agent is working.';
}
