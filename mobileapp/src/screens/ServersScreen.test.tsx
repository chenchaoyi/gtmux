import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AccessibilityInfo, Alert, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ServersScreen} from './ServersScreen';
import {useApp} from '../state/AppContext';
import {useAgentsOptional} from '../state/AgentsContext';
import {paletteFor} from '../ui/theme';
import {makeT} from '../i18n';
import {forgetHosts} from '../state/hostInfo';
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
/** A Mac's connect target, whatever its status says. */
function row(name: string) { return tree.root.findAllByType(TouchableOpacity).find(n => (n.props.accessibilityLabel ?? '').startsWith(`${name},`))!; }
// Whether each Mac answers its health probe; the screen asks while it is shown.
let answers: Record<string, boolean>;
const realFetch = globalThis.fetch;
async function render() { await act(async () => { tree = renderer.create(<ServersScreen />); }); }
beforeEach(() => {
  app = {t: makeT('en'), pal: paletteFor('dark'), servers: macs, activeUrl: macs[0].url,
    selectServer: jest.fn(), removeServer: jest.fn().mockResolvedValue(undefined), renameServer: jest.fn().mockResolvedValue(undefined), moveServer: jest.fn().mockResolvedValue(undefined), disconnect: jest.fn(), pushEnabled: true,
    pushKinds: {waiting: true, done: true}, pushSync: {}, setServerPushEnabled: jest.fn().mockResolvedValue(undefined), retryPushSync: jest.fn()};
  agents = {conn: 'live', client: {serverMode: jest.fn().mockResolvedValue({state: 'off'})}};
  (useApp as jest.Mock).mockImplementation(() => app);
  (useAgentsOptional as jest.Mock).mockImplementation(() => agents);
  answers = {[macs[0].url]: true, [macs[1].url]: false, [macs[2].url]: true};
  globalThis.fetch = jest.fn((u: string) => Promise.resolve({ok: !!answers[u.replace(/\/api\/health$/, '')]})) as any;
});
afterEach(() => { act(() => tree?.unmount()); jest.restoreAllMocks(); globalThis.fetch = realFetch; jest.useRealTimers(); });
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
  await act(async () => { row('Home Mac').props.onPress(); });
  expect(app.selectServer).toHaveBeenCalledWith(macs[1].url);
});
test('two layers: a check on the open Mac, and on every row whether it answers', async () => {
  const alert = jest.spyOn(Alert, 'alert');
  await render();
  // Which one is open: the check, on that row only.
  const checks = tree.root.findAll(n => typeof n.props.testID === 'string' && n.props.testID.startsWith('server-current-'));
  expect([...new Set(checks.map(n => n.props.testID))]).toEqual(['server-current-0']); // host and composite share it
  // Whether each answers: the open one by its link, the others by the probe.
  expect(row('Office Mac').props.accessibilityLabel).toBe('Office Mac, current, Connected');
  expect(row('Home Mac').props.accessibilityLabel).toBe("Home Mac, Can't reach");
  expect(row('Guest Mac').props.accessibilityLabel).toBe('Guest Mac, Available');
  expect(globalThis.fetch).toHaveBeenCalledWith('https://home.example/api/health', expect.anything());
  // The address is in More, not on the row.
  expect(texts()).not.toContain('https://');
  act(() => button('Home Mac · More options').props.onPress());
  expect(alert.mock.calls[0].slice(0, 2)).toEqual(['Home Mac', macs[1].url]);
});
test('before the probe answers, a Mac reads as checking', async () => {
  globalThis.fetch = jest.fn(() => new Promise(() => {})) as any;
  await render();
  expect(row('Home Mac').props.accessibilityLabel).toBe('Home Mac, Checking…');
});
test('a pending setting is said on the status line, with no retry control', async () => {
  app.pushSync[macs[1].url] = 'pending'; // Home: muted, and off
  await render();
  expect(row('Home Mac').props.accessibilityLabel).toBe("Home Mac, Can't reach · it may still notify until it answers");
  expect(texts()).not.toMatch(/Retry/);
  expect(app.retryPushSync).not.toHaveBeenCalled();
});
test('a Mac with a pending setting gets it again the moment it answers', async () => {
  jest.useFakeTimers();
  app.pushSync[macs[1].url] = 'pending';
  await render();
  expect(app.retryPushSync).not.toHaveBeenCalled();
  answers[macs[1].url] = true; // Home comes back
  await act(async () => { jest.advanceTimersByTime(15_000); });
  expect(app.retryPushSync).toHaveBeenCalledTimes(1);
  await act(async () => { jest.advanceTimersByTime(15_000); }); // still answering: no second push
  expect(app.retryPushSync).toHaveBeenCalledTimes(1);
});
test('nothing moves: every row is the same height, and a sync in flight shows nothing', async () => {
  await render();
  const heights = () => tree.root.findAllByType(View)
    .filter(n => StyleSheet.flatten(n.props.style)?.height === 62).length;
  expect(heights()).toBe(3);
  app.pushSync = {[macs[0].url]: 'syncing', [macs[1].url]: 'syncing'}; // a bell was tapped
  agents = {...agents, conn: 'connecting'}; // and a switch is under way
  await act(async () => { tree.update(<ServersScreen />); });
  expect(heights()).toBe(3);
  expect(texts()).not.toMatch(/Updating/);
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
  expect(row('Office Mac').props.accessibilityLabel).toBe('Office Mac, current, Connected, server mode');
  app.activeUrl = macs[1].url;
  agents = {conn: 'offline', client: {serverMode: jest.fn().mockReturnValue(new Promise(() => {}))}};
  await act(async () => { tree.update(<ServersScreen />); });
  expect(row('Home Mac').props.accessibilityLabel).toBe("Home Mac, current, Can't reach");
  expect(texts()).toContain("Home Mac Can't reach");
  expect(row('Office Mac').props.accessibilityLabel).toBe('Office Mac, Available');
  expect(tree.root.findAllByType(TouchableOpacity).some(n => /server mode/.test(n.props.accessibilityLabel ?? ''))).toBe(false);
  expect(texts()).not.toContain('Connected');
});
test('a Mac that refused this phone says so: the radar sends the reader here to pair again', async () => {
  agents = {conn: 'unauthorized', client: {serverMode: jest.fn().mockReturnValue(new Promise(() => {}))}};
  await render();
  expect(row('Office Mac').props.accessibilityLabel).toBe('Office Mac, current, Access rejected');
  expect(texts()).toContain('Office Mac Access rejected');
  expect(texts()).not.toContain("Office Mac Can't reach"); // refused is not unreachable
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

// A removal that could not be saved says so; it used to fail without a word (F12).
test('a removal that fails says the Mac is still in the list', async () => {
  const alert = jest.spyOn(Alert, 'alert');
  await render();
  (app.removeServer as jest.Mock).mockRejectedValueOnce(new Error('keychain locked'));
  act(() => button('Home Mac · More options').props.onPress());
  act(() => alert.mock.calls[0][2]!.find(a => a.style === 'destructive')!.onPress!());
  await act(async () => {
    alert.mock.calls[1][2]!.find(a => a.style === 'destructive')!.onPress!();
    await new Promise<void>(r => setTimeout(() => r(), 0));
  });
  expect(alert.mock.calls.map(c => c[0])).toContain("Couldn't remove this Mac, so it is still in the list.");
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
    const office = row('Office Mac');
    expect(office.props.accessibilityActions.map((a: any) => a.name)).toEqual(['moveDown']);
    expect(row('Home Mac').props.accessibilityActions.map((a: any) => a.name)).toEqual(['moveUp']);
    expect(row('Guest Mac').props.accessibilityActions).toEqual([]);
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
      row('Home Mac').props.onLongPress();
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
      row('Home Mac').props.onLongPress();
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
      row('Home Mac').props.onLongPress();
    });
    expect(scrolling()).toBe(false);
    await act(async () => {
      row('Home Mac').props.onPressOut();
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
      row('Home Mac').props.onLongPress();
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
    expect(row('Guest Mac').props.onLongPress).toBeUndefined();
    expect(texts()).toContain('Hold a Mac and drag it to change the order.');
    app.servers = [macs[0], macs[2]];
    await act(async () => {
      tree.update(<ServersScreen />);
    });
    expect(row('Office Mac').props.onLongPress).toBeUndefined();
    expect(texts()).not.toContain('Hold a Mac and drag it');
  });
});

// What each owned Mac is (GET /api/host): a clause on its status line, so the row keeps
// its two lines, and the full details behind ••• → Details. A share link is never asked.
test('an owned Mac shows what it is, and Details opens what it reported', async () => {
  forgetHosts();
  const asked: string[] = [];
  globalThis.fetch = jest.fn((u: string) => {
    if (u.endsWith('/api/host')) {
      asked.push(u);
      return Promise.resolve({ok: true, status: 200, headers: {get: () => null}, json: async () => ({
        hostname: 'studio.local', computer_name: 'Studio', os: 'macOS', os_version: '26.1', os_build: '25B78',
        arch: 'arm64', cpu: 'Apple M4 Max', cores: 16, memory_bytes: 64 * 2 ** 30, gtmux_version: '1.0.95', serve_started: 1})});
    }
    return Promise.resolve({ok: !!answers[u.replace(/\/api\/health$/, '')]});
  }) as any;
  await render();
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise<void>(r => setTimeout(() => r(), 0)); });
  expect(texts()).toContain('Studio · macOS 26.1');
  expect(asked.some(u => u.startsWith(macs[2].url))).toBe(false); // the guest link is never asked

  const alert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
  act(() => button('Office Mac · More options').props.onPress());
  act(() => alert.mock.calls[0][2]!.find(a => a.text === 'Details')!.onPress!());
  for (let i = 0; i < 3; i++) await act(async () => { await new Promise<void>(r => setTimeout(() => r(), 0)); });
  const shown = texts();
  for (const want of ['Computer name', 'Studio', 'macOS 26.1 (25B78)', 'Apple M4 Max · arm64', '64 GB', '1.0.95']) {
    expect(shown).toContain(want);
  }
});


// %12's review of #1429: an answer is the credential's, not the address's, and it ages.
describe('host details follow the credential and expire', () => {
  const studio = {hostname: 'studio.local', computer_name: 'Studio', os: 'macOS', os_version: '26.1', arch: 'arm64', cores: 16, gtmux_version: '1.0.95', serve_started: 1};
  let hostAnswers: number;
  let hostStatus: number;
  beforeEach(() => {
    forgetHosts();
    hostAnswers = 0;
    hostStatus = 200;
    globalThis.fetch = jest.fn((u: string) => {
      if (u.endsWith('/api/host')) {
        hostAnswers++;
        return Promise.resolve({ok: hostStatus === 200, status: hostStatus, headers: {get: () => null}, json: async () => (hostStatus === 200 ? studio : {})});
      }
      return Promise.resolve({ok: !!answers[u.replace(/\/api\/health$/, '')]});
    }) as any;
  });
  const settle = async (n = 5) => {
    for (let i = 0; i < n; i++) await act(async () => { await Promise.resolve(); });
  };
  test('the same address re-paired as a share link never shows the owner\'s answer', async () => {
    await render();
    await settle();
    expect(texts()).toContain('Studio · macOS 26.1');
    act(() => tree.unmount());
    app = {...app, servers: [{...macs[0], token: 'z', scope: 'guest'}, macs[1]]};
    await render();
    await settle();
    expect(texts()).not.toContain('Studio');
  });
  test('an answer is asked again once it is five minutes old', async () => {
    jest.useFakeTimers({now: 1_000_000, doNotFake: ['nextTick', 'queueMicrotask']});
    await render();
    await settle();
    expect(hostAnswers).toBe(1);
    await act(async () => { jest.advanceTimersByTime(4 * 60_000); });
    await settle();
    expect(hostAnswers).toBe(1);
    await act(async () => { jest.advanceTimersByTime(2 * 60_000); });
    await settle();
    expect(hostAnswers).toBe(2);
  });
  test('a credential the Mac rejects says so, not that it is a share link', async () => {
    hostStatus = 401;
    await render();
    await settle();
    const alert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
    act(() => button('Office Mac · More options').props.onPress());
    act(() => alert.mock.calls[0][2]!.find(a => a.text === 'Details')!.onPress!());
    await settle();
    expect(texts()).toContain("no longer accepts this phone's credentials");
    expect(texts()).not.toContain("A share link doesn't include");
  });
  // The list says so too, not "Available" (%6's observation, 2026-10-06). The open Mac
  // speaks for its live link, so this is about every other Mac.
  test('a Mac that refuses this phone reads Access rejected on the list', async () => {
    hostStatus = 401;
    answers[macs[1].url] = true;
    await render();
    await settle();
    expect(row('Home Mac').props.accessibilityLabel).toContain('Access rejected');
    expect(row('Guest Mac').props.accessibilityLabel).toContain('Available'); // never asked
  });
});
