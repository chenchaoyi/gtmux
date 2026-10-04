import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {ChatView} from './ChatView';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';
import {paletteFor} from '../ui/theme';

// A long prompt is collapsed to 8 lines behind "Show all", so a pasted dump does not bury
// the conversation. A message HQ sent is something the reader is there to read: on
// 2026-10-04 a 9-line HQ relay hid the ask in its last line behind "…".
const agent = {pane_id: '%6', agent: 'Claude Code', status: 'idle'} as Agent;
const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => {
  for (const tree of mounted.splice(0)) act(() => tree.unmount());
});
const long = Array.from({length: 9}, (_, i) => `line ${i + 1}: ` + 'x'.repeat(70)).join('\n');

function mount(turn: Partial<TranscriptTurn>) {
  let t!: renderer.ReactTestRenderer;
  act(() => {
    t = renderer.create(
      <ChatView agent={agent} lines={[]} status="idle" fontSize={13} pal={paletteFor('dark')} lang="en"
        turns={[{prompt: long, response: 'ok', time: '2026-10-04T09:21:00Z', ...turn} as TranscriptTurn]} loading={false} />,
    );
  });
  mounted.push(t);
  return t;
}
const promptText = (t: renderer.ReactTestRenderer) =>
  t.root.findAllByType(Text).find(n => n.props.children === long)!;
const showAll = (t: renderer.ReactTestRenderer) => JSON.stringify(t.toJSON()).includes('Show all');

test('a long prompt from HQ is shown whole, with no Show all toggle', () => {
  const t = mount({from: {kind: 'hq', label: 'HQ'}} as Partial<TranscriptTurn>);
  expect(promptText(t).props.numberOfLines).toBeUndefined();
  expect(showAll(t)).toBe(false);
});

test('the same prompt typed by the reader is still collapsed', () => {
  const t = mount({});
  expect(promptText(t).props.numberOfLines).toBe(8);
  expect(showAll(t)).toBe(true);
});

test('the same prompt dispatched by another session is still collapsed', () => {
  const t = mount({from: {kind: 'agent', label: 'Codex', agent: 'Codex'}} as Partial<TranscriptTurn>);
  expect(promptText(t).props.numberOfLines).toBe(8);
});
