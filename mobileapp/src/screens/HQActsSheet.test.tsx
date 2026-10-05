import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import {HQActsSheet} from './HQActsSheet';
import {Act} from './hqActsModel';
import {paletteFor} from '../ui/theme';

// The header's "HQ did" row opens this list (2026-10-05). It used to switch to the Console,
// which was usually open already, and which shows no acts at all when HQ's console is a
// terminal: the commander tapped the row and nothing happened.

const now = 1_800_000_000;
const acts: Act[] = [
  {ts: now - 60, kind: 'gtmux:audit:send', verb: 'dispatched', target: '%6', detail: 'audit 1.0.92', link: {kind: 'pane', id: '%6'}},
  {ts: now - 600, kind: 'knowledge:record', verb: 'recorded', target: '', detail: 'release notes say only what was verified', link: {kind: 'entry', id: 'pitfalls/release-notes'}},
  {ts: now - 900, kind: 'self-check', verb: 'self-audited', target: '', detail: 'board current'},
];

function render(list: Act[]) {
  const onOpenPane = jest.fn();
  const onOpenEntry = jest.fn();
  const onClose = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(
      <HQActsSheet visible acts={list} now={now} pal={paletteFor('dark')} zh={false}
        onClose={onClose} onOpenPane={onOpenPane} onOpenEntry={onOpenEntry} />,
    );
  });
  const texts = () => tree.root.findAllByType(Text).map(n => [n.props.children].flat().join('')).join(' | ');
  const rows = () => tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID === 'hq-act');
  return {tree, texts, rows, onOpenPane, onOpenEntry, onClose};
}

test('lists what HQ did: what the list is for, the 24-hour tally, and every act', () => {
  const {texts, rows} = render(acts);
  expect(texts()).toContain('What HQ did');
  expect(texts()).toMatch(/listed so you can check it/);
  expect(texts()).toContain('dispatched 1  ·  recorded 1  ·  self-audited 1');
  expect(rows()).toHaveLength(3);
});

test('an act leads where it acted: the session, or the knowledge entry it wrote', () => {
  const {rows, onOpenPane, onOpenEntry} = render(acts);
  act(() => rows()[0].props.onPress());
  expect(onOpenPane).toHaveBeenCalledWith('%6');
  act(() => rows()[1].props.onPress());
  expect(onOpenEntry).toHaveBeenCalledWith('pitfalls/release-notes');
  // An act with nowhere to go is a row that does not pretend to be a button.
  expect(rows()[2].props.disabled).toBe(true);
});

test('nothing recent says so, and the sheet closes', () => {
  const {texts, tree, onClose} = render([]);
  expect(texts()).toContain('Your supervisor has not acted recently.');
  act(() => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === 'hq-acts-sheet-close')!.props.onPress());
  expect(onClose).toHaveBeenCalled();
});
