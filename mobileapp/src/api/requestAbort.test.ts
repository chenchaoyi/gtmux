import {abortAtDeadline, requestAbortKind} from './requestAbort';

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
