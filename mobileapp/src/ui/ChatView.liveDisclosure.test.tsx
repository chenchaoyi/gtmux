import React from 'react';
import {ScrollView, StyleSheet, Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent, StatusName} from '../api/types';
import {GtmuxClient, TranscriptTurn} from '../api/client';
import {TestIds} from '../constants/testIds';
import {AnsiLine} from './ansi';
import {statusLabel} from '../i18n';

jest.mock('../api/clock', () => ({serverNowSec: () => 2000}));
const agent = {pane_id: '%1', session_id: 'session-one', agent: 'Codex', status: 'working'} as Agent;
const output: AnsiLine[] = [[{text: 'synthetic terminal output', color: '#fff', bold: true}]];
const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => {for (const tree of mounted.splice(0)) act(() => tree.unmount());});

function view({lines = output, lang = 'en', current = agent, since = 1958, status = 'working', turns = [], base = 'https://server.example', appearance = 'dark'}: {
  lines?: AnsiLine[]; lang?: 'en' | 'zh'; current?: Agent; since?: number;
  status?: StatusName; turns?: TranscriptTurn[]; base?: string; appearance?: 'light' | 'dark';
} = {}) {
  return <ChatView client={new GtmuxClient(base, 'synthetic-token')} agent={current} lines={lines}
    status={status} fontSize={13} pal={paletteFor(appearance)} lang={lang} turns={turns} loading={false} workingSince={since} />;
}
function mount() {
  let tree!: renderer.ReactTestRenderer;
  act(() => {tree = renderer.create(view());});
  mounted.push(tree);
  return tree;
}
const toggle = (tree: renderer.ReactTestRenderer) => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.chatLiveToggle)!;
const preview = (tree: renderer.ReactTestRenderer) => tree.root.findAll(n => n.type === ScrollView && n.props.nestedScrollEnabled);
const text = (tree: renderer.ReactTestRenderer) => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' | ');

test.each(['light', 'dark'] as const)('MCE-002 %s Live has one outlined disclosure and an emphasized action', appearance => {
  const tree = mount();
  act(() => {tree.update(view({appearance}));});
  const control = toggle(tree);
  const style = StyleSheet.flatten(control.props.style);
  expect(style.minHeight).toBeGreaterThanOrEqual(48);
  expect(style.borderWidth).toBeGreaterThan(0);
  expect(style.backgroundColor).toBe(paletteFor(appearance).surface);
  expect(tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID === TestIds.detail.chatLiveToggle)).toHaveLength(1);
  const action = control.findAllByType(Text).find(node => node.props.children === 'Show live view');
  expect(StyleSheet.flatten(action?.props.style).fontWeight).toBe('600');
  act(() => control.props.onPress());
  expect(toggle(tree).props.accessibilityState).toEqual({expanded: true});
  expect(text(tree)).toContain('synthetic terminal output');
  act(() => toggle(tree).props.onPress());
  expect(toggle(tree).props.accessibilityState).toEqual({expanded: false});
});

test.each(['en', 'zh'] as const)('%s: the default is a status row; the same disclosure opens and closes the bounded screen', lang => {
  const tree = mount();
  act(() => {tree.update(view({lang}));});
  expect(toggle(tree).props.accessibilityState).toEqual({expanded: false});
  expect(toggle(tree).props.accessibilityLabel).toContain(lang === 'zh' ? '展开实时画面' : 'Show live view');
  expect(text(tree)).not.toContain('synthetic terminal output');
  expect(preview(tree)).toHaveLength(0); // expensive/raw subtree is not hidden offscreen
  act(() => toggle(tree).props.onPress());
  expect(toggle(tree).props.accessibilityState).toEqual({expanded: true});
  expect(toggle(tree).props.accessibilityLabel).toContain(lang === 'zh' ? '收起实时画面' : 'Hide live view');
  expect(text(tree)).toContain('synthetic terminal output');
  expect(preview(tree)).toHaveLength(2);
  expect(preview(tree)[0].props.style.maxHeight).toBe(220);
  expect(preview(tree)[1].props.horizontal).toBe(true);
  act(() => toggle(tree).props.onPress());
  expect(preview(tree)).toHaveLength(0);
});

test('capture and transcript refresh retain expansion, including first output and temporary empty captures', () => {
  const tree = mount();
  act(() => {tree.update(view({lines: []}));});
  expect(text(tree)).toContain('Thinking… 42s');
  act(() => toggle(tree).props.onPress());
  expect(text(tree)).toContain('No terminal output yet');
  act(() => {tree.update(view({turns: [{prompt: 'hello', response: 'progress', time: '2026-01-01T00:00:00Z'}]}));});
  expect(toggle(tree).props.accessibilityState.expanded).toBe(true);
  expect(text(tree)).toContain('Working · 42s');
  expect(text(tree)).toContain('synthetic terminal output');
  act(() => {tree.update(view({lines: []}));});
  expect(toggle(tree).props.accessibilityState.expanded).toBe(true);
});

test.each([
  {current: {...agent, pane_id: '%2'}},
  {current: {...agent, session_id: 'session-two'}},
  {current: {...agent, home_id: 'another-home'}},
  {base: 'https://another-server.example'},
])('a new working scope starts folded: %j', next => {
  const tree = mount();
  act(() => toggle(tree).props.onPress());
  act(() => {tree.update(view(next));});
  expect(toggle(tree).props.accessibilityState.expanded).toBe(false);
});

test.each(['idle', 'waiting'] as const)('%s keeps an open Live preview through a transcript redraw', status => {
  const tree = mount();
  act(() => toggle(tree).props.onPress());
  const contents = preview(tree)[0];
  act(() => {tree.update(view({status, turns: [{prompt: 'New turn', response: 'Recorded reply'}]}));});
  expect(preview(tree)[0]).toBe(contents);
  expect(toggle(tree).props.accessibilityState.expanded).toBe(true);
  expect(toggle(tree).props.accessibilityLabel).toContain(statusLabel(status, 'en'));
  act(() => toggle(tree).props.onPress());
  expect(toggle(tree)).toBeUndefined();
  act(() => {tree.update(view());});
  expect(toggle(tree).props.accessibilityState.expanded).toBe(false);
});


test('a native conversation without terminal access keeps the status without an empty disclosure', () => {
  const tree = mount();
  act(() => {tree.update(view({current: {...agent, source: 'native'}, lines: []}));});
  expect(text(tree)).toContain('Thinking… 42s');
  expect(toggle(tree)).toBeUndefined();
  expect(preview(tree)).toHaveLength(0);
});

test('radar timestamp corrections do not collapse the live view, and explicit folding survives updates', () => {
  const tree = mount();
  act(() => toggle(tree).props.onPress());
  act(() => tree.update(view({since: 1999})));
  expect(toggle(tree).props.accessibilityState.expanded).toBe(true);
  expect(text(tree)).toContain('synthetic terminal output');
  act(() => toggle(tree).props.onPress());
  act(() => tree.update(view({since: 0})));
  expect(toggle(tree).props.accessibilityState.expanded).toBe(false);
});
