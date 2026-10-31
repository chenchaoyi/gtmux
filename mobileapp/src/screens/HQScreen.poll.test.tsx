import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Keyboard, type KeyboardEvent} from 'react-native';
import {UsageSheet} from './UsageSheet';
import {KnowledgeSheet} from './KnowledgeSheet';
import {HQResourceRequest} from '../state/WorkspaceContext';
import {HQView} from './HQScreen';
import {ChatView} from '../ui/ChatView';
import {HQHeader} from './HQHeader';
import {demoHQWorkspace} from '../ui/demoHQWorkspace';
import {Agent} from '../api/types';
import type {DigestRow} from '../api/client';

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
  hqWorkspace: undefined as undefined | jest.Mock,
  tasks: jest.fn(async () => []),
  digest: jest.fn(async (): Promise<DigestRow[]> => []),
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

async function mount(status: string, layout: 'regular' | 'compact', openResource?: HQResourceRequest, showInspector = true) {
  const hq = {pane_id: '%hq', agent: 'Claude Code', status, role: 'supervisor'} as Agent;
  mockContext.agents = [hq];
  await act(async () => {
    tree = renderer.create(<HQView agent={hq} layout={layout} openResource={openResource} showInspector={showInspector} />);
  });
}

beforeEach(() => {
  jest.useFakeTimers();
  mockClient.hqWorkspace = undefined;
  globalThis.fetch = jest.fn(() => {
    throw new Error('no real HTTP in this test');
  }) as unknown as typeof fetch;
  mockClient.transcript.mockReset();
});

test('a narrow iPad HQ uses zone tabs while keeping its resource sheets regular', async () => {
  mockClient.transcript.mockResolvedValue({turns: []});
  await mount('idle', 'regular', undefined, false);
  expect(tree!.root.findAllByProps({testID: 'hq-inspector'})).toHaveLength(0);
  expect(tree!.root.findAllByProps({testID: 'hq-tab-console'}).length).toBeGreaterThan(0);
  expect(tree!.root.findByType(KnowledgeSheet).props.layout).toBe('regular');
  expect(tree!.root.findByType(UsageSheet).props.layout).toBe('regular');
});
afterEach(() => {
  if (tree) act(() => tree!.unmount());
  tree = undefined;
  jest.useRealTimers();
  jest.restoreAllMocks();
});

test('the software keyboard hides HQ chrome and secondary composer rows, then restores them', async () => {
  const listeners = new Map<string, (event: KeyboardEvent) => void>();
  jest.spyOn(Keyboard, 'isVisible').mockReturnValue(false);
  jest.spyOn(Keyboard, 'addListener').mockImplementation((name, listener) => {
    listeners.set(name, listener);
    return {remove: () => { listeners.delete(name); }} as ReturnType<typeof Keyboard.addListener>;
  });
  mockClient.transcript.mockResolvedValue({turns: []});
  await mount('idle', 'compact');
  const chrome = () => tree!.root.findAllByProps({testID: 'hq-chrome'})[0];
  const chips = () => tree!.root.findAllByProps({testID: 'hq-chip-Brief'});
  expect(chips().length).toBeGreaterThan(0);
  await act(async () => listeners.get('keyboardWillShow')!({duration: 0, endCoordinates: {height: 200}} as KeyboardEvent));
  expect(chrome().props.pointerEvents).toBe('none');
  expect(chrome().props.accessibilityElementsHidden).toBe(true);
  expect(chips()).toHaveLength(0);
  await act(async () => listeners.get('keyboardWillHide')!({duration: 0, endCoordinates: {height: 0}} as KeyboardEvent));
  expect(chrome().props.accessibilityElementsHidden).toBe(false);
  expect(chips().length).toBeGreaterThan(0);
});

test('HQ zone tabs expose their selected state and change the active zone', async () => {
  mockClient.transcript.mockResolvedValue({turns: []});
  await mount('idle', 'compact');
  const tab = (zone: string) => tree!.root.findAllByProps({testID: `hq-tab-${zone}`})[0];
  expect(tab('console').props.accessibilityRole).toBe('tab');
  expect(tab('console').props.accessibilityState).toEqual({selected: true});
  await act(async () => tab('calls').props.onPress());
  expect(tab('calls').props.accessibilityState).toEqual({selected: true});
  expect(tab('console').props.accessibilityState).toEqual({selected: false});
});

test('a waiting card keeps its selection and two actions as separate buttons', async () => {
  mockClient.transcript.mockResolvedValue({turns: []});
  mockClient.digest.mockResolvedValueOnce([
    {pane_id: '%worker', loc: 'api', source: 'tmux', agent: 'Codex', status: 'waiting', ask: 'Run tests?'},
  ]);
  await mount('idle', 'compact');
  const button = (id: string) => tree!.root.findAllByProps({testID: id})[0];
  expect(button('hq-call-api').props.onPress).toBeUndefined();
  for (const id of ['hq-call-select-api', 'hq-call-open-api', 'hq-call-ask-api']) {
    expect(button(id).props.accessibilityRole).toBe('button');
  }
  await act(async () => button('hq-call-select-api').props.onPress());
  expect(button('hq-call-select-api').props.accessibilityState).toEqual({selected: true});
  await act(async () => button('hq-call-select-api').props.onPress());
  expect(button('hq-call-select-api').props.accessibilityState).toEqual({selected: false});
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

test('slow HQ usage is coalesced, deadline recovers, and its late reply cannot replace the new reading', async () => {
  let answerOld!: (value: any) => void;
  const old = new Promise<any>(resolve => {answerOld = resolve;});
  const fresh = {sessions: [], types: [], limits: {windows: []}, history: {today_out: 42, week_out: 42}};
  mockClient.usage.mockReset();
  mockClient.usage.mockReturnValueOnce(old).mockResolvedValue(fresh as any);
  mockClient.transcript.mockResolvedValue({turns: []});
  await mount('idle', 'compact');
  await act(async () => {jest.advanceTimersByTime(6000);});
  expect(mockClient.usage).toHaveBeenCalledTimes(1);
  await act(async () => {jest.advanceTimersByTime(9000);});
  await act(async () => {jest.advanceTimersByTime(3000);});
  expect(tree!.root.findByType(UsageSheet).props.usage).toEqual(fresh);
  const signal = (mockClient.usage.mock.calls as unknown as [AbortSignal][])[0][0];
  expect(signal.aborted).toBe(true);
  await act(async () => {answerOld({history: {today_out: 1}});});
  expect(tree!.root.findByType(UsageSheet).props.usage).toEqual(fresh);
  mockClient.usage.mockReset().mockResolvedValue(null);
});

// Tools inspect the already-mounted HQ subscription; they must not start a second poll.
test('HQ Trace shares the existing transcript and preserves it when closed', async () => {
 const {HQHeader} = require('./HQHeader');
 const {TraceModal} = require('../trace/TraceModal');
 mockClient.transcript.mockResolvedValue({turns: [{prompt: 'p', response: 'HQ record'}], dropped: 0});
 await mount('working', 'regular');
 const count = calls();
 act(() => tree!.root.findByType(HQHeader).props.tools.props.onTrace());
 expect(tree!.root.findByType(TraceModal).props.visible).toBe(true);
 expect(tree!.root.findByType(TraceModal).props.turns[0].response).toBe('HQ record');
 expect(calls()).toBe(count);
 act(() => tree!.root.findByType(TraceModal).props.onClose());
 expect(shown()).toBe('HQ record');
 expect(calls()).toBe(count);
});

test.each([401, 403])('HQ denial %s closes Trace and clears cached records', async status => {
 const {HQHeader} = require('./HQHeader');
 const {TraceModal} = require('../trace/TraceModal');
 const {ApiError} = require('../api/client');
 mockClient.transcript.mockResolvedValueOnce({turns: [{response: 'private'}]});
 await mount('working', 'regular');
 act(() => tree!.root.findByType(HQHeader).props.tools.props.onTrace());
 mockClient.transcript.mockRejectedValueOnce(new ApiError(status, 'transcript'));
 await act(async () => {jest.advanceTimersByTime(4000);});
 expect(tree!.root.findByType(TraceModal).props.visible).toBe(false);
 expect(tree!.root.findByType(TraceModal).props.turns).toEqual([]);
});

test('HQ can load an earlier session from Trace through the existing read lane', async () => {
 const {HQHeader} = require('./HQHeader');
 const {TraceModal} = require('../trace/TraceModal');
 mockClient.transcript.mockResolvedValueOnce({turns: [{session_id: 'now', response: 'now'}], earlierAvailable: true});
 await mount('working', 'regular');
 act(() => tree!.root.findByType(HQHeader).props.tools.props.onTrace());
 let resolve: (value: any) => void = () => {};
 mockClient.transcript.mockImplementationOnce(() => new Promise(r => {resolve = r;}));
 act(() => tree!.root.findByType(TraceModal).props.onLoadEarlier());
 await act(async () => {});
 expect(mockClient.transcript.mock.calls.at(-1)[2]).toBe(1);
 expect(tree!.root.findByType(TraceModal).props.loadingEarlier).toBe(true);
 await act(async () => resolve({turns: [{session_id: 'older', response: 'older'}, {session_id: 'now', response: 'now'}], earlierAvailable: false}));
 expect(tree!.root.findByType(TraceModal).props.turns.map((t: any) => t.session_id)).toEqual(['older', 'now']);
 expect(tree!.root.findByType(TraceModal).props.loadingEarlier).toBe(false);
});

test('workspace-backed conversations use the same matters and keep them current without legacy tabs', async () => {
  let data = demoHQWorkspace(false);
  mockClient.hqWorkspace = jest.fn(async () => data);
  mockClient.digest.mockResolvedValue([{pane_id: '%worker', loc: 'worker', source: 'tmux', agent: 'Codex', status: 'waiting'}]);
  mockClient.transcript.mockResolvedValue({turns: []});
  await mount('idle', 'regular');
  await act(async () => tree!.root.findAllByProps({testID: 'hq-overview-talk'})[0].props.onPress());
  const header = () => tree!.root.findByType(HQHeader);
  expect(tree!.root.findAllByProps({testID: 'hq-tab-calls'})).toHaveLength(0);
  expect(tree!.root.findAllByProps({testID: 'hq-tab-console'})).toHaveLength(0);
  expect(tree!.root.findAllByProps({testID: 'hq-inspector'})).toHaveLength(0);
  expect(header().props.model.verdict).toBe('1 needs input · 2 ongoing');
  expect(header().props.pendingMatters).toBe(1);
  expect(header().props.model.rows.some((row: {key: string}) => row.key === 'context')).toBe(false);
  expect(header().props.backLabel).toBe('Overview');
  expect(header().props.open).toBe(false);
  await act(async () => header().props.onToggle());
  expect(header().props.open).toBe(true);
  expect(tree!.root.findByType(ChatView)).toBeDefined();
  data = {...data, matters: data.matters.map(m => m.id === 'api-refactor' ? {...m, state: 'needs-input' as const} : m)};
  await act(async () => jest.advanceTimersByTime(10000));
  expect(header().props.model.verdict).toBe('2 need input · 1 ongoing');
  expect(header().props.pendingMatters).toBe(2);
  expect(mockClient.hqWorkspace!.mock.calls.length).toBeGreaterThanOrEqual(2);
  await act(async () => header().props.onBack());
  expect(tree!.root.findAllByProps({testID: 'hq-overview-talk'}).length).toBeGreaterThan(0);
});
