import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AppState, Text, TouchableOpacity} from 'react-native';
import * as Keychain from 'react-native-keychain';
import {loadPendingRestore, savePendingRestore, restorePendingService} from '../api/restorePending';
import {RestoreSessionsSheet} from './RestoreSessionsSheet';
import {RestoreError} from '../api/restore';
import {paletteFor} from '../ui/theme';

jest.mock('react-native-keychain', () => {
  const values = new Map<string, string>();
  return {
    getGenericPassword: jest.fn(async (o: any) => values.has(o.service) ? {password: values.get(o.service)} : false),
    setGenericPassword: jest.fn(async (_user: string, value: string, o: any) => {values.set(o.service, value); return true;}),
    resetGenericPassword: jest.fn(async (o: any) => {values.delete(o.service); return true;}),
  };
});
const mac = {name: 'Local read server', url: 'https://local-read.example', token: 'synthetic'};
const pending = {id: '1800000000-localreadfixture-1234567890', revision: 'a'.repeat(64)};
const plan = {revision: 'b'.repeat(64), saved_at: 1800000000, server_time: 1800000000, resume_mode: 'auto', sessions: [{name: 'saved', windows: 1, panes: 1, agents: 1, resumable: 1, state: 'missing'}]};
let tree: renderer.ReactTestRenderer;
let client: any;
const controls = (id: string) => tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID === id);
const text = () => tree.root.findAllByType(Text).flatMap(n => n.props.children).join(' ');
const mount = async () => {await act(async () => {tree = renderer.create(<RestoreSessionsSheet mac={mac} pal={paletteFor('dark')} lang="zh" client={client} onClose={() => {}} />);});};
const press = async (id: string) => {expect(controls(id)).toHaveLength(1); await act(async () => {await controls(id)[0].props.onPress();});};
beforeEach(async () => {
  await Keychain.resetGenericPassword({service: restorePendingService(mac)});
  jest.clearAllMocks();
  Object.defineProperty(AppState, 'currentState', {configurable: true, value: 'active'});
  client = {restorePreview: jest.fn().mockResolvedValue({plan}), restoreStatus: jest.fn().mockRejectedValue(new RestoreError('not_found', 404)), restoreStart: jest.fn(async (id: string, revision: string) => ({id, revision, phase: 'completed', sessions: []}))};
});
afterEach(() => {if (tree) act(() => tree.unmount()); jest.restoreAllMocks();});

test('RSR011 transient native read rejection cannot replace a saved pre-admission ID', async () => {
  await savePendingRestore(mac, pending); jest.clearAllMocks();
  (Keychain.getGenericPassword as jest.Mock).mockRejectedValueOnce(new Error('synthetic read unavailable'));
  await mount();
  expect(client.restorePreview).not.toHaveBeenCalled();
  await press('restore-check');
  await press('restore-start');
  expect({statusCalls: client.restoreStatus.mock.calls, starts: client.restoreStart.mock.calls, saved: await loadPendingRestore(mac)}).toEqual({statusCalls: [[pending.id]], starts: [[pending.id, pending.revision]], saved: pending});
  expect(Keychain.resetGenericPassword).not.toHaveBeenCalled();
});

test('RSR011 repeated native read rejection stays reachable without preview mutation or clear', async () => {
  await savePendingRestore(mac, pending); jest.clearAllMocks();
  const get = Keychain.getGenericPassword as jest.Mock;
  get.mockRejectedValueOnce(new Error('first unavailable')).mockRejectedValueOnce(new Error('still unavailable'));
  await mount();
  await press('restore-check');
  expect(text()).toContain('无法读取上次请求记录');
  expect(controls('restore-check')).toHaveLength(1);
  expect(controls('restore-start')).toHaveLength(0);
  expect(get).toHaveBeenCalledTimes(2);
  expect(client.restorePreview).not.toHaveBeenCalled();
  expect(client.restoreStatus).not.toHaveBeenCalled();
  expect(client.restoreStart).not.toHaveBeenCalled();
  expect(Keychain.setGenericPassword).not.toHaveBeenCalled();
  expect(Keychain.resetGenericPassword).not.toHaveBeenCalled();
  expect(await loadPendingRestore(mac)).toEqual(pending);
});

test('RSR011 confirmed empty local read can resume read-only preview after a native failure', async () => {
  (Keychain.getGenericPassword as jest.Mock).mockRejectedValueOnce(new Error('synthetic unavailable'));
  await mount();
  await press('restore-check');
  expect(Keychain.getGenericPassword).toHaveBeenCalledTimes(2);
  expect(client.restorePreview).toHaveBeenCalledTimes(1);
  expect(controls('restore-start')).toHaveLength(1);
  expect(text()).toContain('saved');
  expect(client.restoreStart).not.toHaveBeenCalled();
  expect(Keychain.setGenericPassword).not.toHaveBeenCalled();
});

test.each(['resolve', 'reject'])('RSR011 late old-pairing local read %s cannot grant a new target readiness', async outcome => {
  (Keychain.getGenericPassword as jest.Mock).mockRejectedValueOnce(new Error('old initial failure'));
  await mount();
  let resolve!: (value: unknown) => void; let reject!: (error: Error) => void;
  (Keychain.getGenericPassword as jest.Mock).mockReturnValueOnce(new Promise((r, j) => {resolve = r; reject = j;}));
  let check!: Promise<void>;
  act(() => {check = controls('restore-check')[0].props.onPress();});
  const newer = {...mac, name: 'New paired server', token: 'new-synthetic-token'};
  (Keychain.getGenericPassword as jest.Mock).mockRejectedValueOnce(new Error('new local read unavailable'));
  const newerClient: any = {restorePreview: jest.fn().mockResolvedValue({plan}), restoreStatus: jest.fn(), restoreStart: jest.fn()};
  await act(async () => {tree.update(<RestoreSessionsSheet mac={newer} pal={paletteFor('dark')} lang="zh" client={newerClient} onClose={() => {}} />);});
  await act(async () => {
    if (outcome === 'resolve') resolve({password: JSON.stringify({...pending, token: mac.token})});
    else reject(new Error('late old failure'));
    await check;
  });
  expect(text()).toContain('New paired server');
  expect(text()).toContain('无法读取上次请求记录');
  expect(controls('restore-start')).toHaveLength(0);
  expect(controls('restore-check')).toHaveLength(1);
  expect(newerClient.restorePreview).not.toHaveBeenCalled();
  expect(newerClient.restoreStatus).not.toHaveBeenCalled();
  expect(newerClient.restoreStart).not.toHaveBeenCalled();
  expect(Keychain.setGenericPassword).not.toHaveBeenCalled();
  expect(Keychain.resetGenericPassword).not.toHaveBeenCalled();
});
