import {GtmuxClient} from './client';
import {SEND_TIMEOUT_MS, SendDeadlineError} from './sendDeadline';

let fetchMock: jest.Mock;
const flush = async () => {for (let i = 0; i < 8; i++) await Promise.resolve();};
const response = (json: () => Promise<unknown>, status = 200) => ({ok: status === 200, status, headers: {get: () => null}, json} as unknown as Response);
beforeEach(() => {jest.useFakeTimers(); fetchMock = jest.fn(); global.fetch = fetchMock;});
afterEach(() => {jest.useRealTimers(); jest.restoreAllMocks();});

test.each(['transport', 'body'])('MSD-004 bounds a hung %s and never retries or accepts a late answer', async stage => {
  let resolve!: (r: any) => void;
  const never = new Promise(resolvePromise => {resolve = resolvePromise;});
  fetchMock.mockReturnValue(stage === 'transport' ? never : Promise.resolve(response(() => never)));
  const client = new GtmuxClient('http://synthetic', 'synthetic-token');
  const outcomes: string[] = [];
  const send = client.sendResult('%1', {text: 'synthetic', enter: true, send_id: 'same-id'}).then(() => outcomes.push('success'), error => {
    expect(error).toBeInstanceOf(SendDeadlineError); outcomes.push('timeout');
  });
  await flush();
  jest.advanceTimersByTime(SEND_TIMEOUT_MS - 1); await flush(); expect(outcomes).toEqual([]);
  jest.advanceTimersByTime(1); await flush(); await send;
  expect(outcomes).toEqual(['timeout']); expect(fetchMock).toHaveBeenCalledTimes(1);
  const init = fetchMock.mock.calls[0][1]; expect(init.signal.aborted).toBe(true);
  expect(JSON.parse(init.body)).toMatchObject({id: '%1', send_id: 'same-id'});
  resolve(stage === 'transport' ? response(async () => ({status: 'ok'})) : {status: 'ok'}); await flush();
  expect(outcomes).toEqual(['timeout']); expect(fetchMock).toHaveBeenCalledTimes(1);
});

test('MSD-004 keeps exact receipts, legacy success and the server refusal', async () => {
  const timers = jest.spyOn(global, 'setTimeout');
  const clears = jest.spyOn(global, 'clearTimeout');
  const client = new GtmuxClient('http://synthetic', 'synthetic-token');
  for (const state of ['queued', 'accepted', 'submitted']) {
    fetchMock.mockResolvedValue(response(async () => ({status: 'ok', delivery: {state, send_id: 'same-id'}})));
    expect(await client.sendResult('%1', {text: 'synthetic', send_id: 'same-id'})).toMatchObject({ok: true, delivery: state});
  }
  fetchMock.mockResolvedValue(response(async () => ({status: 'ok', delivery: {state: 'queued', send_id: 'other'}})));
  expect(await client.sendResult('%1', {text: 'synthetic', send_id: 'same-id'})).toMatchObject({ok: true, delivery: undefined});
  fetchMock.mockResolvedValue(response(async () => ({status: 'ok'})));
  expect(await client.sendResult('%1', {text: 'synthetic'})).toMatchObject({ok: true, delivery: undefined});
  fetchMock.mockResolvedValue(response(async () => ({error: 'approval pending'}), 409));
  expect(await client.sendResult('%1', {text: 'synthetic'})).toEqual({ok: false, status: 409, reason: 'approval pending'});
  const sendTimers = timers.mock.results.filter((_, i) => timers.mock.calls[i][1] === SEND_TIMEOUT_MS);
  expect(sendTimers).toHaveLength(6);
  for (const timer of sendTimers) expect(clears).toHaveBeenCalledWith(timer.value);
});
