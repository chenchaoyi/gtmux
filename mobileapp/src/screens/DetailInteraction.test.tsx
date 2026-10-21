import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {Composer} from '../ui/Composer';
import {SessionInteractionCard} from '../ui/SessionInteractionCard';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
let tree: renderer.ReactTestRenderer;
afterEach(() => act(() => tree?.unmount()));
const flush = async () => {for (let i = 0; i < 8; i++) await act(async () => {await Promise.resolve();});};
async function mount(status = 'waiting', guest = false) {
  const agent: any = {pane_id: '%12', session: 'test', window: '0', pane: '0', agent: 'Codex', status, source: 'tmux'};
  const client = {base: 'https://mac', pane: jest.fn().mockResolvedValue({text: 'screen', cols: 80}), transcript: jest.fn().mockResolvedValue({turns: [], dropped: 0}),
    interaction: jest.fn().mockResolvedValue({text: 'Allow test tool?\n› 1. Allow once\n  2. Cancel', options: [{n: 1, label: 'Allow once'}, {n: 2, label: 'Cancel'}], awaiting_reply: true, truncated: false}),
    sendResult: jest.fn().mockResolvedValue({ok: true}), panes: jest.fn().mockResolvedValue([]), tasks: jest.fn().mockResolvedValue([]), theme: jest.fn().mockResolvedValue(undefined)};
  (useAgents as jest.Mock).mockReturnValue({client, agents: [agent], conn: 'live', isGuest: guest, inputPanes: [], demo: false});
  (useApp as jest.Mock).mockReturnValue({pal: paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Mac'}, returnSends: false, defaultDetailMode: 'chat'});
  await act(async () => {tree = renderer.create(<DetailView agent={agent} initialMode="chat" />);});await flush();
  return client;
}
test('Codex permission choices appear in Chat without visiting Terminal, and a rapid second tap cannot answer the next prompt', async () => {
  const client = await mount();
  expect(client.interaction).toHaveBeenCalledWith('%12', expect.anything());
  const choice = tree.root.findByProps({testID: 'reply-1'});
  await act(async () => {choice.props.onPress();choice.props.onPress();});await flush();
  expect(client.sendResult).toHaveBeenCalledTimes(1);
  expect(client.sendResult.mock.calls[0]).toEqual(['%12', expect.objectContaining({text: '1'})]);
  expect(client.sendResult.mock.calls[0][1].enter).toBeUndefined();
});
test('manual interaction reads an unrecognized prompt without changing the session status and reuses the composer controller', async () => {
  const client = await mount('idle');
  expect(client.interaction).not.toHaveBeenCalled();
  await act(async () => tree.root.findByProps({testID: 'detail-interact'}).props.onPress());await flush();
  expect(client.interaction).toHaveBeenCalledTimes(1);
  const card = tree.root.findByType(SessionInteractionCard);
  const composer = tree.root.findByType((Composer as unknown as {type: React.ComponentType<any>}).type);
  expect(card.props.waiting).toBe(false);expect(card.props.control).toBe(composer.props.keyController);
});
test('a view-only guest sees the prompt but cannot choose or type', async () => {
  const client = await mount('waiting', true);
  expect(tree.root.findByProps({testID: 'reply-1'}).props.disabled).toBe(true);
  await act(async () => tree.root.findByProps({testID: 'reply-1'}).props.onPress());
  expect(client.sendResult).not.toHaveBeenCalled();
});
