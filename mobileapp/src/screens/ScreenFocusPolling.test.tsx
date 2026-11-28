import React from 'react';
import {AppState} from 'react-native';
import {NavigationContext} from '@react-navigation/native';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {RadarPanel} from './RadarPanel';
import {HQView} from './HQScreen';
import type {Agent} from '../api/types';

jest.mock('../state/AgentsContext', () => ({useAgents: () => mockAgentsContext, useAgentsOptional: () => mockAgentsContext}));
jest.mock('../state/AppContext', () => ({
  useApp: () => ({
    t: (k: string) => k,
    pal: require('../ui/theme').paletteFor('dark'),
    lang: 'en',
    mac: {name: 'Mac', url: 'https://mac.invalid'},
    fontPref: 'system',
    returnSends: false,
    defaultDetailMode: 'terminal',
  }),
}));
jest.mock('../state/WorkspaceContext', () => ({
  useWorkspace: () => ({select: jest.fn(), cursor: null}),
}));
jest.mock('../ui/ChatView', () => ({ChatView: () => null}));
jest.mock('../ui/NativeTerm', () => ({NativeTerm: () => null, TERM_BG: '#17171a'}));
jest.mock('../ui/Composer', () => ({Composer: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));

function makeNavController(initialFocused = true) {
  let focused = initialFocused;
  const focusSubs = new Set<() => void>();
  const blurSubs = new Set<() => void>();
  return {
    nav: {
      navigate: jest.fn(),
      isFocused: () => focused,
      addListener: (event: 'focus' | 'blur', cb: () => void) => {
        const bag = event === 'focus' ? focusSubs : blurSubs;
        bag.add(cb);
        return () => bag.delete(cb);
      },
    } as any,
    blur() {
      focused = false;
      blurSubs.forEach(cb => cb());
    },
    focus() {
      focused = true;
      focusSubs.forEach(cb => cb());
    },
  };
}

const makeClient = () => ({
  base: 'https://mac.invalid',
  token: 'tok',
  pane: jest.fn(async (_id: string, signal?: AbortSignal) => ({text: 'hi', signal})),
  panes: jest.fn(async (signal?: AbortSignal) => [{pane_id: '%2', session: 'dev', tier: 'plain', signal}]),
  tasks: jest.fn(async (signal?: AbortSignal) => [{id: 't1', signal}]),
  transcript: jest.fn(async () => ({turns: []})),
  interaction: jest.fn(async () => ({options: []})),
  inputCapabilities: jest.fn(async () => ({version: 1, actions: []})),
  theme: jest.fn(async () => null),
  usage: jest.fn(async (signal?: AbortSignal) => ({
    sessions: [],
    types: [],
    limits: {windows: []},
    resource: {machine: {tier: signal ? 'green' : 'red'}},
  })),
  serverMode: jest.fn(async (signal?: AbortSignal) => ({
    state: 'off',
    system_disablesleep: Boolean(signal && false),
  })),
  host: jest.fn(async () => ({ok: true, info: {capabilities: {desktop_focus: true}}})),
  digest: jest.fn(async () => []),
  hqBoard: jest.fn(async () => ({exists: false})),
  hqKnowledge: jest.fn(async () => ({
    entries: [],
    topics: [],
    promotions: {pending: 0},
    candidates: {pending: 0},
  })),
  hqEvents: jest.fn(async () => []),
  hqWorkspace: undefined as undefined | jest.Mock,
});

let mockClient = makeClient();
let mockAgentsContext: any = {};

beforeEach(() => {
  jest.useFakeTimers();
  (AppState as any).currentState = 'active';
  mockClient = makeClient();
  mockAgentsContext = {
    client: mockClient,
    agents: [],
    conn: 'live',
    lastUpdated: Date.now(),
    banner: null,
    dismissBanner: jest.fn(),
    refresh: jest.fn(async () => {}),
    isGuest: false,
    shareStale: false,
    inputPanes: [],
    demo: false,
    serverModeRev: 0,
    read: {hasValue: true, refreshing: false, error: null},
  };
});

afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});

test('RadarPanel, DetailView, and HQView pass AbortSignal, pause polling on blur or background, and resume on focus', async () => {
  const appListeners = new Set<(state: string) => void>();
  jest.spyOn(AppState, 'addEventListener').mockImplementation((_type: any, cb: any) => {
    appListeners.add(cb);
    return {remove: () => appListeners.delete(cb)} as any;
  });

  const radarNav = makeNavController(true);
  let radarTree!: renderer.ReactTestRenderer;
  await act(async () => {
    radarTree = renderer.create(
      <NavigationContext.Provider value={radarNav.nav}>
        <RadarPanel variant="screen" />
      </NavigationContext.Provider>,
    );
  });
  expect(mockClient.usage.mock.calls[0][0]).toBeInstanceOf(AbortSignal);
  expect(mockClient.serverMode.mock.calls[0][0]).toBeInstanceOf(AbortSignal);

  act(() => radarNav.blur());
  mockClient.usage.mockClear();
  mockClient.serverMode.mockClear();
  await act(async () => jest.advanceTimersByTime(60_000));
  expect(mockClient.usage).not.toHaveBeenCalled();
  expect(mockClient.serverMode).not.toHaveBeenCalled();

  await act(async () => radarNav.focus());
  expect(mockClient.usage).toHaveBeenCalledTimes(1);
  expect(mockClient.serverMode).toHaveBeenCalledTimes(1);
  act(() => radarTree.unmount());

  const hqAgent: Agent = {
    pane_id: '%1',
    session: 'dev',
    session_id: 's1',
    agent: 'Codex',
    status: 'idle',
    source: 'tmux',
    role: 'supervisor',
  } as Agent;
  mockAgentsContext.agents = [hqAgent];
  const detailNav = makeNavController(true);
  let detailTree!: renderer.ReactTestRenderer;
  await act(async () => {
    detailTree = renderer.create(
      <NavigationContext.Provider value={detailNav.nav}>
        <DetailView agent={hqAgent} initialMode="chat" />
      </NavigationContext.Provider>,
    );
  });
  expect(mockClient.tasks.mock.calls[0][0]).toBeInstanceOf(AbortSignal);
  expect(mockClient.panes.mock.calls[0][0]).toBeInstanceOf(AbortSignal);

  act(() => detailNav.blur());
  mockClient.pane.mockClear();
  mockClient.panes.mockClear();
  mockClient.tasks.mockClear();
  mockClient.transcript.mockClear();
  await act(async () => jest.advanceTimersByTime(30_000));
  expect(mockClient.pane).not.toHaveBeenCalled();
  expect(mockClient.panes).not.toHaveBeenCalled();
  expect(mockClient.tasks).not.toHaveBeenCalled();
  expect(mockClient.transcript).not.toHaveBeenCalled();

  await act(async () => detailNav.focus());
  expect(mockClient.pane).toHaveBeenCalledTimes(1);
  expect(mockClient.panes).toHaveBeenCalledTimes(1);
  expect(mockClient.tasks).toHaveBeenCalledTimes(1);
  expect(mockClient.transcript).toHaveBeenCalledTimes(1);

  act(() => {
    (AppState as any).currentState = 'background';
    appListeners.forEach(cb => cb('background'));
  });
  mockClient.pane.mockClear();
  mockClient.panes.mockClear();
  await act(async () => jest.advanceTimersByTime(15_000));
  expect(mockClient.pane).not.toHaveBeenCalled();
  expect(mockClient.panes).not.toHaveBeenCalled();
  act(() => {
    (AppState as any).currentState = 'active';
    appListeners.forEach(cb => cb('active'));
  });
  act(() => detailTree.unmount());

  const hqNav = makeNavController(true);
  let hqTree!: renderer.ReactTestRenderer;
  await act(async () => {
    hqTree = renderer.create(
      <NavigationContext.Provider value={hqNav.nav}>
        <HQView agent={hqAgent} layout="compact" />
      </NavigationContext.Provider>,
    );
  });
  act(() => hqNav.blur());
  mockClient.pane.mockClear();
  mockClient.digest.mockClear();
  mockClient.usage.mockClear();
  await act(async () => jest.advanceTimersByTime(60_000));
  expect(mockClient.pane).not.toHaveBeenCalled();
  expect(mockClient.digest).not.toHaveBeenCalled();
  expect(mockClient.usage).not.toHaveBeenCalled();

  await act(async () => hqNav.focus());
  expect(mockClient.pane).toHaveBeenCalledTimes(1);
  expect(mockClient.digest).toHaveBeenCalledTimes(1);
  expect(mockClient.usage).toHaveBeenCalledTimes(1);
  act(() => hqTree.unmount());
});
