import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AgentsProvider, useAgents} from './AgentsContext';
import {ApiError} from '../api/client';
import {subscribe} from '../api/events';
const mockReads: Record<string, () => Promise<any[]>> = {};
jest.mock('../api/client', () => ({...jest.requireActual('../api/client'), GtmuxClient: jest.fn((_base, token) => ({
  agents: () => mockReads[token](), addresses: async () => ({addresses: []}),
  share: async () => ({all: true, panes: []}),
}))}));
const mockStreams: {token: string; handlers: any; close: jest.Mock}[] = [];
jest.mock('../api/events', () => ({subscribe: jest.fn((_base, token, handlers) => {
  const close = jest.fn(); mockStreams.push({token, handlers, close}); return close;
})}));
jest.mock('../native/liveActivity', () => ({LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: async () => null}, apnsEnv: () => 'sandbox'}));
let seen: ReturnType<typeof useAgents>;
function Probe() { seen = useAgents(); return null; }
const view = (token: string) => <AgentsProvider base="https://synthetic.invalid" token={token}><Probe /></AgentsProvider>;
let tree: renderer.ReactTestRenderer;
beforeEach(() => { mockStreams.length = 0; jest.clearAllMocks(); mockReads.fresh = async () => []; });
afterEach(() => { act(() => tree?.unmount()); });

test('re-pair closes the old stream and late old-credential failures cannot reject the new connection', async () => {
  let reject!: (e: Error) => void;
  const pending = new Promise<any[]>((_, no) => { reject = no; });
  mockReads.old = () => pending;
  await act(async () => { tree = renderer.create(view('old')); });
  await act(async () => { tree.update(view('fresh')); });
  expect(seen.conn).toBe('live');
  expect(subscribe).toHaveBeenCalledTimes(2);
  expect(mockStreams[0].close).toHaveBeenCalledTimes(1);
  await act(async () => { reject(new ApiError(401, 'agents')); });
  expect(seen.conn).toBe('live');
  await act(async () => { mockStreams[0].handlers.onError(401); });
  expect(seen.conn).toBe('live');
});

test('a late old-credential success cannot clear the fresh connection failure', async () => {
  let resolve!: (v: any[]) => void;
  const pending = new Promise<any[]>(yes => { resolve = yes; });
  mockReads.old = () => pending;
  mockReads.fresh = async () => { throw new ApiError(401, 'agents'); };
  await act(async () => { tree = renderer.create(view('old')); });
  await act(async () => { tree.update(view('fresh')); });
  expect(seen.conn).toBe('unauthorized');
  await act(async () => { resolve([]); mockStreams[0].handlers.onOpen(); });
  expect(seen.conn).toBe('unauthorized');
});
