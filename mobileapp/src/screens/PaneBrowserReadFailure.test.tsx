// A failed /api/panes read is not a Mac with no panes (%12, 2026-10-06): the browser used
// to get [] for both and said "No tmux panes" on a failed read. Rendered for real, the Mac
// replaced; the poll is driven by fake timers.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {PaneBrowserView} from './PaneBrowserScreen';
import {useApp} from '../state/AppContext';
import {useAgents, useAgentsOptional} from '../state/AgentsContext';
import {useWorkspace} from '../state/WorkspaceContext';
import {paletteFor} from '../ui/theme';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {TestIds} from '../constants/testIds';
import {PaneRow} from '../api/types';
import {ApiError} from '../api/client';

jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: jest.fn()}));

let tree: renderer.ReactTestRenderer | undefined;
const rows: PaneRow[] = [
  {pane_id: '%1', session: 'dev', loc: 'dev:0.0', window: '0', pane: '0', command: 'bash', tier: 'plain'},
  {pane_id: '%2', session: 'dev', loc: 'dev:0.1', window: '0', pane: '1', command: 'vim', tier: 'plain'},
];
const text = () => tree!.root.findAllByType(Text).map(n => n.props.children).flat().join(' ');
const has = (id: string) => tree!.root.findAllByProps({testID: id}).length > 0;

function setup(panes: jest.Mock, lang: 'en' | 'zh' = 'en') {
  (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
  const context = {isGuest: false, agents: [], client: {panes}};
  (useAgents as jest.Mock).mockReturnValue(context);
  (useAgentsOptional as jest.Mock).mockReturnValue(context);
  (useApp as jest.Mock).mockReturnValue({pal: paletteFor('dark'), lang, mac: {name: 'Studio'}});
  (useWorkspace as jest.Mock).mockReturnValue({select: jest.fn()});
}
async function mount(layout: 'compact' | 'regular' = 'compact') {
  await act(async () => {
    tree = renderer.create(<PaneBrowserView layout={layout} />);
  });
}
// One poll: the 3s interval fires and its read settles.
async function poll() {
  await act(async () => {
    jest.advanceTimersByTime(3000);
  });
  await act(async () => {});
}

beforeEach(() => jest.useFakeTimers());
afterEach(() => {
  act(() => tree?.unmount());
  tree = undefined;
  jest.useRealTimers();
  jest.clearAllMocks();
});

const firstReadFails: [string, 'compact' | 'regular', 'en' | 'zh', string, string, string][] = [
  ['compact en', 'compact', 'en', 'Could not read the panes on Studio', 'Trying again every few seconds', 'could not read'],
  ['regular en', 'regular', 'en', 'Could not read the panes on Studio', 'Trying again every few seconds', 'could not read'],
  ['compact zh', 'compact', 'zh', '读不到 Studio 上的 pane', '每隔几秒会再试一次', '读不到'],
  ['regular zh', 'regular', 'zh', '读不到 Studio 上的 pane', '每隔几秒会再试一次', '读不到'],
];
test.each(firstReadFails)('a first read that fails says so, not "no panes" (%s)', async (_n, layout, lang, title, hint, header) => {
  setup(jest.fn().mockRejectedValue(new ApiError(503, 'panes')), lang);
  await mount(layout);
  expect(has(TestIds.panes.readFailed)).toBe(true);
  expect(has(TestIds.panes.loading)).toBe(false);
  const t = text();
  expect(t).toContain(title);
  expect(t).toContain(hint);
  expect(t).toContain(header);
  expect(t).not.toMatch(/No tmux panes|没有 tmux pane|0 panes|0 个 pane/);
});

test.each(['en', 'zh'] as const)('an empty list is still "no panes" (%s)', async lang => {
  setup(jest.fn().mockResolvedValue([]), lang);
  await mount();
  expect(has(TestIds.panes.readFailed)).toBe(false);
  expect(text()).toContain(lang === 'zh' ? '没有 tmux pane' : 'No tmux panes');
});

test('a failure after a good read keeps the rows, marked not refreshed, until a read lands', async () => {
  const panes = jest.fn().mockResolvedValueOnce(rows).mockRejectedValueOnce(new Error('panes: the answer is not a list')).mockResolvedValue(rows.slice(0, 1));
  setup(panes);
  await mount();
  expect(text()).toContain('2 panes · 1 session');
  expect(text()).not.toContain('not refreshed');

  await poll();
  expect(panes).toHaveBeenCalledTimes(2);
  expect(has(`${TestIds.panes.row}-%2`)).toBe(true);
  expect(text()).toContain('2 panes · 1 session · not refreshed');
  expect(has(TestIds.panes.readFailed)).toBe(false);

  await poll();
  expect(text()).toContain('1 pane · 1 session');
  expect(text()).not.toContain('not refreshed');
  expect(has(`${TestIds.panes.row}-%2`)).toBe(false);
});

test('a first read that fails recovers on the next poll', async () => {
  const panes = jest.fn().mockRejectedValueOnce(new ApiError(500, 'panes')).mockResolvedValue(rows);
  setup(panes, 'zh');
  await mount();
  expect(has(TestIds.panes.readFailed)).toBe(true);
  await poll();
  expect(has(TestIds.panes.readFailed)).toBe(false);
  expect(text()).toContain('2 个 pane · 1 个会话');
  expect(text()).not.toContain('刷新失败');
});
