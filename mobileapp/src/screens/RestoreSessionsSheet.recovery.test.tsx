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
const mac = {name: 'Recovery server', url: 'https://recovery.example', token: 'synthetic'};
const pending = {id: '1800000000-recoveryfixture-1234567890', revision: 'a'.repeat(64)};
const plan = {revision: pending.revision, server_time: 1800000000, resume_mode: 'auto', sessions: [{name: 'saved', windows: 1, panes: 1, agents: 1, resumable: 1, state: 'missing'}]};
const job = (phase = 'partial', name = 'saved') => ({...pending, created_at: 1800000000, updated_at: 1800000000, phase, sessions: [{name, state: 'restored'}]});
let tree: renderer.ReactTestRenderer;
let client: any;
const text = () => tree.root.findAllByType(Text).flatMap(n => n.props.children).join(' ');
const controls = (id: string) => tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID === id);
const mount = async () => {await act(async () => {tree = renderer.create(<RestoreSessionsSheet mac={mac} pal={paletteFor('dark')} lang="zh" client={client} onClose={() => {}} />);});};
beforeEach(async () => {
  await Keychain.resetGenericPassword({service: restorePendingService(mac)});
  jest.clearAllMocks();
  Object.defineProperty(AppState, 'currentState', {configurable: true, value: 'active'});
  client = {restorePreview: jest.fn().mockResolvedValue({plan}), restoreStatus: jest.fn().mockResolvedValue(job()), restoreStart: jest.fn()};
});
afterEach(() => {if (tree) act(() => tree.unmount()); jest.restoreAllMocks();});

test('RSR010 persisted progress remains readable when a new preview is unavailable', async () => {
  await savePendingRestore(mac, pending);
  client.restorePreview.mockRejectedValue(new RestoreError('unavailable', 503));
  await mount();
  expect({statusCalls: client.restoreStatus.mock.calls, hasCheck: controls('restore-check').length > 0, starts: client.restoreStart.mock.calls.length}).toEqual({statusCalls: [[pending.id]], hasCheck: true, starts: 0});
  expect(text()).toContain('请核对结果');
  expect(await loadPendingRestore(mac)).toEqual(pending);
});

test('RSR010 failed status and preview keep a reachable same-ID read-only retry', async () => {
  await savePendingRestore(mac, pending);
  client.restoreStatus.mockRejectedValueOnce(new RestoreError('unavailable', 503)).mockResolvedValueOnce(job('completed'));
  client.restorePreview.mockRejectedValue(new RestoreError('unavailable', 503));
  await mount();
  expect(controls('restore-check')).toHaveLength(1);
  await act(async () => {await controls('restore-check')[0].props.onPress();});
  expect(client.restoreStatus.mock.calls).toEqual([[pending.id], [pending.id]]);
  expect(client.restoreStart).not.toHaveBeenCalled();
  expect(Keychain.resetGenericPassword).not.toHaveBeenCalled();
  expect(await loadPendingRestore(mac)).toEqual(pending);
  expect(text()).toContain('恢复已完成');
});

test('RSR010 failed initial preview can be retried without starting recovery', async () => {
  client.restorePreview.mockRejectedValueOnce(new RestoreError('unavailable', 503));
  await mount();
  expect(controls('restore-check')).toHaveLength(1);
  await act(async () => {await controls('restore-check')[0].props.onPress();});
  expect(client.restorePreview).toHaveBeenCalledTimes(2);
  expect(client.restoreStatus).not.toHaveBeenCalled();
  expect(client.restoreStart).not.toHaveBeenCalled();
  expect(text()).toContain('saved');
});

test('RSR010 late stored-job status cannot replace a different paired server', async () => {
  await savePendingRestore(mac, pending);
  let resolve!: (value: unknown) => void;
  client.restoreStatus.mockReturnValueOnce(new Promise(r => {resolve = r;}));
  await mount();
  await act(async () => {tree.update(<RestoreSessionsSheet mac={{...mac, name: 'Other server', url: 'https://other-recovery.example'}} pal={paletteFor('dark')} lang="zh" client={client} onClose={() => {}} />);});
  await act(async () => resolve(job('completed', 'WRONG OLD JOB')));
  expect(text()).toContain('Other server');
  expect(text()).not.toContain('WRONG OLD JOB');
  expect(client.restoreStart).not.toHaveBeenCalled();
});

test('RSR010 unreadable local identity cannot be cleared into a new request', async () => {
  await Keychain.setGenericPassword('restore', '{invalid', {service: restorePendingService(mac)});
  await mount();
  expect(text()).toContain('无法读取上次请求记录');
  expect(controls('restore-start')).toHaveLength(0);
  expect(client.restoreStart).not.toHaveBeenCalled();
  expect(Keychain.resetGenericPassword).not.toHaveBeenCalled();
});
