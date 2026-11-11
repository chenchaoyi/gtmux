import {ConversationReadError, GtmuxClient} from './client';

const client = new GtmuxClient('http://synthetic.invalid', 'synthetic');
afterEach(() => {jest.restoreAllMocks(); delete (globalThis as any).fetch;});
test('CAS005 transcript rejects association and source failures instead of returning empty history', async () => {
  for (const [status, code, association] of [[409, 'conversation-unassociated', true], [503, 'conversation-unavailable', false], [409, 'unknown', false]] as const) {
    globalThis.fetch = jest.fn(async () => ({ok: false, status, headers: {get: () => null}, json: async () => ({code})})) as unknown as typeof fetch;
    await expect(client.transcript('%fixture')).rejects.toMatchObject({status, association});
    await expect(client.transcript('%fixture')).rejects.toBeInstanceOf(ConversationReadError);
  }
  globalThis.fetch = jest.fn(async () => ({ok: true, status: 200, headers: {get: () => null}, json: async () => []})) as unknown as typeof fetch;
  expect((await client.transcript('%fixture')).turns).toEqual([]);
});
