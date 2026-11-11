import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {HQView} from './HQScreen';
import {ChatView} from '../ui/ChatView';
import {ConversationReadError} from '../api/client';
import {Agent} from '../api/types';

jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/ChatView', () => ({ChatView: () => null}));
jest.mock('../ui/Composer', () => ({Composer: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));

const mockClient = {
  base: 'https://synthetic.invalid', token: 'synthetic',
  tasks: jest.fn(async () => []), digest: jest.fn(async () => []),
  usage: jest.fn(async () => null), hqBoard: jest.fn(async () => ({exists: false})),
  hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})),
  hqEvents: jest.fn(async () => []), pane: jest.fn(async () => ({text: ''})),
  transcript: jest.fn(),
};
const agent = {pane_id: '%fixture', agent: 'Codex', status: 'idle', role: 'supervisor'} as Agent;
const mockContext = {client: mockClient, agents: [agent], conn: 'connected', demo: false};

test('CAS008 HQ association failure keeps history and clears after retry', async () => {
  jest.useFakeTimers();
  const oldFetch = globalThis.fetch;
  globalThis.fetch = jest.fn(() => {throw new Error('no real HTTP in this fixture');}) as unknown as typeof fetch;
  let tree: renderer.ReactTestRenderer | undefined;
  try {
    mockClient.transcript.mockResolvedValue({turns: [{prompt: 'kept question', response: 'kept HQ reply', time: ''}], dropped: 0});
    await act(async () => {tree = renderer.create(<HQView agent={agent} layout="compact" />);});
    const chat = () => tree!.root.findByType(ChatView).props;
    expect(chat().turns[0].response).toBe('kept HQ reply');
    mockClient.transcript.mockRejectedValueOnce(new ConversationReadError(409, true));
    await act(async () => chat().onRetry());
    expect(chat().associationFailed).toBe(true);
    expect(chat().turns[0].response).toBe('kept HQ reply');
    await act(async () => chat().onRetry());
    expect(chat().associationFailed).toBeFalsy();
  } finally {
    act(() => tree?.unmount());
    jest.clearAllTimers(); jest.useRealTimers(); globalThis.fetch = oldFetch;
  }
});
