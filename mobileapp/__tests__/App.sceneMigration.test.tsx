import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Dimensions, FlatList, NativeModules, ScrollView, StyleSheet, TouchableOpacity, View} from 'react-native';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import * as Keychain from 'react-native-keychain';
import App from '../App';
import {Debug} from '../src/debug';
import {HQView} from '../src/screens/HQScreen';
import {RadarPanel} from '../src/screens/RadarPanel';
import {ServersScreen} from '../src/screens/ServersScreen';
import {Splash} from '../src/ui/Splash';
import {WorkspaceProvider} from '../src/state/WorkspaceContext';
import {AnchoredMenu} from '../src/ui/AnchoredMenu';
import {DiffModal} from '../src/ui/DiffModal';
import {WindowMetricsProvider} from '../src/ui/windowMetrics';
import {paletteFor} from '../src/ui/theme';

// In addition to the shared Jest native/navigation/storage boundaries, the
// native stack host below mounts the real initial Radar and size-class decision.
// Real consumers remain; this is not native stack or UIKit hit-testing evidence.
jest.mock('@react-navigation/native-stack', () => ({
  createNativeStackNavigator: () => ({
    Navigator: ({children}: any) => children,
    Screen: ({name, component: Component}: any) => name === 'Radar' ? <Component /> : null,
  }),
}));
jest.mock('react-native-safe-area-context/src/NativeSafeAreaProvider', () => ({NativeSafeAreaProvider: 'NativeSafeAreaProvider'}));
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context/jest/mock').default,
  ...jest.requireActual('react-native-safe-area-context/src/SafeAreaContext'),
}));

const originalFetch = global.fetch;
const originalNative = NativeModules.GtmuxNotificationResponses;
const originalWindow = Dimensions.get('window');
const originalDebug = {...Debug};
let tree: renderer.ReactTestRenderer | undefined;
let agents: any[] = [];
const scene = (width: number, height: number) => ({frame: {x: 0, y: 0, width, height}, insets: {top: 24, bottom: 20, left: 0, right: 0}});
function setNativeWindow(width: number, height: number) {
  Dimensions.set({window: {...originalWindow, width, height}, screen: {...originalWindow, width, height}});
}
async function rootLayout(width: number, height: number) {
  await act(async () => {
    tree!.root.findAllByType(View).find(n => n.props.testID === 'window-metrics-root')!.props.onLayout({nativeEvent: {layout: {x: 0, y: 0, width, height}}});
  });
}
beforeEach(() => {
  agents = [];
  setNativeWindow(393, 852);
  Debug.skipWhatsNew = true;
  NativeModules.GtmuxNotificationResponses = {currentSceneMetrics: jest.fn().mockResolvedValue(scene(1024, 768)), drain: jest.fn().mockResolvedValue([]), drainSceneLinks: jest.fn().mockResolvedValue([])};
  (Keychain.getGenericPassword as jest.Mock).mockResolvedValue({username: 'servers', password: JSON.stringify({servers: [{url: 'https://fixture.test', token: 'synthetic', name: 'Fixture'}], activeUrl: 'https://fixture.test'})});
  global.fetch = jest.fn(async (url: any) => ({ok: true, status: 200, headers: {get: () => null}, json: async () => String(url).endsWith('/api/agents') ? agents : String(url).endsWith('/api/share') ? {all: true, input: true} : {}, text: async () => ''})) as any;
});
afterEach(async () => {
  await act(async () => tree?.unmount());
  tree = undefined;
  NativeModules.GtmuxNotificationResponses = originalNative;
  Object.assign(Debug, originalDebug);
  (Keychain.getGenericPassword as jest.Mock).mockResolvedValue(false);
  global.fetch = originalFetch;
  Dimensions.set({window: originalWindow, screen: originalWindow});
});

test('main App keeps compact navigation props and a fresh root changes its real shell', async () => {
  await act(async () => {tree = renderer.create(<App />);});
  expect(tree!.root.findByType(WorkspaceProvider).props).not.toHaveProperty('inlineSelection');
  expect(tree!.root.findAllByType(RadarPanel).some(n => n.props.variant === 'screen')).toBe(true);
  await rootLayout(1024, 768);
  expect(Dimensions.get('window').width).toBe(393);
  expect(tree!.root.findAllByType(RadarPanel).some(n => n.props.variant === 'sidebar')).toBe(true);
});

test('main App sidebar geometry follows the new root even when RN retains its old phone width', async () => {
  await act(async () => {tree = renderer.create(<App />);});
  await rootLayout(1024, 768);
  const panel = tree!.root.findAllByType(RadarPanel).find(n => n.props.variant === 'sidebar')!;
  const sidebar = panel.parent!;
  expect(Dimensions.get('window').width).toBe(393);
  expect(StyleSheet.flatten(sidebar.props.style).width).toBe(300);
  await rootLayout(900, 768);
  expect(StyleSheet.flatten(tree!.root.findAllByType(RadarPanel).find(n => n.props.variant === 'sidebar')!.parent!.props.style).width).toBe(280);
  expect(Dimensions.get('window').width).toBe(393);
});

test('a real anchored menu stays inside a new narrow root while RN width remains stale', async () => {
  const pal = paletteFor('light');
  await act(async () => {tree = renderer.create(
    <WindowMetricsProvider><SafeAreaProvider initialMetrics={scene(213, 600)}>
      <AnchoredMenu visible anchor={{x: 160, y: 100, width: 44, height: 44}} title="Fixture" subtitle={[]}
        sections={[[{key: 'open', label: 'Open', icon: 'info', onPress: jest.fn()}]]}
        pal={pal} closeLabel="Close" onClose={jest.fn()} testID="closure-menu" />
    </SafeAreaProvider></WindowMetricsProvider>,
  );});
  await rootLayout(213, 600);
  const box = tree!.root.findAllByType(View).find(n => n.props.testID === 'closure-menu')!;
  await act(async () => box.props.onLayout({nativeEvent: {layout: {height: 100}}}));
  const style = StyleSheet.flatten(box.props.style);
  expect(Dimensions.get('window').width).toBe(393);
  expect(style.width).toBe(197); // 213pt canvas minus the existing two8pt edges.
  expect(style.left + style.width).toBeLessThanOrEqual(205);
});


test('retained DiffModal follows its actual viewport layout and keeps wrap and close usable with stale RN dimensions', async () => {
  const close = jest.fn();
  const diff = jest.fn(async () => 'diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new');
  await act(async () => {tree = renderer.create(
    <WindowMetricsProvider><SafeAreaProvider initialMetrics={scene(1024, 768)}>
      <DiffModal visible paneId="%13" client={{diff} as any} pal={paletteFor('light')} lang="en" onClose={close} />
    </SafeAreaProvider></WindowMetricsProvider>,
  );});
  await rootLayout(1024, 768);
  const viewport = tree!.root.findAllByType(ScrollView).find(n => n.props.testID === 'diff-horizontal')!;
  await act(async () => viewport.props.onLayout({nativeEvent: {layout: {width: 960, height: 500}}}));
  const list = tree!.root.findByType(FlatList);
  expect(Dimensions.get('window').width).toBe(393);
  expect(StyleSheet.flatten(list.props.style).width).toBe(960);
  expect(list.props.data.some((line: string) => line.includes('new'))).toBe(true);
  const wrap = tree!.root.findAllByType(TouchableOpacity).find(n => n.props.testID === 'diff-wrap')!;
  await act(async () => {wrap.props.onPress(); wrap.props.onPress();});
  await rootLayout(768, 1024);
  await act(async () => viewport.props.onLayout({nativeEvent: {layout: {width: 704, height: 700}}}));
  expect(StyleSheet.flatten(tree!.root.findByType(FlatList).props.style).width).toBe(704);
  const closeButton = tree!.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel === 'Close changes')!;
  await act(async () => closeButton.props.onPress());
  expect(close).toHaveBeenCalledTimes(1);
  expect(diff).toHaveBeenCalledTimes(1);
});


test('real HQ inspector follows root resize and sidebar hiding while RN remains stale', async () => {
  agents = [{role: 'supervisor', agent: 'Codex', pane_id: '%6', session_id: 'fixture-hq', session: 'fixture', status: 'idle'}];
  await act(async () => {tree = renderer.create(<App />);});
  await rootLayout(1180, 768);
  expect(tree!.root.findByType(HQView).props.showInspector).toBe(true);
  await rootLayout(900, 768);
  expect(tree!.root.findByType(HQView).props.showInspector).toBe(false);
  const panel = tree!.root.findAllByType(RadarPanel).find(n => n.props.variant === 'sidebar')!;
  await act(async () => panel.props.onHideSidebar());
  expect(tree!.root.findByType(HQView).props.showInspector).toBe(true);
  expect(Dimensions.get('window').width).toBe(393);
});

test('real menu position and height follow a second root resize with nonzero safe-area edges', async () => {
  const metrics = {...scene(400, 500), insets: {top: 24, bottom: 20, left: 10, right: 14}};
  await act(async () => {tree = renderer.create(<WindowMetricsProvider><SafeAreaProvider initialMetrics={metrics}>
    <AnchoredMenu visible anchor={{x: 235, y: 200, width: 44, height: 44}} title="Fixture" subtitle={[]}
      sections={[[{key: 'open', label: 'Open', icon: 'info', onPress: jest.fn()}]]}
      pal={paletteFor('light')} closeLabel="Close" onClose={jest.fn()} testID="resize-menu" />
  </SafeAreaProvider></WindowMetricsProvider>);});
  await rootLayout(400, 500);
  const box = () => tree!.root.findAllByType(View).find(n => n.props.testID === 'resize-menu')!;
  await act(async () => box().props.onLayout({nativeEvent: {layout: {height: 100}}}));
  const initial = StyleSheet.flatten(box().props.style);
  expect(initial.maxHeight).toBe(440); expect(initial.top).toBe(200 + 44 + 4);
  await rootLayout(300, 320);
  const resized = StyleSheet.flatten(box().props.style);
  expect(resized.width).toBe(250); expect(resized.maxHeight).toBe(260);
  expect(resized.left).toBeGreaterThanOrEqual(18); expect(resized.left + resized.width).toBeLessThanOrEqual(278);
  expect(resized.top).toBe(200 - 4 - 100); expect(resized.top + 100).toBeLessThanOrEqual(292);
  expect(Dimensions.get('window').width).toBe(393);
});


test('real App keeps safe-area ownership while startup resolves from Splash to unpaired Servers', async () => {
  let restore: (value: false) => void;
  (Keychain.getGenericPassword as jest.Mock).mockReturnValue(new Promise<false>(resolve => {restore = resolve;}));
  await act(async () => {tree = renderer.create(<App />);});
  const splashProvider = tree!.root.findAllByType(SafeAreaProvider).find(n => n.props.children?.type === Splash);
  expect(splashProvider).toBeDefined();
  const host = tree!.root.findByType('NativeSafeAreaProvider' as never);
  await act(async () => host.props.onInsetsChange({nativeEvent: scene(393, 852)}));
  expect(tree!.root.findAll(n => n.props.testID === 'launch-mark').length).toBeGreaterThan(0);
  await act(async () => restore!(false));
  expect(tree!.root.findAllByType(SafeAreaProvider).some(n => n.props.children?.type === ServersScreen)).toBe(true);
  expect(tree!.root.findAll(n => n.props.testID === 'servers-screen').length).toBeGreaterThan(0);
  expect(tree!.root.findAllByType(RadarPanel)).toHaveLength(0);
});
