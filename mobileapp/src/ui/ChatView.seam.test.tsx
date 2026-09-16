import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {Agent, TranscriptTurn} from '../api/types';
import {paletteFor} from '../ui/theme';

// A seam says "a new session began HERE, between these two turns". At the top of the
// view the session-start line already says when this conversation began, and drawing
// the seam under it said the same thing twice (2026-09-16).
const agent = {pane_id: '%6', agent: 'Claude Code', status: 'idle'} as Agent;
const brk = {kind: 'clear', at: 1789528668} as TranscriptTurn['session_break'];
const turn = (prompt: string, response: string, session_break?: TranscriptTurn['session_break']): TranscriptTurn =>
  ({prompt, response, time: '2026-09-16T03:25:00Z', session_break} as TranscriptTurn);

function mount(turns: TranscriptTurn[]) {
  let t!: renderer.ReactTestRenderer;
  act(() => {
    t = renderer.create(
      <ChatView agent={agent} lines={[]} status="idle" fontSize={13} pal={paletteFor('dark')} lang="en" turns={turns} sessionReset={{at: 1789528668, cmd: '/clear'}} />,
    );
  });
  return t;
}
const seams = (t: renderer.ReactTestRenderer) => t.root.findAllByProps({testID: 'chat-session-seam'}).length;

test('no seam under the session-start line when the break is on the first shown turn', () => {
  expect(seams(mount([turn('', 'brief', brk), turn('hi', 'reply')]))).toBe(0);
});

test('a seam between two shown turns when the later one began a session', () => {
  expect(seams(mount([turn('before', 'old reply'), turn('', 'brief', brk)]))).toBeGreaterThan(0);
});
