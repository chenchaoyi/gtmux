import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AppState, Text} from 'react-native';
import * as Keychain from 'react-native-keychain';
import {loadPendingRestore, restorePendingService} from '../api/restorePending';
import {RestoreSessionsSheet} from './RestoreSessionsSheet';
import {RestoreError} from '../api/restore';
import {paletteFor} from '../ui/theme';
jest.mock('react-native-keychain', () => {const values = new Map<string, string>(); return {getGenericPassword: jest.fn(async (o: any) => values.has(o.service) ? {password: values.get(o.service)} : false), setGenericPassword: jest.fn(async (_user: string, v: string, o: any) => {values.set(o.service, v); return true;}), resetGenericPassword: jest.fn(async (o: any) => {values.delete(o.service); return true;})};});
const mac = {name: 'Office server', url: 'https://office.example', token: 'synthetic'};
const rev = 'a'.repeat(64);
const plan = {revision: rev, server_time: 1800000000, saved_at: 1800000000, resume_mode: 'auto', sessions: [{name: 'saved', windows: 1, panes: 2, agents: 1, resumable: 1, state: 'missing'}]};
let tree: renderer.ReactTestRenderer;
let client: any;
const text = () => tree.root.findAllByType(Text).flatMap(n => n.props.children).join(' ');
const press = (id: string) => tree.root.findByProps({testID: id}).props.onPress();
const job = (id: string, phase = 'layout') => ({id, revision: rev, created_at: 1800000000, updated_at: 1800000000, phase, sessions: plan.sessions, agents: [{loc: 'saved:0.0', pane: '%7', state: 'pending'}]});
const deferred = () => {let resolve!: (v: any) => void; const promise = new Promise(r => {resolve = r;}); return {promise, resolve};};
async function mount(server: any = mac, c = client) {await act(async () => {tree = renderer.create(<RestoreSessionsSheet mac={server} pal={paletteFor('dark')} lang="zh" client={c} onClose={() => {}} />);});}
beforeEach(async () => {await Keychain.resetGenericPassword({service: restorePendingService(mac)}); await Keychain.resetGenericPassword({service: restorePendingService({...mac,url:"https://home.example"})}); Object.defineProperty(AppState, 'currentState', {configurable: true, value: 'active'}); client = {restorePreview: jest.fn().mockResolvedValue({plan}), restoreStart: jest.fn((id: string) => Promise.resolve(job(id))), restoreStatus: jest.fn((id: string) => Promise.resolve(job(id)))};});
afterEach(() => {if (tree) act(() => tree.unmount()); jest.useRealTimers(); jest.restoreAllMocks();});
jest.setTimeout(20000);
test('preview is not a restore; duplicate clicks start one persisted request and reopening only reads it', async () => {
  await mount(); expect(client.restoreStart).not.toHaveBeenCalled(); expect(text()).toContain('Office server'); expect(text()).toContain('saved');
  const wait = deferred(); client.restoreStart.mockImplementation((id: string) => wait.promise.then(() => job(id)));
  await act(async () => {press('restore-start'); press('restore-start');});
  expect(client.restoreStart).toHaveBeenCalledTimes(1);
  const [id, revision] = client.restoreStart.mock.calls[0]; expect(revision).toBe(rev); expect((await loadPendingRestore(mac))?.id).toBe(id);
  await act(async () => wait.resolve(null)); expect(text()).toContain('恢复会话与窗格'); expect(text()).toContain('尚未确认启动');
  act(() => tree.unmount()); await mount(); expect(client.restoreStart).toHaveBeenCalledTimes(1); expect(client.restoreStatus).toHaveBeenCalledWith(id);
});
test('response loss retry reuses the exact request and never changes to another transport', async () => {
  client.restoreStart.mockRejectedValueOnce(new RestoreError('uncertain'));
  await mount(); await act(async () => {await press('restore-start');}); const id = client.restoreStart.mock.calls[0][0]; expect(text()).toContain('重试同一请求');
  await act(async () => {await press('restore-start');}); expect(client.restoreStart.mock.calls.map((r: any[]) => r[0])).toEqual([id, id]);
});
test('host change during pending local persistence cannot post to either target', async () => {
  const gate = deferred(); jest.spyOn(Keychain, 'setGenericPassword').mockImplementationOnce(() => gate.promise as any);
  await mount(); await act(async () => {press('restore-start');});
  const other = {...mac, name: 'Home server', url: 'https://home.example'};
  await act(async () => {tree.update(<RestoreSessionsSheet mac={other} pal={paletteFor('dark')} lang="zh" client={client} onClose={() => {}} />);});
  await act(async () => gate.resolve(null)); expect(client.restoreStart).not.toHaveBeenCalled(); expect(text()).toContain('Home server');
});
test('late old host preview cannot replace the new server result; guest never reads or starts', async () => {
  const old = deferred(); client.restorePreview.mockReturnValueOnce(old.promise);
  await mount(); await act(async () => {tree.update(<RestoreSessionsSheet mac={{...mac, url: 'https://new.example', name: 'New server'}} pal={paletteFor('dark')} lang="zh" client={client} onClose={() => {}} />);});
  await act(async () => old.resolve({plan: {...plan, sessions: [{...plan.sessions[0], name: 'OLD WRONG SESSION'}]}})); expect(text()).not.toContain('OLD WRONG SESSION');
  act(() => tree.unmount()); client.restorePreview.mockClear(); await mount({...mac, scope: 'guest'}); expect(client.restorePreview).not.toHaveBeenCalled(); expect(client.restoreStart).not.toHaveBeenCalled();
});
test('closed sheet stops polling and partial results remain visibly partial', async () => {
  const id = '1800000000-request-1234567890'; client.restorePreview.mockResolvedValue({plan, job: job(id)});
  jest.useFakeTimers(); await mount(); const before = client.restoreStatus.mock.calls.length;
  await act(async () => {await jest.advanceTimersByTimeAsync(2100);}); expect(client.restoreStatus.mock.calls.length).toBeGreaterThan(before);
  await act(async () => {tree.update(<RestoreSessionsSheet mac={null} pal={paletteFor('dark')} lang="zh" client={client} onClose={() => {}} />);}); const after = client.restoreStatus.mock.calls.length;
  await act(async () => {await jest.advanceTimersByTimeAsync(7000);}); expect(client.restoreStatus).toHaveBeenCalledTimes(after);
  act(() => tree.unmount()); client.restorePreview.mockResolvedValue({plan, job: job(id, 'partial')}); await mount(); expect(text()).toContain('请核对结果'); expect(text()).toContain('尚未确认启动');
});
test('local persistence failure prevents POST; stale preview refresh discards only an explicitly refused request', async () => {
  await mount(); jest.spyOn(Keychain, 'setGenericPassword').mockRejectedValueOnce(new Error('disk')); await act(async () => {await press('restore-start');}); expect(client.restoreStart).not.toHaveBeenCalled();
  client.restoreStart.mockRejectedValueOnce(new RestoreError('plan_changed', 409)); await act(async () => {await press('restore-start');}); expect(text()).toContain('存档工作区已变化');
  await act(async () => {await press('restore-new-preview');}); expect(await loadPendingRestore(mac)).toBeNull(); expect(client.restoreStart).toHaveBeenCalledTimes(1);
});

test('re-pairing the same server address cannot reuse another credential’s pending request', async () => {
 await mount(); await act(async () => {await press('restore-start');});
 expect(await loadPendingRestore(mac)).not.toBeNull(); expect(await loadPendingRestore({...mac,token:'new-pairing'})).toBeNull();
});
