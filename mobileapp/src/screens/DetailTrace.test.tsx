import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { DetailView } from './DetailScreen';
import { ChatView } from '../ui/ChatView';
import { TraceModal } from '../trace/TraceModal';
import { SessionToolsMenu } from '../ui/SessionToolsMenu';
import { useAgents } from '../state/AgentsContext';
import { useApp } from '../state/AppContext';
import { paletteFor } from '../ui/theme';
import { toAgent } from '../api/types';
jest.mock('../state/AgentsContext', () => ({
  useAgents: jest.fn(),
  useAgentsOptional: jest.fn(),
}));
jest.mock('../state/AppContext', () => ({ useApp: jest.fn() }));
jest.mock('../ui/ChatView', () => ({ ChatView: () => null }));
jest.mock('../trace/TraceModal', () => ({ TraceModal: () => null }));
let tree: renderer.ReactTestRenderer;
let client: any;
let agent: any;
const deferred = () => {
  let resolve: (value: any) => void = () => {};
  const promise = new Promise(r => {
    resolve = r;
  });
  return { promise, resolve };
};
const flush = async () => {
  await act(async () => {});
};
beforeEach(() => {
  jest.useFakeTimers();
  agent = toAgent({
    pane_id: '%fixture',
    source: 'tmux',
    agent: 'Claude Code',
    session_id: 'one',
    home_id: 'home-a',
    status: 'working',
  });
  client = {
    base: 'https://synthetic.invalid',
    transcript: jest
      .fn()
      .mockResolvedValue({
        turns: [{ prompt: 'request', response: 'answer' }],
        dropped: 0,
        etag: 'one',
      }),
    pane: jest.fn().mockResolvedValue({ text: '', cols: 80 }),
    options: jest.fn().mockResolvedValue([]),
    panes: jest.fn().mockResolvedValue([]),
    tasks: jest.fn().mockResolvedValue([]),
    theme: jest.fn().mockResolvedValue(undefined),
    sendResult: jest.fn(),
  };
  (useAgents as jest.Mock).mockImplementation(() => ({
    client,
    agents: [agent],
    conn: 'live',
    isGuest: false,
    inputPanes: [],
  }));
  (useApp as jest.Mock).mockReturnValue({
    pal: paletteFor('light'),
    lang: 'en',
    fontPref: 'auto',
    defaultDetailMode: 'chat',
  });
});
afterEach(() => {
  act(() => tree?.unmount());
  jest.clearAllTimers();
  jest.useRealTimers();
  jest.clearAllMocks();
});
async function mount() {
  await act(async () => {
    tree = renderer.create(<DetailView agent={agent} initialMode="chat" />);
  });
}
test('opening and closing Trace shares Chat’s in-flight read without aborting or starting a second subscription', async () => {
  const pending = deferred();
  client.transcript.mockReturnValueOnce(pending.promise);
  await mount();
  const signal = client.transcript.mock.calls[0][3];
  act(() => tree.root.findByType(SessionToolsMenu).props.onTrace());
  expect(signal.aborted).toBe(false);
  expect(client.transcript).toHaveBeenCalledTimes(1);
  act(() => tree.root.findByType(TraceModal).props.onClose());
  expect(signal.aborted).toBe(false);
  expect(client.transcript).toHaveBeenCalledTimes(1);
  await act(async () =>
    pending.resolve({
      turns: [{ response: 'shared' }],
      dropped: 0,
      etag: 'shared',
    }),
  );
  expect(tree.root.findByType(ChatView).props.turns[0].response).toBe('shared');
  expect(tree.root.findByType(TraceModal).props.turns[0].response).toBe(
    'shared',
  );
  await act(async () => {
    jest.advanceTimersByTime(2000);
  });
  expect(client.transcript).toHaveBeenCalledTimes(2);
});
test('the same pane and session in a different home cannot show or accept the old reply', async () => {
  const old = deferred();
  client.transcript.mockReturnValueOnce(old.promise);
  await mount();
  const oldSignal = client.transcript.mock.calls[0][3];
  const fresh = deferred();
  client.transcript.mockReturnValueOnce(fresh.promise);
  agent = { ...agent, home_id: 'home-b' };
  await act(async () =>
    tree.update(<DetailView agent={agent} initialMode="chat" />),
  );
  expect(oldSignal.aborted).toBe(true);
  expect(tree.root.findByType(ChatView).props.turns).toEqual([]);
  await act(async () =>
    old.resolve({ turns: [{ response: 'PRIVATE OLD' }], dropped: 0 }),
  );
  expect(tree.root.findByType(ChatView).props.turns).toEqual([]);
  await act(async () =>
    fresh.resolve({ turns: [{ response: 'new home' }], dropped: 0 }),
  );
  expect(tree.root.findByType(ChatView).props.turns[0].response).toBe(
    'new home',
  );
  await flush();
});

test.each([401, 403])(
  'pane denial %s closes tools and clears old records',
  async status => {
    const { ApiError } = require('../api/client');
    await mount();
    act(() => tree.root.findByType(SessionToolsMenu).props.onTrace());
    client.transcript.mockRejectedValueOnce(new ApiError(status, 'transcript'));
    await act(async () => {
      jest.advanceTimersByTime(2000);
    });
    expect(tree.root.findByType(TraceModal).props.visible).toBe(false);
    expect(tree.root.findByType(ChatView).props.turns).toEqual([]);
  },
);
