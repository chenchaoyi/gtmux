import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {TouchableOpacity, View} from 'react-native';
import {HQConsoleView} from './HQScreen';
import {DetailView} from './DetailScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {MessageReceipt} from '../ui/MessageReceipt';

jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext, useAgentsOptional: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Fixture'}, returnSends: false, defaultDetailMode: 'chat'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));

const agent: any = {pane_id: '%fixture', session: 'fixture', window: '0', pane: '0', loc: 'fixture:0.0', session_id: 'thread', home_id: 'home', agent: 'Codex', status: 'idle', source: 'tmux', role: 'supervisor'};
const mockClient = {
  base: 'https://fixture.invalid', token: 'fixture', sendResult: jest.fn(), transcript: jest.fn(), traceHistory: jest.fn(),
  pane: jest.fn(async () => ({text: '', cols: 80})), panes: jest.fn(async () => []), options: jest.fn(async () => []), tasks: jest.fn(async () => []),
  theme: jest.fn(async () => undefined), digest: jest.fn(async () => []), usage: jest.fn(async () => null), hqBoard: jest.fn(async () => ({exists: false})),
  hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})), hqEvents: jest.fn(async () => []),
};
const mockContext = {client: mockClient, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false};
const ComposerFn = (Composer as unknown as {type: React.ComponentType<any>}).type;
let tree: renderer.ReactTestRenderer;
const flush = async () => {for (let i = 0; i < 8; i++) await act(async () => {await Promise.resolve();});};
const marks = () => tree.root.findAllByType(View).filter(n => n.props.testID === 'chat-pending-echo');
const chat = () => tree.root.findByType(ChatView).props;
async function advance(ms: number) {
  // Let real promise continuations settle between ticks, as they do in an active UI.
  while (ms > 0) {const step = Math.min(ms, 1000); await act(async () => {jest.advanceTimersByTime(step);}); await flush(); ms -= step;}
}
async function press(id: string) {
  const button = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === id);
  expect(button).toBeDefined();
  await act(async () => {button!.props.onPress();});
  await flush();
}
async function mount(surface = 'HQ') {
  await act(async () => {tree = renderer.create(surface === 'HQ'
    ? <HQConsoleView agent={agent} layout="compact" workspaceMode={false}/>
    : <DetailView agent={agent} initialMode="chat"/>);});
  await flush();
}
async function send(text: string) {
  await act(async () => {tree.root.findByType(ComposerFn).props.onSend({text, enter: true});});
  await flush();
}
beforeEach(() => {
  jest.useFakeTimers({now: Date.UTC(2026, 10, 15)});
  jest.clearAllMocks();
  mockClient.transcript.mockResolvedValue({turns: [], dropped: 0});
  mockClient.sendResult.mockResolvedValue({ok: true, delivery: 'accepted'});
});
afterEach(() => {act(() => tree?.unmount()); jest.clearAllTimers(); jest.useRealTimers();});

test('CAR-004 actual compact HQ tab remount uses the original successful-send deadline', async () => {
  await mount(); await send('one pending message');
  const id = mockClient.sendResult.mock.calls[0][1].send_id;
  expect(marks()).toHaveLength(1);
  await advance(1000); await press('hq-tab-calls');
  expect(tree.root.findAllByType(ChatView)).toHaveLength(0);
  await advance(500); await press('hq-tab-console');
  expect(marks()).toHaveLength(1);
  await advance(999); expect(marks()).toHaveLength(1);
  await advance(1); expect(marks()).toHaveLength(0);
  await press('hq-tab-calls'); await advance(1000); await press('hq-tab-console');
  expect(marks()).toHaveLength(0);
  expect(chat().pendingEcho.id).toBe(id);
  expect(chat().pendingPrompt).toBe('one pending message');
  await press('hq-tab-calls'); await advance(2500); await press('hq-tab-console');
  expect(marks()).toHaveLength(0);
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});

test.each(['HQ', 'Detail'])('CAR-004 %s successful ack owner gives a new send an independent lifetime', async surface => {
  await mount(surface); await send('first');
  const first = chat().pendingEcho;
  expect(first.receiptExpiresAt).toBe(Date.now() + 2500);
  await advance(1000); await send('second');
  const second = chat().pendingEcho;
  expect(second.id).not.toBe(first.id);
  expect(second.receiptExpiresAt).toBe(Date.now() + 2500);
  await advance(1500); expect(marks()).toHaveLength(1);
  expect(chat().pendingPrompt).toBe('second');
  await advance(1000); expect(marks()).toHaveLength(0);
  expect(chat().pendingPrompt).toBe('second');
  expect(mockClient.sendResult).toHaveBeenCalledTimes(2);
});

test.each(['queued', 'sending'])('CAR-004 HQ %s survives navigation without a success expiry', async delivery => {
  mockClient.sendResult.mockImplementation(() => delivery === 'queued'
    ? Promise.resolve({ok: true, delivery: 'queued'}) : new Promise(() => {}));
  await mount(); await send('kept work');
  await advance(10000); await press('hq-tab-calls'); await advance(1000); await press('hq-tab-console');
  expect(marks()).toHaveLength(1);
  expect(marks()[0].props.accessibilityLabel).toBe(delivery === 'queued' ? 'Queued · waiting for the next turn' : 'Sending…');
  expect(chat().pendingEcho.receiptExpiresAt).toBeUndefined();
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});

test.each(['HQ', 'Detail'])('CAR-004 %s source confirmation before a late ack stays quiet', async surface => {
  let accept!: (result: any) => void;
  mockClient.sendResult.mockReturnValue(new Promise(resolve => {accept = resolve;}));
  await mount(surface); await send('recorded first');
  const echo = chat().pendingEcho;
  mockClient.transcript.mockResolvedValue({turns: [{record_id: 'new', session_id: 'thread', prompt: 'recorded first', response: 'answer', time: new Date().toISOString()}], dropped: 0});
  await act(async () => {chat().onRetry();}); await flush();
  expect(marks()).toHaveLength(0);
  await act(async () => {accept({ok: true, delivery: 'accepted'});}); await flush();
  expect(marks()).toHaveLength(0);
  expect(echo.receiptExpiresAt).toBeUndefined();
  expect(chat().pendingPrompt).toBeFalsy();
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});


test('CAR-004 successful feedback without a known owner deadline never invents a lifetime', async () => {
  await act(async () => {tree = renderer.create(<MessageReceipt delivered delivery="accepted" zh={false}/>);});
  expect(marks()).toHaveLength(0);
  await advance(3500);
  await act(async () => {tree.update(<MessageReceipt key="remount" delivered delivery="accepted" zh={false}/>);});
  expect(marks()).toHaveLength(0);
});
