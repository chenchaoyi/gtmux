import {GtmuxClient} from './client';

const originalFetch = globalThis.fetch;
const client = () => new GtmuxClient('https://mac.example', 'synthetic-token');
const response = (value: unknown) => ({ok: true, status: 200, headers: {get: () => null}, json: async () => value}) as unknown as Response;
let fetcher: jest.Mock;
beforeEach(() => {fetcher = jest.fn(); globalThis.fetch = fetcher;});
afterEach(() => {globalThis.fetch = originalFetch;});

test('a real empty agents response is successful data and ordinary rows still map', async () => {
  fetcher.mockResolvedValueOnce(response([])).mockResolvedValueOnce(response([{pane_id: '%1', agent: 'codex', status: 'working'}]));
  expect(await client().agents()).toEqual([]);
  expect(await client().agents()).toEqual([expect.objectContaining({pane_id: '%1', agent: 'codex', status: 'working'})]);
});
test.each([null, {not: 'a-list'}, 'unavailable'])('a malformed successful agents body stays a read failure: %p', async value => {
  fetcher.mockResolvedValueOnce(response(value));
  await expect(client().agents()).rejects.toThrow('agents response is not a list');
});
