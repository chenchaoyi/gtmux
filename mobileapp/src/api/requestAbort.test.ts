import {abortAtDeadline, requestAbortKind, requestTimeoutMs} from './requestAbort';

test('only an observed signal abort is cancellation; marked deadlines remain timeouts', () => {
  expect(requestAbortKind()).toBeUndefined();
  const active = new AbortController();
  expect(requestAbortKind(active.signal)).toBeUndefined();
  active.abort();
  expect(requestAbortKind(active.signal)).toBe('cancelled');
  // A late timer cannot turn an already-cancelled read into a timeout.
  abortAtDeadline(active);
  expect(requestAbortKind(active.signal)).toBe('cancelled');
  const deadline = new AbortController();
  abortAtDeadline(deadline);
  expect(requestAbortKind(deadline.signal)).toBe('timeout');
});

test('records the configured deadline independently of delayed JS execution', () => {
  const request = new AbortController();
  abortAtDeadline(request, 15_000);
  expect(requestAbortKind(request.signal)).toBe('timeout');
  expect(requestTimeoutMs(request.signal)).toBe(15_000);
  const cancelled = new AbortController(); cancelled.abort();
  abortAtDeadline(cancelled, 4000);
  expect(requestTimeoutMs(cancelled.signal)).toBeUndefined();
});
