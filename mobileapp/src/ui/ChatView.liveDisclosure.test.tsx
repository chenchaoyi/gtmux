import React from 'react';
import {ScrollView, Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent, StatusName} from '../api/types';
import {GtmuxClient, TranscriptTurn} from '../api/client';
import {TestIds} from '../constants/testIds';
import {AnsiLine} from './ansi';

jest.mock('../api/clock', () => ({serverNowSec: () => 2000}));
const agent = {pane_id: '%1', session_id: 'session-one', agent: 'Codex', status: 'working'} as Agent;
const output: AnsiLine[] = [[{text: 'synthetic terminal output', color: '#fff', bold: true}]];
const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => {for (const tree of mounted.splice(0)) act(() => tree.unmount());});

function view({lines = output, lang = 'en', current = agent, since = 1958, status = 'working', turns = [], base = 'https://server.example'}: {
  lines?: AnsiLine[]; lang?: 'en' | 'zh'; current?: Agent; since?: number;
  status?: StatusName; turns?: TranscriptTurn[]; base?: string;
} = {}) {
  return <ChatView client={new GtmuxClient(base, 'synthetic-token')} agent={current} lines={lines}
    status={status} fontSize={13} pal={paletteFor('dark')} lang={lang} turns={turns} loading={false} workingSince={since} />;
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
  {since: 1999},
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

test.each(['idle', 'waiting'] as const)('%s unmounts only the working preview and the next turn starts collapsed', status => {
  const tree = mount();
  act(() => toggle(tree).props.onPress());
  act(() => {tree.update(view({status}));});
  expect(tree.root.findAll(n => n.props.testID === TestIds.detail.chatLiveToggle)).toHaveLength(0);
  expect(preview(tree)).toHaveLength(0);
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
