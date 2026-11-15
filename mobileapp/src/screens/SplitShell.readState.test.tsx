import React from 'react';
import {AccessibilityInfo, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {NavigationContext} from '@react-navigation/native';
import {useAgents, useAgentsOptional} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {makeT} from '../i18n';
import {paletteFor} from '../ui/theme';
import {toAgent} from '../api/types';
import {SplitShell} from './SplitShell';
import {RadarPanel} from './RadarPanel';

let mockWorkspace: any;
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/WorkspaceContext', () => ({...jest.requireActual('../state/WorkspaceContext'), useWorkspace: () => mockWorkspace}));
jest.mock('./RadarPanel', () => ({RadarPanel: jest.fn(() => null)}));
jest.mock('./DetailScreen', () => ({DetailView: () => require('react').createElement(require('react-native').View, {testID: 'detail-leaf'})}));
jest.mock('./HQScreen', () => ({HQView: () => null}));
jest.mock('./PaneBrowserScreen', () => ({PaneBrowserView: () => null}));

let tree: renderer.ReactTestRenderer;
let state: any;
let app: any;
const navigation = {navigate: jest.fn()} as any;
const text = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(text).join('');
const words = () => tree.root.findAllByType(Text).map(text).join('|');
const has = (id: string) => tree.root.findAllByProps({testID: id}).length > 0;
const root = () => <NavigationContext.Provider value={navigation}><SplitShell /></NavigationContext.Provider>;
const mount = async () => {await act(async () => {tree = renderer.create(root());});};
const update = async () => {await act(async () => {tree.update(root());});};
beforeEach(() => {
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);
  jest.spyOn(AsyncStorage, 'getItem').mockResolvedValue(null);
  mockWorkspace = {selection: null, select: jest.fn(), mode: 'regular'};
  state = {agents: [], conn: 'live', read: {hasValue: false, refreshing: true, error: null}, isGuest: false, shareStale: false,
    refresh: jest.fn(async () => {}), client: {}};
  app = {lang: 'en', pal: paletteFor('light'), t: makeT('en'), mac: {name: 'Mac', url: 'https://mac.example'}};
  (useAgents as jest.Mock).mockImplementation(() => state);
  (useAgentsOptional as jest.Mock).mockImplementation(() => state);
  (useApp as jest.Mock).mockImplementation(() => app);
});
afterEach(async () => {await act(async () => {tree?.unmount();}); jest.restoreAllMocks(); jest.clearAllMocks();});

for (const lang of ['en', 'zh'] as const) for (const theme of ['light', 'dark'] as const) {
  test(`regular main respects loading and timeout in ${lang} ${theme}`, async () => {
    app = {...app, lang, pal: paletteFor(theme), t: makeT(lang)};
    await mount();
    expect(has('radar-loading')).toBe(true);
    expect(words()).not.toMatch(/No coding agents running|暂无运行中的|Start a coding agent|在服务器上启动/);
    expect(has('new-session-empty')).toBe(false);
    state = {...state, conn: 'offline', read: {hasValue: false, refreshing: false, error: 'timeout'}};
    await update();
    expect(has('radar-read-failed')).toBe(true);
    expect(words()).toContain(lang === 'zh' ? '加载超时' : 'Loading timed out');
    expect(words()).not.toMatch(/No coding agents running|暂无运行中的/);
    expect(has('radar-empty-quote')).toBe(false);
  });
}

test('hidden sidebar does not turn an unfinished main read into an empty fleet', async () => {
  (AsyncStorage.getItem as jest.Mock).mockResolvedValue('1');
  await mount();
  expect(tree.root.findAllByType(RadarPanel)).toHaveLength(0);
  expect(has('radar-loading')).toBe(true);
  expect(words()).not.toContain('No coding agents running');
});

test('unselected access and expired guest states retain shared guidance and no owner creation', async () => {
  state = {...state, conn: 'unauthorized', isGuest: true, read: {hasValue: false, refreshing: false, error: 'access'}};
  await mount(); expect(words()).toContain('Ask the owner'); expect(has('new-session-empty')).toBe(false);
  state = {...state, conn: 'live', shareStale: true}; await update();
  expect(has('radar-access-state')).toBe(true); expect(has('radar-empty-quote')).toBe(false);
});

test('only a successful empty main welcomes creation while a populated roster keeps automatic selection', async () => {
  state = {...state, read: {hasValue: true, refreshing: false, error: null}};
  await mount(); expect(has('radar-confirmed-empty')).toBe(true); expect(has('new-session-empty')).toBe(true);
  // The real sidebar already contains the excerpt; do not repeat it in the main pane.
  expect(has('radar-empty-quote')).toBe(false);
  const agent = toAgent({pane_id: '%cached', status: 'working', agent: 'codex'});
  state = {...state, agents: [agent]}; await update();
  expect(words()).toContain('Select a session'); expect(has('radar-confirmed-empty')).toBe(false);
  expect(mockWorkspace.select).toHaveBeenCalledWith({kind: 'pane', agent});
  mockWorkspace = {...mockWorkspace, selection: {kind: 'pane', agent}};
  state = {...state, read: {hasValue: false, refreshing: true, error: null}}; await update();
  expect(has('detail-leaf')).toBe(true); expect(has('radar-loading')).toBe(false);
});
