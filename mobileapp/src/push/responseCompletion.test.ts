import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {NativeModules} from 'react-native';
import {resetLaunchNotificationForTesting, setupPush} from './index';

jest.mock('react-native', () => ({Platform: {OS: 'ios'}, AppState: {addEventListener: jest.fn(() => ({remove: jest.fn()}))}, NativeModules: {GtmuxNotificationResponses: {drain: jest.fn(async () => []), acknowledge: jest.fn()}}}));
jest.mock('react-native-keychain', () => ({setGenericPassword: jest.fn(async () => {})}));
jest.mock('@react-native-community/push-notification-ios', () => ({__esModule: true, default: {
  addEventListener: jest.fn(), removeEventListener: jest.fn(), setNotificationCategories: jest.fn(),
  requestPermissions: jest.fn(async () => ({})), getInitialNotification: jest.fn(async () => null), FetchResult: {NoData: 'noData'},
}}));

beforeEach(() => { resetLaunchNotificationForTesting(); jest.clearAllMocks(); });

test.each(['resolve', 'reject'] as const)('waits for a %s reply and suppresses a duplicate while pending', async outcome => {
  const listeners: Record<string, (item: any) => void> = {};
  (PushNotificationIOS.addEventListener as jest.Mock).mockImplementation((name, callback) => {listeners[name] = callback;});
  let settle!: () => void;
  const pending = new Promise<void>((resolve, reject) => {settle = outcome === 'resolve' ? resolve : () => reject(new Error('offline'));});
  const send = jest.fn(() => pending);
  const now = jest.spyOn(Date, 'now').mockReturnValue(1000);
  const finish = jest.fn();
  const item = {getData: () => ({pane: '%12', server: 'fixture', userInteraction: true, gtmuxResponseId: 'id'}), getActionIdentifier: () => '2', finish};
  const stop = await setupPush(jest.fn(), jest.fn(), send);
  listeners.notification(item);
  now.mockReturnValue(32000); // receipt TTL alone must not release an in-flight send
  listeners.notification(item);
  expect(send).toHaveBeenCalledTimes(1);
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).not.toHaveBeenCalled();
  expect(finish).toHaveBeenCalledTimes(1); // duplicate copy is finished, first stays pending
  settle();
  await Promise.resolve(); await Promise.resolve();
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).toHaveBeenCalledTimes(1);
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).toHaveBeenCalledWith('id', '2');
  expect(finish).toHaveBeenCalledTimes(2);
  listeners.notification(item);
  expect(send).toHaveBeenCalledTimes(1);
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).toHaveBeenCalledTimes(2);
  stop();
  now.mockRestore();
});

test('a normal notification tap acknowledges without waiting for a reply', async () => {
  const listeners: Record<string, (item: any) => void> = {};
  (PushNotificationIOS.addEventListener as jest.Mock).mockImplementation((name, callback) => {listeners[name] = callback;});
  const open = jest.fn(), finish = jest.fn();
  const stop = await setupPush(open, jest.fn(), jest.fn());
  listeners.notification({getData: () => ({pane: '%12', gtmuxResponseId: 'tap', userInteraction: true}), getActionIdentifier: () => undefined, finish});
  expect(open).toHaveBeenCalledTimes(1);
  expect(NativeModules.GtmuxNotificationResponses.acknowledge).toHaveBeenCalledWith('tap', '');
  expect(finish).toHaveBeenCalledTimes(1);
  stop();
});
