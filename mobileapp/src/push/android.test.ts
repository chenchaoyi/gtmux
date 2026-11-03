import {DeviceEventEmitter, NativeModules} from 'react-native';
import {loadPushToken, getPushToken, resetLaunchNotificationForTesting, setupPush} from './index.android';

const native = {token: jest.fn(), initial: jest.fn(), badge: jest.fn(), addListener: jest.fn(), removeListeners: jest.fn()};
beforeEach(() => {
  NativeModules.PushNotifications = native;
  jest.clearAllMocks(); resetLaunchNotificationForTesting(); native.token.mockResolvedValue(null); native.initial.mockResolvedValue(null);
});
afterEach(() => {delete NativeModules.PushNotifications; DeviceEventEmitter.removeAllListeners('onPushToken');DeviceEventEmitter.removeAllListeners('onPushTap');});

test('missing configuration does not create a fake registration', async () => {
  const token = jest.fn(); const stop = await setupPush(jest.fn(),token,jest.fn());
  expect(token).not.toHaveBeenCalled(); expect(await loadPushToken()).toBeNull();stop();
});
test('FCM token refresh, one-use cold tap and live tap share lifecycle without executing digits', async () => {
  native.token.mockResolvedValue('synthetic-fcm-token');native.initial.mockResolvedValue({pane:'%12',server:'Test host',digit:'1'});
  const tap=jest.fn(), token=jest.fn(), reply=jest.fn();const stop=await setupPush(tap,token,reply);
  expect(token).toHaveBeenCalledWith('synthetic-fcm-token');expect(getPushToken()).toBe('synthetic-fcm-token');
  expect(tap).toHaveBeenCalledWith('%12','Test host');expect(reply).not.toHaveBeenCalled();
  DeviceEventEmitter.emit('onPushTap',{pane:'%13',server:'Test host'});expect(tap).toHaveBeenCalledTimes(2);
  DeviceEventEmitter.emit('onPushTap',{pane:'not-a-pane',server:'Test host'});expect(tap).toHaveBeenCalledTimes(2);
  DeviceEventEmitter.emit('onPushToken',{token:'rotated-synthetic-token'});expect(token).toHaveBeenLastCalledWith('rotated-synthetic-token');
  stop();DeviceEventEmitter.emit('onPushTap',{pane:'%14',server:'Test host'});expect(tap).toHaveBeenCalledTimes(2);
  const stopAgain=await setupPush(tap,token,reply);expect(native.initial).toHaveBeenCalledTimes(1);stopAgain();
});
