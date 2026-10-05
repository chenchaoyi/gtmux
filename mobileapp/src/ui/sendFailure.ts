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

export type FailureKind = 'draft' | 'gone' | 'key' | 'unconfirmed' | 'refused' | 'unknown';

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
  // The Mac refused THIS PHONE (its token was revoked, or is wrong): no retry can land,
  // only pairing again. Read off the status, which is the server's verdict; the body
  // of a 401 is not one of the refusals below.
  if (status === 401 || status === 403) return 'refused';
  const r = (reason || '').toLowerCase();
  if (r.includes('unsent text')) return 'draft';
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
 * The agent queues it behind the current turn. The server does not report that — the
 * phone's send path never runs the queued detection — so this is read off the target's
 * status, which the radar already knows. It is an inference and is worded as one: it says
 * what will happen, not that it has been observed.
 */
export function busyNote(status: string | undefined, zh: boolean): string {
  if (status !== 'working') return '';
  return zh ? '已送出。它正在跑，这条会排在这一轮之后' : 'Sent. It is running, so this will be handled after the current turn';
}
