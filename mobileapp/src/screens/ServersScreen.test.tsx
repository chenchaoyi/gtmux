import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AccessibilityInfo, Alert, ScrollView, Text, TouchableOpacity} from 'react-native';
import {ServersScreen} from './ServersScreen';
import {useApp} from '../state/AppContext';
import {useAgentsOptional} from '../state/AgentsContext';
import {paletteFor} from '../ui/theme';
import {makeT} from '../i18n';
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgentsOptional: jest.fn()}));
jest.mock('./PairingScreen', () => ({PairingScreen: () => null}));
jest.mock('./DemoScreen', () => ({DemoScreen: () => null}));
// The first render in this file loads React Native's lazily required modules (Animated and
// the rest of what a row draws), and a cold transform cache bills that to the first test:
// measured 7.8s on main and 5.8s on this branch with --no-cache, against ~30ms for every
// later test, and CI's first test stopped at jest's 5s default with no assertion failing
// (run 37257758811). The budget is the file's start-up, not the screen's behaviour.
jest.setTimeout(20_000);
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
    selectServer: jest.fn(), removeServer: jest.fn(), renameServer: jest.fn().mockResolvedValue(undefined), moveServer: jest.fn().mockResolvedValue(undefined), disconnect: jest.fn(), pushEnabled: true,
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

// Reordering (ReorderableList): hold a row, drag it, let go; or VoiceOver's actions.
describe('the reader orders the list', () => {
  // A touch as PanResponder reads it: our handler reads pageY, PanResponder its history.
  // PanResponder moves by each record's previous → current step, so the record carries both.
  let clock = 1000;
  let lastY: number | null = null;
  function touch(y: number) {
    clock += 16;
    const prev = lastY ?? y;
    lastY = y;
    const rec = {touchActive: true, startPageX: 0, startPageY: y, startTimeStamp: clock, currentPageX: 0, currentPageY: y,
      currentTimeStamp: clock, previousPageX: 0, previousPageY: prev, previousTimeStamp: clock - 16};
    return {nativeEvent: {pageX: 0, pageY: y, locationX: 0, locationY: y, touches: [], changedTouches: []},
      touchHistory: {numberActiveTouches: 1, indexOfSingleActiveTouch: 0, mostRecentTimeStamp: clock, touchBank: [rec]}};
  }
  const rowsIn = (id: string) => tree.root.findAll(n => typeof n.type === 'string' && n.props.testID === id)[0]
    // The drag wrappers: the only views that capture a move (a Touchable takes the grant, not the capture).
    .findAll(n => typeof n.type === 'string' && typeof n.props.onMoveShouldSetResponderCapture === 'function');
  const order = () => tree.root.findAllByType(TouchableOpacity).map(n => n.props.accessibilityLabel)
    .filter((l: string) => /^(Office|Home|Guest) Mac, /.test(l ?? '')).map((l: string) => l.split(',')[0]);
  async function layOut(id: string) {
    await act(async () => {
      rowsIn(id).forEach((r, i) => r.props.onLayout({nativeEvent: {layout: {x: 0, y: i * 52, width: 360, height: 52}}}));
    });
  }
  const scrolling = () => tree.root.findByType(ScrollView).props.scrollEnabled;
  beforeEach(() => {
    lastY = null;
  });

  test('VoiceOver can move a row without dragging, and hears where it went', async () => {
    const say = jest.spyOn(AccessibilityInfo, 'announceForAccessibility').mockImplementation(() => {});
    await render();
    const office = button('Office Mac, Connected');
    expect(office.props.accessibilityActions.map((a: any) => a.name)).toEqual(['moveDown']);
    expect(button('Home Mac, Connect').props.accessibilityActions.map((a: any) => a.name)).toEqual(['moveUp']);
    expect(button('Guest Mac, Connect').props.accessibilityActions).toEqual([]);
    await act(async () => {
      office.props.onAccessibilityAction({nativeEvent: {actionName: 'moveDown'}});
    });
    expect(app.moveServer).toHaveBeenCalledWith(macs[0].url, 1);
    expect(say).toHaveBeenCalledWith('Office Mac, now 2 of 2');
    expect(app.selectServer).not.toHaveBeenCalled();
  });

  test('hold, drag, let go: the row moves to where it was dropped, and the page scrolls again', async () => {
    await render();
    await layOut('servers-mine');
    await act(async () => {
      button('Home Mac, Connect').props.onLongPress();
    });
    expect(scrolling()).toBe(false);
    const home = rowsIn('servers-mine')[1];
    await act(async () => {
      home.props.onResponderGrant(touch(78));
      home.props.onResponderMove(touch(38)); // up 40: its top edge passes Office's centre
      home.props.onResponderRelease(touch(38));
    });
    expect(app.moveServer).toHaveBeenCalledWith(macs[1].url, 0);
    expect(order()).toEqual(['Home Mac', 'Office Mac', 'Guest Mac']); // shown at once
    expect(scrolling()).toBe(true);
    expect(app.selectServer).not.toHaveBeenCalled();
    expect(app.activeUrl).toBe(macs[0].url);
  });

  test('a drag the system takes away, or a hold let go in place, moves nothing', async () => {
    await render();
    await layOut('servers-mine');
    await act(async () => {
      button('Home Mac, Connect').props.onLongPress();
    });
    const home = rowsIn('servers-mine')[1];
    await act(async () => {
      home.props.onResponderGrant(touch(78));
      home.props.onResponderMove(touch(30));
      home.props.onResponderTerminate(touch(30));
    });
    expect(app.moveServer).not.toHaveBeenCalled();
    expect(scrolling()).toBe(true);
    expect(order()).toEqual(['Office Mac', 'Home Mac', 'Guest Mac']);
    // Held and let go without moving.
    await act(async () => {
      button('Home Mac, Connect').props.onLongPress();
    });
    expect(scrolling()).toBe(false);
    await act(async () => {
      button('Home Mac, Connect').props.onPressOut();
    });
    expect(scrolling()).toBe(true);
    expect(app.moveServer).not.toHaveBeenCalled();
  });

  test('a move that does not save goes back, and says so', async () => {
    const alert = jest.spyOn(Alert, 'alert');
    app.moveServer.mockRejectedValueOnce(new Error('keychain locked'));
    await render();
    await layOut('servers-mine');
    await act(async () => {
      button('Home Mac, Connect').props.onLongPress();
    });
    const home = rowsIn('servers-mine')[1];
    await act(async () => {
      home.props.onResponderGrant(touch(78));
      home.props.onResponderMove(touch(38));
      home.props.onResponderRelease(touch(38));
    });
    expect(alert).toHaveBeenCalledWith("Couldn't save the new order, so the list is as it was.");
    expect(order()).toEqual(['Office Mac', 'Home Mac', 'Guest Mac']);
  });

  test('a section of one has nothing to drag; the hint shows only when there is', async () => {
    await render();
    expect(button('Guest Mac, Connect').props.onLongPress).toBeUndefined();
    expect(texts()).toContain('Hold a Mac and drag it to change the order.');
    app.servers = [macs[0], macs[2]];
    await act(async () => {
      tree.update(<ServersScreen />);
    });
    expect(button('Office Mac, Connected').props.onLongPress).toBeUndefined();
    expect(texts()).not.toContain('Hold a Mac and drag it');
  });
});
