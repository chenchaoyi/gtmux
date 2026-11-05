import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {BackHandler, Button as MockButton, Text as MockText, StyleSheet} from 'react-native';
import {NavigationContext} from '@react-navigation/native';
import {WorkspaceProvider} from '../state/WorkspaceContext';
import {SplitShell} from './SplitShell';

let mockWide = false;
const mockAgent: any = {pane_id: '%1', session: 'work', window: '0', pane: '1', loc: 'work:0.1', agent: 'Codex', source: 'tmux'};
jest.mock('../ui/layout', () => ({useSizeClass: () => mockWide ? 'regular' : 'compact', sidebarWidth: () => 280, hqInspectorFits: () => false}));
jest.mock('../state/AgentsContext', () => ({useAgents: () => ({agents: [mockAgent]})}));
jest.mock('../state/AppContext', () => ({useApp: () => ({t: (x: string) => x, lang: 'en', pal: {bg: '#fff', fg3: '#000', divLoud: '#000'}})}));
jest.mock('./RadarPanel', () => ({RadarPanel: () => { const {select} = require('../state/WorkspaceContext').useWorkspace(); return <MockButton title="Open" onPress={() => select({kind: 'pane', agent: mockAgent})} />; }}));
jest.mock('./DetailScreen', () => ({DetailView: ({onBack}: any) => { const [draft, setDraft] = require('react').useState('initial'); return <><MockText>{draft}</MockText><MockButton title="Edit" onPress={() => setDraft('draft and reader state')} /><MockButton title="Back" onPress={onBack} /></>; }}));
jest.mock('./HQScreen', () => ({HQView: () => null}));
jest.mock('./PaneBrowserScreen', () => ({PaneBrowserView: () => null}));
jest.mock('../ui/BrandMark', () => ({BrandMark: () => null}));
jest.mock('../ui/Icons', () => ({SidebarIcon: () => null}));

function App() {
  return <WorkspaceProvider mode={mockWide ? 'regular' : 'compact'} navigate={jest.fn()} inlineSelection><SplitShell /></WorkspaceProvider>;
}

test('workspace leaves screen edge insets to the visible screen and protects the wide sidebar', () => {
  mockWide = false;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<App />); });
  expect(tree.root.findByProps({testID: 'radar-split'}).props.edges).toBeUndefined();
  expect(tree.root.findByProps({testID: 'workspace-list'}).props.edges).toEqual([]);
  mockWide = true;
  act(() => tree.update(<App />));
  expect(tree.root.findByProps({testID: 'workspace-list'}).props.edges).toEqual(['top', 'left']);
  act(() => tree.unmount());
});

test('resizing keeps the mounted detail state and back/reopen remains usable', () => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<App />); });
  const press = (title: string) => act(() => tree.root.findAllByType(MockButton).find(x => x.props.title === title)!.props.onPress());
  press('Open');
  press('Edit');
  mockWide = true;
  act(() => tree.update(<App />));
  expect(tree.root.findAllByType(MockText).some(x => x.props.children === 'draft and reader state')).toBe(true);
  mockWide = false;
  act(() => tree.update(<App />));
  expect(tree.root.findAllByType(MockText).some(x => x.props.children === 'draft and reader state')).toBe(true);
  press('Back');
  press('Open');
  expect(tree.root.findAllByType(MockText).some(x => x.props.children === 'draft and reader state')).toBe(true);
});


test('folding after the wide workspace chooses its first session keeps that reader visible', () => {
  mockWide = false;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<App />); });
  mockWide = true;
  act(() => tree.update(<App />));
  expect(tree.root.findAllByType(MockText).some(x => x.props.children === 'initial')).toBe(true);
  mockWide = false;
  act(() => tree.update(<App />));
  expect(StyleSheet.flatten(tree.root.findByProps({testID: 'workspace-reader'}).props.style).display).not.toBe('none');
  expect(StyleSheet.flatten(tree.root.findByProps({testID: 'workspace-list'}).props.style).display).toBe('none');
  act(() => tree.unmount());
});

test('returning to the compact list survives expanding and folding the workspace', () => {
  mockWide = false;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<App />); });
  const press = (title: string) => act(() => tree.root.findAllByType(MockButton).find(x => x.props.title === title)!.props.onPress());
  press('Open');
  press('Back');
  expect(StyleSheet.flatten(tree.root.findByProps({testID: 'workspace-list'}).props.style).display).not.toBe('none');
  mockWide = true;
  act(() => tree.update(<App />));
  mockWide = false;
  act(() => tree.update(<App />));
  expect(StyleSheet.flatten(tree.root.findByProps({testID: 'workspace-list'}).props.style).display).not.toBe('none');
  expect(StyleSheet.flatten(tree.root.findByProps({testID: 'workspace-reader'}).props.style).display).toBe('none');
  act(() => tree.unmount());
});

test('selecting a session while expanded returns to its reader when folded', () => {
  mockWide = false;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<App />); });
  const press = (title: string) => act(() => tree.root.findAllByType(MockButton).find(x => x.props.title === title)!.props.onPress());
  press('Open');
  press('Back');
  mockWide = true;
  act(() => tree.update(<App />));
  press('Open');
  mockWide = false;
  act(() => tree.update(<App />));
  expect(StyleSheet.flatten(tree.root.findByProps({testID: 'workspace-reader'}).props.style).display).not.toBe('none');
  expect(StyleSheet.flatten(tree.root.findByProps({testID: 'workspace-list'}).props.style).display).toBe('none');
  act(() => tree.unmount());
});


test('a hidden workspace does not consume Android back on another stack page', () => {
  mockWide = false;
  let listener: (() => boolean) | undefined;
  const subscription = jest.spyOn(BackHandler, 'addEventListener').mockImplementation((_event, callback) => {
    listener = callback as () => boolean;
    return {remove: jest.fn()};
  });
  const navigation = {push: jest.fn(), isFocused: () => false};
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<NavigationContext.Provider value={navigation as any}><App /></NavigationContext.Provider>); });
  act(() => tree.root.findAllByType(MockButton).find(x => x.props.title === 'Open')!.props.onPress());
  expect(listener?.()).toBe(false);
  act(() => tree.unmount());
  subscription.mockRestore();
});
