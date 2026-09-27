import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';
import {paletteFor} from '../ui/theme';
import {AgentAvatar} from './AgentAvatar';

// A seam says "a new session began HERE, between these two turns". At the top of the
// view the session-start line already says when this conversation began, and drawing
// the seam under it said the same thing twice (2026-09-16).
const agent = {pane_id: '%6', agent: 'Claude Code', status: 'idle'} as Agent;
const mounted: renderer.ReactTestRenderer[] = [];
afterEach(() => {
  for (const tree of mounted.splice(0)) act(() => tree.unmount());
});
const brk = {kind: 'clear', at: 1789528668} as TranscriptTurn['session_break'];
const turn = (prompt: string, response: string, session_break?: TranscriptTurn['session_break']): TranscriptTurn =>
  ({prompt, response, time: '2026-09-16T03:25:00Z', session_break} as TranscriptTurn);

function mount(turns: TranscriptTurn[], current = agent) {
  let t!: renderer.ReactTestRenderer;
  act(() => {
    t = renderer.create(
      <ChatView agent={current} lines={[]} status="idle" fontSize={13} pal={paletteFor('dark')} lang="en" turns={turns} loading={false} sessionReset={{kind: 'clear', at: 1789528668}} />,
    );
  });
  mounted.push(t);
  return t;
}
const seams = (t: renderer.ReactTestRenderer) => t.root.findAllByProps({testID: 'chat-session-seam'}).length;

test('no seam under the session-start line when the break is on the first shown turn', () => {
  expect(seams(mount([turn('', 'brief', brk), turn('hi', 'reply')]))).toBe(0);
});

test('a seam between two shown turns when the later one began a session', () => {
  expect(seams(mount([turn('before', 'old reply'), turn('', 'brief', brk)]))).toBeGreaterThan(0);
});

test('HQ history keeps Claude and Codex replies under their own agents', () => {
  const view = mount([
    {...turn('old', 'Claude reply'), agent: 'claude'},
    {...turn('new', 'Codex reply', {kind: '', at: 1789528668}), agent: 'codex'},
  ]);
  const avatars = view.root.findAllByType(AgentAvatar).map(a => a.props.agent.agent);
  expect(avatars).toContain('Claude Code');
  expect(avatars).toContain('Codex');
  expect(JSON.stringify(view.toJSON())).toContain('switched to Codex');
});

test('HQ says why a wake-only session has replies without manual messages', () => {
  const hq = {...agent, role: 'supervisor'};
  const view = mount([{...turn('', 'automatic reply'), agent: 'codex'}], hq);
  expect(JSON.stringify(view.toJSON())).toContain('automatic wake-ups, with no manual messages');
});
