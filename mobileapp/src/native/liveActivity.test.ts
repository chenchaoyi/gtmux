import {NativeModules} from 'react-native';
import {apnsEnv} from './liveActivity';

// apnsEnv maps the native APNS_ENV constant (which mirrors Apple's aps-environment
// value, "development"/"production") to the APNs endpoint contract ("sandbox"/
// "production"). The bug this guards: a Release-configuration DEV build (__DEV__ is
// false) reporting "development" must resolve to SANDBOX, not fall through to
// production — otherwise its sandbox token is routed to the wrong APNs host and
// every backgrounded push / Live Activity update is silently dropped.
describe('apnsEnv', () => {
  const set = (v: unknown) => {
    (NativeModules as {LiveActivityModule?: {apnsEnv?: unknown}}).LiveActivityModule = {apnsEnv: v};
  };

  it("maps Apple's 'development' to sandbox", () => {
    set('development');
    expect(apnsEnv()).toBe('sandbox');
  });

  it('passes through the endpoint contract values', () => {
    set('sandbox');
    expect(apnsEnv()).toBe('sandbox');
    set('production');
    expect(apnsEnv()).toBe('production');
  });

  it('falls back to __DEV__ only when the constant is absent/garbage', () => {
    set(undefined);
    expect(apnsEnv()).toBe(__DEV__ ? 'sandbox' : 'production');
  });
});

// The lock screen shows the server's name as the user named it on this phone. The name is
// a static attribute of the activity, so a rename used to leave the old name until the
// activity ended; a different name now starts it again (the native side replaces it).
describe('a renamed server restarts the activity under its new name', () => {
  it('starts once, updates, and restarts on a new name', () => {
    jest.isolateModules(() => {
      const start = jest.fn().mockResolvedValue('id');
      const update = jest.fn();
      const end = jest.fn();
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const RN = require('react-native');
      RN.NativeModules.LiveActivityModule = {start, update, end, getPushToken: jest.fn().mockResolvedValue('')};
      RN.Platform.OS = 'ios';
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const {LiveActivity} = require('./liveActivity');
      LiveActivity.sync(1, 0, 0, 't', 's', [], 0, 'Mac Studio');
      LiveActivity.sync(1, 1, 0, 't', 's', [], 0, 'Mac Studio');
      expect(start).toHaveBeenCalledTimes(1);
      expect(update).toHaveBeenCalledTimes(1);
      LiveActivity.sync(1, 1, 0, 't', 's', [], 0, 'Studio (renamed)');
      expect(start).toHaveBeenCalledTimes(2);
      expect(start.mock.calls[1][6]).toBe('Studio (renamed)');
      LiveActivity.sync(2, 1, 0, 't', 's', [], 0, 'Studio (renamed)');
      expect(update).toHaveBeenCalledTimes(2);
    });
  });
});

