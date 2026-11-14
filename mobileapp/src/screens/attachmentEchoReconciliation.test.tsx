import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {HQConsoleView} from './HQScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';

jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext, useAgentsOptional: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Synthetic'}, returnSends: false, defaultDetailMode: 'chat'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));
jest.mock('../ui/ChatView', () => {
  const ReactModule = require('react');
  const actual = jest.requireActual('../ui/ChatView');
  return {...actual, ChatView: function ObservedChatView(props: any) {
    // Capture committed frames, including the one before passive effect cleanup.
    ReactModule.useLayoutEffect(() => {
      mockFrames.push({pending: props.pendingPrompt, prompts: props.turns.map((t: any) => t.prompt)});
    });
    return ReactModule.createElement(actual.ChatView, props);
  }};
});

const mockFrames: {pending?: string; prompts: string[]}[] = [];
const agent: any = {pane_id: '%1', session: 'synthetic', window: '0', pane: '0', loc: 'synthetic:0.0', session_id: 'synthetic-thread', home_id: 'synthetic-home', agent: 'Codex', status: 'working', task: '', latest: false, activity: false, source: 'tmux', role: 'supervisor'};
const mockClient = {
  base: 'https://synthetic.invalid', token: 'synthetic', sendResult: jest.fn(),
  pane: jest.fn(async () => ({text: 'ready', cols: 80})), transcript: jest.fn(),
  panes: jest.fn(async () => []), options: jest.fn(async () => []), tasks: jest.fn(async () => []),
  theme: jest.fn(async () => undefined), digest: jest.fn(async () => []), usage: jest.fn(async () => null),
  hqBoard: jest.fn(async () => ({exists: false})),
  hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})),
  hqEvents: jest.fn(async () => []),
};
const mockContext = {client: mockClient, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false};
const ComposerFn = (Composer as unknown as {type: React.ComponentType<any>}).type;
let tree: renderer.ReactTestRenderer;
const flush = async () => {for (let i = 0; i < 8; i++) await act(async () => {await Promise.resolve();});};
const turn = (prompt: string, time: string) => ({prompt, time, response: '', session_id: 'synthetic-thread'});
const text = 'Look at this\n/tmp/echo-fixture.png';
const recorded = text + '\n![Image #1](/tmp/echo-fixture.png)\n';
async function mount(surface: string) {
  await act(async () => {tree = renderer.create(surface === 'HQ' ? <HQConsoleView agent={agent} workspaceMode /> : <DetailView agent={agent} initialMode="chat" />);});
  await flush();
}
async function send(body: string) {
  await act(async () => {tree.root.findByType(ComposerFn).props.onSend({text: body, enter: true});});
  await flush();
}
async function poll(turns: any[]) {
  mockClient.transcript.mockResolvedValue({turns, dropped: 0});
  await act(async () => {jest.advanceTimersByTime(8000);});
  await flush();
}
const chat = () => tree.root.findByType(ChatView).props;
beforeEach(() => {
  jest.useFakeTimers(); jest.clearAllMocks(); mockFrames.length = 0;
  mockClient.transcript.mockResolvedValue({turns: [], dropped: 0});
  mockClient.sendResult.mockReset().mockResolvedValue({ok: true, delivery: 'accepted'});
});
afterEach(() => {act(() => tree?.unmount()); jest.useRealTimers();});

test.each(['HQ', 'Detail'])('AEC-002 %s retires image echo in its first recorded commit and ignores late receipt', async surface => {
  let accept!: (value: any) => void;
  mockClient.sendResult.mockReturnValue(new Promise(resolve => {accept = resolve;}));
  await mount(surface); await send(text);
  expect(chat().pendingPrompt).toBe(text);
  mockFrames.length = 0;
  await poll([turn(recorded, '2026-11-14T00:01:00Z')]);
  expect(chat().turns).toHaveLength(1);
  expect(chat().pendingPrompt).toBeFalsy();
  expect(mockFrames.filter(frame => frame.prompts.includes(recorded) && frame.pending === text)).toHaveLength(0);
  expect(tree.root.findAllByProps({testID: 'chat-pending-echo'})).toHaveLength(0);
  await act(async () => {accept({ok: true, delivery: 'accepted'});}); await flush();
  expect(chat().pendingPrompt).toBeFalsy();
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});

test.each(['HQ', 'Detail'])('AEC-002 %s keeps an old identical occurrence pending but retires a new one', async surface => {
  const old = turn(recorded, '2026-11-14T00:00:00Z');
  mockClient.transcript.mockResolvedValue({turns: [old], dropped: 0});
  await mount(surface); await send(text); await poll([old]);
  expect(chat().pendingPrompt).toBe(text);
  expect(chat().pendingDelivered).toBe(true);
  mockFrames.length = 0;
  await poll([old, turn(recorded, '2026-11-14T00:01:00Z')]);
  expect(chat().pendingPrompt).toBeFalsy();
  expect(mockFrames.filter(frame => frame.prompts.length === 2 && frame.pending === text)).toHaveLength(0);
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});

test.each(['HQ', 'Detail'])('AEC-002 %s retires plain-text echo without one duplicate committed frame', async surface => {
  await mount(surface); await send('plain instruction');
  mockFrames.length = 0;
  await poll([turn('plain instruction', '2026-11-14T00:01:00Z')]);
  expect(chat().pendingPrompt).toBeFalsy();
  expect(mockFrames.filter(frame => frame.prompts.includes('plain instruction') && frame.pending === 'plain instruction')).toHaveLength(0);
  expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
});
