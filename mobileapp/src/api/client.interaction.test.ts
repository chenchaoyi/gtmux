import {GtmuxClient} from './client';

const response = (body: unknown, status = 200) => ({ok: status < 400, status, json: async () => body, headers: {get: () => null}});
const snapshot = {text: 'Allow test tool?', options: [{n: 1, label: 'Allow once'}], awaiting_reply: true, truncated: false};
const client = () => new GtmuxClient('http://mac.local:8765', 'synthetic-token');
let fetchMock: jest.Mock;
beforeEach(() => {fetchMock = jest.fn(); (globalThis as any).fetch = fetchMock;});
afterEach(() => {delete (globalThis as any).fetch;});

test('reads the scoped current prompt with authentication', async () => {
  fetchMock.mockResolvedValue(response(snapshot));
  expect(await client().interaction('%12')).toEqual(snapshot);
  expect(fetchMock.mock.calls[0][0]).toBe('http://mac.local:8765/api/interaction?id=%2512');
  expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe('Bearer synthetic-token');
});
test.each([404, 501])('an old server (%s) retains options and explains missing screen support', async status => {
  fetchMock.mockResolvedValueOnce(response({}, status)).mockResolvedValueOnce(response({options: snapshot.options}));
  expect(await client().interaction('%12')).toEqual({text: '', options: snapshot.options, awaiting_reply: false, truncated: false, unavailable: true});
  expect(fetchMock.mock.calls[1][0]).toContain('/api/options?');
});
test.each([403, 500])('read failures (%s) never fall back to stale options', async status => {
  fetchMock.mockResolvedValue(response({}, status));
  await expect(client().interaction('%12')).rejects.toThrow('Current prompt unavailable');
  expect(fetchMock).toHaveBeenCalledTimes(1);
});
test('a disappeared pane is not treated as an old server', async () => {
  fetchMock.mockResolvedValue(response({error: 'pane not found'}, 404));
  await expect(client().interaction('%12')).rejects.toThrow('Pane not found');
  expect(fetchMock).toHaveBeenCalledTimes(1);
});
test.each([{...snapshot, options: [{n: 0, label: 'bad'}]}, {...snapshot, text: null}, {...snapshot, awaiting_reply: 'yes'}])('rejects malformed current prompts', async body => {
  fetchMock.mockResolvedValue(response(body));
  await expect(client().interaction('%12')).rejects.toThrow('Invalid current prompt');
});
