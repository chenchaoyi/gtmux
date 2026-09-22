// withDeadline stops WAITING for a promise after ms, answering `onTimeout` instead. It
// does not cancel the work behind the promise; use an AbortController for that where the
// work is yours to cancel.
//
// Pairing waited on three requests with no bound but iOS's own idle timeout, about a
// minute each. On 2026-09-22 a VPN on the phone swallowed them, and the scan spun for
// long enough that it read as forever, with nothing to say what was wrong.
export function withDeadline<T>(p: Promise<T>, ms: number, onTimeout: T): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const t = setTimeout(() => resolve(onTimeout), ms);
    p.then(
      v => {
        clearTimeout(t);
        resolve(v);
      },
      e => {
        clearTimeout(t);
        reject(e);
      },
    );
  });
}

/** How long each pairing step may wait for an answer before it counts as unreachable. */
export const PAIR_STEP_MS = 15_000;

/** What checking a server before saving it found. */
export type ServerCheck = 'ok' | 'unreachable' | 'rejected';

/**
 * checkServer asks a server whether it is there and whether it takes the token, each step
 * bounded. Only the server refusing the token (401/403) is REJECTED. Anything else that
 * stops the authed call, a step that never answers, a dropped connection or an edge's
 * 502, is UNREACHABLE: "token rejected" there sends the reader to the wrong fix, and it
 * was the answer every such failure used to produce.
 */
export async function checkServer(
  client: {health(): Promise<boolean>; agents(): Promise<unknown>},
  stepMs: number = PAIR_STEP_MS,
): Promise<ServerCheck> {
  if (!(await withDeadline(client.health(), stepMs, false))) {
    return 'unreachable';
  }
  const authed = await withDeadline(
    client.agents().then(
      () => 'ok' as const,
      (e: unknown) => (isAuthRefusal(e) ? ('rejected' as const) : ('unreachable' as const)),
    ),
    stepMs,
    'unreachable' as const,
  );
  return authed;
}

/** isAuthRefusal: the server answered and refused the credential (ApiError 401/403). */
function isAuthRefusal(e: unknown): boolean {
  const status = (e as {status?: unknown} | null)?.status;
  return status === 401 || status === 403;
}
