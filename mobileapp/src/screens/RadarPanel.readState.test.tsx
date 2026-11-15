import React from 'react';
import {AccessibilityInfo, Linking, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NavigationContext} from '@react-navigation/native';
import {useAgents, useAgentsOptional} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {toAgent} from '../api/types';
import {makeT} from '../i18n';
import {paletteFor} from '../ui/theme';
import {SectionList} from '../ui/SectionList';
import {RadarSummary} from '../ui/RadarSummary';
import {RadarPanel} from './RadarPanel';
import {RadarReadState} from './RadarReadState';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn(), mode: 'compact'})}));
jest.mock('../ui/RowSheet', () => ({RowSheet: () => null}));
jest.mock('../ui/HQDisc', () => ({HQDisc: () => null, DISC_CLEARANCE: 0}));
jest.mock('../ui/HQCard', () => ({HQCard: () => null}));
jest.mock('../ui/AgentRow', () => {
  const React = require('react');
  const {Text: Label} = require('react-native');
  return {AgentRow: ({agent}: any) => React.createElement(Label, null, agent.pane_id)};
});
let tree: renderer.ReactTestRenderer;
let state: any;
let app: any;
const navigation = {navigate: jest.fn()} as any;
const content = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(content).join('');
const words = () => tree.root.findAllByType(Text).map(content).join('|');
const has = (id: string) => tree.root.findAllByProps({testID: id}).length > 0;
const client = () => ({host: jest.fn().mockResolvedValue({ok: false}), usage: jest.fn().mockResolvedValue(null), serverMode: jest.fn().mockResolvedValue(null)});
const root = (variant: 'screen' | 'sidebar' = 'screen') => <NavigationContext.Provider value={navigation}><RadarPanel variant={variant} /></NavigationContext.Provider>;
const mount = async (variant: 'screen' | 'sidebar' = 'screen') => {await act(async () => {tree = renderer.create(root(variant));});};
const update = async (variant: 'screen' | 'sidebar' = 'screen') => {await act(async () => {tree.update(root(variant));});};
beforeEach(() => {
  jest.useFakeTimers(); jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);
  state = {agents: [], conn: 'live', lastUpdated: null, read: {hasValue: false, refreshing: true, error: null}, banner: null, dismissBanner: jest.fn(),
    refresh: jest.fn(async () => {}), isGuest: false, shareStale: false, client: client()};
  app = {lang: 'en', pal: paletteFor('light'), t: makeT('en'), mac: {url: 'https://mac.example', name: 'Mac'}};
  (useAgents as jest.Mock).mockImplementation(() => state); (useAgentsOptional as jest.Mock).mockImplementation(() => state);
  (useApp as jest.Mock).mockImplementation(() => app);
});
afterEach(async () => {await act(async () => {tree?.unmount();}); jest.useRealTimers(); jest.restoreAllMocks();});

test('stream connection before the first result displays loading rather than zero or empty', async () => {
  await mount(); expect(tree.root.findAllByType(RadarSummary)).toHaveLength(0);
  expect(has('radar-loading')).toBe(true); expect(has('radar-empty-quote')).toBe(false);
  expect(words()).toContain('Loading sessions'); expect(words()).not.toMatch(/Start with a session|No coding agents running|0 agents/);
});

for (const variant of ['screen', 'sidebar'] as const) for (const theme of ['light', 'dark'] as const) for (const lang of ['en', 'zh'] as const) {
  test(`confirmed empty is stable in ${variant} ${theme} ${lang}`, async () => {
    app = {...app, lang, pal: paletteFor(theme), t: makeT(lang)};
    state = {...state, lastUpdated: 1, read: {hasValue: true, refreshing: false, error: null}};
    await mount(variant); const quote = words();
    expect(has('radar-confirmed-empty')).toBe(true); expect(has('radar-empty-quote')).toBe(true);
    expect(tree.root.findAllByType(RadarSummary)).toHaveLength(1);
    state = {...state, read: {...state.read, refreshing: true}}; await update(variant);
    expect(words()).toBe(quote);
    const open = jest.spyOn(Linking, 'openURL').mockResolvedValue(undefined);
    const source = tree.root.findAll(n => n.props.accessibilityRole === 'link' && typeof n.props.onPress === 'function')[0];
    act(() => {source.props.onPress();}); expect(open).toHaveBeenCalledWith('https://sites.prh.com/dune/read-an-excerpt');
    expect(has('new-session-empty')).toBe(true);
  });
}

test('failed and unavailable access reads never display a cheerful empty quote or owner creation for guests', async () => {
  state = {...state, conn: 'offline', read: {hasValue: false, refreshing: false, error: 'failed'}};
  await mount(); expect(has('radar-read-failed')).toBe(true); expect(has('radar-empty-quote')).toBe(false);
  expect(has('new-session-empty')).toBe(false); expect(tree.root.findAllByType(RadarSummary)).toHaveLength(0);
  state = {...state, conn: 'unauthorized', isGuest: true, read: {...state.read, error: 'access'}}; await update();
  expect(words()).toContain('Ask the owner'); expect(words()).not.toMatch(/pair again|Start with a session/);
  expect(has('new-session-open')).toBe(false); expect(has('radar-empty-quote')).toBe(false);
  state = {...state, conn: 'live', shareStale: true}; await update();
  expect(has('radar-access-state')).toBe(true); expect(has('radar-empty-quote')).toBe(false);
});

test('a pull indicator follows the actual request beyond 600ms and stale completions cannot clear a new pull', async () => {
  state = {...state, read: {hasValue: true, refreshing: false, error: null}};
  const resolve: (() => void)[] = [];
  state.refresh.mockImplementation(() => new Promise<void>(done => {resolve.push(done);})); await mount();
  act(() => {void tree.root.findByType(SectionList).props.onRefresh();});
  await act(async () => {jest.advanceTimersByTime(600);}); expect(tree.root.findByType(SectionList).props.refreshing).toBe(true);
  state = {...state, client: client(), refresh: jest.fn(() => new Promise<void>(done => {resolve.push(done);})), read: {hasValue: false, refreshing: true, error: null}};
  await update(); act(() => {void tree.root.findByType(SectionList).props.onRefresh();});
  await act(async () => {resolve[0]();}); expect(tree.root.findByType(SectionList).props.refreshing).toBe(true);
  await act(async () => {resolve[1]();}); expect(tree.root.findByType(SectionList).props.refreshing).toBe(false);
});

test('cached worker rows remain during a failed refresh and a filter is not an empty fleet', async () => {
  state = {...state, agents: [toAgent({pane_id: '%cached', status: 'working', agent: 'codex'})], lastUpdated: 1, conn: 'offline', read: {hasValue: true, refreshing: false, error: 'failed'}};
  await mount(); expect(tree.root.findByType(SectionList).props.agents[0].pane_id).toBe('%cached');
  expect(tree.root.findByType(SectionList).props.stale).toBe(true); expect(has('radar-empty-quote')).toBe(false);
  await act(async () => {tree.unmount(); tree = renderer.create(<RadarReadState read={{hasValue: true, refreshing: false, error: null}} conn="live" isGuest={false} staleShare={false} filtered lang="en" pal={paletteFor('light')} demo={false} onRetry={jest.fn()} onShowAll={jest.fn()} />);});
  expect(has('radar-filter-empty')).toBe(true); expect(has('radar-empty-quote')).toBe(false);
});
