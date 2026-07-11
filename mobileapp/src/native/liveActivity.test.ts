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
