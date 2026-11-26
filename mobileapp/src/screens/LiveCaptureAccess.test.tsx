import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {HQConsoleView} from './HQScreen';
import {Text, ScrollView, TouchableOpacity, View} from 'react-native';
import {TestIds} from '../constants/testIds';
import {GtmuxClient} from '../api/client';
import type {Agent} from '../api/types';

jest.mock('../state/AgentsContext', () => ({
  useAgents: () => mockContext, useAgentsOptional: () => mockContext,
}));
jest.mock('../state/AppContext', () => ({
  useApp: () => ({
    pal: require('../ui/theme').paletteFor('dark'), lang: 'en', defaultDetailMode: 'chat',
    fontPref: 'auto', returnSends: false, mac: {name: 'Fixture'},
  }),
}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/Composer', () => ({Composer: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));

function client() {
  const empty = async () => [];
  return {
    base: 'https://fixture.invalid', token: 'fixture', pane: jest.fn(),
    transcript: async () => ({turns: [], dropped: 0, readOnlyHistory: true}),
    sendResult: jest.fn(), panes: empty, tasks: empty, options: empty, theme: async () => undefined,
    digest: empty, usage: async () => null, hqBoard: async () => ({exists: false}),
    hqKnowledge: async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}}),
    hqEvents: empty,
  };
}
let mockContext: {client: ReturnType<typeof client>; agents: typeof agent[]; conn: string;
  isGuest: boolean; inputPanes: string[]; demo: boolean};
const agent = {pane_id: '%fixture-a', session_id: 'session-a', home_id: 'home-a', session: 'Fixture chat', agent: 'Codex', source: 'tmux', status: 'working', role: 'supervisor'};
const marker = 'Synthetic terminal row 73';
const screen = Array.from({length: 74}, (_, n) => 'Synthetic terminal row ' + n).join('\n');
const liveText = (tree: renderer.ReactTestRenderer) => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' | ');
const liveScroll = (tree: renderer.ReactTestRenderer) => tree.root.findAllByType(ScrollView).find(n => n.props.nestedScrollEnabled && !n.props.horizontal);
const disclosure = (tree: renderer.ReactTestRenderer) => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.chatLiveToggle);
const grip = (tree: renderer.ReactTestRenderer) => tree.root.findAllByType(View).find(n => n.props.testID === 'chat-live-resize-grip');
let tree: renderer.ReactTestRenderer;
let stub: ReturnType<typeof client>;
let fetchSaved: typeof fetch;
beforeEach(() => {
  jest.useFakeTimers();
  fetchSaved = globalThis.fetch;
  stub = client();
  stub.pane.mockResolvedValue({text: screen, cols: 80});
  globalThis.fetch = jest.fn(async () => {throw new Error('fixture network only');});
  mockContext = {client: stub, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false};
});
afterEach(() => {
  if (tree) act(() => tree.unmount());
  jest.clearAllTimers();
  jest.useRealTimers();
  globalThis.fetch = fetchSaved;
});

test.each(['Detail', 'HQ'])('LRB-002 %s keeps the captured rows and viewport through waiting and idle', async surface => {
  const view = () => surface === 'Detail' ? <DetailView agent={agent as Agent} initialMode="chat"/> : <HQConsoleView agent={agent as Agent} showInspector={false}/>;
  await act(async () => {tree = renderer.create(view());});
  expect(disclosure(tree)).toBeDefined();
  act(() => disclosure(tree)!.props.onPress());
  expect(liveText(tree)).toContain(marker);
  const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
  act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
  act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  const old = liveScroll(tree)!;
  expect(old.props.style.maxHeight).toBe(276);
  for (const status of ['waiting', 'idle']) {
    mockContext = {...mockContext, agents: [{...agent, status}]};
    await act(async () => tree.update(view()));
    expect(disclosure(tree)?.props.accessibilityState?.expanded).toBe(true);
    expect(liveText(tree)).toContain(marker);
    expect(liveScroll(tree)).toBe(old);
    expect(liveScroll(tree)?.props.style.maxHeight).toBe(276);
  }
  act(() => disclosure(tree)!.props.onPress());
  expect(liveScroll(tree)).toBeUndefined();
});

describe.each(['Detail', 'HQ'])('%s pane polling', surface => {
  test.each([403, 404])('LRB-003 HTTP %i clears the retained screen until a fresh capture', async status => {
    let responseStatus = 200;
    let capture = screen;
    const http = new GtmuxClient(stub.base, stub.token);
    stub.pane.mockImplementation((pane: string, signal: AbortSignal) => http.pane(pane, signal));
    globalThis.fetch = jest.fn(async () => ({
      ok: responseStatus === 200, status: responseStatus,
      headers: {get: () => null}, json: async () => ({text: capture, cols: 80}),
    })) as unknown as typeof fetch;
    const view = () => surface === 'Detail' ? <DetailView agent={agent as Agent} initialMode="chat"/>
      : <HQConsoleView agent={agent as Agent} showInspector={false}/>;
    await act(async () => {tree = renderer.create(view());});
    act(() => disclosure(tree)!.props.onPress());
    mockContext = {...mockContext, agents: [{...agent, status: 'waiting'}]};
    await act(async () => tree.update(view()));
    const poll = async () => {
      const before = stub.pane.mock.calls.length;
      await act(async () => {jest.advanceTimersByTime(8000);});
      expect(stub.pane.mock.calls.length).toBeGreaterThan(before);
    };
    const retained = liveScroll(tree)!;
    act(() => retained.props.onScrollBeginDrag());
    capture = 'Later frame before access loss';
    await poll();
    expect(liveText(tree)).toContain(marker);
    expect(liveText(tree)).not.toContain(capture);
    responseStatus = 503;
    await poll();
    expect(liveText(tree)).toContain(marker);
    responseStatus = status;
    await poll();
    expect(liveText(tree)).not.toContain(marker);
    expect(liveScroll(tree)).toBeUndefined();
    responseStatus = 200;
    capture = 'Fresh authorized capture';
    await poll();
    expect(liveText(tree)).toContain(capture);
    expect(liveText(tree)).not.toContain(marker);
  });
});
