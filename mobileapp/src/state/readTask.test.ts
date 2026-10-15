import {requestAbortKind} from '../api/requestAbort';
import {createReadTask, startReadPolling} from './readTask';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {resolve = yes; reject = no;});
  return {promise, resolve, reject};
}
const flush = async () => {for (let i = 0; i < 10; i++) await Promise.resolve();};
beforeEach(() => jest.useFakeTimers());
afterEach(() => jest.useRealTimers());

test('slow periodic and manual refreshes share one read; failure permits recovery', async () => {
  const first = deferred<string>();
  const read = jest.fn().mockReturnValueOnce(first.promise).mockResolvedValue('recovered');
  const apply = jest.fn();
  const task = createReadTask(read, apply);
  const stop = startReadPolling(task, 1000);
  await flush();
  jest.advanceTimersByTime(5000); task.run(); await flush();
  expect(read).toHaveBeenCalledTimes(1);
  first.reject(new Error('offline')); await flush();
  expect(apply).not.toHaveBeenCalled();
  task.run(); await flush();
  expect(apply).toHaveBeenCalledWith('recovered');
  stop(); expect(jest.getTimerCount()).toBe(0);
});

test('a transport ignoring abort cannot publish after its deadline or hold the slot', async () => {
  const first = deferred<string>();
  const signals: AbortSignal[] = [];
  const read = jest.fn((signal: AbortSignal) => {signals.push(signal); return signals.length === 1 ? first.promise : Promise.resolve('new');});
  const apply = jest.fn(), onError = jest.fn();
  const task = createReadTask(read, apply, {timeoutMs: 50, onError});
  task.run(); await flush();
  jest.advanceTimersByTime(50); await flush();
  expect(requestAbortKind(signals[0])).toBe('timeout'); expect(onError).toHaveBeenCalledTimes(1);
  task.run(); await flush();
  first.resolve('old'); await flush();
  expect(apply.mock.calls).toEqual([['new']]);
  task.dispose(); expect(jest.getTimerCount()).toBe(0);
});

test('disposal retires an old scope; invalidation retires pre-action captures', async () => {
  const first = deferred<string>(), next = deferred<string>();
  const apply = jest.fn(), signals: AbortSignal[] = [];
  const task = createReadTask(signal => {signals.push(signal); return signals.length === 1 ? first.promise : next.promise;}, apply);
  task.run(); await flush(); task.invalidate(); task.run(); await flush();
  first.resolve('before send'); next.resolve('after send'); await flush();
  expect(requestAbortKind(signals[0])).toBe('cancelled'); expect(apply.mock.calls).toEqual([['after send']]);
  task.dispose(); task.run(); await flush(); expect(signals).toHaveLength(2);
  expect(jest.getTimerCount()).toBe(0);
});

test('a disposed scope ignores late success and failure', async () => {
  for (const fail of [false, true]) {
    const pending = deferred<string>(); const apply = jest.fn(), onError = jest.fn(), onSettled = jest.fn();
    const task = createReadTask(() => pending.promise, apply, {onError, onSettled});
    task.run(); await flush(); task.dispose();
    if (fail) pending.reject(new Error('late')); else pending.resolve('late');
    await flush(); expect(apply).not.toHaveBeenCalled(); expect(onError).not.toHaveBeenCalled(); expect(onSettled).not.toHaveBeenCalled();
  }
  expect(jest.getTimerCount()).toBe(0);
});
