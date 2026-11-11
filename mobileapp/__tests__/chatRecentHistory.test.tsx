import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {ChatView} from '../src/ui/ChatView';
import {DetailView} from '../src/screens/DetailScreen';
import {HQConsoleView} from '../src/screens/HQScreen';
import {paletteFor} from '../src/ui/theme';
import {TestIds} from '../src/constants/testIds';
import {ApiError, GtmuxClient, type TranscriptTurn} from '../src/api/client';
import {toAgent} from '../src/api/types';

jest.mock('../src/state/AgentsContext', () => ({useAgents: () => mockContext, useAgentsOptional: () => mockContext}));
jest.mock('../src/state/AppContext', () => ({useApp: () => ({pal: require('../src/ui/theme').paletteFor('dark'), lang: 'en', fontPref: 'auto', defaultDetailMode: 'chat'})}));
jest.mock('../src/state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../src/screens/HQHeader', () => ({HQHeader: () => null}));
jest.mock('../src/trace/TraceModal', () => ({TraceModal: () => null}));
const mockClient: any = {
 base: 'https://synthetic.invalid', tasks: jest.fn(async () => []), digest: jest.fn(async () => []), usage: jest.fn(async () => null),
 hqBoard: jest.fn(async () => ({exists: false})), hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})),
 hqEvents: jest.fn(async () => []), pane: jest.fn(async () => ({text: '', cols: 80})), transcript: jest.fn(), traceHistory: jest.fn(),
 options: jest.fn(async () => []), panes: jest.fn(async () => []), theme: jest.fn(async () => undefined), sendResult: jest.fn(),
};
const agent = toAgent({pane_id: '%fixture', source: 'tmux', agent: 'Codex', session_id: 'session-a', home_id: 'home-a', status: 'idle'});
const mockContext: any = {client: mockClient, agents: [agent], conn: 'connected', demo: false, isGuest: false, inputPanes: []};
const turn = (n: number, extra: Partial<TranscriptTurn> = {}): TranscriptTurn => ({record_id: `record-${n}`, session_id: 'session-a', turn_ordinal: n, prompt: `question ${n}`, response: `answer ${n}`, ...extra});
let tree: renderer.ReactTestRenderer;
const flush = async () => {await act(async () => {});};
const press = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0];
const text = () => tree.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');
const props = (turns: TranscriptTurn[], a = agent) => ({client: mockClient, agent: a, lines: [], status: 'idle' as const, fontSize: 13, pal: paletteFor('dark'), lang: 'en' as const, turns, loading: false});
function deferred<T>() {let resolve!: (value: T) => void; const promise = new Promise<T>(r => {resolve = r;}); return {promise, resolve};}
beforeEach(() => {jest.useFakeTimers(); jest.clearAllMocks(); mockContext.agents = [agent]; mockClient.transcript.mockReset().mockResolvedValue({turns: [turn(102, {history_before: 'before-102'})], dropped: 101}); mockClient.traceHistory.mockReset();});
afterEach(() => {act(() => tree?.unmount()); jest.useRealTimers();});

test.each(['Detail', 'HQ'])('LCL-003 %s requests recent data and exposes a working error/retry action', async surface => {
 const hq = {...agent, role: 'supervisor'} as typeof agent; mockContext.agents = [surface === 'HQ' ? hq : agent];
 mockClient.transcript.mockRejectedValueOnce(new Error('synthetic read failure'));
 await act(async () => {tree = renderer.create(surface === 'HQ' ? <HQConsoleView agent={hq} workspaceMode /> : <DetailView agent={agent} initialMode="chat" />);});
 expect(mockClient.transcript.mock.calls[0][4]).toBe(true);
 expect(tree.root.findByType(ChatView).props.failed).toBe(true);
 expect(text()).toContain('Conversation could not be loaded');
 await act(async () => {press('chat-read-retry').props.onPress();});
 expect(mockClient.transcript).toHaveBeenCalledTimes(2);
 expect(tree.root.findByType(ChatView).props.failed).toBeFalsy();
 expect(text()).toContain('question 102');
});

test('LCL-003 earlier source pages survive latest polling and a failed page keeps the content', async () => {
 mockClient.traceHistory.mockResolvedValueOnce([turn(100, {history_before: 'before-100'}), turn(101)]).mockRejectedValueOnce(new Error('page unavailable'));
 await act(async () => {tree = renderer.create(<ChatView {...props([turn(102, {history_before: 'before-102'})])} droppedTurns={101} />);});
 expect(press(TestIds.detail.chatEarlier).props.disabled).toBe(false);
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 expect(mockClient.traceHistory.mock.calls[0][1]).toBe('before-102');
 expect(text()).toContain('question 100');
 await act(async () => {tree.update(<ChatView {...props([turn(102, {response: 'new latest answer'}), turn(103, {history_before: 'before-103'})])} droppedTurns={102} />);});
 expect(text()).toContain('question 100'); expect(text()).toContain('new latest answer'); expect(text()).toContain('question 103');
 expect(tree.root.findAll(n => n.props.testID === 'chat-user-row' && typeof n.type === 'string')).toHaveLength(4);
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 expect(text()).toContain('Earlier history could not be loaded'); expect(text()).toContain('question 100');
 expect(mockClient.traceHistory.mock.calls[1][1]).toBe('before-100');
});

test('LCL-003 target changes reject late history and noncooperative history reads stop showing loading after 15 seconds', async () => {
 const late = deferred<TranscriptTurn[]>();mockClient.traceHistory.mockReturnValueOnce(late.promise);
 await act(async () => {tree = renderer.create(<ChatView {...props([turn(102, {history_before: 'before-102'})])} />);});
 act(() => press(TestIds.detail.chatEarlier).props.onPress());
 const oldSignal = mockClient.traceHistory.mock.calls[0][2];
 const other = {...agent, session_id: 'session-b', home_id: 'home-b'};
 await act(async () => {tree.update(<ChatView {...props([turn(201, {session_id: 'session-b', history_before: 'before-201'})], other)} />);});
 expect(oldSignal.aborted).toBe(true);
 await act(async () => {late.resolve([turn(99, {prompt: 'OLD PRIVATE PAGE'})]);});
 expect(text()).not.toContain('OLD PRIVATE PAGE'); expect(text()).toContain('question 201');
 mockClient.traceHistory.mockReturnValueOnce(new Promise(() => {}));
 act(() => press(TestIds.detail.chatEarlier).props.onPress());
 await act(async () => {jest.advanceTimersByTime(15001);}); await flush();
 expect(text()).toContain('Earlier history could not be loaded');
 expect(press(TestIds.detail.chatEarlier).props.disabled).toBe(false);
});

test('LCL-003 actual client recent query is opt-in and record identity survives prepending', async () => {
 const previous = globalThis.fetch;
 const fetchMock = jest.fn(async (_url: string, _options?: unknown) => ({ok: true, status: 200, headers: {get: () => null}, json: async () => [turn(102)]}));
 globalThis.fetch = fetchMock as unknown as typeof fetch;
 try {
  const client = new GtmuxClient('https://synthetic.invalid', 'synthetic-token');
  await client.transcript('%1');
  expect(fetchMock.mock.calls[0][0]).toBe('https://synthetic.invalid/api/transcript?id=%251&context=1');
  await client.transcript('%1', undefined, 0, undefined, true);
  expect(fetchMock.mock.calls[1][0]).toBe('https://synthetic.invalid/api/transcript?id=%251&recent=1&context=1');
  const {turnIdentity} = require('../src/ui/participantProfile');
  expect(turnIdentity(turn(102), 0)).toBe(turnIdentity(turn(102), 40));
 } finally {globalThis.fetch = previous;}
});


test('LCL-003 authorization loss retires the paged snapshot until a new source snapshot arrives', async () => {
 mockClient.traceHistory.mockResolvedValueOnce([turn(100, {history_before: 'before-100'}), turn(101)]).mockRejectedValueOnce(new ApiError(403, 'history'));
 const recent = [turn(102, {history_before: 'before-102'})];
 await act(async () => {tree = renderer.create(<ChatView {...props(recent)} onRetry={jest.fn()} />);});
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 expect(text()).toContain('question 100');
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 expect(text()).not.toContain('question 100'); expect(text()).not.toContain('question 102');
 expect(text()).toContain('Conversation could not be loaded');
 await act(async () => {tree.update(<ChatView {...props([turn(103)])} />);});
 expect(text()).toContain('question 103'); expect(text()).not.toContain('question 100');
});


test('LCL-003 HQ predecessor stitch stays chronological and a source page mounts at most twenty more turns', async () => {
 const {mergeChatHistory} = require('../src/ui/useChatHistory');
 const older = [turn(81), turn(82), turn(100)];
 const previous = turn(45, {session_id: 'previous', record_id: 'previous-45', prompt: 'previous question'});
 expect(mergeChatHistory(older, [previous, turn(100, {response: 'updated'})]).map((t: TranscriptTurn) => [t.session_id, t.turn_ordinal])).toEqual([['previous', 45], ['session-a', 81], ['session-a', 82], ['session-a', 100]]);
 mockClient.traceHistory.mockResolvedValueOnce(Array.from({length: 50}, (_, i) => turn(i + 51, i ? {} : {history_before: 'before-51'})));
 await act(async () => {tree = renderer.create(<ChatView {...props([turn(101, {history_before: 'before-101'})])} />);});
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 const mounted = tree.root.findAll(n => n.props.testID === 'chat-user-row' && typeof n.type === 'string');
 expect(mounted).toHaveLength(21); expect(text()).toContain('question 81'); expect(text()).not.toContain('question 80');
});

test('LCL-003 expanded history stays visible while newer turns wait behind the live edge control', async () => {
 mockClient.traceHistory.mockResolvedValueOnce(Array.from({length: 50}, (_, i) => turn(i + 51, i ? {} : {history_before: 'before-51'})));
 await act(async () => {tree = renderer.create(<ChatView {...props([turn(101, {history_before: 'before-101'})])} />);});
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 expect(text()).toContain('question 81');
 await act(async () => {tree.update(<ChatView {...props([turn(101, {response: 'latest update'}), turn(102)])} />);});
 expect(text()).toContain('question 81'); expect(text()).toContain('latest update'); expect(text()).not.toContain('question 102');
 expect(tree.root.findAll(n => n.props.testID === 'chat-user-row' && typeof n.type === 'string')).toHaveLength(21);
 act(() => press(TestIds.detail.jumpBottom).props.onPress());
 expect(text()).toContain('question 102');
 expect(tree.root.findAll(n => n.props.testID === 'chat-user-row' && typeof n.type === 'string')).toHaveLength(20);
});


test('LCL-003 unmount cancels the deadline even when a history reader ignores AbortSignal', async () => {
 mockClient.traceHistory.mockReturnValueOnce(new Promise(() => {}));
 const {useChatHistory} = require('../src/ui/useChatHistory');
 let history!: ReturnType<typeof useChatHistory>;
 const recent = [turn(101, {history_before: 'before-101'})];
 function Harness() {history = useChatHistory(mockClient, agent, recent, () => {}); return null;}
 const abort = jest.spyOn(AbortController.prototype, 'abort');
 const timers = jest.spyOn(globalThis, 'setTimeout');
 const clear = jest.spyOn(globalThis, 'clearTimeout');
 try {
  await act(async () => {tree = renderer.create(<Harness />);});
  act(() => {history.load();});
  const index = timers.mock.calls.findIndex(args => args[1] === 15000);
  expect(index).toBeGreaterThanOrEqual(0);
  const deadline = timers.mock.results[index].value;
  await act(async () => {tree.unmount();});
  expect(mockClient.traceHistory.mock.calls[0][2].aborted).toBe(true);
  expect(clear).toHaveBeenCalledWith(deadline);
  expect(abort).toHaveBeenCalledTimes(1);
  await act(async () => {jest.advanceTimersByTime(15001);});
  expect(abort).toHaveBeenCalledTimes(1);
 } finally {abort.mockRestore(); timers.mockRestore(); clear.mockRestore();}
});
