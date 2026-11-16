import React from 'react';
import {AccessibilityInfo, AppState, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NavigationContext} from '@react-navigation/native';
import {ApiError, GtmuxClient} from '../api/client';
import type {Agent} from '../api/types';
import {toAgent} from '../api/types';
import {AgentsProvider, useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {makeT} from '../i18n';
import {paletteFor} from '../ui/theme';
import {SectionList} from '../ui/SectionList';
import {OfflineBanner} from '../ui/OfflineBanner';
import {RadarPanel} from './RadarPanel';

type Request = {finish: (value: Agent[]) => void; fail: (error: Error) => void; signal?: AbortSignal};
const requests: Request[] = [];
const mockStreams: {onOpen: () => void; onAgents: () => void}[] = [];
jest.mock('../api/events', () => ({subscribe: jest.fn((_base, _token, handlers) => {mockStreams.push(handlers); return () => {};})}));
jest.mock('../native/liveActivity', () => ({LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: jest.fn().mockResolvedValue(null)}, apnsEnv: () => 'sandbox'}));
jest.mock('../push', () => ({setBadge: jest.fn()}));
jest.mock('../pairing/follow', () => ({findLiveAddress: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn(), mode: 'compact'})}));
jest.mock('../ui/RowSheet', () => ({RowSheet: () => null}));
jest.mock('../ui/HQDisc', () => ({HQDisc: () => null, DISC_CLEARANCE: 0}));
jest.mock('../ui/HQCard', () => ({HQCard: () => null}));
jest.mock('../ui/AgentRow', () => ({AgentRow: ({agent}: any) => require('react').createElement(require('react-native').Text, null, agent.pane_id)}));

let tree: renderer.ReactTestRenderer;
let value: ReturnType<typeof useAgents>;
const navigation = {navigate: jest.fn()} as any;
function Panel() {value = useAgents(); return <RadarPanel variant="screen" />;}
const node = (base = 'https://a.example', scope: 'owner' | 'guest' = 'owner') => <NavigationContext.Provider value={navigation}>
  <AgentsProvider base={base} token={base} scope={scope} liveActivity={false}><Panel /></AgentsProvider>
</NavigationContext.Provider>;
const flush = async () => {await act(async () => {await Promise.resolve(); await Promise.resolve();});};
const mount = async (scope: 'owner' | 'guest' = 'owner') => {await act(async () => {tree = renderer.create(node('https://a.example', scope));}); await flush();};
const finish = async (i: number, rows: Agent[]) => {await act(async () => {requests[i].finish(rows);}); await flush();};
const rows = (id: string) => [toAgent({pane_id: id, status: 'working', agent: 'codex'})];
const text = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(text).join('');
const words = () => tree.root.findAllByType(Text).map(text).join('|');
const notices = () => tree.root.findAllByType(OfflineBanner);
const refresh = async () => {act(() => {void value.refresh();}); await flush();};
beforeEach(() => {
  jest.useFakeTimers({now: 1_800_000_000_000}); requests.length = 0; mockStreams.length = 0;
  (AppState as any).currentState = 'active';
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);
  (useApp as jest.Mock).mockReturnValue({lang: 'en', pal: paletteFor('light'), t: makeT('en'), mac: {name: 'Mac', url: 'https://a.example'}});
  jest.spyOn(GtmuxClient.prototype, 'host').mockResolvedValue({ok: false, why: 'unreachable'});
  jest.spyOn(GtmuxClient.prototype, 'usage').mockResolvedValue(null);
  jest.spyOn(GtmuxClient.prototype, 'serverMode').mockResolvedValue(null);
  jest.spyOn(GtmuxClient.prototype, 'share').mockResolvedValue({all: true, input: true, panes: [], view_panes: []});
  jest.spyOn(GtmuxClient.prototype, 'addresses').mockResolvedValue({addresses: []});
  jest.spyOn(GtmuxClient.prototype, 'agents').mockImplementation(signal => new Promise<Agent[]>((resolve, reject) => {requests.push({finish: resolve, fail: reject, signal});}));
});
afterEach(async () => {await act(async () => {tree?.unmount();}); jest.restoreAllMocks(); jest.clearAllMocks(); jest.useRealTimers();});

test.each(['failed', 'timeout'] as const)('a cached read failure remains visible when SSE reconnects before the next result: %s', async kind => {
  await mount(); await finish(0, rows('%cached')); const updated = value.lastUpdated;
  await refresh();
  if (kind === 'failed') await act(async () => {requests[1].fail(new Error('agents response is not a list'));});
  else await act(async () => {jest.advanceTimersByTime(15_000);});
  await flush(); expect(notices()).toHaveLength(1); expect(value.read.error).toBe(kind);
  act(() => {mockStreams[0].onOpen();}); await flush();
  expect(value.conn).toBe('live'); expect(value.read.refreshing).toBe(true);
  expect(notices()).toHaveLength(1);
  expect(value.lastUpdated).toBe(updated); expect(words()).toContain('updated ');
  expect(words()).toContain('Session list is out of date');
  expect(words()).toContain('Saved sessions');
  expect(words()).not.toContain("Can't reach");
  expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%cached');
  expect(tree.root.findByType(SectionList).props.stale).toBe(true);
  const retry = tree.root.findAllByProps({testID: 'radar-read-retry'}).find(n => typeof n.props.onPress === 'function')!;
  expect(retry.props.disabled).toBe(false); expect(words()).toContain('Retry');
  expect(words()).not.toContain('Updating');
  jest.setSystemTime(Date.now() + 1_000); await finish(2, rows('%fresh'));
  expect(value.read.error).toBeNull(); expect(notices()).toHaveLength(0);
  expect(value.lastUpdated).not.toBe(updated); expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%fresh');
});

test('manual cached-list retry ends at its deadline even when an automatic read follows', async () => {
  await mount(); await finish(0, rows('%cached')); const updated = value.lastUpdated;
  await refresh();
  await act(async () => {requests[1].fail(new Error('synthetic failure'));}); await flush();
  const retry = () => tree.root.findAllByProps({testID: 'radar-read-retry'}).find(n => typeof n.props.onPress === 'function')!;
  act(() => {void retry().props.onPress();}); await flush();
  expect(retry().props.disabled).toBe(true);
  expect(retry().props.accessibilityState.busy).toBe(true);
  expect(words()).toContain('Updating session list');
  await act(async () => {jest.advanceTimersByTime(15_000);}); await flush();
  expect(requests[2].signal?.aborted).toBe(true);
  expect(retry().props.disabled).toBe(false);
  expect(retry().props.accessibilityState.busy).toBe(false);
  expect(words()).toContain('Session list is out of date');
  expect(value.lastUpdated).toBe(updated);
  expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%cached');
  await finish(2, rows('%late'));
  expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%cached');
});

test('changing the client drops cached failure and rejects a late old reread', async () => {
  await mount(); await finish(0, rows('%old')); await refresh();
  await act(async () => {requests[1].fail(new Error('synthetic failure'));}); await flush();
  act(() => {mockStreams[0].onOpen();}); await flush();
  await act(async () => {tree.update(node('https://b.example'));}); await flush();
  expect(notices()).toHaveLength(0); expect(value.read.hasValue).toBe(false); expect(value.agents).toEqual([]);
  expect(requests[2].signal?.aborted).toBe(true);
  await finish(2, rows('%late-old')); expect(value.agents).toEqual([]);
  await finish(3, []); expect(value.read.error).toBeNull(); expect(notices()).toHaveLength(0);
});

test.each(['owner', 'guest'] as const)('cached access failure stays visible across reconnect with the existing %s authority', async scope => {
  jest.spyOn(GtmuxClient.prototype, 'share').mockResolvedValue({all: scope === 'owner', input: true, panes: [], view_panes: []});
  await mount(scope); await finish(0, rows('%cached')); await refresh();
  await act(async () => {requests[1].fail(new ApiError(401, 'synthetic refusal'));}); await flush();
  act(() => {mockStreams[0].onOpen();}); await flush();
  expect(notices()).toHaveLength(1); expect(words()).toContain('updated ');
  if (scope === 'guest') {
    expect(words()).toContain('Ask the owner');
    expect(tree.root.findAllByProps({testID: 'radar-read-retry'})).toHaveLength(0);
    expect(tree.root.findAllByProps({testID: 'new-session-open'})).toHaveLength(0);
  } else {
    const settings = tree.root.findAllByProps({testID: 'radar-read-retry'}).find(n => typeof n.props.onPress === 'function')!;
    act(() => {settings.props.onPress();}); expect(navigation.navigate).toHaveBeenCalledWith('Servers');
  }
});
