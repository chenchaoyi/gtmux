import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Linking, NativeModules, Platform} from 'react-native';
import {isUsageWidgetLink, UsageWidgetBridge, useUsageWidgetContext} from './UsageWidgetBridge';
import {UsageSheet} from '../screens/UsageSheet';
import {GtmuxClient} from '../api/client';
import {AppState} from 'react-native';

let mockApp: any = {ready: true, mac: {url: 'https://studio.example', token: 'synthetic', scope: 'owner'}};
const mockUsage = jest.fn();
let mockAgents: any = {client: {usage: mockUsage}, agents: [], isGuest: false};
jest.mock('../state/AppContext', () => ({useApp: () => mockApp}));
jest.mock('../state/AgentsContext', () => ({useAgents: () => mockAgents}));
jest.mock('../ui/layout', () => ({useSizeClass: () => 'compact'}));
jest.mock('../screens/UsageSheet', () => ({UsageSheet: jest.fn(() => null)}));

function Context() { useUsageWidgetContext(); return null; }
const sheet = () => (UsageSheet as jest.Mock).mock.calls.at(-1)![0];

afterEach(() => jest.restoreAllMocks());

test('pairing lifecycle sends only the current owner URL, including same-address token rotation', async () => {
  const configure = jest.fn().mockResolvedValue(null);
  NativeModules.UsageWidgetsModule = {configure};
  Object.defineProperty(Platform, 'OS', {value: 'ios'});
  mockApp = {ready: false, mac: {url: 'https://studio.example', token: 'synthetic', scope: 'owner'}};
  let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<Context />); });
  expect(configure).not.toHaveBeenCalled();
  mockApp = {...mockApp, ready: true};
  await act(async () => tree.update(<Context />));
  expect(configure).toHaveBeenLastCalledWith('https://studio.example');
  mockApp.mac.token = 'rotated-synthetic';
  await act(async () => tree.update(<Context />));
  expect(configure).toHaveBeenCalledTimes(2);
  mockApp.mac.scope = 'guest';
  await act(async () => tree.update(<Context />));
  expect(configure).toHaveBeenLastCalledWith(null);
  mockApp.mac = null;
  await act(async () => tree.update(<Context />));
  expect(configure).toHaveBeenLastCalledWith(null);
  expect(JSON.stringify(configure.mock.calls)).not.toContain('synthetic');
  await act(async () => tree.unmount());
});

test('a widget opens usage without HQ, dismisses, and ignores guest links and private URL parameters', async () => {
  mockApp = {ready: true, pal: {}, lang: 'zh'};
  mockAgents = {client: {usage: mockUsage}, agents: [], isGuest: false};
  mockUsage.mockResolvedValue({limits: {windows: []}});
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue('gtmux://usage');
  let onURL: any;
  const remove = jest.fn();
  const subscribe = jest.spyOn(Linking, 'addEventListener').mockImplementation((_, callback) => {
    onURL = callback; return {remove} as unknown as ReturnType<typeof Linking.addEventListener>;
  });
  subscribe.mockClear();
  let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
  expect(sheet().visible).toBe(true);
  expect(mockUsage).toHaveBeenCalledWith(expect.any(AbortSignal));
  expect(sheet().usage).toEqual({limits: {windows: []}});
  await act(async () => sheet().onClose());
  expect(sheet().visible).toBe(false);
  await act(async () => onURL({url: 'gtmux://usage?server=other&token=secret'}));
  expect(sheet().visible).toBe(false);
  mockAgents = {...mockAgents, isGuest: true};
  await act(async () => tree.update(<UsageWidgetBridge />));
  expect(subscribe).toHaveBeenCalledTimes(1);
  expect(remove).not.toHaveBeenCalled();
  await act(async () => onURL({url: 'gtmux://usage'}));
  expect(sheet().visible).toBe(false);
  const beforeOwner = mockUsage.mock.calls.length;
  mockAgents = {...mockAgents, isGuest: false};
  await act(async () => tree.update(<UsageWidgetBridge />));
  expect(sheet().visible).toBe(false);
  expect(mockUsage).toHaveBeenCalledTimes(beforeOwner);
  await act(async () => tree.unmount());
  expect(remove).toHaveBeenCalledTimes(1);
});

test('widget links cannot select a server or smuggle credentials', () => {
  expect(isUsageWidgetLink('gtmux://usage')).toBe(true);
  expect(isUsageWidgetLink('gtmux://usage/')).toBe(true);
  for (const url of ['https://usage', 'gtmux://usage?token=x', 'gtmux://usage/other', 'gtmux://user:token@usage']) {
    expect(isUsageWidgetLink(url)).toBe(false);
  }
});

test('changing a connection closes the old sheet and discards its late response', async () => {
  mockApp = {ready: true, pal: {}, lang: 'en'};
  let resolveOld!: (value: any) => void;
  const old = jest.fn(() => new Promise(resolve => { resolveOld = resolve; }));
  mockAgents = {client: {usage: old}, agents: [], isGuest: false};
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue(null);
  let onURL: any;
  const remove = jest.fn();
  const subscribe = jest.spyOn(Linking, 'addEventListener').mockImplementation((_, callback) => {
    onURL = callback; return {remove} as unknown as ReturnType<typeof Linking.addEventListener>;
  });
  subscribe.mockClear();
  let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
  await act(async () => onURL({url: 'gtmux://usage'}));
  expect(sheet().visible).toBe(true);
  mockAgents = {...mockAgents, client: {usage: jest.fn().mockResolvedValue({limits: {at: 2}})}};
  await act(async () => tree.update(<UsageWidgetBridge />));
  expect(subscribe).toHaveBeenCalledTimes(1);
  expect(remove).not.toHaveBeenCalled();
  expect(sheet().visible).toBe(false);
  await act(async () => onURL({url: 'gtmux://usage'}));
  expect(sheet().usage).toEqual({limits: {at: 2}});
  await act(async () => resolveOld({limits: {at: 1}}));
  expect(sheet().usage).toEqual({limits: {at: 2}});
  await act(async () => tree.unmount());
  expect(remove).toHaveBeenCalledTimes(1);
});

test('a headless scene link is retained until the visual consumer opens and acknowledges it', async () => {
  mockApp = {ready: true, pal: {}, lang: 'en'};
  const usage = jest.fn().mockResolvedValue({limits: {windows: []}});
  mockAgents = {client: {usage}, agents: [], isGuest: false};
  const previous = NativeModules.GtmuxNotificationResponses;
  let pending = [{id: 'scene-1', url: 'gtmux://usage'}];
  const acknowledgeSceneLink = jest.fn((id: string) => { pending = pending.filter(item => item.id !== id); });
  NativeModules.GtmuxNotificationResponses = {pendingSceneLinks: jest.fn(async () => pending), acknowledgeSceneLink};
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue(null);
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
    expect(sheet().visible).toBe(true);
    expect(usage).toHaveBeenCalledTimes(1);
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('scene-1');
    await act(async () => sheet().onClose());
    await act(async () => tree.update(<UsageWidgetBridge />));
    expect(sheet().visible).toBe(false);
  } finally {
    if (tree) await act(async () => tree.unmount());
    NativeModules.GtmuxNotificationResponses = previous;
  }
});

test('a queued scene link rejected for a guest cannot open after switching to owner', async () => {
  mockApp = {ready: true, pal: {}, lang: 'en'};
  const usage = jest.fn().mockResolvedValue({limits: {windows: []}});
  mockAgents = {client: {usage}, agents: [], isGuest: true};
  const previous = NativeModules.GtmuxNotificationResponses;
  const acknowledgeSceneLink = jest.fn();
  NativeModules.GtmuxNotificationResponses = {
    pendingSceneLinks: jest.fn(async () => [{id: 'guest-scene', url: 'gtmux://usage'}]), acknowledgeSceneLink,
  };
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue(null);
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('guest-scene');
    expect(sheet().visible).toBe(false);
    mockAgents = {...mockAgents, isGuest: false};
    await act(async () => tree.update(<UsageWidgetBridge />));
    expect(sheet().visible).toBe(false);
    expect(usage).not.toHaveBeenCalled();
  } finally {
    if (tree) await act(async () => tree.unmount());
    NativeModules.GtmuxNotificationResponses = previous;
  }
});

test('a retained scene link is retried when the app becomes active', async () => {
  mockApp = {ready: true, pal: {}, lang: 'en'};
  const usage = jest.fn().mockResolvedValue({limits: {windows: []}});
  mockAgents = {client: {usage}, agents: [], isGuest: false};
  const previous = NativeModules.GtmuxNotificationResponses;
  const acknowledgeSceneLink = jest.fn();
  const pendingSceneLinks = jest.fn().mockRejectedValueOnce(new Error('not ready'))
    .mockResolvedValue([{id: 'retry-scene', url: 'gtmux://usage'}]);
  NativeModules.GtmuxNotificationResponses = {pendingSceneLinks, acknowledgeSceneLink};
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue(null);
  let onState: any;
  jest.spyOn(AppState, 'addEventListener').mockImplementation((_, callback) => {
    onState = callback; return {remove: jest.fn()} as any;
  });
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
    expect(sheet().visible).toBe(false);
    await act(async () => onState('active'));
    expect(sheet().visible).toBe(true);
    expect(usage).toHaveBeenCalledTimes(1);
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('retry-scene');
  } finally {
    if (tree) await act(async () => tree.unmount());
    NativeModules.GtmuxNotificationResponses = previous;
  }
});

test('an active transition during a failed drain still retries the retained link', async () => {
  mockApp = {ready: true, pal: {}, lang: 'en'};
  const usage = jest.fn().mockResolvedValue({limits: {windows: []}});
  mockAgents = {client: {usage}, agents: [], isGuest: false};
  const previous = NativeModules.GtmuxNotificationResponses;
  let rejectFirst!: (reason: Error) => void;
  const acknowledgeSceneLink = jest.fn();
  const pendingSceneLinks = jest.fn()
    .mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectFirst = reject; }))
    .mockResolvedValue([{id: 'active-race', url: 'gtmux://usage'}]);
  NativeModules.GtmuxNotificationResponses = {pendingSceneLinks, acknowledgeSceneLink};
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue(null);
  let onState: any;
  jest.spyOn(AppState, 'addEventListener').mockImplementation((_, callback) => {
    onState = callback; return {remove: jest.fn()} as any;
  });
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
    await act(async () => { onState('active'); rejectFirst(new Error('not ready')); });
    expect(sheet().visible).toBe(true);
    expect(usage).toHaveBeenCalledTimes(1);
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('active-race');
  } finally {
    if (tree) await act(async () => tree.unmount());
    NativeModules.GtmuxNotificationResponses = previous;
  }
});

test('foreground recovery bypasses a stalled scene-link drain and suppresses its late copy', async () => {
  mockApp = {ready: true, pal: {}, lang: 'en'};
  const usage = jest.fn().mockResolvedValue({limits: {windows: []}});
  mockAgents = {client: {usage}, agents: [], isGuest: false};
  const previous = NativeModules.GtmuxNotificationResponses;
  const item = {id: 'stalled-link', url: 'gtmux://usage'};
  let resolveFirst!: (value: Array<typeof item>) => void;
  const acknowledgeSceneLink = jest.fn();
  const pendingSceneLinks = jest.fn()
    .mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve; }))
    .mockResolvedValue([item]);
  NativeModules.GtmuxNotificationResponses = {pendingSceneLinks, acknowledgeSceneLink};
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue(null);
  let onState: any;
  jest.spyOn(AppState, 'addEventListener').mockImplementation((_, callback) => {
    onState = callback; return {remove: jest.fn()} as any;
  });
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
    expect(sheet().visible).toBe(false);
    await act(async () => onState('active'));
    expect(sheet().visible).toBe(true);
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('stalled-link');
    await act(async () => sheet().onClose());
    await act(async () => resolveFirst([item]));
    expect(sheet().visible).toBe(false);
    expect(usage).toHaveBeenCalledTimes(1);
  } finally {
    if (tree) await act(async () => tree.unmount());
    NativeModules.GtmuxNotificationResponses = previous;
  }
});


test('Android snapshot labels handle unnamed plans and contain no pairing credential', async () => {
  const platform = Platform.OS, state = AppState.currentState;
  const configure = jest.fn().mockResolvedValue(null), updateSnapshot = jest.fn().mockResolvedValue(null);
  NativeModules.UsageWidgetsModule = {configure, updateSnapshot};
  Object.defineProperty(Platform, 'OS', {configurable: true, value: 'android'});
  AppState.currentState = 'active';
  mockApp = {ready: true, lang: 'en', mac: {url: 'https://studio.example', token: 'synthetic-secret', scope: 'owner', name: 'Test host'}};
  jest.spyOn(GtmuxClient.prototype, 'usage').mockResolvedValue({limits: {windows: [{label: 'session', pct_used: 24}, {label: 'weekly', agent: 'Codex', pct_used: 54}]}} as any);
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => {tree = renderer.create(<Context />);});
    const snapshot = updateSnapshot.mock.calls[0][0];
    expect(snapshot.windows[0].label).toBe('session');
    expect(snapshot.windows[1].label).toBe('Codex · weekly');
    expect(JSON.stringify(snapshot)).not.toContain('synthetic-secret');
    mockApp = {...mockApp, mac: {...mockApp.mac, scope: 'guest'}};
    await act(async () => tree.update(<Context />));
    expect(configure).toHaveBeenLastCalledWith(null);
    expect(updateSnapshot).toHaveBeenCalledTimes(1);
  } finally {if (tree) await act(async () => tree.unmount()); AppState.currentState = state; Object.defineProperty(Platform, 'OS', {configurable: true, value: platform});}
});
