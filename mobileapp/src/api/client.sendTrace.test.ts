import {GtmuxClient} from './client';
import {Debug} from '../debug';

jest.mock('../debug', () => ({Debug: {logNet: true, record: jest.fn()}}));
const originalFetch = global.fetch;
const fetchMock = jest.fn();
const client = new GtmuxClient('http://fixture:8765', 'private-fixture-token');
const response = {ok: true, status: 200, headers: {get: () => null}, json: async () => ({id: '%12', text: 'snapshot'})} as unknown as Response;
const stages = () => (Debug.record as jest.Mock).mock.calls.map(([event]) => event).filter(event => event.event.startsWith('send-fetch-'));
beforeEach(() => {global.fetch = fetchMock; fetchMock.mockReset(); (Debug.record as jest.Mock).mockClear(); Debug.logNet = true;});
afterEach(() => {global.fetch = originalFetch;});

test('a pending send exposes fetch call and return without pretending the request completed', async () => {
  let resolve!: (value: Response) => void;
  fetchMock.mockImplementation(() => new Promise<Response>(done => {resolve = done;}));
  let settled = false;
  const send = client.send('%12', {text: 'private-fixture-reply'}).then(value => {settled = true; return value;});
  await Promise.resolve();
  expect(settled).toBe(false);
  expect(stages()).toEqual([{event: 'send-fetch-call', at: expect.any(Number)}, {event: 'send-fetch-returned', at: expect.any(Number)}]);
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(fetchMock).toHaveBeenCalledWith('http://fixture:8765/api/send', {
    method: 'POST', headers: {Authorization: 'Bearer private-fixture-token', 'Content-Type': 'application/json', 'X-Gtmux-Client': expect.stringMatching(/^iOS /)},
    body: JSON.stringify({id: '%12', text: 'private-fixture-reply'}),
  });
  expect(JSON.stringify(stages())).not.toMatch(/private|fixture|%12/);
  resolve(response);
  await expect(send).resolves.toEqual({id: '%12', text: 'snapshot'});
});

test.each(['throw', 'reject'])('a fetch %s preserves the original failure and its actual boundary', async mode => {
  const error = new Error('controlled fetch failure');
  fetchMock.mockImplementation(() => {if (mode === 'throw') throw error; return Promise.reject(error);});
  await expect(client.send('%12', {text: '2'})).rejects.toBe(error);
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(stages().map(stage => stage.event)).toEqual(mode === 'throw' ? ['send-fetch-call'] : ['send-fetch-call', 'send-fetch-returned']);
});

test('normal logging-off sends have no new diagnostic records', async () => {
  Debug.logNet = false;
  fetchMock.mockResolvedValue(response);
  await client.send('%12', {text: '2'});
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(Debug.record).not.toHaveBeenCalled();
});
