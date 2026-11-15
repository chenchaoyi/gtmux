import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AppState} from 'react-native';
import {GtmuxClient} from '../api/client';
import type {Agent} from '../api/types';
import {toAgent} from '../api/types';
import {AgentsProvider, useAgents} from './AgentsContext';

type Value = ReturnType<typeof useAgents>;
type Request = {client: GtmuxClient; signal?: AbortSignal; finish: (value: Agent[]) => void; fail: (error: Error) => void};
const requests: Request[] = [];
const streams: {onOpen: () => void; onAgents: () => void}[] = [];
jest.mock('../api/events', () => ({subscribe: jest.fn((_base, _token, handlers) => {streams.push(handlers); return () => {};})}));
jest.mock('../native/liveActivity', () => ({LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: jest.fn().mockResolvedValue(null)}, apnsEnv: () => 'sandbox'}));
jest.mock('../push', () => ({setBadge: jest.fn()}));
jest.mock('../pairing/follow', () => ({findLiveAddress: jest.fn()}));
let tree: renderer.ReactTestRenderer;
let value: Value;
const frames: {base: string; ids: (string | undefined)[]; hasValue: boolean}[] = [];
function Probe() {value = useAgents(); frames.push({base: value.client.base, ids: value.agents.map(a => a.pane_id), hasValue: value.read.hasValue}); return null;}
const flush = async () => {await act(async () => {for (let i = 0; i < 20; i++) await Promise.resolve();});};
const node = (base = 'https://a.example', token = 'a') => <AgentsProvider base={base} token={token} liveActivity={false}><Probe /></AgentsProvider>;
const mount = async () => {await act(async () => {tree = renderer.create(node());}); await flush();};
const finish = async (i: number, agents: Agent[]) => {await act(async () => {requests[i].finish(agents);}); await flush();};
const rows = (id: string) => [toAgent({pane_id: id, status: 'working', agent: 'codex'})];
let readSpy: jest.SpyInstance;
const originalFetch = globalThis.fetch;
beforeEach(() => {
  jest.useFakeTimers(); requests.length = 0; streams.length = 0; frames.length = 0;
  (AppState as any).currentState = 'active';
  jest.spyOn(GtmuxClient.prototype, 'share').mockResolvedValue({all: true, input: true, panes: [], view_panes: []});
  jest.spyOn(GtmuxClient.prototype, 'addresses').mockResolvedValue({addresses: []});
  readSpy = jest.spyOn(GtmuxClient.prototype, 'agents').mockImplementation(function (this: GtmuxClient, signal?: AbortSignal) {
    return new Promise<Agent[]>((resolve, reject) => {requests.push({client: this, signal, finish: resolve, fail: reject});});
  });
});
afterEach(async () => {await act(async () => {tree?.unmount();}); globalThis.fetch = originalFetch; jest.restoreAllMocks(); jest.useRealTimers();});

test('a connected stream cannot establish an empty snapshot before its scoped read', async () => {
  await mount(); expect(requests).toHaveLength(1);
  expect(value.read).toMatchObject({hasValue: false, refreshing: true}); expect(value.lastUpdated).toBeNull();
  act(() => {streams[0].onOpen();}); await flush();
  expect(value.conn).toBe('live'); expect(value.read.hasValue).toBe(false); expect(requests).toHaveLength(1);
  await finish(0, []); expect(value.read.hasValue).toBe(true); expect(requests).toHaveLength(2);
  await finish(1, []); expect(value.read).toEqual({hasValue: true, refreshing: false, error: null});
});

test('bursty refreshes retain one trailing read and an owned refresh settles without waiting for future bursts', async () => {
  await mount(); await finish(0, rows('%1'));
  let done = false;
  act(() => {void value.refresh().then(() => {done = true;});}); await flush();
  for (let i = 0; i < 8; i++) act(() => {streams[0].onAgents();}); await flush();
  expect(requests).toHaveLength(2); expect(done).toBe(false);
  await finish(1, rows('%2')); expect(requests).toHaveLength(3); expect(done).toBe(true);
  expect(value.read.refreshing).toBe(true); expect(value.agents[0].pane_id).toBe('%2');
  await finish(2, rows('%3')); expect(requests).toHaveLength(3); expect(value.read.refreshing).toBe(false);
  expect(value.agents[0].pane_id).toBe('%3');
});

test('changed credentials never borrow a previous snapshot or admit a late retired response', async () => {
  await mount(); await finish(0, rows('%a'));
  const oldRefresh = value.refresh;
  act(() => {void value.refresh();}); await flush(); const oldSignal = requests[1].signal;
  await act(async () => {tree.update(node('https://b.example', 'b'));}); await flush();
  expect(value.client.base).toBe('https://b.example'); expect(value.agents).toEqual([]); expect(value.read.hasValue).toBe(false);
  expect(oldSignal?.aborted).toBe(true);
  expect(frames.filter(f => f.base === 'https://b.example').every(f => f.ids.length === 0 && !f.hasValue)).toBe(true);
  await finish(1, rows('%late-a')); expect(value.agents).toEqual([]);
  await finish(2, []); expect(value.read.hasValue).toBe(true); expect(value.agents).toEqual([]);
  await act(async () => {await oldRefresh();}); expect(requests).toHaveLength(3);
});

test('a failed cached refresh preserves its last readable snapshot and timestamp', async () => {
  await mount(); await finish(0, rows('%cached')); const at = value.lastUpdated;
  act(() => {void value.refresh();}); await flush();
  expect(value.agents[0].pane_id).toBe('%cached');
  await act(async () => {requests[1].fail(new Error('synthetic offline'));}); await flush();
  expect(value.read).toEqual({hasValue: true, refreshing: false, error: 'failed'});
  expect(value.lastUpdated).toBe(at); expect(value.agents[0].pane_id).toBe('%cached'); expect(value.conn).toBe('offline');
});

test('whole JSON-body timeout retires the real client read and an explicit retry can succeed', async () => {
  readSpy.mockRestore();
  let finishBody!: (value: unknown) => void;
  const fetcher = jest.fn().mockResolvedValueOnce({ok: true, status: 200, headers: {get: () => null}, json: () => new Promise(resolve => {finishBody = resolve;})})
    .mockResolvedValueOnce({ok: true, status: 200, headers: {get: () => null}, json: async () => []});
  globalThis.fetch = fetcher;
  await mount(); expect(value.read.hasValue).toBe(false);
  await act(async () => {jest.advanceTimersByTime(15_000);}); await flush();
  expect(value.read).toEqual({hasValue: false, refreshing: false, error: 'timeout'});
  expect(fetcher.mock.calls[0][1].signal.aborted).toBe(true);
  await act(async () => {await value.refresh();}); await flush(); expect(value.read.hasValue).toBe(true);
  await act(async () => {finishBody([{pane_id: '%late'}]);}); await flush();
  expect(value.agents).toEqual([]); expect(value.read.error).toBeNull();
});
