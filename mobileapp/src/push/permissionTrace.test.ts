import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {resetLaunchNotificationForTesting, setupPush} from './index';
import {Debug} from '../debug';

jest.mock('../debug', () => ({Debug: {logNet: true, record: jest.fn()}}));
jest.mock('react-native', () => ({Platform: {OS: 'ios'}, AppState: {addEventListener: () => ({remove: jest.fn()})}, NativeModules: {}}));
jest.mock('@react-native-community/push-notification-ios', () => ({__esModule: true, default: {
  addEventListener: jest.fn(), removeEventListener: jest.fn(), setNotificationCategories: jest.fn(),
  requestPermissions: jest.fn(), checkPermissions: jest.fn(), getInitialNotification: async () => null,
}}));

let resolvePermission: (value: object) => void;
let settingsCallback: (value: object) => void;
let lifetime: AbortController;
const events = () => (Debug.record as jest.Mock).mock.calls.map(([event]) => event.event);
beforeEach(() => {
  jest.useFakeTimers(); resetLaunchNotificationForTesting(); lifetime = new AbortController(); Debug.logNet = true;
  (Debug.record as jest.Mock).mockClear();
  (PushNotificationIOS.requestPermissions as jest.Mock).mockImplementation(() => new Promise(resolve => {resolvePermission = resolve;}));
  (PushNotificationIOS.checkPermissions as jest.Mock).mockReset().mockImplementation(callback => {settingsCallback = callback;});
});
afterEach(() => {lifetime.abort(); jest.useRealTimers();});

test('probe entry distinguishes a pending native settings callback from a timer that never ran', async () => {
  const pending = setupPush(jest.fn(), jest.fn(), jest.fn(), true, lifetime.signal);
  jest.advanceTimersByTime(4999);
  expect(events()).not.toContain('push-permission-probe-start');
  jest.advanceTimersByTime(1);
  expect(events()).toContain('push-permission-probe-start');
  expect(events()).not.toContain('push-permission-settings');
  expect(PushNotificationIOS.checkPermissions).toHaveBeenCalledTimes(1);
  settingsCallback({authorizationStatus: 2, alert: true});
  expect(events()).toContain('push-permission-settings');
  resolvePermission({alert: true});
  const stop = await pending; stop(); stop();
  expect(events().filter(event => event === 'push-subscription-closed')).toHaveLength(1);
});

test('abort before the probe records closure and cancels the existing timer', async () => {
  const pending = setupPush(jest.fn(), jest.fn(), jest.fn(), true, lifetime.signal);
  lifetime.abort(); jest.advanceTimersByTime(5000);
  expect(events()).toContain('push-subscription-closed');
  expect(events()).not.toContain('push-permission-probe-start');
  expect(PushNotificationIOS.checkPermissions).not.toHaveBeenCalled();
  resolvePermission({alert: true}); await pending;
});

test('logging off adds neither a probe nor teardown records', async () => {
  Debug.logNet = false;
  const pending = setupPush(jest.fn(), jest.fn(), jest.fn(), true, lifetime.signal);
  jest.advanceTimersByTime(5000);
  resolvePermission({alert: true}); (await pending)();
  expect(Debug.record).not.toHaveBeenCalled();
  expect(PushNotificationIOS.checkPermissions).not.toHaveBeenCalled();
});
