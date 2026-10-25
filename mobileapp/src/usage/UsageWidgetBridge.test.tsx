import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Linking, NativeModules, Platform} from 'react-native';
import {isUsageWidgetLink, UsageWidgetBridge, useUsageWidgetContext} from './UsageWidgetBridge';
import {UsageSheet} from '../screens/UsageSheet';

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
  jest.spyOn(Linking, 'addEventListener').mockImplementation((_, callback) => {
    onURL = callback; return {remove} as unknown as ReturnType<typeof Linking.addEventListener>;
  });
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
  await act(async () => onURL({url: 'gtmux://usage'}));
  expect(sheet().visible).toBe(false);
  await act(async () => tree.unmount());
  expect(remove).toHaveBeenCalled();
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
  jest.spyOn(Linking, 'addEventListener').mockImplementation((_, callback) => {
    onURL = callback; return {remove: jest.fn()} as unknown as ReturnType<typeof Linking.addEventListener>;
  });
  let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
  await act(async () => onURL({url: 'gtmux://usage'}));
  expect(sheet().visible).toBe(true);
  mockAgents = {...mockAgents, client: {usage: jest.fn().mockResolvedValue({limits: {at: 2}})}};
  await act(async () => tree.update(<UsageWidgetBridge />));
  expect(sheet().visible).toBe(false);
  await act(async () => onURL({url: 'gtmux://usage'}));
  expect(sheet().usage).toEqual({limits: {at: 2}});
  await act(async () => resolveOld({limits: {at: 1}}));
  expect(sheet().usage).toEqual({limits: {at: 2}});
  await act(async () => tree.unmount());
});
