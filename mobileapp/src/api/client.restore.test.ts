import {GtmuxClient} from './client';
import {RestoreError, parseRestoreSnapshot, restoreRequestID, type RestorePlan} from './restore';
const base = 'https://fixture.example';
const revision = 'a'.repeat(64);
const plan: RestorePlan = {revision, server_time: 1800000000, saved_at: 1800000000, resume_mode: 'auto', sessions: [{name: 'saved', windows: 1, panes: 1, agents: 1, resumable: 1, state: 'missing'}]};
const id = '1800000000-request-1234567890';
const job = {id, revision, created_at: 1800000000, updated_at: 1800000000, phase: 'queued', sessions: plan.sessions};
const answer = (v: any, status = 200) => ({ok: status < 400, status, json: async () => v, headers: {get: () => null}} as unknown as Response);
const originalFetch = globalThis.fetch;
afterEach(() => {globalThis.fetch = originalFetch; jest.useRealTimers();});
test('preview and status are read only; start carries exact stable ID and revision', async () => {
  const fetch = jest.fn().mockResolvedValueOnce(answer({plan})).mockResolvedValueOnce(answer(job)).mockResolvedValueOnce(answer(job, 202)); globalThis.fetch = fetch;
  const c = new GtmuxClient(base, 'synthetic-token');
  await c.restorePreview(); await c.restoreStatus(id); await c.restoreStart(id, revision);
  expect(fetch.mock.calls[0][1].method).toBeUndefined(); expect(fetch.mock.calls[1][0]).toContain(`/api/restore/job?id=${id}`);
  expect(JSON.parse(fetch.mock.calls[2][1].body)).toEqual({request_id: id, revision});
  expect(fetch.mock.calls[2][1].headers.Authorization).toBe('Bearer synthetic-token');
  expect(restoreRequestID(plan)).toMatch(/^1800000000-[A-Za-z0-9-]{16,64}$/);
});
test('normal 15s timeout is uncertain and never retries a mutation', async () => {
  jest.useFakeTimers(); const fetch = jest.fn((_u, init) => new Promise((_ok, reject) => init.signal.addEventListener('abort', () => reject(new Error('aborted'))))); globalThis.fetch = fetch as any;
  const p = new GtmuxClient(base, 't').restoreStart(id, revision).catch(e => e);
  await jest.advanceTimersByTimeAsync(15001); expect((await p).code).toBe('uncertain'); expect(fetch).toHaveBeenCalledTimes(1);
});
test('unsupported, malformed, foreign ID and typed conflicts never become success', async () => {
  const c = new GtmuxClient(base, 't');
  globalThis.fetch = jest.fn().mockResolvedValue(answer({}, 404)); await expect(c.restorePreview()).rejects.toMatchObject({code: 'unsupported'});
  globalThis.fetch = jest.fn().mockResolvedValue(answer({...job, id: '1800000000-another-request-id'})); await expect(c.restoreStart(id, revision)).rejects.toMatchObject({code: 'uncertain'}); await expect(c.restoreStatus(id)).rejects.toMatchObject({code: 'invalid_response'});
  globalThis.fetch = jest.fn().mockResolvedValue(answer({code: 'plan_changed'}, 409)); await expect(c.restoreStart(id, revision)).rejects.toMatchObject({code: 'plan_changed'});
  for (const bad of [{...plan, sessions: new Array(257).fill(plan.sessions[0])}, {...plan, revision: 'bad'}, {...plan, resume_mode: 'unknown'}]) expect(() => parseRestoreSnapshot({plan: bad})).toThrow(RestoreError);
});
