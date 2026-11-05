import React from 'react';
import { TextInput } from 'react-native';
import renderer, { act } from 'react-test-renderer';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { DetailView } from './DetailScreen';
import { useAgents } from '../state/AgentsContext';
import { useApp } from '../state/AppContext';
import { paletteFor } from '../ui/theme';
import { draftKeyFor } from '../state/drafts';
import type { Agent } from '../api/types';
jest.mock('../state/AgentsContext', () => ({
  useAgents: jest.fn(),
  useAgentsOptional: jest.fn(),
}));
jest.mock('../state/AppContext', () => ({ useApp: jest.fn() }));
const makeClient = (base: string) => ({
  base,
  token: 'synthetic',
  pane: jest.fn(async () => ({ text: 'Synthetic', cols: 80 })),
  transcript: jest.fn(async () => ({ turns: [], dropped: 0 })),
  sendResult: jest.fn(async () => ({ ok: true })),
  panes: jest.fn(async () => []),
  tasks: jest.fn(async () => []),
  theme: jest.fn(async () => undefined),
});
let tree: renderer.ReactTestRenderer,
  agent: Agent,
  client: ReturnType<typeof makeClient>;
let saved: Record<string, string>;
const flush = () =>
  act(async () => {
    for (let i = 0; i < 8; i++) await Promise.resolve();
  });
async function render() {
  (useAgents as jest.Mock).mockReturnValue({
    client,
    agents: [agent],
    conn: 'live',
    isGuest: false,
    inputPanes: [],
    demo: false,
  });
  await act(async () => {
    if (tree) tree.update(<DetailView agent={agent} initialMode="chat" />);
    else
      tree = renderer.create(<DetailView agent={agent} initialMode="chat" />);
  });
  await flush();
}
function input() {
  if (!tree.root.findAllByType(TextInput).length)
    act(() =>
      tree.root.findByProps({ testID: 'composer-kbd' }).props.onPress(),
    );
  return tree.root.findAllByType(TextInput)[0];
}
beforeEach(() => {
  jest.useFakeTimers();
  saved = {};
  client = makeClient('https://a.invalid');
  agent = {
    pane_id: '%12',
    session: 'fixture',
    session_id: 'session-a',
    window: '0',
    pane: '0',
    agent: 'Codex',
    status: 'working',
    source: 'tmux',
  } as Agent;
  (useApp as jest.Mock).mockReturnValue({
    pal: paletteFor('dark'),
    lang: 'en',
    fontPref: 'auto',
    mac: { name: 'Fixture' },
    returnSends: false,
    defaultDetailMode: 'chat',
  });
  (AsyncStorage.getItem as jest.Mock).mockImplementation(
    async (k: string) => saved[k] ?? null,
  );
  (AsyncStorage.setItem as jest.Mock).mockImplementation(
    async (k: string, v: string) => {
      saved[k] = v;
    },
  );
});
afterEach(() => {
  act(() => tree?.unmount());
  tree = undefined as any;
  jest.useRealTimers();
});
test('mounted Detail isolates drafts on server and session switches and sends only to the current client', async () => {
  await render();
  const a = input();
  act(() => a.props.onChangeText('Mac A draft'));
  await act(async () => jest.advanceTimersByTime(450));
  await flush();
  const old = client;
  client = makeClient('https://b.invalid');
  await render();
  expect(input().props.value).toBe('');
  const b = input();
  act(() => b.props.onChangeText('Mac B draft'));
  await act(async () => jest.advanceTimersByTime(450));
  await flush();
  agent = { ...agent, session_id: 'session-b' };
  await render();
  expect(input().props.value).toBe('');
  const c = input();
  act(() => c.props.onChangeText('Current session only'));
  await act(async () =>
    tree.root.findByProps({ testID: 'composer-send' }).props.onPress(),
  );
  await flush();
  expect(old.sendResult).not.toHaveBeenCalled();
  expect(client.sendResult).toHaveBeenCalledWith(
    '%12',
    expect.objectContaining({ text: 'Current session only' }),
  );
  const drafts = JSON.parse(saved['gtmux.drafts']);
  expect(drafts[draftKeyFor(old.base, '%12', 'session-a')!].text).toBe(
    'Mac A draft',
  );
  expect(drafts[draftKeyFor(client.base, '%12', 'session-a')!].text).toBe(
    'Mac B draft',
  );
});
test('late draft storage result from a retired Detail cannot overwrite the next server input', async () => {
  let release!: (s: string) => void;
  (AsyncStorage.getItem as jest.Mock).mockImplementation((k: string) =>
    k === 'gtmux.drafts'
      ? new Promise<string>(r => {
          release = r;
        })
      : Promise.resolve(null),
  );
  await render();
  const oldLoad = release;
  client = makeClient('https://b.invalid');
  await render();
  const b = input();
  act(() => b.props.onChangeText('Typed on B'));
  await act(async () =>
    oldLoad(
      JSON.stringify({
        [draftKeyFor('https://a.invalid', '%12', 'session-a')!]: {
          text: 'Late A',
          at: Date.now(),
        },
      }),
    ),
  );
  expect(input().props.value).toBe('Typed on B');
  expect(client.sendResult).not.toHaveBeenCalled();
});
