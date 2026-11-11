import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';
import {NativeTerm} from '../ui/NativeTerm';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
const ComposerFn = (Composer as unknown as {type: React.ComponentType<any>}).type;
let tree: renderer.ReactTestRenderer;
const flush = async () => {for (let i = 0; i < 8; i++) await act(async () => {await Promise.resolve();});};
afterEach(() => {act(() => tree?.unmount()); jest.useRealTimers();});
async function mount(sendResult: jest.Mock, mode: 'terminal' | 'chat') {
  const agent: any = {pane_id: '%1', session: 'synthetic', window: '0', pane: '0', loc: 'synthetic:0.0', agent: 'Codex', status: 'working', task: '', latest: false, activity: false, source: 'tmux'};
  const client = {sendResult, pane: jest.fn().mockResolvedValue({text: 'ready', cols: 80}), transcript: jest.fn().mockResolvedValue({turns: [], dropped: 0}), panes: jest.fn().mockResolvedValue([]), options: jest.fn().mockResolvedValue([]), tasks: jest.fn().mockResolvedValue([]), theme: jest.fn().mockResolvedValue(undefined)};
  (useAgents as jest.Mock).mockReturnValue({client, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false});
  (useApp as jest.Mock).mockReturnValue({pal: paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Mac'}, returnSends: false, defaultDetailMode: mode});
  await act(async () => {tree = renderer.create(<DetailView agent={agent} initialMode={mode} />);}); await flush(); return client;
}
async function send(text: string) {await act(async () => {tree.root.findByType(ComposerFn).props.onSend({text, enter: true});}); await flush();}

test.each(['legacy', 'native'] as const)('MSD-009 late text receipt cannot replace newer %s control-key screen', async route => {
  jest.useFakeTimers();
  let accept!: (r: any) => void;
  const sendResult = jest.fn().mockReturnValueOnce(new Promise(resolve => {accept = resolve;})).mockResolvedValue({ok: true, pane: {text:'newer-screen', cols:80}});
  await mount(sendResult, 'terminal'); await send('synthetic prompt');
  await act(async () => tree.root.findByType(ComposerFn).props[route === 'legacy' ? 'onSend' : 'onSendKey']({key: 'Enter'})); await flush();
  expect(tree.root.findByType(NativeTerm).props.text).toBe('newer-screen');
  await act(async () => accept({ok: true, delivery:'accepted', pane:{text:'old-screen',cols:80}})); await flush();
  expect(tree.root.findByType(NativeTerm).props.text).toBe('newer-screen');
  await act(async () => tree.root.findByProps({testID: 'detail-mode-chat'}).props.onPress()); await act(async () => {jest.advanceTimersByTime(64);}); await flush();
  expect(tree.root.findByType(ChatView).props).toMatchObject({pendingDelivered:true,pendingDelivery:'accepted'});
});
