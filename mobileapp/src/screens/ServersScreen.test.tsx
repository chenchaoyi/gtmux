import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, Switch, Text, TouchableOpacity} from 'react-native';
import {ServersScreen} from './ServersScreen';
import {useApp} from '../state/AppContext';
import {useAgentsOptional} from '../state/AgentsContext';
import {paletteFor} from '../ui/theme';
import {makeT} from '../i18n';
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgentsOptional: jest.fn()}));
jest.mock('./PairingScreen', () => ({PairingScreen: () => null}));
jest.mock('./DemoScreen', () => ({DemoScreen: () => null}));
const macs = [
  {name: 'Office Mac', url: 'https://office.example', token: 'a'},
  {name: 'Home Mac', url: 'https://home.example', token: 'b', pushEnabled: false},
  {name: 'Guest Mac', url: 'https://guest.example', token: 'c', scope: 'guest'},
];
let app: any;
let agents: any;
let tree: renderer.ReactTestRenderer;
function texts() { return tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' '); }
function button(label: string) { return tree.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel === label)!; }
async function render() { await act(async () => { tree = renderer.create(<ServersScreen />); }); }
beforeEach(() => {
  app = {t: makeT('en'), pal: paletteFor('dark'), servers: macs, activeUrl: macs[0].url,
    selectServer: jest.fn(), removeServer: jest.fn(), disconnect: jest.fn(), pushEnabled: true,
    pushKinds: {waiting: true, done: true}, pushSync: {}, setServerPushEnabled: jest.fn().mockResolvedValue(undefined), retryPushSync: jest.fn()};
  agents = {conn: 'live', client: {serverMode: jest.fn().mockResolvedValue({state: 'off'})}};
  (useApp as jest.Mock).mockImplementation(() => app);
  (useAgentsOptional as jest.Mock).mockImplementation(() => agents);
});
afterEach(() => { act(() => tree?.unmount()); jest.restoreAllMocks(); });
test('connect and notification controls are independent; guests have no switch', async () => {
  await render();
  const switches = tree.root.findAllByType(Switch);
  expect(switches).toHaveLength(2);
  expect(switches.map(s => s.props.value)).toEqual([true, false]);
  await act(async () => { switches[1].props.onValueChange(true); });
  expect(app.setServerPushEnabled).toHaveBeenCalledWith(macs[1].url, true);
  expect(app.selectServer).not.toHaveBeenCalled();
  await act(async () => { button('Home Mac, Connect').props.onPress(); });
  expect(app.selectServer).toHaveBeenCalledWith(macs[1].url);
});
test('pending muted source has a truthful notice and independent retry', async () => {
  app.pushSync[macs[1].url] = 'pending';
  await render();
  expect(texts()).toContain('This Mac may still send notifications');
  act(() => button('Home Mac · Retry sync').props.onPress());
  expect(app.retryPushSync).toHaveBeenCalledTimes(1);
});
test('global pause preserves the per-Mac preference', async () => {
  app.pushEnabled = false;
  await render();
  expect(texts()).toContain('Notifications are paused in Settings.');
  expect(tree.root.findAllByType(Switch)[0].props.value).toBe(true);
});
test('offline selected Mac is not connected; the server-mode marker does not follow a switch', async () => {
  agents.client.serverMode.mockResolvedValue({system_disablesleep: true});
  await render();
  expect(texts()).toContain('server mode');
  app.activeUrl = macs[1].url;
  agents = {conn: 'offline', client: {serverMode: jest.fn().mockReturnValue(new Promise(() => {}))}};
  await act(async () => { tree.update(<ServersScreen />); });
  expect(button('Home Mac, Offline')).toBeDefined();
  expect(texts()).not.toContain('server mode');
  expect(texts()).not.toContain('Connected');
});
test('removal is in More and requires confirmation', async () => {
  const alert = jest.spyOn(Alert, 'alert');
  await render();
  act(() => button('Home Mac · More options').props.onPress());
  act(() => alert.mock.calls[0][2]!.find(a => a.style === 'destructive')!.onPress!());
  expect(app.removeServer).not.toHaveBeenCalled();
  act(() => alert.mock.calls[1][2]!.find(a => a.style === 'destructive')!.onPress!());
  expect(app.removeServer).toHaveBeenCalledWith(macs[1].url);
});
