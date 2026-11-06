import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DeviceEventEmitter, Linking, NativeModules} from 'react-native';
import {UsageWidgetBridge} from './UsageWidgetBridge';
import {UsageSheet} from '../screens/UsageSheet';

const mockUsage = jest.fn();
let mockAgents: any = {client: {usage: mockUsage}, agents: [], isGuest: false};
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: {}, lang: 'en'})}));
jest.mock('../state/AgentsContext', () => ({useAgents: () => mockAgents}));
jest.mock('../ui/layout', () => ({useSizeClass: () => 'compact'}));
jest.mock('../screens/UsageSheet', () => ({UsageSheet: jest.fn(() => null)}));

test('retained scene availability delivers each permitted intent once and rejects guest replay', async () => {
  const previous = NativeModules.GtmuxNotificationResponses;
  let pending = [{id: 'before-listener', url: 'gtmux://usage'}];
  const acknowledgeSceneLink = jest.fn((id: string) => { pending = pending.filter(item => item.id !== id); });
  const addListener = jest.fn(), removeListeners = jest.fn();
  NativeModules.GtmuxNotificationResponses = {
    addListener, removeListeners, acknowledgeSceneLink, pendingSceneLinks: jest.fn(async () => [...pending]),
  };
  mockUsage.mockResolvedValue({limits: {windows: []}});
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue(null);
  const sheet = () => (UsageSheet as jest.Mock).mock.calls.at(-1)![0];
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
    expect(addListener).toHaveBeenCalledWith('gtmuxSceneLinksAvailable');
    expect(sheet().visible).toBe(true);
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('before-listener');
    await act(async () => sheet().onClose());
    await act(async () => DeviceEventEmitter.emit('gtmuxSceneLinksAvailable'));
    expect(sheet().visible).toBe(false);

    mockAgents = {...mockAgents, isGuest: true};
    await act(async () => tree.update(<UsageWidgetBridge />));
    pending.push({id: 'guest-intent', url: 'gtmux://usage'});
    await act(async () => DeviceEventEmitter.emit('gtmuxSceneLinksAvailable'));
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('guest-intent');
    expect(sheet().visible).toBe(false);
    mockAgents = {...mockAgents, isGuest: false};
    await act(async () => tree.update(<UsageWidgetBridge />));
    expect(sheet().visible).toBe(false);
    pending.push({id: 'later-owner-intent', url: 'gtmux://usage'});
    await act(async () => DeviceEventEmitter.emit('gtmuxSceneLinksAvailable'));
    expect(sheet().visible).toBe(true);
    expect(acknowledgeSceneLink).toHaveBeenCalledWith('later-owner-intent');
  } finally {
    if (tree) await act(async () => tree.unmount());
    NativeModules.GtmuxNotificationResponses = previous;
    jest.restoreAllMocks();
  }
  expect(removeListeners).toHaveBeenCalled();
});
