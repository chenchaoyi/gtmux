import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {RestoreSessionsSheet} from './RestoreSessionsSheet';
import {parseRestoreJob} from '../api/restore';
import {paletteFor} from '../ui/theme';

jest.mock('react-native-keychain', () => ({getGenericPassword: jest.fn(async () => false), setGenericPassword: jest.fn(async () => true), resetGenericPassword: jest.fn(async () => true)}));
const mac = {name: 'Synthetic restore host', url: 'https://synthetic.invalid', token: 'synthetic'};
const revision = 'a'.repeat(64);
const sessions = [{name: 'saved', windows: 1, panes: 3, agents: 3, resumable: 3, state: 'restored'}];
let tree: renderer.ReactTestRenderer | undefined;
afterEach(() => {if (tree) act(() => tree!.unmount()); tree = undefined;});

test.each(['en', 'zh'] as const)('RRV003 shows timeout and original uncertainty in %s', async lang => {
  const job = parseRestoreJob({id: '1800000000-visible-1234567890', revision, created_at: 1800000000, updated_at: 1800000000, phase: 'partial', sessions,
    agents: ['pending', 'unknown', 'send-failed'].map((issue_state, i) => ({loc: `saved:0.${i}`, pane: `%${7 + i}`, state: 'startup-timeout', issue_state}))});
  const client = {restorePreview: jest.fn(async () => ({plan: {revision, server_time: 1800000000, saved_at: 1800000000, resume_mode: 'auto' as const, sessions}, job})), restoreStatus: jest.fn(async () => job), restoreStart: jest.fn(async () => job)};
  await act(async () => {tree = renderer.create(<RestoreSessionsSheet mac={mac} pal={paletteFor('dark')} lang={lang} client={client} onClose={() => {}} />);});
  const text = tree!.root.findAllByType(Text).map(n => n.props.children).join(' ');
  expect(text).toContain(lang === 'en' ? 'Startup wait timed out' : '启动等待已超时');
  expect(text).toContain(lang === 'en' ? 'Send result unknown · Startup wait timed out' : '发送结果未知 · 启动等待已超时');
  expect(text).toContain(lang === 'en' ? 'Send failed · Startup wait timed out' : '发送失败 · 启动等待已超时');
  expect(text).toContain(lang === 'en' ? 'no automatic replay' : '不会自动重发');
  expect(client.restoreStart).not.toHaveBeenCalled();
});
