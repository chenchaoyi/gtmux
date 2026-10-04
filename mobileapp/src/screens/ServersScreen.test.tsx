import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, Text, TouchableOpacity} from 'react-native';
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
function bells() { return tree.root.findAllByType(TouchableOpacity).filter(n => n.props.accessibilityRole === 'switch'); }
async function render() { await act(async () => { tree = renderer.create(<ServersScreen />); }); }
beforeEach(() => {
  app = {t: makeT('en'), pal: paletteFor('dark'), servers: macs, activeUrl: macs[0].url,
    selectServer: jest.fn(), removeServer: jest.fn(), renameServer: jest.fn().mockResolvedValue(undefined), disconnect: jest.fn(), pushEnabled: true,
    pushKinds: {waiting: true, done: true}, pushSync: {}, setServerPushEnabled: jest.fn().mockResolvedValue(undefined), retryPushSync: jest.fn()};
  agents = {conn: 'live', client: {serverMode: jest.fn().mockResolvedValue({state: 'off'})}};
  (useApp as jest.Mock).mockImplementation(() => app);
  (useAgentsOptional as jest.Mock).mockImplementation(() => agents);
});
afterEach(() => { act(() => tree?.unmount()); jest.restoreAllMocks(); });
test('connect and notification controls are independent; guests have no switch', async () => {
  await render();
  expect(bells()).toHaveLength(2);
  expect(bells().map(b => b.props.accessibilityState.checked)).toEqual([true, false]);
  expect(bells()[1].props.accessibilityLabel).toBe('Home Mac · Notifications');
  await act(async () => { bells()[1].props.onPress(); });
  expect(app.setServerPushEnabled).toHaveBeenCalledWith(macs[1].url, true);
  await act(async () => { bells()[0].props.onPress(); });
  expect(app.setServerPushEnabled).toHaveBeenCalledWith(macs[0].url, false);
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
test('one line per Mac: the address moves to More, a healthy row has no second line', async () => {
  const alert = jest.spyOn(Alert, 'alert');
  await render();
  expect(texts()).not.toContain('https://');
  expect(texts()).not.toContain('Connect ');
  expect(button('Office Mac, Connected')).toBeDefined();
  expect(button('Home Mac, Connect')).toBeDefined();
  act(() => button('Home Mac · More options').props.onPress());
  expect(alert.mock.calls[0].slice(0, 2)).toEqual(['Home Mac', macs[1].url]);
});
test('global pause preserves the per-Mac preference and is said once', async () => {
  app.pushEnabled = false;
  await render();
  expect(texts().split('Notifications are paused in Settings.')).toHaveLength(2);
  expect(bells()[0].props.accessibilityState.checked).toBe(true);
});
test('offline selected Mac is not connected; the server-mode marker does not follow a switch', async () => {
  agents.client.serverMode.mockResolvedValue({system_disablesleep: true});
  await render();
  expect(button('Office Mac, Connected, server mode')).toBeDefined();
  app.activeUrl = macs[1].url;
  agents = {conn: 'offline', client: {serverMode: jest.fn().mockReturnValue(new Promise(() => {}))}};
  await act(async () => { tree.update(<ServersScreen />); });
  expect(button('Home Mac, Offline')).toBeDefined();
  expect(texts()).toContain('Home Mac Offline');
  expect(button('Office Mac, Connect')).toBeDefined();
  expect(tree.root.findAllByType(TouchableOpacity).some(n => /server mode/.test(n.props.accessibilityLabel ?? ''))).toBe(false);
  expect(texts()).not.toContain('Connected');
});
test('a Mac that refused this phone says so: the radar sends the reader here to pair again', async () => {
  agents = {conn: 'unauthorized', client: {serverMode: jest.fn().mockReturnValue(new Promise(() => {}))}};
  await render();
  expect(button('Office Mac, Access rejected')).toBeDefined();
  expect(texts()).toContain('Office Mac Access rejected');
  expect(texts()).not.toContain('Offline');
});
test('rename is in More, prefilled, and says the Mac keeps its own name', async () => {
  const alert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
  const prompt = jest.spyOn(Alert, 'prompt').mockImplementation(() => {});
  app.servers = [{...macs[0], name: 'Desk', macName: 'Office Mac'}, macs[1]];
  await render();
  act(() => button('Desk · More options').props.onPress());
  expect(alert.mock.calls[0][1]).toBe(`Office Mac\n${macs[0].url}`);
  act(() => alert.mock.calls[0][2]!.find(a => a.text === 'Rename')!.onPress!());
  const [title, hint, buttons, type, initial] = prompt.mock.calls[0] as any[];
  expect([title, type, initial]).toEqual(['Rename', 'plain-text', 'Desk']);
  expect(hint).toContain('“Office Mac”');
  await act(async () => { buttons.find((b: any) => b.text === 'Save').onPress('Work'); });
  expect(app.renameServer).toHaveBeenCalledWith(macs[0].url, 'Work');
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
