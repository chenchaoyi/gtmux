import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, NativeModules} from 'react-native';
import PushNotificationIOS from '@react-native-community/push-notification-ios';
import App from '../App';
import {useApp} from '../src/state/AppContext';
import {loadServers, saveServers, ServerStore} from '../src/pairing/store';
import {resetLaunchNotificationForTesting, setupPush} from '../src/push';
import {Debug} from '../src/debug';

jest.mock('react-native-safe-area-context/src/NativeSafeAreaProvider', () => ({NativeSafeAreaProvider: 'NativeSafeAreaProvider'}));
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context/jest/mock').default,
  ...jest.requireActual('react-native-safe-area-context/src/SafeAreaContext'),
}));
jest.mock('../src/push', () => ({
  ...jest.requireActual('../src/push'), setupPush: jest.fn(jest.requireActual('../src/push').setupPush),
}));
// Real AppContext useState, serialize queue and persist; hold only the storage boundary.
jest.mock('../src/pairing/store', () => ({
  ...jest.requireActual('../src/pairing/store'), loadServers: jest.fn(), saveServers: jest.fn(),
}));
jest.mock('../src/state/AgentsContext', () => ({
  AgentsProvider: ({children, base}: any) => {
    mockApp = require('../src/state/AppContext').useApp();
    mockRenders.push({active: mockApp.activeUrl, pending: mockApp.pendingPane});
    require('react').useEffect(() => {mockMounts.push(base); return () => {mockUnmounts.push(base);};}, [base]);
    return children;
  }, useAgents: () => ({agents: mockAgents}),
}));
jest.mock('../src/screens/ServersScreen', () => ({ServersScreen: () => {
  mockApp = require('../src/state/AppContext').useApp(); return null;
}}));
// Observe the real context even while the no-server SafeAreaProvider gates UI.
jest.mock('../src/usage/UsageWidgetBridge', () => ({UsageWidgetBridge: () => null, useUsageWidgetContext: () => {
  mockApp = require('../src/state/AppContext').useApp();
}}));
jest.mock('@react-navigation/native', () => {
  const R = require('react');
  const ref = jest.requireActual('@react-navigation/core').createNavigationContainerRef();
  return {
    NavigationContainer: ({children}: any) => {
      const {activeUrl} = require('../src/state/AppContext').useApp();
      R.useLayoutEffect(() => {
        ref.current = {isReady: () => true,
          navigate: (route: string, params: any) => mockNavigate(activeUrl, route, params),
          addListener: (name: string, cb: () => void) => {
            if (name === 'ready') {mockReadyListeners.add(cb); mockAllReady.push(cb);}
            return () => mockReadyListeners.delete(cb);
          },
        };
        for (const cb of mockReadyListeners) cb();
        return () => {ref.current = null; mockReadyListeners.clear();};
      }, [activeUrl]);
      return children;
    },
    useNavigationContainerRef: () => ref, NavigationContext: R.createContext(undefined),
    useNavigation: () => ({navigate: jest.fn(), goBack: jest.fn()}),
    DefaultTheme: {colors: {}}, DarkTheme: {colors: {}},
  };
});
const home = {url: 'https://home.test', token: 'fixture', name: 'Home'};
const office = {url: 'https://office.test', token: 'fixture', name: 'Office'};
const studio = {url: 'https://studio.test', token: 'fixture', name: 'Studio'};
let mockApp: ReturnType<typeof useApp>;
let mockAgents: any[] = [];
const mockRenders: {active: string | null; pending: string | null}[] = [];
const mockMounts: string[] = [], mockUnmounts: string[] = [];
const mockNavigate = jest.fn();
const mockReadyListeners = new Set<() => void>();
const mockAllReady: (() => void)[] = [];
const listeners: Record<string, (value: any) => void> = {};
const saves: {store: ServerStore; resolve: () => void; reject: (error: Error) => void}[] = [];
const nativeBefore = NativeModules.GtmuxNotificationResponses;
const debugBefore = {...Debug};
let tree: renderer.ReactTestRenderer | undefined;
beforeEach(() => {
  jest.useFakeTimers(); jest.clearAllMocks(); resetLaunchNotificationForTesting();
  mockAgents = []; saves.length = 0; mockRenders.length = 0; mockMounts.length = 0; mockUnmounts.length = 0;
  mockReadyListeners.clear(); mockAllReady.length = 0;
  (loadServers as jest.Mock).mockResolvedValue({servers: [home, office, studio], activeUrl: home.url});
  (saveServers as jest.Mock).mockImplementation((store: ServerStore) => new Promise<void>((resolve, reject) => saves.push({store, resolve, reject})));
  NativeModules.GtmuxNotificationResponses = {drain: jest.fn().mockResolvedValue([]), acknowledge: jest.fn()};
  Object.assign(Debug, {skipWhatsNew: true, noPush: false, logNet: false, pairUrl: '', pairToken: '', seedServers: '', resetServers: false, shotMode: false});
  (PushNotificationIOS as any).FetchResult = {NoData: 'noData'};
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(null);
  (PushNotificationIOS.addEventListener as jest.Mock).mockImplementation((name, cb) => {listeners[name] = cb;});
});
afterEach(async () => {
  await act(async () => tree?.unmount()); tree = undefined;
  NativeModules.GtmuxNotificationResponses = nativeBefore; Object.assign(Debug, debugBefore);
  jest.useRealTimers(); jest.restoreAllMocks();
});
async function mount() {await act(async () => {tree = renderer.create(<App />);}); expect(mockApp.activeUrl).toBe(home.url);}
async function visual() {
  await act(async () => tree!.root.findByType('NativeSafeAreaProvider' as never).props.onInsetsChange({nativeEvent: {
    frame: {x: 0, y: 0, width: 393, height: 852}, insets: {top: 59, right: 0, bottom: 34, left: 0},
  }}));
}
async function tap(id: string, server: string, pane = '%12') {
  await act(async () => listeners.notification({getData: () => ({pane, server, userInteraction: true, gtmuxResponseId: id}), finish: jest.fn()}));
}
async function release(index: number) {await act(async () => saves[index].resolve());}
async function ready(callbacks = [...mockReadyListeners]) {await act(async () => callbacks.forEach(cb => cb()));}
function expectNavigation(target: string, pane: string, count = 1) {
  expect(mockNavigate).toHaveBeenCalledTimes(count);
  expect(mockNavigate).toHaveBeenLastCalledWith(target, 'Detail', expect.objectContaining({agent: expect.objectContaining({pane_id: pane})}));
}
async function manual(target: string | null, index: number) {
  let selection!: Promise<void>;
  await act(async () => {selection = target === null ? mockApp.disconnect() : mockApp.selectServer(target);});
  await release(index); await selection;
}

it('real persistence gap never routes a cross-server tap on the old ready workspace', async () => {
  await mount(); await visual(); const renders = mockRenders.length;
  await tap('office-ready', 'Office', '%21');
  expect(saves).toHaveLength(1); expect(saves[0].store.activeUrl).toBe(office.url);
  expect(mockRenders.length).toBeGreaterThan(renders);
  expect(mockApp.activeUrl).toBe(home.url); expect(mockMounts).toEqual([home.url]);
  expect(mockNavigate).not.toHaveBeenCalled();
  const oldReady = [...mockAllReady]; await release(0);
  expect(mockApp.activeUrl).toBe(office.url);
  expect(mockMounts).toEqual([home.url, office.url]); expect(mockUnmounts).toEqual([home.url]);
  await ready(oldReady); expect(mockNavigate).not.toHaveBeenCalled();
  await visual(); await ready(); await tap('office-ready', 'Office', '%21');
  expectNavigation(office.url, '%21');
});
it('real persistence gap preserves a cross-server tap past old headless bridge teardown', async () => {
  await mount(); await tap('office-headless', 'Office', '%22');
  expect(mockApp.activeUrl).toBe(home.url); expect(saves).toHaveLength(1); expect(mockNavigate).not.toHaveBeenCalled();
  await release(0); expect(mockApp.activeUrl).toBe(office.url); expect(mockUnmounts).toEqual([home.url]);
  await act(async () => {jest.advanceTimersByTime(3000);});
  mockAgents = [{pane_id: '%22', session_id: 'latest-office', source: 'tmux'}];
  await act(async () => {mockApp.setLangPref('en');});
  await visual(); await ready(); expectNavigation(office.url, '%22');
  expect(mockNavigate.mock.calls[0][2].agent.session_id).toBe('latest-office');
});
it('new cross-server generation survives an older selection completion and opens only its target', async () => {
  await mount(); await tap('older-office', 'Office', '%21'); await tap('newer-studio', 'Studio', '%31');
  expect(saves).toHaveLength(1); await release(0);
  expect(saves).toHaveLength(2); expect(mockApp.activeUrl).toBe(office.url);
  await visual(); await ready(); expect(mockNavigate).not.toHaveBeenCalled();
  await release(1); await visual(); await ready(); expectNavigation(studio.url, '%31');
});
it('a new same-origin tap orders its selection after an outstanding foreign write', async () => {
  await mount(); await tap('old-office', 'Office', '%21'); await tap('new-home', 'Home', '%11');
  await release(0); expect(saves).toHaveLength(2); expect(saves[1].store.activeUrl).toBe(home.url);
  await visual(); await ready(); expect(mockNavigate).not.toHaveBeenCalled();
  await release(1); await visual(); await ready(); expectNavigation(home.url, '%11');
});
it('failed selection is discarded and cannot replay when the user later visits its server', async () => {
  await mount(); await tap('failed-office', 'Office', '%21');
  await act(async () => saves[0].reject(new Error('controlled Keychain failure')));
  expect(mockApp.activeUrl).toBe(home.url); await visual(); expect(mockNavigate).not.toHaveBeenCalled();
  await manual(office.url, 1); await visual(); await ready(); expect(mockNavigate).not.toHaveBeenCalled();
});
it('an older rejected write cannot clear a newer generation', async () => {
  await mount(); await tap('fail-old', 'Office', '%21'); await tap('live-new', 'Studio', '%31');
  await act(async () => saves[0].reject(new Error('controlled old failure')));
  expect(saves).toHaveLength(2); await release(1); await visual(); expectNavigation(studio.url, '%31');
});
it('a committed manual departure cancels an unshown target intent even after revisiting', async () => {
  await mount(); await tap('leave-target', 'Office', '%21'); await release(0);
  await manual(studio.url, 1); await visual(); expect(mockNavigate).not.toHaveBeenCalled();
  await manual(office.url, 2); await visual(); await ready(); expect(mockNavigate).not.toHaveBeenCalled();
});
it('disconnect clears a waiting intent before a later reconnect', async () => {
  await mount(); await tap('disconnect-target', 'Office', '%21'); await release(0);
  await manual(null, 1); expect(mockApp.activeUrl).toBeNull();
  await manual(office.url, 2); await visual(); expect(mockNavigate).not.toHaveBeenCalled();
});
it('root unmount rejects late selection completion and old ready callbacks', async () => {
  await mount(); await visual(); await tap('unmounted', 'Office', '%21'); const callbacks = [...mockAllReady];
  await act(async () => tree!.unmount()); tree = undefined; await release(0); await ready(callbacks);
  expect(mockReadyListeners.size).toBe(0); expect(mockNavigate).not.toHaveBeenCalled();
});
it('same-server taps still wait for readiness then deliver fresh responses once without saving', async () => {
  await mount(); await tap('same-cold', 'Home', '%11'); expect(mockNavigate).not.toHaveBeenCalled();
  await visual(); await ready(); expectNavigation(home.url, '%11');
  await tap('same-cold', 'Home', '%11'); await tap('same-warm', 'Home', '%12'); await ready();
  expectNavigation(home.url, '%12', 2); expect(saves).toHaveLength(0);
});


it('starts the actual notification bridge while the visual safe area has no metrics', async () => {
  await mount();
  expect(setupPush).toHaveBeenCalledTimes(1);
  expect(tree!.root.findByType('NativeSafeAreaProvider' as never).props.children).toBeNull();
});

it('a cold same-server tap waits beyond the inset gate then opens the latest agent exactly once', async () => {
  const data = {pane: '%12', server: 'Home', userInteraction: true, gtmuxResponseId: 'cold'};
  (NativeModules.GtmuxNotificationResponses.drain as jest.Mock).mockResolvedValue([data]);
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue({getData: () => data, finish: jest.fn()});
  await mount();
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).toHaveBeenCalledWith('cold', '');
  expect(mockNavigate).not.toHaveBeenCalled();
  await act(async () => {jest.advanceTimersByTime(3000);});
  expect(mockNavigate).not.toHaveBeenCalled();
  mockAgents = [{pane_id: '%12', session_id: 'latest-session', source: 'tmux', role: 'supervisor'}];
  await act(async () => {mockApp.setLangPref('en');});
  await visual();
  expect(mockNavigate).toHaveBeenCalledTimes(1);
  expect(mockNavigate).toHaveBeenCalledWith(home.url, 'HQ', expect.objectContaining({agent: mockAgents[0]}));
  await ready(); await tap('cold', 'Home');
  expect(mockNavigate).toHaveBeenCalledTimes(1);
});

it('a warm tap navigates immediately and a new response is not mistaken for a replay', async () => {
  await mount(); await visual();
  await tap('warm1', 'Home'); await tap('warm1', 'Home');
  expectNavigation(home.url, '%12');
  await tap('warm2', 'Home', '%13'); await ready();
  expectNavigation(home.url, '%13', 2);
});

it('a user server change discards a queued old-workspace tap and unsubscribes on unmount', async () => {
  await mount(); await tap('abandoned', 'Home');
  await manual(office.url, 0); await visual(); await ready();
  expect(mockNavigate).not.toHaveBeenCalled();
  const callbacks = [...mockAllReady];
  await act(async () => tree!.unmount()); tree = undefined;
  expect(mockReadyListeners.size).toBe(0); await ready(callbacks);
  expect(mockNavigate).not.toHaveBeenCalled();
});

it('a guest or ambiguous notification source cannot leave a future navigation intent', async () => {
  const alert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
  (loadServers as jest.Mock).mockResolvedValue({servers: [
    {...home, scope: 'guest'}, office, {...office, url: 'https://duplicate.test'},
  ], activeUrl: home.url});
  await mount(); await tap('guest', 'Home'); await tap('ambiguous', 'Office');
  expect(alert).toHaveBeenCalledTimes(2); expect(saves).toHaveLength(0);
  let pairing!: Promise<void>;
  await act(async () => {pairing = mockApp.pair(home);});
  await release(0); await pairing; // actual context/Keychain transaction, no mockApp assignment
  await visual(); await ready(); expect(mockNavigate).not.toHaveBeenCalled();
});
