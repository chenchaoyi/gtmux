import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity, View} from 'react-native';
import {DetailView} from './DetailScreen';
import {HQConsoleView} from './HQScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';

jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext, useAgentsOptional: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Synthetic'}, returnSends: false, defaultDetailMode: 'chat'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));

const agent: any = {pane_id: '%fixture', session: 'synthetic', window: '0', pane: '0', loc: 'synthetic:0.0', session_id: 'same-thread', home_id: 'same-home', agent: 'Codex', status: 'working', task: '', source: 'tmux', role: 'supervisor'};
const mockClient = {
  base: 'https://synthetic.invalid', token: 'synthetic', sendResult: jest.fn(), transcript: jest.fn(), traceHistory: jest.fn(),
  pane: jest.fn(async () => ({text: 'ready', cols: 80})), panes: jest.fn(async () => []), options: jest.fn(async () => []), tasks: jest.fn(async () => []),
  theme: jest.fn(async () => undefined), digest: jest.fn(async () => []), usage: jest.fn(async () => null), hqBoard: jest.fn(async () => ({exists: false})),
  hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})), hqEvents: jest.fn(async () => []),
};
const mockContext = {client: mockClient, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false};
const ComposerFn = (Composer as unknown as {type: React.ComponentType<any>}).type;
let tree: renderer.ReactTestRenderer;
const deferred = () => {let resolve!: (value: any) => void; return {promise: new Promise(r => {resolve = r;}), resolve: (value: any) => resolve(value)};};
const flush = async () => {for (let i = 0; i < 8; i++) await act(async () => {await Promise.resolve();});};
const record = (id: string, ordinal: number, prompt: string) => ({session_id: 'same-thread', record_id: id, turn_ordinal: ordinal, time: `2026-11-15T00:${String(ordinal - 190).padStart(2, '0')}:00Z`, prompt, response: ''});
const recent = {...record('recent', 200, 'different recent request'), history_before: 'cursor-recent'};
const bodies = ['repeat', 'repeat image\n/tmp/public-icon.png'];
const recordedPrompt = (body: string) => body.includes('.png') ? body + '\n![Image #1](/tmp/public-icon.png)' : body;
const bubbles = () => tree.root.findAllByType(View).filter(n => n.props.testID === 'chat-user-bubble');
const statuses = (id: string) => tree.root.findAllByType(Text).filter(n => n.props.testID === id);
const chat = () => tree.root.findByType(ChatView).props;
function press(id: string) {const button = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === id); expect(button).toBeDefined(); act(() => button!.props.onPress());}
async function mount(surface: string) {await act(async () => {tree = renderer.create(surface === 'HQ' ? <HQConsoleView agent={agent} workspaceMode /> : <DetailView agent={agent} initialMode="chat" />);}); await flush();}
async function send(body: string) {await act(async () => {tree.root.findByType(ComposerFn).props.onSend({text: body, enter: true});}); await flush();}
async function poll(turns: any[]) {mockClient.transcript.mockResolvedValue({turns, dropped: 0}); await act(async () => {jest.advanceTimersByTime(8000);}); await flush();}
beforeEach(() => {jest.useFakeTimers(); jest.clearAllMocks(); mockClient.transcript.mockResolvedValue({turns: [recent], dropped: 199}); mockClient.traceHistory.mockReset(); mockClient.sendResult.mockReset();});
afterEach(() => {act(() => tree?.unmount()); jest.clearAllTimers(); jest.useRealTimers();});

const combinations = ['HQ', 'Detail'].flatMap(surface => bodies.flatMap(body => ['loaded', 'in-flight'].map(history => ({surface, body, history}))));
test.each(combinations)('CRR-001 $surface $history earlier $body cannot acknowledge a send and true source keeps its bubble', async ({surface, body, history}) => {
  const page = deferred(), receipt = deferred();
  mockClient.traceHistory.mockReturnValue(page.promise); mockClient.sendResult.mockReturnValue(receipt.promise);
  await mount(surface); press('detail-chat-earlier'); await flush();
  const old = record('old', 199, recordedPrompt(body));
  if (history === 'loaded') {await act(async () => page.resolve([old])); await flush();}
  await send(body);
  if (history === 'in-flight') {await act(async () => page.resolve([old])); await flush();}
  expect(chat().turns).toEqual([recent]); // Parent baseline never included the earlier page.
  expect(statuses('chat-pending-echo')).toHaveLength(1);
  expect(statuses('chat-send-receipt')).toHaveLength(0);
  expect(bubbles()).toHaveLength(3);
  const pending = bubbles().at(-1);
  const [target, payload] = mockClient.sendResult.mock.calls[0];
  expect(target).toBe(agent.pane_id); expect(payload.text).toBe(body); expect(typeof payload.send_id).toBe('string');
  await act(async () => receipt.resolve({ok: true, delivery: 'accepted'})); await flush();
  expect(statuses('chat-pending-echo')).toHaveLength(1);
  expect(statuses('chat-send-receipt')).toHaveLength(0);
  // The true recent source is index0 but index2 after the earlier-prefix merge.
  await poll([record('true-new', 201, recordedPrompt(body))]);
  expect(statuses('chat-pending-echo')).toHaveLength(0);
  expect(statuses('chat-send-receipt')).toHaveLength(1);
  expect(bubbles()).toHaveLength(3); expect(bubbles().at(-1)).toBe(pending);
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});

test.each(['HQ', 'Detail'])('CRR-001 %s new source before a late receipt cannot revive or remount an echo', async surface => {
  const body = bodies[1], receipt = deferred(); mockClient.traceHistory.mockResolvedValue([record('old', 199, recordedPrompt(body))]); mockClient.sendResult.mockReturnValue(receipt.promise);
  await mount(surface); press('detail-chat-earlier'); await flush(); await send(body);
  expect(statuses('chat-pending-echo')).toHaveLength(1); const pending = bubbles().at(-1);
  await poll([recent, record('true-new', 201, recordedPrompt(body))]);
  expect(bubbles().at(-1)).toBe(pending); expect(statuses('chat-send-receipt')).toHaveLength(1);
  await act(async () => receipt.resolve({ok: true, delivery: 'accepted'})); await flush();
  expect(bubbles()).toHaveLength(3); expect(bubbles().at(-1)).toBe(pending); expect(statuses('chat-pending-echo')).toHaveLength(0);
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});

test.each(['HQ', 'Detail'].flatMap(surface => ['loaded', 'in-flight'].map(history => ({surface, history}))))('CRR-001 $surface $history refused send never labels an older image occurrence Received', async ({surface, history}) => {
  const body = bodies[1], page = deferred(), receipt = deferred(); mockClient.traceHistory.mockReturnValue(page.promise); mockClient.sendResult.mockReturnValue(receipt.promise);
  await mount(surface); press('detail-chat-earlier'); await flush();
  if (history === 'loaded') {await act(async () => page.resolve([record('old', 199, recordedPrompt(body))])); await flush();}
  await send(body);
  if (history === 'in-flight') {await act(async () => page.resolve([record('old', 199, recordedPrompt(body))])); await flush();}
  expect(statuses('chat-pending-echo')).toHaveLength(1);
  await act(async () => receipt.resolve({ok: false, reason: 'synthetic refusal', status: 409})); await flush();
  expect(statuses('chat-pending-echo')).toHaveLength(0); expect(statuses('chat-send-receipt')).toHaveLength(0); expect(bubbles()).toHaveLength(2);
  await poll([recent]); expect(statuses('chat-send-receipt')).toHaveLength(0); expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});
