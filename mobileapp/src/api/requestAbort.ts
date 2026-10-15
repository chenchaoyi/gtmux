// RN fetch implementations need not preserve AbortSignal.reason. Mark our own
// deadlines before aborting; an unmarked caller abort is lifecycle cancellation.
const deadlines = new WeakSet<AbortSignal>();

export function abortAtDeadline(controller: AbortController): void {
  if (controller.signal.aborted) return;
  deadlines.add(controller.signal);
  controller.abort();
}
export function requestAbortKind(signal?: AbortSignal | null): 'timeout' | 'cancelled' | undefined {
  return signal?.aborted ? deadlines.has(signal) ? 'timeout' : 'cancelled' : undefined;
}
