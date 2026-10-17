import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import {AccessHistoryControls} from './AccessHistoryControls';
import {AccessHistoryScreen} from './AccessHistoryScreen';
import {ApiError} from '../api/client';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {paletteFor} from '../ui/theme';
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn()}));
const record = {id: 'r1', at: '2026-10-16T10:00:00+08:00', actor: 'phone:known', name: 'My iPhone', scope: 'device', operation: 'send', label_en: 'Send terminal input', label_zh: '发送终端输入', target: '%21', status: 200, outcome: 'ok', platform: 'iOS 26', source_ip: '203.0.113.1'};
let client: any, tree: renderer.ReactTestRenderer;
const words = () => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' ');
const press = async (label: string) => {await act(async () => {tree.root.findAllByType(TouchableOpacity).find(n => n.findAllByType(Text).some(t => t.props.children === label))!.props.onPress();});};
async function mount() {await act(async () => {tree = renderer.create(<AccessHistoryScreen navigation={{goBack: jest.fn()}} />);});}
beforeEach(() => {
  client = {accessHistory: jest.fn().mockResolvedValue({entries: [record], next: 'cursor', retain_days: 30})};
  (useApp as jest.Mock).mockReturnValue({lang: 'en', pal: paletteFor('dark'), mac: {scope: 'device'}});
  (useAgents as jest.Mock).mockImplementation(() => ({client}));
});
afterEach(() => {act(() => tree?.unmount()); jest.restoreAllMocks();});
test('renders verified metadata and paginates without repeating records', async () => {
  await mount();
  expect(words()).toContain('My iPhone'); expect(words()).toContain('%21'); expect(words()).toContain('Accepted');
  client.accessHistory.mockResolvedValueOnce({entries: [record, {...record, id: 'r2', name: 'Browser'}], retain_days: 30});
  await press('Load earlier');
  expect(client.accessHistory).toHaveBeenLastCalledWith('cursor', expect.anything(), {});
  expect(words().match(/My iPhone/g)).toHaveLength(1); expect(words()).toContain('Browser');
  expect(words()).toContain('Recording rules');
});
test('does not request history for guest credentials', async () => {
  (useApp as jest.Mock).mockReturnValue({lang: 'en', pal: paletteFor('dark'), mac: {scope: 'guest'}});
  await mount(); expect(client.accessHistory).not.toHaveBeenCalled(); expect(words()).toContain('Guest links cannot read');
});
test('old servers and failed pages stay explicit and can retry', async () => {
  client.accessHistory.mockRejectedValueOnce(new ApiError(404, 'history'));
  await mount(); expect(words()).toContain('Update this server'); expect(words()).not.toContain('No access history yet');
  await press('Retry'); expect(words()).toContain('My iPhone');
  client.accessHistory.mockRejectedValueOnce(new ApiError(410, 'history'));
  await press('Load earlier'); expect(words()).toContain('page expired'); expect(words()).toContain('My iPhone');
});
test('a late response from another server cannot replace the current history', async () => {
  let finish!: (page: unknown) => void;
  client.accessHistory.mockReturnValueOnce(new Promise(resolve => {finish = resolve;}));
  await mount();
  client = {accessHistory: jest.fn().mockResolvedValue({entries: [{...record, id: 'new', name: 'New server device'}], retain_days: 30})};
  await act(async () => {tree.update(<AccessHistoryScreen navigation={{goBack: jest.fn()}} />);});
  await act(async () => {finish({entries: [{...record, name: 'Stale server device'}], retain_days: 30});});
  expect(words()).toContain('New server device'); expect(words()).not.toContain('Stale server device');
});

test('shows device identity inline without a disclosure', async () => {
  client.accessHistory.mockResolvedValueOnce({entries: [{...record, device_id: 'device-full-identity', duration_ms: 24, method: 'POST'}], retain_days: 30});
  await mount(); expect(words()).toContain('device-full-identity'); expect(words()).toContain('203.0.113.1');
  expect(words()).toContain('24 ms'); expect(words()).toContain('POST');
  expect(tree.root.findAllByType(TouchableOpacity).some(n => n.props.accessibilityState?.expanded !== undefined)).toBe(false);
});

test('Chinese dates follow UI language and equal-day records share one heading', async () => {
  (useApp as jest.Mock).mockReturnValue({lang: 'zh', pal: paletteFor('light'), mac: {scope: 'device'}});
  client.accessHistory.mockResolvedValueOnce({entries: [record, {...record, id: 'two'}], retain_days: 30});
  await mount();
  expect(words()).toContain('2026年10月'); expect(words()).not.toContain('Oct');
  const dates = tree.root.findAllByType(Text).filter(n => n.props.accessibilityRole === 'header' && String(n.props.children).includes('2026'));
  expect(dates).toHaveLength(1);
});

test('a failed archive with HTTP 200 is shown as failed, not accepted', async () => {
  client.accessHistory.mockResolvedValueOnce({entries: [{...record, outcome: 'failed', label_en: 'Download HQ archive', session_id: '12345678-abcd-1234-abcd-123456789abc', target: undefined}], retain_days: 30});
  await mount(); expect(words()).toContain('Failed · 200'); expect(words()).not.toContain('Accepted');
  expect(words()).toContain('Codex · 12345678-abcd-1234-abcd-123456789abc');
});

test('device summaries accumulate pages and drill into an exact device without carrying the cursor', async () => {
  const g = {key: 'device:full-a', device_id: 'full-a', actor: 'phone:a', name: 'Same name', scope: 'device', count: 3, accepted: 3, refused: 0, failed: 0, first_at: record.at, last_at: record.at};
  client.accessHistory.mockImplementation((_before: string, _signal: unknown, filter: any) => Promise.resolve(filter.group_by ? {entries: [], groups: [g], next: 'group-cursor', retain_days: 30} : {entries: [record], groups: [g], retain_days: 30}));
  await mount(); await press('Sources'); expect(words()).toContain('3 recorded');
  await press('Continue summary'); expect(words()).toContain('6 recorded');
  const row = tree.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel?.startsWith('View source log:'))!;
  await act(async () => row.props.onPress());
  expect(client.accessHistory).toHaveBeenLastCalledWith(undefined, expect.anything(), expect.objectContaining({device_id: 'full-a', group_by: undefined}));
});

test('changing filters ignores a late unfiltered response', async () => {
  let finish!: (page: unknown) => void;
  client.accessHistory.mockReturnValueOnce(new Promise(resolve => {finish = resolve;}));
  await mount();
  client.accessHistory.mockResolvedValueOnce({entries: [{...record, name: 'Filtered device'}], retain_days: 30});
  await act(async () => tree.root.findByType(AccessHistoryControls).props.onChange({outcome: 'failed'}));
  await act(async () => finish({entries: [{...record, name: 'Stale unfiltered device'}], retain_days: 30}));
  expect(words()).toContain('Filtered device'); expect(words()).not.toContain('Stale unfiltered device');
});


test('switching server resets identity filters instead of sending another server’s device ID', async () => {
  await mount();
  await act(async () => tree.root.findByType(AccessHistoryControls).props.onChange({device_id: 'old-server-device', outcome: 'failed'}));
  client = {accessHistory: jest.fn().mockResolvedValue({entries: [record], retain_days: 30})};
  await act(async () => tree.update(<AccessHistoryScreen navigation={{goBack: jest.fn()}} />));
  expect(client.accessHistory).toHaveBeenCalledTimes(1);
  expect(client.accessHistory).toHaveBeenLastCalledWith(undefined, expect.anything(), {});
});
