import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, NativeModules} from 'react-native';
import PushNotificationIOS from '@react-native-community/push-notification-ios';
import App from '../App';
import {resetLaunchNotificationForTesting, setupPush} from '../src/push';
import {Debug} from '../src/debug';

jest.mock('react-native-safe-area-context/src/NativeSafeAreaProvider', () => ({
  NativeSafeAreaProvider: 'NativeSafeAreaProvider',
}));
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context/jest/mock').default,
  ...jest.requireActual('react-native-safe-area-context/src/SafeAreaContext'),
}));
jest.mock('../src/state/AppContext', () => ({AppProvider: ({children}: any) => children, useApp: () => mockApp}));
jest.mock('../src/state/AgentsContext', () => ({
  AgentsProvider: ({children}: any) => children, useAgents: () => ({agents: mockAgents}),
}));
jest.mock('../src/usage/UsageWidgetBridge', () => ({UsageWidgetBridge: () => null, useUsageWidgetContext: () => {}}));
jest.mock('../src/push', () => ({
  ...jest.requireActual('../src/push'), setupPush: jest.fn(jest.requireActual('../src/push').setupPush),
}));
// Keep the installed pre-mount ref behavior; only the native navigator host is
// controlled. Actual WorkspaceProvider and SafeAreaProvider gates remain.
jest.mock('@react-navigation/native', () => {
  const R = require('react');
  const ref = jest.requireActual('@react-navigation/core').createNavigationContainerRef();
  return {
    NavigationContainer: ({children}: any) => {
      R.useLayoutEffect(() => {
        const target = {
          isReady: () => true, navigate: mockNavigate,
          addListener: (name: string, cb: () => void) => {
            if (name === 'ready') mockReadyListeners.add(cb);
            return () => mockReadyListeners.delete(cb);
          },
        };
        ref.current = target;
        for (const cb of mockReadyListeners) cb();
        return () => {ref.current = null; mockReadyListeners.clear();};
      }, []);
      return children;
    },
    useNavigationContainerRef: () => ref,
    NavigationContext: R.createContext(undefined),
    useNavigation: () => ({navigate: jest.fn(), goBack: jest.fn()}),
    DefaultTheme: {colors: {}}, DarkTheme: {colors: {}},
  };
});

const home = {url: 'https://home.test', token: 'fixture', name: 'Home'};
const office = {url: 'https://office.test', token: 'fixture', name: 'Office'};
let mockApp: any, mockAgents: any[] = [];
const mockNavigate = jest.fn();
const mockReadyListeners = new Set<() => void>();
const listeners: Record<string, (value: any) => void> = {};
const nativeBefore = NativeModules.GtmuxNotificationResponses;
const debugBefore = {...Debug};
let tree: renderer.ReactTestRenderer | undefined;
const response = (id: string, server = 'Home', pane = '%12') => ({pane, server, userInteraction: true, gtmuxResponseId: id});
const notification = (data: any) => ({getData: () => data, finish: jest.fn()});

beforeEach(() => {
  jest.useFakeTimers();
  resetLaunchNotificationForTesting();
  jest.clearAllMocks();
  mockReadyListeners.clear();
  mockAgents = [];
  mockApp = {ready: true, mac: home, pal: {}, lang: 'en', scheme: 'light', pushEnabled: true, pushKinds: {},
    servers: [home, office], activeUrl: home.url, pendingPane: null, setPushToken: jest.fn(),
    setPendingPane: jest.fn((pane: string | null) => {mockApp.pendingPane = pane;}), selectServer: jest.fn()};
  NativeModules.GtmuxNotificationResponses = {drain: jest.fn().mockResolvedValue([]), acknowledge: jest.fn()};
  Object.assign(Debug, {skipWhatsNew: true, noPush: false, logNet: false});
  (PushNotificationIOS as any).FetchResult = {NoData: 'noData'};
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(null);
  (PushNotificationIOS.addEventListener as jest.Mock).mockImplementation((name, cb) => {listeners[name] = cb;});
});
afterEach(async () => {
  await act(async () => tree?.unmount());
  tree = undefined;
  NativeModules.GtmuxNotificationResponses = nativeBefore;
  Object.assign(Debug, debugBefore);
  jest.useRealTimers();
  jest.restoreAllMocks();
});
async function mount() {await act(async () => {tree = renderer.create(<App />);});}
async function showVisual() {
  await act(async () => {
    tree!.root.findByType('NativeSafeAreaProvider' as never).props.onInsetsChange({nativeEvent: {
      frame: {x: 0, y: 0, width: 393, height: 852}, insets: {top: 59, right: 0, bottom: 34, left: 0},
    }});
  });
}
async function receive(data: any) {await act(async () => listeners.notification(notification(data)));}
async function readyAgain() {await act(async () => {for (const cb of mockReadyListeners) cb();});}

it('starts the actual notification bridge while the visual safe area has no metrics', async () => {
  await mount();
  expect(setupPush).toHaveBeenCalledTimes(1);
  expect(tree!.root.findByType('NativeSafeAreaProvider' as never).props.children).toBeNull();
});

it('a cold same-server tap waits beyond the inset gate then opens the latest agent exactly once', async () => {
  const data = response('cold');
  (NativeModules.GtmuxNotificationResponses.drain as jest.Mock).mockResolvedValue([data]);
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(notification(data));
  await mount();
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).toHaveBeenCalledWith('cold', '');
  expect(mockNavigate).not.toHaveBeenCalled();
  await act(async () => {jest.advanceTimersByTime(3000);});
  expect(mockNavigate).not.toHaveBeenCalled();
  mockAgents = [{pane_id: '%12', session_id: 'latest-session', source: 'tmux', role: 'supervisor'}];
  await act(async () => tree!.update(<App />));
  await showVisual();
  expect(mockNavigate).toHaveBeenCalledTimes(1);
  expect(mockNavigate).toHaveBeenCalledWith('HQ', expect.objectContaining({agent: mockAgents[0]}));
  await readyAgain(); await receive(data);
  expect(mockNavigate).toHaveBeenCalledTimes(1);
});

it('a warm tap navigates immediately and a new response is not mistaken for a replay', async () => {
  await mount(); await showVisual();
  await receive(response('warm1')); await receive(response('warm1'));
  expect(mockNavigate).toHaveBeenCalledTimes(1);
  expect(mockNavigate).toHaveBeenLastCalledWith('Detail', expect.objectContaining({agent: expect.objectContaining({pane_id: '%12'})}));
  await receive(response('warm2', 'Home', '%13')); await readyAgain();
  expect(mockNavigate).toHaveBeenCalledTimes(2);
  expect(mockNavigate).toHaveBeenLastCalledWith('Detail', expect.objectContaining({agent: expect.objectContaining({pane_id: '%13'})}));
});

it('a user server change discards a queued old-workspace tap and unsubscribes on unmount', async () => {
  await mount(); await receive(response('abandoned'));
  mockApp.mac = office; mockApp.activeUrl = office.url;
  await act(async () => tree!.update(<App />));
  await showVisual(); await readyAgain();
  expect(mockNavigate).not.toHaveBeenCalled();
  await act(async () => tree!.unmount()); tree = undefined;
  expect(mockReadyListeners.size).toBe(0);
  await readyAgain();
  expect(mockNavigate).not.toHaveBeenCalled();
});

it('a guest or ambiguous notification source cannot leave a future navigation intent', async () => {
  const alert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
  mockApp.servers = [{...home, scope: 'guest'}, office];
  await mount(); await receive(response('guest'));
  mockApp.servers = [home, {...home, url: 'https://duplicate.test'}, office];
  await act(async () => tree!.update(<App />));
  await receive(response('ambiguous'));
  expect(alert).toHaveBeenCalledTimes(2);
  mockApp.servers = [home, office];
  await act(async () => tree!.update(<App />));
  await showVisual(); await readyAgain();
  expect(mockNavigate).not.toHaveBeenCalled();
});
