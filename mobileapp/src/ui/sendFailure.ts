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
  // 401: the Mac refused THIS PHONE (its token was revoked, or is wrong); no retry can
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
          ? '没发出去。那个窗格里有人正在打字，等一下再试，或去 Mac 上发'
          : 'Not sent. Someone is typing in that pane, so try again in a moment, or send from the Mac',
        action: 'retry',
        actionLabel: zh ? '重试' : 'Retry',
        show: true,
      };
    case 'asking':
      // The pane is showing the agent's choice menu (a permission prompt, a question). It
      // used to read as someone typing, and a retry cannot get past it: answering it can.
      return {
        title: zh
          ? '没发出去：它正在问你问题（权限提示或提问），先回答它'
          : 'Not sent: it is asking you something (a permission prompt or a question). Answer it first',
        action: 'none',
        actionLabel: '',
        show: true,
      };
    case 'gone':
      return {
        title: zh ? '没发出去。这个会话已经不在了' : 'Not sent. That session is gone',
        action: 'back-to-radar',
        actionLabel: zh ? '回雷达' : 'Back to radar',
        show: true,
      };
    case 'refused':
      // Retry was offered here and could never work: every attempt is refused the same
      // way. The text goes back into the box (DetailView), so nothing is lost on the way to
      // pairing again (simulator, 2026-10-05).
      return {
        title: zh
          ? '没发出去：这台 Mac 拒绝了这部手机。文字已放回输入框，重新配对后再发'
          : 'Not sent: this Mac refused this phone. Your text is back in the box; pair again to send it',
        action: 'pair-again',
        actionLabel: zh ? '去配对' : 'Pair again',
        show: true,
      };
    case 'not-shared':
      // A guest whose typing into this pane was taken back, or a share gone stale: the
      // owner decides, and neither a retry nor pairing again changes it. The text goes
      // back into the box, as for a refusal.
      return {
        title: zh
          ? '没发出去：这个窗格没有对这个连接开放输入。文字已放回输入框'
          : 'Not sent: typing into this pane is not shared with this connection. Your text is back in the box',
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
          ? '没发出去。输入框没有确认收到完整内容'
          : "Not sent. The input box didn't confirm the full message",
        action: 'retry',
        actionLabel: zh ? '重发' : 'Retry',
        show: true,
      };
    default:
      return {
        title: zh ? '没发出去' : 'Not sent',
        action: 'retry',
        actionLabel: zh ? '重发' : 'Retry',
        show: true,
      };
  }
}

/**
 * busyNote is what to say when the send DID land in a session that is mid-turn.
 *
 * The agent takes it when it is ready, which may be only once it finishes what it is
 * doing. Nothing on this path says when: the server does not report it (the phone's send
 * never runs the queued detection), and agents differ on whether a message waits for the
 * turn to end. So this is read off the target's status, which the radar already knows, and
 * says no more than that: not "queued", not "after the current turn". The web composer
 * says the same sentence.
 */
export function busyNote(status: string | undefined, zh: boolean): string {
  if (status !== 'working') return '';
  return zh ? '已送出。它正在忙，可能要等手头的事做完才会处理这条' : 'Sent. It is working, so it may only get to this once it finishes what it is doing';
}
