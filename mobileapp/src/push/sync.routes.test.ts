import {AppState} from 'react-native';
import {syncServerPush, PushTarget} from './sync';

const server = {url: 'https://old.invalid', alts: ['https://good.invalid'], token: 'same-owner', name: 'Mac'};
test('PFR-004 a verified reported alternate is preferred without retrying a dead primary', async () => {
  const calls: string[] = [], success = jest.fn();
  const factory = (url: string): PushTarget => ({registerPush: async () => {calls.push(url); return true;}, unregisterPush: async () => true});
  expect(await syncServerPush(server, 'apns', true, ['done'], 'production', factory, {preferredUrl: server.alts[0], onSuccess: success})).toBe(true);
  expect(calls).toEqual([server.alts[0]]); expect(success).toHaveBeenCalledWith(server.alts[0]);
});
test('PFR-004 an unknown route is never attempted and background work cannot record route success', async () => {
  const calls: string[] = [], success = jest.fn();
  const factory = (url: string, auth: string): PushTarget => ({registerPush: async () => {expect(auth).toBe(server.token); calls.push(url); return true;}, unregisterPush: async () => true});
  expect(await syncServerPush(server, 'apns', true, [], 'production', factory, {preferredUrl: 'https://foreign.invalid', onSuccess: success})).toBe(true);
  expect(calls).toEqual([server.url]); expect(success).toHaveBeenCalledWith(server.url);
  const prior = AppState.currentState; AppState.currentState = 'background'; success.mockClear();
  try {expect(await syncServerPush(server, 'apns', true, [], 'production', factory, {preferredUrl: server.url, onSuccess: success})).toBe(false); expect(success).not.toHaveBeenCalled();}
  finally {AppState.currentState = prior;}
});
