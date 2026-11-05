import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {resetLaunchNotificationForTesting, setupPush} from './index';

jest.mock('react-native', () => ({Platform: {OS: 'ios'}, AppState: {addEventListener: jest.fn(() => ({remove: jest.fn()}))}, NativeModules: {}}));
jest.mock('react-native-keychain', () => ({setGenericPassword: jest.fn(async () => {})}));
jest.mock('@react-native-community/push-notification-ios', () => ({__esModule: true, default: {
  addEventListener: jest.fn(), removeEventListener: jest.fn(), setNotificationCategories: jest.fn(),
  requestPermissions: jest.fn(async () => ({alert: true})), getInitialNotification: jest.fn(), FetchResult: {NoData: 'noData'},
}}));

beforeEach(() => {resetLaunchNotificationForTesting(); jest.clearAllMocks();});
const response = () => ({getData: () => ({pane: '%12', gtmuxResponseId: 'one', userInteraction: true}), getActionIdentifier: () => '2', finish: jest.fn()});

test('requests permission despite a stalled initial response, then delivers the late action once', async () => {
  let deliver!: (value: object) => void;
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockReturnValue(new Promise(resolve => {deliver = resolve;}));
  const reply = jest.fn();
  const stop = await setupPush(jest.fn(), jest.fn(), reply);
  expect(PushNotificationIOS.requestPermissions).toHaveBeenCalledTimes(1);
  expect(reply).not.toHaveBeenCalled();
  deliver(response());
  await Promise.resolve(); await Promise.resolve();
  expect(reply).toHaveBeenCalledTimes(1);
  expect(reply).toHaveBeenCalledWith('%12', undefined, '2');
  stop();
});

test('ignores an initial response that arrives after teardown', async () => {
  let deliver!: (value: object) => void;
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockReturnValue(new Promise(resolve => {deliver = resolve;}));
  const reply = jest.fn();
  const stop = await setupPush(jest.fn(), jest.fn(), reply);
  stop();
  deliver(response());
  await Promise.resolve(); await Promise.resolve();
  expect(reply).not.toHaveBeenCalled();
});

test('hands a late initial response to the live bridge after a server switch', async () => {
  let deliver!: (value: object) => void;
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockReturnValue(new Promise(resolve => {deliver = resolve;}));
  const oldReply = jest.fn(), currentReply = jest.fn();
  const stopOld = await setupPush(jest.fn(), jest.fn(), oldReply);
  stopOld();
  const stopCurrent = await setupPush(jest.fn(), jest.fn(), currentReply);
  expect(PushNotificationIOS.getInitialNotification).toHaveBeenCalledTimes(1);
  deliver(response());
  await Promise.resolve(); await Promise.resolve();
  expect(oldReply).not.toHaveBeenCalled();
  expect(currentReply).toHaveBeenCalledTimes(1);
  stopCurrent();
});

test('cancels a bridge immediately when permission is pending and hands the action to its replacement', async () => {
  let deliver!: (value: object) => void;
  let allowOld!: (value: object) => void;
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockReturnValue(new Promise(resolve => {deliver = resolve;}));
  (PushNotificationIOS.requestPermissions as jest.Mock).mockReturnValueOnce(new Promise(resolve => {allowOld = resolve;}));
  const oldReply = jest.fn(), currentReply = jest.fn(), lifecycle = new AbortController();
  const oldSetup = setupPush(jest.fn(), jest.fn(), oldReply, true, lifecycle.signal);
  await Promise.resolve();
  lifecycle.abort();
  const removed = (PushNotificationIOS.removeEventListener as jest.Mock).mock.calls.length;
  expect(removed).toBe(3);
  const stopCurrent = await setupPush(jest.fn(), jest.fn(), currentReply);
  deliver(response());
  await Promise.resolve(); await Promise.resolve();
  expect(oldReply).not.toHaveBeenCalled();
  expect(currentReply).toHaveBeenCalledTimes(1);
  allowOld({alert: true});
  (await oldSetup)();
  expect(PushNotificationIOS.removeEventListener).toHaveBeenCalledTimes(removed);
  stopCurrent();
});

test('delivers an initial action without waiting for permission', async () => {
  let allow!: (value: object) => void;
  (PushNotificationIOS.getInitialNotification as jest.Mock).mockResolvedValue(response());
  (PushNotificationIOS.requestPermissions as jest.Mock).mockReturnValueOnce(new Promise(resolve => {allow = resolve;}));
  const reply = jest.fn();
  const setup = setupPush(jest.fn(), jest.fn(), reply);
  await Promise.resolve(); await Promise.resolve();
  expect(reply).toHaveBeenCalledTimes(1);
  allow({alert: true});
  (await setup)();
});
