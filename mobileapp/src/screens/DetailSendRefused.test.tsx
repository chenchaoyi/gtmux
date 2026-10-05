// A send the Mac refuses because it refuses THIS PHONE (401/403): the bar says pair
// again, with no retry that could never land, and the text goes back into the box, where
// it is kept as the pane's draft. The Detail screen rendered for real, the Mac replaced.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {NavigationContext} from '@react-navigation/native';
import {DetailView} from './DetailScreen';
import {Composer} from '../ui/Composer';
import {SendFailedBar} from '../ui/SendFailedBar';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));

let tree: renderer.ReactTestRenderer | undefined;
afterEach(() => {
  act(() => tree?.unmount());
  tree = undefined;
});
const flush = async () => {
  for (let i = 0; i < 6; i++) await act(async () => { await Promise.resolve(); });
};

async function mount(sendResult: jest.Mock, navigate?: jest.Mock) {
  const agent: any = {pane_id: '%12', session: 'dev', window: '0', pane: '0', loc: 'dev:0.0', agent: 'Claude Code',
    status: 'idle', task: '', latest: false, activity: false, source: 'tmux'};
  const client = {
    pane: jest.fn().mockResolvedValue({text: 'ready\n', cols: 80, cursor: {x: 0, up: 0, visible: true}}),
    transcript: jest.fn().mockResolvedValue({turns: [], dropped: 0}),
    sendResult,
    panes: jest.fn().mockResolvedValue([]),
    options: jest.fn().mockResolvedValue([]),
    tasks: jest.fn().mockResolvedValue([]),
    theme: jest.fn().mockResolvedValue(undefined),
    upload: jest.fn(),
  };
  (useAgents as jest.Mock).mockReturnValue({client, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false});
  (useApp as jest.Mock).mockReturnValue({
    pal: paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Mac'}, returnSends: false, defaultDetailMode: 'terminal',
  });
  const view = <DetailView agent={agent} initialMode="terminal" />;
  await act(async () => {
    tree = renderer.create(navigate ? <NavigationContext.Provider value={{navigate} as any}>{view}</NavigationContext.Provider> : view);
  });
  await flush();
  return tree!;
}
const send = async (t: renderer.ReactTestRenderer, text: string) => {
  await act(async () => {
    t.root.findByType(Composer).props.onSend({text, enter: true});
  });
  await flush();
};

test('refused: pair again, no retry, and the text is back in the box', async () => {
  const navigate = jest.fn();
  const sendResult = jest.fn().mockResolvedValue({ok: false, status: 401, reason: 'unauthorized'});
  const t = await mount(sendResult, navigate);
  await send(t, 'please look at the build');
  const bar = t.root.findByType(SendFailedBar);
  expect(bar.props.status).toBe(401);
  expect(t.root.findAllByProps({testID: 'send-failed-retry'})).toHaveLength(0);
  expect(t.root.findByType(Composer).props.prefill).toMatchObject({text: 'please look at the build'});
  await act(async () => {
    t.root.findByProps({testID: 'send-failed-pair-again'}).props.onPress();
  });
  expect(navigate).toHaveBeenCalledWith('Servers');
  expect(sendResult).toHaveBeenCalledTimes(1); // nothing re-sent behind the reader's back
});

test('any other refusal keeps its retry and does not refill the box', async () => {
  const sendResult = jest.fn().mockResolvedValue({ok: false, status: 409, reason: 'send failed: not confirmed'});
  const t = await mount(sendResult);
  await send(t, 'run the tests');
  expect(t.root.findByProps({testID: 'send-failed-retry'})).toBeDefined();
  expect(t.root.findByType(Composer).props.prefill).toBeNull();
});
