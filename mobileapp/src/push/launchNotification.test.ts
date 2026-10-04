import PushNotificationIOS from '@react-native-community/push-notification-ios';
import {resetLaunchNotificationForTesting, setupPush} from './index';

jest.mock('react-native', () => ({Platform: {OS: 'ios'}}));
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
