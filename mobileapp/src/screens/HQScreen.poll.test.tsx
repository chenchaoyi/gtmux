import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {UsageSheet} from './UsageSheet';
import {KnowledgeSheet} from './KnowledgeSheet';
import {HQResourceRequest} from '../state/WorkspaceContext';
import {HQView} from './HQScreen';
import {ChatView} from '../ui/ChatView';
import {Agent} from '../api/types';

// The visible HQ console re-reads its transcript whatever the status. It used to poll
// only while working, so an idle or waiting HQ whose conversation moved without a
// status change kept showing the old reply (%12's reproduction, 2026-10-06).
jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/ChatView', () => ({ChatView: () => null}));
jest.mock('../ui/Composer', () => ({Composer: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));

const mockClient = {
  tasks: jest.fn(async () => []),
  digest: jest.fn(async () => []),
  usage: jest.fn(async () => null),
  hqBoard: jest.fn(async () => ({exists: false})),
  hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})),
  hqEvents: jest.fn(async () => []),
  pane: jest.fn(async () => ({text: ''})),
  transcript: jest.fn(),
  base: 'https://example.invalid',
  token: 'synthetic-only',
};
const mockContext = {client: mockClient, agents: [] as Agent[], conn: 'connected', demo: false};

let tree: renderer.ReactTestRenderer | undefined;
const shown = () => tree!.root.findByType(ChatView).props.turns[0]?.response;
const calls = () => mockClient.transcript.mock.calls.length;

async function mount(status: string, layout: 'regular' | 'compact', openResource?: HQResourceRequest) {
  const hq = {pane_id: '%hq', agent: 'Claude Code', status, role: 'supervisor'} as Agent;
  mockContext.agents = [hq];
  await act(async () => {
    tree = renderer.create(<HQView agent={hq} layout={layout} openResource={openResource} />);
  });
}

beforeEach(() => {
  jest.useFakeTimers();
  globalThis.fetch = jest.fn(() => {
    throw new Error('no real HTTP in this test');
  }) as unknown as typeof fetch;
  mockClient.transcript.mockReset();
});
afterEach(() => {
  if (tree) act(() => tree!.unmount());
  tree = undefined;
  jest.useRealTimers();
});

describe.each(['regular', 'compact'] as const)('%s layout', layout => {
  test.each(['idle', 'waiting', 'working'] as const)('%s: a new reply arrives with no status change', async status => {
    let reply = 'old reply';
    mockClient.transcript.mockImplementation(async () => ({turns: [{prompt: 'p', response: reply}], etag: 'W/"1"'}));
    await mount(status, layout);
    expect(shown()).toBe('old reply');
    reply = 'new reply';
    await act(async () => {
      jest.advanceTimersByTime(10_000);
    });
    expect(shown()).toBe('new reply');
  });
});

test('an unchanged conversation keeps its turns, and the next request carries the ETag', async () => {
  mockClient.transcript
    .mockImplementationOnce(async () => ({turns: [{prompt: 'p', response: 'kept'}], etag: 'W/"7"'}))
    .mockImplementation(async () => ({turns: [], etag: 'W/"7"', unchanged: true}));
  await mount('idle', 'regular');
  await act(async () => {
    jest.advanceTimersByTime(10_000);
  });
  expect(calls()).toBeGreaterThan(1);
  expect(mockClient.transcript.mock.calls[1][1]).toBe('W/"7"');
  expect(shown()).toBe('kept');
});

test('a slow request is not stacked: one at a time', async () => {
  mockClient.transcript
    .mockImplementationOnce(async () => ({turns: [{prompt: 'p', response: 'first'}], etag: 'W/"1"'}))
    .mockImplementation(() => new Promise(() => {})); // never answers
  await mount('idle', 'regular');
  await act(async () => {
    jest.advanceTimersByTime(60_000);
  });
  expect(calls()).toBe(2); // the first load, then one that is still waiting
});

test('nothing is asked or set after the console goes away', async () => {
  mockClient.transcript.mockImplementation(async () => ({turns: [{prompt: 'p', response: 'r'}], etag: 'W/"1"'}));
  await mount('idle', 'regular');
  const before = calls();
  act(() => tree!.unmount());
  tree = undefined;
  await act(async () => {
    jest.advanceTimersByTime(60_000);
  });
  expect(calls()).toBe(before);
});


test.each(['usage', 'knowledge'] as const)('shortcut opens only %s and can be requested again on an existing HQ view', async page => {
  mockClient.transcript.mockResolvedValue({turns: []});
  await mount('idle', 'compact', {page, at: 1});
  expect(tree!.root.findByType(UsageSheet).props.visible).toBe(page === 'usage');
  expect(tree!.root.findByType(KnowledgeSheet).props.visible).toBe(page === 'knowledge');
  const sheet = tree!.root.findByType(page === 'usage' ? UsageSheet : KnowledgeSheet);
  act(() => sheet.props.onClose());
  expect(sheet.props.visible).toBe(false);
  await act(async () => tree!.update(<HQView agent={mockContext.agents[0]} layout="compact" openResource={{page, at: 2}} />));
  expect(tree!.root.findByType(page === 'usage' ? UsageSheet : KnowledgeSheet).props.visible).toBe(true);
});

test.each(['usage', 'knowledge'] as const)('%s shortcut distinguishes loading and failed reads from an empty resource', async page => {
  mockClient.transcript.mockResolvedValue({turns: []});
  let reject!: (e: Error) => void;
  const response = new Promise<never>((_r, j) => {reject = j;});
  if (page === 'usage') mockClient.usage.mockImplementationOnce(() => response);
  else mockClient.hqKnowledge.mockImplementationOnce(() => response);
  await mount('idle', 'compact', {page, at: 1});
  const sheet = () => tree!.root.findByType(page === 'usage' ? UsageSheet : KnowledgeSheet);
  const loading = page === 'usage' ? 'loading' : 'indexLoading';
  const error = page === 'usage' ? 'loadError' : 'indexError';
  expect(sheet().props[loading]).toBe(true);
  expect(tree!.root.findAllByProps({testID: `${page}-load-state`}).length).toBeGreaterThan(0);
  await act(async () => reject(new Error('offline')));
  expect(sheet().props[error]).toBe(true);
  expect(sheet().props[loading]).toBe(false);
  expect(tree!.root.findAllByProps({testID: `${page}-load-state`}).length).toBeGreaterThan(0);
  await act(async () => jest.advanceTimersByTime(3000));
  expect(sheet().props[error]).toBe(false);
  expect(tree!.root.findAllByProps({testID: `${page}-load-state`})).toHaveLength(0);
});
