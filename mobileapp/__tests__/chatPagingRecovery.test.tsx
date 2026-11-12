import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {DetailView} from '../src/screens/DetailScreen';
import {HQConsoleView} from '../src/screens/HQScreen';
import {ChatView} from '../src/ui/ChatView';
import {ApiError, GtmuxClient, type TranscriptTurn} from '../src/api/client';
import {toAgent} from '../src/api/types';
import {TestIds} from '../src/constants/testIds';

jest.mock('../src/state/AgentsContext', () => ({useAgents: () => mockContext, useAgentsOptional: () => mockContext}));
jest.mock('../src/state/AppContext', () => ({useApp: () => ({pal: require('../src/ui/theme').paletteFor('dark'), lang: 'en', fontPref: 'auto', defaultDetailMode: 'chat'})}));
jest.mock('../src/state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../src/screens/HQHeader', () => ({HQHeader: () => null}));
jest.mock('../src/trace/TraceModal', () => ({TraceModal: () => null}));

const agent = toAgent({pane_id: '%fixture', source: 'tmux', agent: 'Codex', session_id: 'session-a', home_id: 'home-a', status: 'idle'});
const mockClient: any = {
  base: 'https://synthetic.invalid', tasks: jest.fn(async () => []), digest: jest.fn(async () => []), usage: jest.fn(async () => null),
  hqBoard: jest.fn(async () => ({exists: false})), hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})),
  hqEvents: jest.fn(async () => []), pane: jest.fn(async () => ({text: '', cols: 80})), transcript: jest.fn(), traceHistory: jest.fn(),
  options: jest.fn(async () => []), panes: jest.fn(async () => []), theme: jest.fn(async () => undefined), sendResult: jest.fn(),
};
const mockContext: any = {client: mockClient, agents: [agent], conn: 'connected', demo: false, isGuest: false, inputPanes: []};
const turn = (session = 'session-a'): TranscriptTurn => ({record_id: `${session}-102`, session_id: session, turn_ordinal: 102, history_before: 'before-102', prompt: `${session} quiet question`, response: 'unchanged answer'});
const response = (status: number, session = 'session-a', etag = 'quiet-v1') => ({ok: status === 200, status, headers: {get: (name: string) => name.toLowerCase() === 'etag' ? etag : null}, json: async () => [turn(session)]});
function deferred<T>() {let resolve!: (value: T) => void; const promise = new Promise<T>(r => {resolve = r;}); return {promise, resolve};}
let tree: renderer.ReactTestRenderer | undefined;
let savedFetch: typeof fetch;
let fetchMock: jest.Mock;
const press = (id: string) => tree!.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0];
const text = () => tree!.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');
const validator = (index: number) => fetchMock.mock.calls[index]?.[1]?.headers?.['If-None-Match'];
const render = (surface: string, selected = agent) => {
  const a = surface === 'HQ' ? {...selected, role: 'supervisor'} as typeof agent : selected;
  mockContext.agents = [a];
  return surface === 'HQ' ? <HQConsoleView agent={a} workspaceMode /> : <DetailView agent={a} initialMode="chat" />;
};
const denyPage = async () => {
  mockClient.traceHistory.mockRejectedValueOnce(new ApiError(403, 'history'));
  await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
  expect(text()).toContain('Conversation could not be loaded');
  expect(text()).not.toContain('quiet question');
};
beforeEach(() => {
  jest.useFakeTimers(); jest.clearAllMocks(); savedFetch = globalThis.fetch;
  fetchMock = jest.fn().mockResolvedValue(response(304)); globalThis.fetch = fetchMock;
  const realClient = new GtmuxClient(mockClient.base, 'synthetic-token');
  mockClient.transcript.mockReset().mockImplementation(realClient.transcript.bind(realClient));
  mockClient.traceHistory.mockReset();
});
afterEach(() => {act(() => tree?.unmount()); tree = undefined; globalThis.fetch = savedFetch; jest.useRealTimers();});

test.each(['Detail', 'HQ'])('CPG005 %s explicit Retry recovers quiet history after authorized 304', async surface => {
  fetchMock.mockResolvedValueOnce(response(200));
  await act(async () => {tree = renderer.create(render(surface));});
  const original = tree!.root.findByType(ChatView).props.turns;
  expect(text()).toContain('session-a quiet question');
  await denyPage();
  await act(async () => {jest.advanceTimersByTime(8000);});
  expect(validator(1)).toBe('quiet-v1');
  expect(tree!.root.findByType(ChatView).props.turns).toBe(original);
  expect(text()).not.toContain('quiet question');
  // The synthetic server only sends a body for an unconditional read. This is
  // the real client parser, not a hand-cloned turns array or changed content.
  fetchMock.mockImplementation(async (_url, options) => response(options.headers['If-None-Match'] ? 304 : 200));
  await act(async () => {press('chat-read-retry').props.onPress();});
  expect({validator: validator(2), restored: text().includes('session-a quiet question'), error: text().includes('Conversation could not be loaded')}).toEqual({validator: undefined, restored: true, error: false});
  expect(tree!.root.findByType(ChatView).props.turns).toEqual(original);
});

test.each(['Detail', 'HQ'])('CPG006 %s failed Retry stays denied and retires a pending conditional poll', async surface => {
  fetchMock.mockResolvedValueOnce(response(200));
  await act(async () => {tree = renderer.create(render(surface));});
  await denyPage();
  const oldPoll = deferred<ReturnType<typeof response>>();
  fetchMock.mockReturnValueOnce(oldPoll.promise);
  await act(async () => {jest.advanceTimersByTime(8000);});
  expect(validator(1)).toBe('quiet-v1');
  const oldSignal = fetchMock.mock.calls[1][1].signal;
  fetchMock.mockRejectedValueOnce(new Error('synthetic unavailable'));
  await act(async () => {press('chat-read-retry').props.onPress();});
  expect({aborted: oldSignal.aborted, requests: fetchMock.mock.calls.length, validator: validator(2)}).toEqual({aborted: true, requests: 3, validator: undefined});
  expect(text()).toContain('Conversation could not be loaded'); expect(text()).not.toContain('quiet question');
  fetchMock.mockResolvedValueOnce(response(200, 'session-a', 'recovered-v2'));
  await act(async () => {press('chat-read-retry').props.onPress();});
  expect(validator(3)).toBeUndefined(); expect(text()).toContain('session-a quiet question');
  await act(async () => {oldPoll.resolve(response(304));});
  await act(async () => {jest.advanceTimersByTime(8000);});
  expect(validator(4)).toBe('recovered-v2');
  expect(text()).toContain('session-a quiet question'); expect(text()).not.toContain('Conversation could not be loaded');
});

test.each(['Detail', 'HQ'])('CPG007 %s reader switch ignores a late recovery response', async surface => {
  fetchMock.mockResolvedValueOnce(response(200));
  await act(async () => {tree = renderer.create(render(surface));});
  await denyPage();
  const oldRetry = deferred<ReturnType<typeof response>>(); fetchMock.mockReturnValueOnce(oldRetry.promise);
  await act(async () => {press('chat-read-retry').props.onPress();});
  const oldSignal = fetchMock.mock.calls[1][1].signal;
  fetchMock.mockResolvedValueOnce(response(200, 'session-b', 'b-v1'));
  await act(async () => {tree!.update(render(surface, {...agent, session_id: 'session-b'}));});
  expect(oldSignal.aborted).toBe(true); expect(text()).toContain('session-b quiet question');
  await act(async () => {oldRetry.resolve(response(200, 'OLD PRIVATE RECOVERY', 'a-late'));});
  expect(text()).not.toContain('OLD PRIVATE RECOVERY'); expect(text()).toContain('session-b quiet question');
  await act(async () => {jest.advanceTimersByTime(8000);});
  expect(validator(3)).toBe('b-v1'); expect(mockClient.sendResult).not.toHaveBeenCalled();
});
