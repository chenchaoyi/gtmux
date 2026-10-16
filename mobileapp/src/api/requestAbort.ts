// RN fetch implementations need not preserve AbortSignal.reason. Mark our own
// deadlines before aborting; an unmarked caller abort is lifecycle cancellation.
const deadlines = new WeakMap<AbortSignal, number | undefined>();

export function abortAtDeadline(controller: AbortController, timeoutMs?: number): void {
  if (controller.signal.aborted) return;
  deadlines.set(controller.signal, timeoutMs);
  controller.abort();
}
export function requestAbortKind(signal?: AbortSignal | null): 'timeout' | 'cancelled' | undefined {
  return signal?.aborted ? deadlines.has(signal) ? 'timeout' : 'cancelled' : undefined;
}

/** Configured deadline, separate from wall time including a suspended JS runtime. */
export function requestTimeoutMs(signal?: AbortSignal | null): number | undefined {
  return signal ? deadlines.get(signal) : undefined;
}
