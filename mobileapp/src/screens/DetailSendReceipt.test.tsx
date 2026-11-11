import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {SendFailedBar} from '../ui/SendFailedBar';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';
import {SendDeadlineError} from '../api/sendDeadline';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
const ComposerFn = (Composer as unknown as {type: React.ComponentType<any>}).type;
let tree: renderer.ReactTestRenderer;
const flush = async () => {for (let i = 0; i < 8; i++) await act(async () => {await Promise.resolve();});};
beforeEach(() => {jest.useFakeTimers();});
afterEach(() => {act(() => tree?.unmount()); jest.useRealTimers();});
async function mount(sendResult: jest.Mock, mode: 'terminal' | 'chat') {
  const agent: any = {pane_id: '%1', session: 'synthetic', window: '0', pane: '0', loc: 'synthetic:0.0', agent: 'Codex', status: 'working', task: '', latest: false, activity: false, source: 'tmux'};
  const client = {sendResult, pane: jest.fn().mockResolvedValue({text: 'ready', cols: 80}), transcript: jest.fn().mockResolvedValue({turns: [], dropped: 0}), panes: jest.fn().mockResolvedValue([]), options: jest.fn().mockResolvedValue([]), tasks: jest.fn().mockResolvedValue([]), theme: jest.fn().mockResolvedValue(undefined)};
  (useAgents as jest.Mock).mockReturnValue({client, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false});
  (useApp as jest.Mock).mockReturnValue({pal: paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Mac'}, returnSends: false, defaultDetailMode: mode});
  await act(async () => {tree = renderer.create(<DetailView agent={agent} initialMode={mode} />);}); await flush(); return client;
}
async function send(text: string) {await act(async () => {tree.root.findByType(ComposerFn).props.onSend({text, enter: true});}); await flush();}

test.each(['chat', 'terminal'] as const)('MSD-005 %s releases failed sends and preserves explicit retry identity', async mode => {
  const sendResult = jest.fn().mockRejectedValueOnce(new SendDeadlineError()).mockResolvedValue({ok: true, delivery: 'queued'});
  await mount(sendResult, mode); await send('synthetic prompt');
  expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(1);
  expect(sendResult).toHaveBeenCalledTimes(1);
  const id = sendResult.mock.calls[0][1].send_id;
  await act(async () => tree.root.findByProps({testID: 'send-failed-retry'}).props.onPress()); await flush();
  expect(sendResult).toHaveBeenCalledTimes(2); expect(sendResult.mock.calls[1][1].send_id).toBe(id);
  if (mode === 'terminal') {
    await act(async () => tree.root.findByProps({testID: 'detail-mode-chat'}).props.onPress());
    await act(async () => {jest.advanceTimersByTime(64);});
    await flush();
  }
  expect(tree.root.findByType(ChatView).props).toMatchObject({pendingPrompt: 'synthetic prompt', pendingDelivered: true, pendingDelivery: 'queued'});
});

test('MSD-005 old failure cannot replace the newer accepted send', async () => {
  let reject!: (error: Error) => void;
  const sendResult = jest.fn().mockReturnValueOnce(new Promise((_, fail) => {reject = fail;})).mockResolvedValue({ok: true, delivery: 'accepted'});
  await mount(sendResult, 'chat'); await send('first'); await send('second');
  await act(async () => reject(new SendDeadlineError())); await flush();
  expect(tree.root.findByType(ChatView).props).toMatchObject({pendingPrompt: 'second', pendingDelivered: true, pendingDelivery: 'accepted'});
  expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(0);
});

test('MSD-005 a control key cannot leave a prior text receipt permanently Sending', async () => {
  let accept!: (r: any) => void;
  const sendResult = jest.fn().mockReturnValueOnce(new Promise(resolve => {accept = resolve;})).mockResolvedValue({ok: true});
  await mount(sendResult, 'chat'); await send('synthetic prompt');
  await act(async () => tree.root.findByType(ComposerFn).props.onSend({key: 'Enter'})); await flush();
  await act(async () => accept({ok: true, delivery: 'accepted'})); await flush();
  expect(tree.root.findByType(ChatView).props).toMatchObject({pendingPrompt: 'synthetic prompt', pendingDelivered: true, pendingDelivery: 'accepted'});
});
