import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
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
  expect(words()).toContain('My iPhone'); expect(words()).toContain('%21'); expect(words()).toContain('Request accepted');
  client.accessHistory.mockResolvedValueOnce({entries: [record, {...record, id: 'r2', name: 'Browser'}], retain_days: 30});
  await press('Load earlier');
  expect(client.accessHistory).toHaveBeenLastCalledWith('cursor', expect.anything());
  expect(words().match(/My iPhone/g)).toHaveLength(1); expect(words()).toContain('Browser');
  expect(words()).toContain('not agent completion');
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

test('keeps full device identity behind an explicit disclosure', async () => {
  client.accessHistory.mockResolvedValueOnce({entries: [{...record, device_id: 'device-full-identity'}], retain_days: 30});
  await mount(); expect(words()).not.toContain('device-full-identity');
  expect(words()).toContain('Source: ');
  await press('Device ID'); expect(words()).toContain('device-full-identity');
  await press('Hide device ID'); expect(words()).not.toContain('device-full-identity');
});
