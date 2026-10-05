import {ApiError} from '../api/client';

// shareWriteFailure — what the Sharing & pairing page says when a change did not take.
//
// Every write on that page failed in silence: a refusal came back as a bare `false`, a
// request nothing answered as an exception nobody caught, and the page then re-read the
// Mac, so a switch slid back to where it was with nothing said (simulator, 2026-10-05).
// The re-read stays: it is what keeps the switches honest. This adds the sentence, and
// keeps apart the two failures that call for different moves: one worth trying again,
// one that no retry will ever fix.

export type ShareWriteFailure =
  /** Nothing answered: the network, the tunnel, the Mac asleep. Worth another try. */
  | 'unreachable'
  /** 401/403: this phone's access was revoked or its token is wrong. Pairing again is the way back. */
  | 'refused'
  /** The Mac answered and turned this one change down. */
  | 'rejected';

export function classifyShareWriteFailure(e: unknown): ShareWriteFailure {
  if (e instanceof ApiError) return e.isAuth ? 'refused' : 'rejected';
  return 'unreachable';
}

export function shareWriteFailureText(kind: ShareWriteFailure, zh: boolean): {text: string; retry: boolean} {
  switch (kind) {
    case 'unreachable':
      return {
        text: zh ? '没连上这台 Mac，这次改动没有生效。' : "Couldn't reach the Mac, so the change didn't take.",
        retry: true,
      };
    case 'refused':
      return {
        text: zh
          ? '这台 Mac 拒绝了这部手机的访问，改动没有生效。重新配对后再改。'
          : 'The Mac refused this phone, so the change didn\'t take. Pair again to change sharing.',
        retry: false,
      };
    default:
      return {
        text: zh ? 'Mac 没有接受这次改动；下面显示的是它现在的实际设置。' : "The Mac didn't accept the change. What you see below is how it is set now.",
        retry: false,
      };
  }
}
