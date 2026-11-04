import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {resetLaunchNotificationForTesting, setupPush} from './index';
import {NativeModules} from 'react-native';

jest.mock('react-native', () => ({Platform: {OS: 'ios'}, NativeModules: {GtmuxNotificationResponses: {drain: jest.fn(), acknowledge: jest.fn()}}}));
jest.mock('react-native-keychain', () => ({setGenericPassword: jest.fn(() => Promise.resolve())}));
jest.mock('@react-native-community/push-notification-ios', () => ({
  __esModule: true,
  default: {
    addEventListener: jest.fn(),
    removeEventListener: jest.fn(),
    setNotificationCategories: jest.fn(),
    requestPermissions: jest.fn(() => Promise.resolve({})),
    checkPermissions: jest.fn(cb => cb({authorizationStatus: 2})),
    getInitialNotification: jest.fn(),
    FetchResult: {NoData: 'noData'},
  },
}));

const launch = {getData: () => ({pane: '%5', server: 'ccy Home MBP2019'})};

beforeEach(() => {
  resetLaunchNotificationForTesting();
  (NativeModules.GtmuxNotificationResponses.drain as jest.Mock).mockResolvedValue([]);
  (NativeModules.GtmuxNotificationResponses.acknowledge as jest.Mock).mockClear();
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(launch);
});

// The notification that launched the app opens its pane once. setupPush runs again each
// time the active Mac changes; replaying the launch tap switched the phone back to the Mac
// that sent it, so picking another Mac in the server list landed on the first one's pane.
test('the launch notification is acted on once, not on every server switch', async () => {
  const taps: Array<[string, string | undefined]> = [];
  const onTap = (pane: string, server?: string) => taps.push([pane, server]);
  await setupPush(onTap, () => {}, () => {});
  expect(taps).toEqual([['%5', 'ccy Home MBP2019']]);
  await setupPush(onTap, () => {}, () => {}); // the bridge remounts after a server switch
  await setupPush(onTap, () => {}, () => {});
  expect(taps).toHaveLength(1);
});

test('a launch with no notification taps nothing', async () => {
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(null);
  const onTap = jest.fn();
  await setupPush(onTap, () => {}, () => {});
  expect(onTap).not.toHaveBeenCalled();
});

function response(id: string, action?: string) {
  return {
    getData: () => ({pane: '%5', server: 'fixture', gtmuxResponseId: id, userInteraction: true, userText: 'typed'}),
    getActionIdentifier: () => action,
    finish: jest.fn(),
  };
}

test.each(['initial-first', 'event-first'] as const)(
  'the same quick action arriving from scene and delegate sends once (%s)',
  async order => {
    const listeners: Record<string, (value: any) => void> = {};
    (PushNotificationIOS.addEventListener as jest.Mock).mockImplementation((name, cb) => { listeners[name] = cb; });
    const item = response('request-1', '2');
    const tap = jest.fn(), reply = jest.fn();
    if (order === 'initial-first') {
      (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(item);
      await setupPush(tap, () => {}, reply);
      listeners.notification(item);
    } else {
      let resolve!: (value: any) => void;
      (PushNotificationIOS.getInitialNotification as jest.Mock).mockReturnValue(new Promise(r => { resolve = r; }));
      const setup = setupPush(tap, () => {}, reply);
      await Promise.resolve();
      listeners.notification(item);
      resolve(item);
      await setup;
    }
    expect(reply).toHaveBeenCalledTimes(1);
    expect(reply).toHaveBeenCalledWith('%5', 'fixture', '2');
    expect(tap).not.toHaveBeenCalled();
  },
);

test('distinct notification responses are not deduplicated', async () => {
  const listeners: Record<string, (value: any) => void> = {};
  (PushNotificationIOS.addEventListener as jest.Mock).mockImplementation((name, cb) => { listeners[name] = cb; });
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(response('one'));
  const tap = jest.fn();
  await setupPush(tap, () => {}, () => {});
  listeners.notification(response('two'));
  expect(tap).toHaveBeenCalledTimes(2);
});

test('an unknown action does not become a normal pane tap', async () => {
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(response('one', 'unknown'));
  const tap = jest.fn(), reply = jest.fn();
  await setupPush(tap, () => {}, reply);
  expect(tap).not.toHaveBeenCalled();
  expect(reply).not.toHaveBeenCalled();
});

test('a foreground delivery or inherited action name cannot trigger a reply', async () => {
  const listeners: Record<string, (value: any) => void> = {};
  (PushNotificationIOS.addEventListener as jest.Mock).mockImplementation((name, cb) => { listeners[name] = cb; });
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(null);
  const tap = jest.fn(), reply = jest.fn();
  await setupPush(tap, () => {}, reply);
  listeners.notification({getData: () => ({pane: '%5'}), getActionIdentifier: () => '2'});
  listeners.notification(response('other', 'toString'));
  expect(tap).not.toHaveBeenCalled();
  expect(reply).not.toHaveBeenCalled();
});

test('a legacy default action opens the pane once across repeated setup', async () => {
  const item = response('body-one', 'com.apple.UNNotificationDefaultActionIdentifier');
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(item);
  const tap = jest.fn();
  await setupPush(tap, () => {}, () => {});
  await setupPush(tap, () => {}, () => {});
  expect(tap).toHaveBeenCalledTimes(1);
  expect(tap).toHaveBeenCalledWith('%5', 'fixture');
});


test('a buffered native response and the launch copy dispatch one quick reply', async () => {
  const data = {pane: '%5', server: 'fixture', gtmuxResponseId: 'buffered', userInteraction: true, actionIdentifier: '2', userText: 'typed'};
  (NativeModules.GtmuxNotificationResponses.drain as jest.Mock).mockResolvedValue([data]);
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(response('buffered', '2'));
  const tap = jest.fn(), reply = jest.fn();
  await setupPush(tap, () => {}, reply);
  expect(reply).toHaveBeenCalledTimes(1);
  expect(reply).toHaveBeenCalledWith('%5', 'fixture', '2');
  expect(tap).not.toHaveBeenCalled();
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).toHaveBeenCalledWith('buffered', '2');
});

test('a native buffer failure still delivers the bridge launch notification', async () => {
  (NativeModules.GtmuxNotificationResponses.drain as jest.Mock).mockRejectedValue(new Error('unavailable'));
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(response('fallback', '1'));
  const reply = jest.fn();
  await setupPush(jest.fn(), () => {}, reply);
  expect(reply).toHaveBeenCalledTimes(1);
  expect(reply).toHaveBeenCalledWith('%5', 'fixture', '1');
});
