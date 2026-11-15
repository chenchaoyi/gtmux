import {GtmuxClient} from './client';
import {classifySendFailure} from '../ui/sendFailure';

const oldFetch = global.fetch;
afterEach(() => {global.fetch = oldFetch;});
test.each(['input-state-unavailable', 'input-awaiting-confirmation', 'draft-present', 'conversation-unavailable', 'target-changed', 'session-not-loaded', 'outcome-unconfirmed', 'input-unconfirmed'])('SFR-003 keeps the optional known %s refusal', async code => {
  const fetch = jest.fn().mockResolvedValue({ok: false, status: 400, headers: {get: () => null}, json: async () => ({error: 'legacy text', code})});
  global.fetch = fetch;
  const client = new GtmuxClient('https://synthetic.invalid', 'synthetic');
  expect(await client.sendResult('%1', {text: 'one attempt', send_id: 'same-id'})).toEqual({ok: false, status: 400, reason: 'legacy text', code});
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(JSON.parse(fetch.mock.calls[0][1].body)).toMatchObject({id: '%1', send_id: 'same-id'});
});
test.each([undefined, 'future-code', 1, {}, ['input-awaiting-confirmation']])('SFR-003 ignores unknown code %p and retains the legacy response', async code => {
  global.fetch = jest.fn().mockResolvedValue({ok: false, status: 400, headers: {get: () => null}, json: async () => ({error: 'legacy text', code})});
  const client = new GtmuxClient('https://synthetic.invalid', 'synthetic');
  expect(await client.sendResult('%1', {text: 'one attempt'})).toEqual({ok: false, status: 400, reason: 'legacy text'});
  expect(await client.send('%1', {text: 'one attempt'})).toBeNull();
});
test.each([[401, 'refused'], [403, 'not-shared']] as const)('SFR-003 status %i takes priority over a question code', async (status, kind) => {
  global.fetch = jest.fn().mockResolvedValue({ok: false, status, headers: {get: () => null}, json: async () => ({error: 'asking', code: 'input-awaiting-confirmation'})});
  const result = await new GtmuxClient('https://synthetic.invalid', 'synthetic').sendResult('%1', {text: 'one attempt'});
  expect(result.ok).toBe(false);
  if (!result.ok) expect(classifySendFailure(result.reason, result.status, result.code)).toBe(kind);
});
