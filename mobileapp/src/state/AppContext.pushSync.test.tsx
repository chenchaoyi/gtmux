import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AppState, AppStateStatus} from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Keychain from 'react-native-keychain';
import {GtmuxClient} from '../api/client';
import {AppProvider, useApp} from './AppContext';
import type {PairedMac} from '../pairing/qr';

jest.mock('../debug', () => {
  const actual = jest.requireActual('../debug');
  return {...actual, Debug: {...actual.Debug, noPush: false}};
});
jest.mock('../push', () => ({loadPushToken: jest.fn(async () => 'apns-fixture'), getPushToken: () => 'apns-fixture'}));
jest.mock('../diag', () => {
  const actual = jest.requireActual('../diag');
  return {...actual, Diag: {...actual.Diag, info: jest.fn(), secret: jest.fn()}, diagBuffer: {load: jest.fn()}};
});

const office: PairedMac = {url: 'https://office.invalid', name: 'Office', token: 'office-fixture', scope: 'owner'};
const home: PairedMac = {url: 'https://home.invalid', name: 'Home', token: 'home-fixture', scope: 'owner'};
const guest: PairedMac = {url: 'https://guest.invalid', name: 'Guest', token: 'guest-fixture', scope: 'guest'};
let tree: renderer.ReactTestRenderer | undefined;
let app: ReturnType<typeof useApp>;
let register: jest.SpyInstance;
let unregister: jest.SpyInstance;
const listeners = new Set<(state: AppStateStatus) => void>();
const priorState = AppState.currentState;
function Probe() { app = useApp(); return null; }
async function flush() { await act(async () => {for (let i = 0; i < 12; i++) await Promise.resolve();}); }
async function start(servers: PairedMac[] = [office, guest]) {
  (Keychain.getGenericPassword as jest.Mock).mockResolvedValue({username: 'servers', password: JSON.stringify({servers, activeUrl: office.url})});
  await act(async () => {tree = renderer.create(<AppProvider><Probe /></AppProvider>);});
  await flush(); expect(app.ready).toBe(true);
}
async function state(value: AppStateStatus) {
  act(() => {AppState.currentState = value; for (const listener of [...listeners]) listener(value);});
  await flush();
}
async function advance(ms: number) {await act(async () => {await jest.advanceTimersByTimeAsync(ms);}); await flush();}
const callsFor = (url: string) => register.mock.contexts.filter((client: GtmuxClient) => client.base === url).length;
beforeEach(() => {
  jest.useFakeTimers(); jest.setSystemTime(1000000); listeners.clear(); AppState.currentState = 'active';
  (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
  (Keychain.setGenericPassword as jest.Mock).mockResolvedValue(undefined);
  jest.spyOn(AppState, 'addEventListener').mockImplementation((_event, fn) => {
    const listener = fn as (value: AppStateStatus) => void; listeners.add(listener);
    return {remove: () => listeners.delete(listener)};
  });
  register = jest.spyOn(GtmuxClient.prototype, 'registerPush').mockResolvedValue(true);
  unregister = jest.spyOn(GtmuxClient.prototype, 'unregisterPush').mockResolvedValue(true);
});
afterEach(async () => {
  if (tree) {act(() => tree!.unmount()); tree = undefined;}
  await flush(); jest.clearAllTimers(); jest.restoreAllMocks(); jest.useRealTimers(); AppState.currentState = priorState;
});

test('PFR-001 unrelated Mac edits and repeated foreground signals do not replay a successful subscription', async () => {
  await start(); expect(callsFor(office.url)).toBe(1); expect(app.pushSync[office.url]).toBe('synced');
  await act(async () => {await app.renameServer(office.url, 'Desk'); await app.moveServer(office.url, 0);});
  await state('active'); await state('active');
  expect(callsFor(office.url)).toBe(1); expect(callsFor(guest.url)).toBe(0);
  expect(app.pushSync[office.url]).toBe('synced');
  await advance(30 * 60 * 1000); await state('active');
  expect(callsFor(office.url)).toBe(2);
});

test('PFR-001 an unrelated edit cannot invalidate the only in-flight result', async () => {
  let finish!: (ok: boolean) => void;
  register.mockImplementation(() => new Promise<boolean>(resolve => {finish = resolve;}));
  await start(); expect(app.pushSync[office.url]).toBe('syncing');
  await act(async () => {await app.renameServer(office.url, 'Desk');});
  await act(async () => {finish(true);}); await flush();
  expect(callsFor(office.url)).toBe(1); expect(app.pushSync[office.url]).toBe('synced');
});

test('PFR-002 failed automatic sync backs off, pauses in background and resumes without a burst', async () => {
  register.mockResolvedValue(false); await start();
  expect(app.pushSync[office.url]).toBe('pending');
  await state('active'); await state('active'); expect(callsFor(office.url)).toBe(1);
  await advance(29999); expect(callsFor(office.url)).toBe(1);
  await advance(1); expect(callsFor(office.url)).toBe(2);
  await advance(59999); expect(callsFor(office.url)).toBe(2);
  await advance(1); expect(callsFor(office.url)).toBe(3);
  await state('background'); await advance(10 * 60 * 1000); expect(callsFor(office.url)).toBe(3);
  await state('active'); expect(callsFor(office.url)).toBe(4);
  await state('active'); expect(callsFor(office.url)).toBe(4);
});

test('PFR-002 explicit retry bypasses cooldown for the requested Mac and reports busy', async () => {
  register.mockResolvedValue(false); await start([office, home]);
  let finish!: (ok: boolean) => void;
  register.mockImplementation(function (this: GtmuxClient) {
    return this.base === office.url ? new Promise<boolean>(resolve => {finish = resolve;}) : Promise.resolve(false);
  });
  act(() => app.retryPushSync({url: office.url, force: true})); await flush();
  expect(callsFor(office.url)).toBe(2); expect(callsFor(home.url)).toBe(1);
  expect(app.pushSync[office.url]).toBe('syncing');
  act(() => app.retryPushSync({url: office.url, force: true})); await flush();
  expect(callsFor(office.url)).toBe(2);
  await act(async () => {finish(true);}); await flush(); expect(app.pushSync[office.url]).toBe('synced');
});

test('PFR-003 a newer disable remains ordered after the old transport, and stale completion cannot resync it', async () => {
  let finish!: (ok: boolean) => void;
  register.mockImplementation(() => new Promise<boolean>(resolve => {finish = resolve;}));
  await start();
  await act(async () => {await app.setServerPushEnabled(office.url, false);});
  expect(unregister).not.toHaveBeenCalled();
  await act(async () => {finish(true);}); await flush();
  expect(unregister).toHaveBeenCalledTimes(1); expect(app.pushSync[office.url]).toBe('synced');
  expect(callsFor(guest.url)).toBe(0);
});

test('PFR-003 a changed owner credential is immediate during cooldown and another Mac remains independent', async () => {
  register.mockImplementation(function (this: GtmuxClient) {return Promise.resolve(this.base !== office.url);});
  await start([office, home]); expect(app.pushSync[office.url]).toBe('pending'); expect(app.pushSync[home.url]).toBe('synced');
  register.mockResolvedValue(true);
  await act(async () => {await app.pair({...office, token: 'fresh-fixture'});}); await flush();
  expect(callsFor(office.url)).toBe(2); expect(callsFor(home.url)).toBe(1);
  expect(app.pushSync[office.url]).toBe('synced');
});

test('PFR-003 unmount invalidates pending callbacks and every retry timer', async () => {
  register.mockResolvedValue(false); await start(); expect(callsFor(office.url)).toBe(1);
  act(() => tree!.unmount()); tree = undefined;
  await advance(60 * 60 * 1000); expect(callsFor(office.url)).toBe(1); expect(listeners.size).toBe(0);
});
