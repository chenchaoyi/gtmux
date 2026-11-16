import React from 'react';
import {AccessibilityInfo, AppState} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NavigationContext} from '@react-navigation/native';
import {GtmuxClient} from '../api/client';
import {type Agent, toAgent} from '../api/types';
import {AgentsProvider, useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {makeT} from '../i18n';
import {paletteFor} from '../ui/theme';
import {SectionList} from '../ui/SectionList';
import {OfflineBanner} from '../ui/OfflineBanner';
import {RadarPanel} from './RadarPanel';

type Request = {finish: (rows: Agent[]) => void; fail: (error: Error) => void; signal?: AbortSignal};
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
function Panel() {value = useAgents(); return <RadarPanel variant="screen" />;}
const navigation = {navigate: jest.fn()} as any;
const node = (base = 'https://a.example') => <NavigationContext.Provider value={navigation}>
  <AgentsProvider base={base} token={base} scope="owner" liveActivity={false}><Panel /></AgentsProvider>
</NavigationContext.Provider>;
const rows = (id: string) => [toAgent({pane_id: id, status: 'working', agent: 'codex'})];
const flush = async () => {await act(async () => {for (let i = 0; i < 8; i++) await Promise.resolve();});};
const mount = async () => {await act(async () => {tree = renderer.create(node());}); await flush();};
const finish = async (i: number, id: string) => {await act(async () => {requests[i].finish(rows(id));}); await flush();};
const fail = async (i: number) => {await act(async () => {requests[i].fail(new Error('synthetic read failure'));}); await flush();};
const tick = async (ms: number) => {await act(async () => {jest.advanceTimersByTime(ms);}); await flush();};
const retry = () => tree.root.findAllByProps({testID: 'radar-read-retry'}).find(n => typeof n.props.onPress === 'function')!;
const clickRetry = async () => {act(() => {void retry().props.onPress();}); await flush();};
const startFailedCache = async () => {
  await mount(); await finish(0, '%cached');
  act(() => {void value.refresh();}); await flush(); await fail(1);
};
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
afterEach(async () => {await act(async () => tree?.unmount()); jest.restoreAllMocks(); jest.clearAllMocks(); jest.clearAllTimers(); jest.useRealTimers();});

test('manual retry expires from its click while a background read owns the lane and cannot clear a newer action', async () => {
  await startFailedCache(); const updated = value.lastUpdated;
  act(() => mockStreams[0].onOpen()); await flush();
  expect(requests).toHaveLength(3);
  await clickRetry();
  expect(retry().props.disabled).toBe(true);
  expect(requests).toHaveLength(3); // manual read is queued behind background
  await tick(14_999); expect(retry().props.disabled).toBe(true);
  await tick(1);
  expect(retry().props.disabled).toBe(false);
  expect(requests).toHaveLength(4); // its trailing read is still pending
  expect(value.lastUpdated).toBe(updated);
  expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%cached');
  await clickRetry();
  await fail(3); // the expired first action settles while action two is busy
  expect(requests).toHaveLength(5);
  expect(retry().props.disabled).toBe(true);
  await finish(2, '%expired-background');
  expect(retry().props.disabled).toBe(true);
  expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%cached');
  act(() => mockStreams[0].onAgents()); await flush();
  await finish(4, '%fresh');
  expect(requests).toHaveLength(6);
  expect(value.read.refreshing).toBe(true); // later automatic read remains active
  expect(tree.root.findByType(SectionList).props.refreshing).toBe(false);
  expect(tree.root.findAllByType(OfflineBanner)).toHaveLength(0);
  expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%fresh');
});

test('client switching retires the old retry deadline and late answer without ending the new action', async () => {
  await startFailedCache(); await clickRetry(); await tick(5_000);
  await act(async () => tree.update(node('https://b.example'))); await flush();
  expect(requests[2].signal?.aborted).toBe(true);
  await finish(3, '%new-client');
  act(() => {void value.refresh();}); await flush(); await fail(4);
  await clickRetry();
  await tick(10_000); // old action's original deadline, only10s into new action
  expect(retry().props.disabled).toBe(true);
  await finish(2, '%old-client-late');
  expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%new-client');
  expect(retry().props.disabled).toBe(true);
  await tick(5_000);
  expect(retry().props.disabled).toBe(false);
});
