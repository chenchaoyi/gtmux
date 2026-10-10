import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, Text, TouchableOpacity} from 'react-native';
import {SettingsScreen} from './SettingsScreen';
import {RouteScreen} from './RouteScreen';
import {SettingsRow} from '../ui/SettingsRow';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {paletteFor} from '../ui/theme';
import {makeT} from '../i18n';
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn()}));
jest.mock('../state/hqMemory', () => ({readCopy: jest.fn().mockResolvedValue(null)}));
let context: any;
let tree: renderer.ReactTestRenderer;
const navigation = {goBack: jest.fn(), navigate: jest.fn(), addListener: jest.fn()};
beforeEach(() => {
  (useApp as jest.Mock).mockReturnValue({t: makeT('en'), lang: 'en', pal: paletteFor('dark'),
    langPref: 'system', themePref: 'system', defaultDetailMode: 'terminal', servers: [], pushKinds: {waiting: true, done: true},
    mac: {name: 'Office', url: 'https://sh.example', route: {id: 'sh', en: 'Shanghai'}}});
  context = {conn: 'live', isGuest: false, client: {routes: jest.fn().mockRejectedValue(new Error('timeout'))}};
  (useAgents as jest.Mock).mockImplementation(() => context);
});
afterEach(() => { act(() => tree?.unmount()); jest.clearAllMocks(); });
async function render(screen: 'settings' | 'route') {
  await act(async () => { tree = renderer.create(screen === 'settings' ? <SettingsScreen navigation={navigation} /> : <RouteScreen navigation={navigation} />); });
}
function texts() { return tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' '); }
test('Settings retains the owner entry and saved name after a route request failure', async () => {
  await render('settings');
  const row = tree.root.findAllByType(SettingsRow).find(n => n.props.label === 'Route')!;
  expect(row.props.icon).toBe('globe');
  expect(tree.root.findAllByType(SettingsRow).find(n => n.props.label === 'Status')!.props.icon).toBe('server');
  expect(row.props.value).toBe('Shanghai');
  expect(row.props.sub).toContain('Could not load routes');
  act(() => row.props.onPress());
  expect(navigation.navigate).toHaveBeenCalledWith('Route');
});
test('guest Settings has neither an entry nor an owner route request', async () => {
  context.isGuest = true;
  await render('settings');
  expect(tree.root.findAllByType(SettingsRow).some(n => n.props.label === 'Route')).toBe(false);
  expect(context.client.routes).not.toHaveBeenCalled();
});
test('route failure has Reload and never pretends there is one route', async () => {
  await render('route');
  expect(texts()).toContain('Could not load routes');
  expect(texts()).not.toContain('has one route');
  const reload = tree.root.findAllByType(TouchableOpacity).find(n => n.findAllByType(Text).some(t => t.props.children === 'Reload routes'))!;
  context.client.routes.mockResolvedValue([]);
  await act(async () => { await reload.props.onPress(); });
  expect(texts()).toContain('No Direct routes available.');
  expect(texts()).not.toContain('Could not load routes');
});

test('a move reply from an earlier Mac cannot select a route on the new Mac', async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = jest.fn().mockResolvedValue({ok: true});
  let resolve!: () => void;
  const reply = new Promise<void>(r => { resolve = r; });
  const routes = [
    {id: 'sh', name: 'Shanghai', en: 'Shanghai', zh: '上海', current: true, url: 'http://sh.example'},
    {id: 'us', name: 'West', en: 'West', zh: '美国西部', current: false, url: 'http://us.example'},
  ];
  context.client = {routes: jest.fn().mockResolvedValue(routes), moveRoute: jest.fn(() => reply)};
  const alert = jest.spyOn(Alert, 'alert');
  try {
    await render('route');
    const target = tree.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel?.startsWith('West'))!;
    act(() => target.props.onPress());
    let pending!: Promise<void>;
    act(() => { pending = alert.mock.calls[0][2]![1].onPress!() as unknown as Promise<void>; });
    context.client = {routes: jest.fn().mockResolvedValue(routes)};
    await act(async () => { tree.update(<RouteScreen navigation={navigation} />); });
    await act(async () => { resolve(); await pending; });
    expect(texts()).toContain('Current route: Shanghai');
    expect(texts()).not.toContain('Current route: West');
  } finally {
    globalThis.fetch = originalFetch;
    alert.mockRestore();
  }
});
