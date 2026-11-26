import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {HQConsoleView} from './HQScreen';
import {Text, ScrollView, TouchableOpacity, View} from 'react-native';
import {TestIds} from '../constants/testIds';
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
let mockContext: {client: ReturnType<typeof client>; agents: Agent[]; conn: string;
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
  mockContext = {client: stub, agents: [agent as Agent], conn: 'live', isGuest: false, inputPanes: [], demo: false};
});
afterEach(() => {
  if (tree) act(() => tree.unmount());
  jest.clearAllTimers();
  jest.useRealTimers();
  globalThis.fetch = fetchSaved;
});

export function updateAgents(agents: Agent[]) {
  mockContext = {...mockContext, agents};
}
export function screenFor(surface: string) {
  return surface === 'Detail' ? <DetailView agent={agent as Agent} initialMode="chat"/>
    : <HQConsoleView agent={agent as Agent} showInspector={false}/>;
}
export async function mount(surface: string) {
  await act(async () => {tree = renderer.create(screenFor(surface));});
  return tree;
}
export {agent, disclosure, grip, liveScroll, liveText, marker, screen, stub, ScrollView, TestIds};
